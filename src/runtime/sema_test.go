package runtime_test

import (
	. "runtime"
	"sync/atomic"
	"testing"
)

// TestSemaHandoff checks that when semrelease+handoff is
// requested, the G that releases the semaphore yields its
// P directly to the first waiter in line.
// See https://github.com/golang/go/issues/33747 for discussion.
func TestSemaHandoff(t *testing.T) {
	const iter = 10000
	fail := 0
	for i := 0; i < iter; i++ {
		res := testSemaHandoff()
		if res != 1 {
			fail++
		}
	}
	// As long as >= 95% of handoffs are direct, we
	// consider the test successful.
	if fail > iter*5/100 {
		t.Fatal("direct handoff failing:", fail)
	}
}

func testSemaHandoff() (res uint32) {
	sema := uint32(1)

	Semacquire(&sema)
	go func() {
		Semacquire(&sema)
		atomic.CompareAndSwapUint32(&res, 0, 1)
	}()
	for SemNwait(&sema) == 0 {
		Gosched()
	}
	Semrelease1(&sema, true, 0)
	atomic.CompareAndSwapUint32(&res, 0, 2)

	return
}
