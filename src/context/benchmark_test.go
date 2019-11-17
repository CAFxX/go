// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package context_test

import (
	. "context"
	"fmt"
	"runtime"
	"sync"
	"testing"
	"time"
)

func BenchmarkCommonParentCancel(b *testing.B) {
	root := WithValue(Background(), "key", "value")
	shared, sharedcancel := WithCancel(root)
	defer sharedcancel()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		x := 0
		for pb.Next() {
			ctx, cancel := WithCancel(shared)
			if ctx.Value("key").(string) != "value" {
				b.Fatal("should not be reached")
			}
			for i := 0; i < 100; i++ {
				x /= x + 1
			}
			cancel()
			for i := 0; i < 100; i++ {
				x /= x + 1
			}
		}
	})
}

func BenchmarkWithTimeout(b *testing.B) {
	for concurrency := 40; concurrency <= 4e5; concurrency *= 100 {
		name := fmt.Sprintf("concurrency=%d", concurrency)
		b.Run(name, func(b *testing.B) {
			benchmarkWithTimeout(b, concurrency)
		})
	}
}

func benchmarkWithTimeout(b *testing.B, concurrentContexts int) {
	gomaxprocs := runtime.GOMAXPROCS(0)
	perPContexts := concurrentContexts / gomaxprocs
	root := Background()

	// Generate concurrent contexts.
	var wg sync.WaitGroup
	ccf := make([][]CancelFunc, gomaxprocs)
	for i := range ccf {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			cf := make([]CancelFunc, perPContexts)
			for j := range cf {
				_, cf[j] = WithTimeout(root, time.Hour)
			}
			ccf[i] = cf
		}(i)
	}
	wg.Wait()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		wcf := make([]CancelFunc, 10)
		for pb.Next() {
			for i := range wcf {
				_, wcf[i] = WithTimeout(root, time.Hour)
			}
			for _, f := range wcf {
				f()
			}
		}
	})
	b.StopTimer()

	for _, cf := range ccf {
		for _, f := range cf {
			f()
		}
	}
}

func BenchmarkCancelTree(b *testing.B) {
	depths := []int{1, 10, 100, 1000}
	for _, d := range depths {
		b.Run(fmt.Sprintf("depth=%d", d), func(b *testing.B) {
			b.Run("Root=Background", func(b *testing.B) {
				for i := 0; i < b.N; i++ {
					buildContextTree(Background(), d)
				}
			})
			b.Run("Root=OpenCanceler", func(b *testing.B) {
				for i := 0; i < b.N; i++ {
					ctx, cancel := WithCancel(Background())
					buildContextTree(ctx, d)
					cancel()
				}
			})
			b.Run("Root=ClosedCanceler", func(b *testing.B) {
				for i := 0; i < b.N; i++ {
					ctx, cancel := WithCancel(Background())
					cancel()
					buildContextTree(ctx, d)
				}
			})
		})
	}
}

func buildContextTree(root Context, depth int) {
	for d := 0; d < depth; d++ {
		root, _ = WithCancel(root)
	}
}

func BenchmarkCheckCanceled(b *testing.B) {
	ctx, cancel := WithCancel(Background())
	cancel()
	b.Run("Err", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			ctx.Err()
		}
	})
	b.Run("Done", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			select {
			case <-ctx.Done():
			default:
			}
		}
	})
}

func BenchmarkValue(b *testing.B) {
	for _, n := range []int{2, 20, 200} {
		ctx := Background()
		for j := 0; j < n; j++ {
			ctx = WithValue(ctx, struct{}{}, struct{}{})
		}
		// measure the time it takes to traverse the whole chain
		b.Run(fmt.Sprintf("Miss/%d", n), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_ = ctx.Value(b)
			}
		})
		// measure the time it takes to visit only the first element in the chain
		b.Run(fmt.Sprintf("Hit/%d", n), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_ = ctx.Value(struct{}{})
			}
		})
	}
}

type otherContext struct {
	Context
}

func BenchmarkValueUnknownContext(b *testing.B) {
	for _, n := range []int{2, 20, 200} {
		ctx := Background()
		// construct a worst-case chain, in which we have alternated
		// known and unknown contexts
		for j := 0; j < n; j += 2 {
			ctx = otherContext{ctx}
			ctx = WithValue(ctx, struct{}{}, struct{}{})
		}
		// measure the time it takes to traverse the whole chain
		b.Run(fmt.Sprintf("Miss/%d", n), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_ = ctx.Value(b)
			}
		})
	}
}
