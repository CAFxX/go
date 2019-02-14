// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Pool is no-op under race detector, so all these tests do not work.
// +build !race

package sync_test

import (
	"bytes"
	"math/rand"
	"runtime"
	"runtime/debug"
	"strconv"
	. "sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestPool(t *testing.T) {
	// disable GC so we can control when it happens.
	defer debug.SetGCPercent(debug.SetGCPercent(-1))
	var p Pool
	if p.Get() != nil {
		t.Fatal("expected empty")
	}

	// Make sure that the goroutine doesn't migrate to another P
	// between Put and Get calls.
	Runtime_procPin()
	p.Put("a")
	p.Put("b")
	if g := p.Get(); g != "a" {
		t.Fatalf("got %#v; want a", g)
	}
	if g := p.Get(); g != "b" {
		t.Fatalf("got %#v; want b", g)
	}
	if g := p.Get(); g != nil {
		t.Fatalf("got %#v; want nil", g)
	}
	Runtime_procUnpin()

	p.Put("c")
	debug.SetGCPercent(100) // to allow following GC to actually run
	runtime.GC()
	runtime.GC() // we now keep some objects until two consecutive GCs
	if g := p.Get(); g != nil {
		t.Fatalf("got %#v; want nil after GC", g)
	}
}

func TestPoolNew(t *testing.T) {
	// disable GC so we can control when it happens.
	defer debug.SetGCPercent(debug.SetGCPercent(-1))

	i := 0
	p := Pool{
		New: func() interface{} {
			i++
			return i
		},
	}
	if v := p.Get(); v != 1 {
		t.Fatalf("got %v; want 1", v)
	}
	if v := p.Get(); v != 2 {
		t.Fatalf("got %v; want 2", v)
	}

	// Make sure that the goroutine doesn't migrate to another P
	// between Put and Get calls.
	Runtime_procPin()
	p.Put(42)
	if v := p.Get(); v != 42 {
		t.Fatalf("got %v; want 42", v)
	}
	Runtime_procUnpin()

	if v := p.Get(); v != 3 {
		t.Fatalf("got %v; want 3", v)
	}
}

// Test that Pool does not hold pointers to previously cached resources.
func TestPoolGC(t *testing.T) {
	testPool(t, true)
}

// Test that Pool releases resources on GC.
func TestPoolRelease(t *testing.T) {
	testPool(t, false)
}

func testPool(t *testing.T, drain bool) {
	var p Pool
	const N = 100
loop:
	for try := 0; try < 3; try++ {
		var fin, fin1 uint32
		for i := 0; i < N; i++ {
			v := new(string)
			runtime.SetFinalizer(v, func(vv *string) {
				atomic.AddUint32(&fin, 1)
			})
			p.Put(v)
		}
		if drain {
			for i := 0; i < N; i++ {
				p.Get()
			}
		}
		for i := 0; i < 5; i++ {
			runtime.GC()
			time.Sleep(time.Duration(i*100+10) * time.Millisecond)
			// 1 pointer can remain on stack or elsewhere
			if fin1 = atomic.LoadUint32(&fin); fin1 >= N-1 {
				continue loop
			}
		}
		t.Fatalf("only %v out of %v resources are finalized on try %v", fin1, N, try)
	}
}

func TestPoolPartialRelease(t *testing.T) {
	if runtime.GOMAXPROCS(-1) <= 1 {
		t.Skip("pool partial release test is only stable when GOMAXPROCS > 1")
	}

	// disable GC so we can control when it happens.
	defer debug.SetGCPercent(debug.SetGCPercent(-1))

	Ps := runtime.GOMAXPROCS(-1)
	Gs := Ps * 10
	Gobjs := 10000

	var p Pool
	var wg WaitGroup
	start := int32(0)
	for i := 0; i < Gs; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			atomic.AddInt32(&start, 1)
			for atomic.LoadInt32(&start) < int32(Ps) {
				// spin until enough Gs are ready to go
			}
			for j := 0; j < Gobjs; j++ {
				p.Put(new (string))
			}
		}()
	}
	wg.Wait()

	waitGC()

	inpool := 0
	for p.Get() != nil {
		inpool++
	}
	objs := Gs * Gobjs
	min, max := objs/2 - objs/Ps, objs/2 + objs/Ps
	if inpool < min || inpool > max {
		// GC should have dropped half of the per-P shards; because we don't know the
		// exact distribution of the objects in the shards when we started, and we don't
		// know which shards have been cleared, we consider the test successful as long
		// as after GC the number of remaining objects is half ± objs/Ps.
		t.Fatalf("objects in pool %d, expected between %d and %d", inpool, min, max)
	}
}

func waitGC() {
	ch := make(chan struct{})
	runtime.SetFinalizer(&[16]byte{}, func(_ interface{}) {
		close(ch)
	})
	runtime.GC()
	<-ch
}

func TestPoolStress(t *testing.T) {
	const P = 10
	N := int(1e6)
	if testing.Short() {
		N /= 100
	}
	var p Pool
	done := make(chan bool)
	for i := 0; i < P; i++ {
		go func() {
			var v interface{} = 0
			for j := 0; j < N; j++ {
				if v == nil {
					v = 0
				}
				p.Put(v)
				v = p.Get()
				if v != nil && v.(int) != 0 {
					t.Errorf("expect 0, got %v", v)
					break
				}
			}
			done <- true
		}()
	}
	for i := 0; i < P; i++ {
		<-done
	}
}

func BenchmarkPool(b *testing.B) {
	var p Pool
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			p.Put(1)
			p.Get()
		}
	})
}

func BenchmarkPoolOverflow(b *testing.B) {
	var p Pool
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			for b := 0; b < 100; b++ {
				p.Put(1)
			}
			for b := 0; b < 100; b++ {
				p.Get()
			}
		}
	})
}

var bufSizes = []int{1 << 8, 1 << 12, 1 << 16, 1 << 20, 1 << 24}

func BenchmarkPoolBuffer(b *testing.B) {
	for _, sz := range bufSizes {
		sz := sz
		b.Run(strconv.Itoa(sz), func(b *testing.B) {
			var p Pool
			var i int64
			b.RunParallel(func(pb *testing.PB) {
				rnd := rand.New(rand.NewSource(atomic.AddInt64(&i, 1)))
				var j int
				for pb.Next() {
					buf, _ := p.Get().(*bytes.Buffer)
					if buf == nil {
						buf = &bytes.Buffer{}
					}
					buf.Grow(rnd.Intn(sz * 2))

					go p.Put(buf)
					j++
					if j%256 == 0 {
						runtime.Gosched()
					}
				}
			})
		})
	}
}

func BenchmarkNoPoolBuffer(b *testing.B) {
	for _, sz := range bufSizes {
		sz := sz
		b.Run(strconv.Itoa(sz), func(b *testing.B) {
			var i int64
			b.RunParallel(func(pb *testing.PB) {
				rnd := rand.New(rand.NewSource(atomic.AddInt64(&i, 1)))
				var j int
				for pb.Next() {
					buf := &bytes.Buffer{}
					buf.Grow(rnd.Intn(sz * 2))

					go runtime.KeepAlive(buf)
					j++
					if j%256 == 0 {
						runtime.Gosched()
					}
				}
			})
		})
	}
}
