package runtime

import (
	"runtime/internal/atomic"
	"unsafe"
)

type internEntry struct {
	s string
}

type internTable struct {
	table [1024]internEntry
	lock  uint32
	seed  uint32
}

func (t *internTable) tryLock() bool {
	return atomic.Xchg(&t.lock, 1) == 0
}

func (t *internTable) unlock() {
	atomic.Store(&t.lock, 0)
}

const internMaxStrLen = 4096

func Intern(s string) string {
	if len(s) > internMaxStrLen {
		return s
	}

	t := getg().m.p.ptr().internTable
	if !t.tryLock() {
		return s
	}

	if fastrand()%128 == 0 {
		incrementalSync(t)
	}

	h := intern_strhash(s, uintptr(t.seed))
	i := h % uintptr(len(t.table))
	if is := t.table[i].s; s == is {
		s = is
	} else if is == "" || fastrand()%128 == 0 {
		t.table[i].s = s
	}
	t.unlock()
	return s
}

func InternBytes(b []byte) string {
	if len(b) > internMaxStrLen {
		return string(b)
	}

	t := getg().m.p.ptr().internTable
	if !t.tryLock() {
		return string(b)
	}

	if fastrand()%128 == 0 {
		incrementalSync(t)
	}

	h := intern_slicehash(b, uintptr(t.seed))
	i := h % uintptr(len(t.table))
	var s string
	if is := t.table[i].s; string(b) == is {
		s = is
	} else if is == "" || fastrand()%128 == 0 {
		s = string(b)
		t.table[i].s = s
	} else {
		s = string(b)
	}
	t.unlock()
	return s
}

func incrementalSync(t *internTable) {
	e := &t.table[fastrandn(uint32(len(t.table)))]
	if e.s == "" {
		return
	}
	lock(&allpLock)
	t2 := allp[fastrandn(uint32(len(allp)))].internTable
	unlock(&allpLock)
	if t == t2 || t2 == nil || !t2.tryLock() {
		return
	}
	h2 := intern_strhash(e.s, uintptr(t2.seed))
	i2 := h2 % uintptr(len(t2.table))
	if e2 := &t2.table[i2]; e.s == e2.s {
		if ptr(e.s) > ptr(e2.s) {
			e.s = e2.s
		} else if ptr(e.s) < ptr(e2.s) {
			e2.s = e.s
		}
	}
	t2.unlock()
}

func intern_strhash(s string, seed uintptr) uintptr {
	return strhash(noescape(unsafe.Pointer(&s)), seed)
}

func intern_slicehash(b []byte, seed uintptr) uintptr {
	s := (*slice)(unsafe.Pointer(&b))
	return memhash(s.array, seed, uintptr(s.len))
}

func ptr(s string) uintptr {
	return uintptr(stringStructOf(&s).str)
}
