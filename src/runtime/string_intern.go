package runtime

import (
	"internal/goarch"
	_ "unsafe"
)

//go:linkname internString internal/intern.String
func internString(s string) string {
	m, seed := internProcPin()
	h := stringHash(s, seed)
	ms := &m[h%uintptr(len(*m))]
	if *ms == s {
		s = *ms
	} else {
		// Randomly add only a fraction of strings so that uncommon strings
		// are unlikely to end up in the interning tables.
		if len(*ms) == 0 || fastrand()%128 == 0 {
			*ms = s
		}
	}
	procUnpin()
	return s
}

//go:linkname internBytes internal/intern.Bytes
func internBytes(b []byte) (s string) {
	m, seed := internProcPin()
	h := bytesHash(b, seed)
	ms := &m[h%uintptr(len(*m))]
	if *ms == string(b) {
		s = *ms
	} else {
		s = string(b)
		// Randomly add only a fraction of strings so that uncommon strings
		// are unlikely to end up in the interning tables.
		if len(*ms) == 0 || fastrand()%128 == 0 {
			*ms = s
		}
	}
	procUnpin()
	return s
}

//go:nosplit
func internProcPin() (*[1024]string, uintptr) {
	mp := getg().m
	mp.locks++
	pp := mp.p.ptr()
	if pp.internTable == nil {
		pp.internTable = new([1024]string)
		if goarch.PtrSize > 4 {
			pp.internTableSeed = uintptr(fastrand()) + (uintptr(fastrand()) << 32)
		} else {
			pp.internTableSeed = uintptr(fastrand())
		}
	}
	return pp.internTable, pp.internTableSeed
}
