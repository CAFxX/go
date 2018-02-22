// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sync

import (
	"internal/race"
	"runtime"
	"sync/atomic"
	"unsafe"
)

// A Pool is a set of temporary objects that may be individually saved and
// retrieved.
//
// Any item stored in the Pool may be removed automatically at any time without
// notification. If the Pool holds the only reference when this happens, the
// item might be deallocated.
//
// A Pool is safe for use by multiple goroutines simultaneously.
//
// Pool's purpose is to cache allocated but unused items for later reuse,
// relieving pressure on the garbage collector. That is, it makes it easy to
// build efficient, thread-safe free lists. However, it is not suitable for all
// free lists.
//
// An appropriate use of a Pool is to manage a group of temporary items
// silently shared among and potentially reused by concurrent independent
// clients of a package. Pool provides a way to amortize allocation overhead
// across many clients.
//
// An example of good use of a Pool is in the fmt package, which maintains a
// dynamically-sized store of temporary output buffers. The store scales under
// load (when many goroutines are actively printing) and shrinks when
// quiescent.
//
// On the other hand, a free list maintained as part of a short-lived object is
// not a suitable use for a Pool, since the overhead does not amortize well in
// that scenario. It is more efficient to have such objects implement their own
// free list.
//
// A Pool must not be copied after first use.
type Pool struct {
	noCopy noCopy

	local     unsafe.Pointer // local fixed-size per-P pool, actual type is [P]poolLocal
	localSize uintptr        // size of the local array

	globalLock uintptr
	global     []interface{}

	// New optionally specifies a function to generate
	// a value when Get would otherwise return nil.
	// It may not be changed concurrently with calls to Get.
	New func() interface{}
}

const (
	globalLocked   uintptr = 1
	globalUnlocked uintptr = 0

	// privateSoftLimit is the threshold for starting to spill elements from the
	// per-P private pools to the global pool
	// TODO: privateSoftLimit should probably be proportional to the number of
	//       Ps to avoid globalLock contention with high P counts
	privateSoftLimit int = 250
)

// Local per-P Pool appendix.
type poolLocalInternal struct {
	private []interface{} // Can be used only by the respective P.
}

type poolLocal struct {
	poolLocalInternal

	// Prevents false sharing on widespread platforms with
	// 128 mod (cache line size) = 0 .
	pad [128 - unsafe.Sizeof(poolLocalInternal{})%128]byte
}

// from runtime
func fastrand() uint32

var poolRaceHash [128]uint64

// poolRaceAddr returns an address to use as the synchronization point
// for race detector logic. We don't use the actual pointer stored in x
// directly, for fear of conflicting with other synchronization on that address.
// Instead, we hash the pointer to get an index into poolRaceHash.
// See discussion on golang.org/cl/31589.
func poolRaceAddr(x interface{}) unsafe.Pointer {
	ptr := uintptr((*[2]unsafe.Pointer)(unsafe.Pointer(&x))[1])
	h := uint32((uint64(uint32(ptr)) * 0x85ebca6b) >> 16)
	return unsafe.Pointer(&poolRaceHash[h%uint32(len(poolRaceHash))])
}

// Put adds x to the pool.
func (p *Pool) Put(x interface{}) {
	if x == nil {
		return
	}

	if race.Enabled {
		if fastrand()%4 == 0 {
			// Randomly drop x on floor.
			return
		}
		race.ReleaseMerge(poolRaceAddr(x))
		race.Disable()
	}

	var l *poolLocal
	pid := runtime_procPin()
	if uintptr(pid) < atomic.LoadUintptr(&p.localSize) {
		l = indexLocal(p.local, pid)
	} else {
		l = p.pinSlow()
	}

	l.private = append(l.private, x)
	if len(l.private) > privateSoftLimit {
		if atomic.CompareAndSwapUintptr(&p.globalLock, globalUnlocked, globalLocked) {
			p.global = append(p.global, l.private[privateSoftLimit/2:]...)
			l.private = l.private[:privateSoftLimit/2]
			atomic.StoreUintptr(&p.globalLock, globalUnlocked)
			// we need to be pinned at least until the atomic store
		}
	}

	runtime_procUnpin()

	if race.Enabled {
		race.Enable()
	}
}

// Get selects an arbitrary item from the Pool, removes it from the
// Pool, and returns it to the caller.
// Get may choose to ignore the pool and treat it as empty.
// Callers should not assume any relation between values passed to Put and
// the values returned by Get.
//
// If Get would otherwise return nil and p.New is non-nil, Get returns
// the result of calling p.New.
func (p *Pool) Get() (x interface{}) {
	if race.Enabled {
		race.Disable()
	}

	var l *poolLocal
	pid := runtime_procPin()
	if uintptr(pid) < atomic.LoadUintptr(&p.localSize) {
		l = indexLocal(p.local, pid)
	} else {
		l = p.pinSlow()
	}

	last := len(l.private) - 1
	if last >= 0 {
		x = l.private[last]
		l.private = l.private[:last]
	} else {
		if atomic.CompareAndSwapUintptr(&p.globalLock, globalUnlocked, globalLocked) {
			if len(p.global) > 0 {
				last := 0
				if len(p.global) > privateSoftLimit/2 {
					last = len(p.global) - privateSoftLimit/2
				}
				l.private = append(l.private, p.global[last+1:]...)
				x = p.global[last]
				p.global = p.global[:last]
			}
			atomic.StoreUintptr(&p.globalLock, globalUnlocked)
			// we need to be pinned at least until the atomic store
		}
	}

	runtime_procUnpin()

	if race.Enabled {
		race.Enable()
		if x != nil {
			race.Acquire(poolRaceAddr(x))
		}
	}

	if x == nil && p.New != nil {
		x = p.New()
	}
	return
}

func (p *Pool) pinSlow() *poolLocal {
	// Retry under the mutex.
	// Can not lock the mutex while pinned.
	runtime_procUnpin()
	allPoolsMu.Lock()
	defer allPoolsMu.Unlock()
	pid := runtime_procPin()
	// poolCleanup won't be called while we are pinned.
	s := p.localSize
	l := p.local
	if uintptr(pid) < s {
		return indexLocal(l, pid)
	}
	if p.local == nil {
		allPools = append(allPools, p)
	}
	// If GOMAXPROCS changes between GCs, we re-allocate the array and lose the old one.
	size := runtime.GOMAXPROCS(0)
	local := make([]poolLocal, size)
	atomic.StorePointer(&p.local, unsafe.Pointer(&local[0])) // store-release
	atomic.StoreUintptr(&p.localSize, uintptr(size))         // store-release
	return &local[pid]
}

func poolCleanup() {
	// This function is called with the world stopped, at the beginning of a garbage collection.
	// It must not allocate and probably should not call any runtime functions.
	// Defensively zero out everything, 2 reasons:
	// 1. To prevent false retention of whole Pools.
	// 2. If GC happens while a goroutine works with l.shared in Put/Get,
	//    it will retain whole Pool. So next cycle memory consumption would be doubled.
	for i, p := range allPools {
		allPools[i] = nil
		for i := 0; i < int(p.localSize); i++ {
			l := indexLocal(p.local, i)
			for j := range l.private {
				l.private[j] = nil
			}
			l.private = nil
		}
		for j := range p.global {
			p.global[j] = nil
		}
		p.global = nil
		p.local = nil
		p.localSize = 0
	}
	allPools = []*Pool{}
}

var (
	allPoolsMu Mutex
	allPools   []*Pool
)

func init() {
	runtime_registerPoolCleanup(poolCleanup)
}

func indexLocal(l unsafe.Pointer, i int) *poolLocal {
	lp := unsafe.Pointer(uintptr(l) + uintptr(i)*unsafe.Sizeof(poolLocal{}))
	return (*poolLocal)(lp)
}

// Implemented in runtime.
func runtime_registerPoolCleanup(cleanup func())
func runtime_procPin() int
func runtime_procUnpin()
