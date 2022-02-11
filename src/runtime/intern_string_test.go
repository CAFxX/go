package runtime_test

import (
	"fmt"
	"math/rand"
	"reflect"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"unsafe"
)

func sameString(s1, s2 string) bool {
	h1 := (*reflect.StringHeader)(unsafe.Pointer(&s1))
	h2 := (*reflect.StringHeader)(unsafe.Pointer(&s2))
	return h1.Len == h2.Len && h1.Data == h2.Data
}

func TestStringNoIntern(t *testing.T) {
	s1 := fmt.Sprintf("hello %s", "world")
	s2 := fmt.Sprintf("hello %s", "world")
	if sameString(s1, s2) {
		t.Fatal("same string")
	}
}

func TestStringIntern(t *testing.T) {
	//runtime.ProcPin()
	//defer runtime.ProcUnpin()

	s1 := runtime.InternString(fmt.Sprintf("hello %s", "world"))
	s2 := runtime.InternString(fmt.Sprintf("hello %s", "world"))
	if !sameString(s1, s2) {
		t.Fatal("not same string")
	}
}

func TestStringInternStress(t *testing.T) {
	if !runtime.Raceenabled {
		t.Skip("test disabled without -race")
	}

	var wg sync.WaitGroup
	var stop uint32
	for i := 0; i < runtime.GOMAXPROCS(0); i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r := rand.New(rand.NewSource(42))
			for atomic.LoadUint32(&stop) == 0 {
				runtime.InternString(strconv.Itoa(r.Intn(10)))
			}
		}()
	}

	time.Sleep(100 * time.Millisecond)
	atomic.StoreUint32(&stop, 1)
	wg.Wait()
}

func BenchmarkStringIntern(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			runtime.InternString("hello world")
		}
	})
}

func BenchmarkBytesIntern(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		s := []byte("hello world")
		for pb.Next() {
			runtime.InternBytes(s)
		}
	})
}
