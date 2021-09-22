//go:build darwin || linux || freebsd || netbsd || openbsd || aix
// +build darwin linux freebsd netbsd openbsd aix

package runtime

import "unsafe"

//go:linkname decommitRangeNet net.runtime_decommitRange
func decommitRangeNet(p []byte) { decommitRange(p) }

//go:linkname decommitRangeOs os.runtime_decommitRange
func decommitRangeOs(p []byte) { decommitRange(p) }

//go:linkname decommitUnusedStackNet net.runtime_decommitUnusedStack
func decommitUnusedStackNet() { decommitUnusedStack() }

//go:linkname decommitUnusedStackOs os.runtime_decommitUnusedStack
func decommitUnusedStackOs() { decommitUnusedStack() }

func decommitRange(p []byte) {
	if len(p) == 0 {
		return
	}
	ptr := uintptr(unsafe.Pointer(&p[0]))
	decommit(ptr, ptr+uintptr(len(p)))
}

func decommitUnusedStack() {
	procPin()
	gp := getg()
	// decommit is nosplit, so it can use at most _StackLimit bytes,
	// so we assume that _StackLimit past the current sp will be used
	// by decommit itself, and we ask to decommit the remaining space.
	decommit(gp.stack.lo, gp.sched.sp-_StackLimit)
	procUnpin()
}

func init() {
	if physPageSize&(physPageSize-1) != 0 {
		throw("physPageSize is not a power-of-two")
	}
}

// nosplit because start/end may point to the stack of the goroutine
// and if we resize the stack when we enter this function then they
// will potentially point to other memory that we should not touch.
//go:nosplit
func decommit(start, end uintptr) {
	start = (start + (physPageSize - 1)) &^ (physPageSize - 1)
	end = end &^ (physPageSize - 1)
	if start >= end {
		return
	}
	if GOOS == "linux" || GOOS == "darwin" || GOOS == "aix" {
		madvise(unsafe.Pointer(start), end-start, _MADV_DONTNEED)
	} else {
		madvise(unsafe.Pointer(start), end-start, _MADV_FREE)
	}
}
