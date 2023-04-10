package runtime

import "unsafe"

type strinterntable struct {
	cur map[string]struct{}
}

var strinterntableold map[string]struct{} // read-only, as it is shared across Ps

const replintvl = 128

//go:nosplit
func (s *strinterntable) get(a string) string {
	if i := strinterncheck(s.cur, a); i != "" {
		return i
	}
	if fastrand()%replintvl != 0 {
		return a
	}
	i := strinterncheck(strinterntableold, a)
	if i == "" {
		i = a
	}
	if s.cur == nil {
		s.cur = map[string]struct{}{}
	}
	s.cur[i] = struct{}{}
	return i
}

//go:nosplit
func (s *strinterntable) getbytes(a []byte) string {
	if i := strinterncheckbytes(s.cur, a); i != "" {
		return i
	}
	if fastrand()%replintvl == 0 {
		return string(a)
	}
	i := strinterncheckbytes(strinterntableold, a)
	if i == "" {
		i = string(a)
	}
	if s.cur == nil {
		s.cur = map[string]struct{}{}
	}
	s.cur[i] = struct{}{}
	return i
}

//go:nosplit
func internstring(a string) string {
	if len(a) == 0 {
		return ""
	}
	gp := getg()
	mp := gp.m
	// G must be pinned while accessing the interning tables
	mp.locks++
	s := mp.p.ptr().strinterntable.get(a)
	mp.locks--
	return s
}

//go:nosplit
func internbytes(a []byte) string {
	if len(a) == 0 {
		return ""
	}
	gp := getg()
	mp := gp.m
	// G must be pinned while accessing the interning tables
	mp.locks++
	s := mp.p.ptr().strinterntable.getbytes(a)
	mp.locks--
	return s
}

func strinterntablecleanup() {
	// This is executed with the world stopped, so we don't need to
	// worry about concurrent accesses, as Ps are not holding on to any
	// of the current or old tables (because when they are holding on
	// to a table the corresponding M's locks must be > 0, in which case
	// the G running on that M can't be preempted, and until all running
	// Gs are preempted the world can't be stopped).
	var maxit map[string]struct{}
	var maxlen int
	for _, pp := range allp {
		if l := len(pp.strinterntable.cur); l > maxlen {
			maxit, maxlen = pp.strinterntable.cur, l
		}
		pp.strinterntable.cur = nil
	}
	// Set as the old table the table with the most entries.
	// This allows, over successive garbage collection cycles, to
	// share entries across all Ps (as on misses on the new tables
	// the old table is consulted as a fallback, and hits on the old
	// table are added to the new tables).
	// Because the old table will from now and to the next call
	// to strinterntablecleanup be shared across all Ps, and because
	// there is no syncrhonization, the old table must always be
	// accessed exclusively in a read-only fashion.
	strinterntableold = maxit
}

//go:nosplit
func strinterncheck(m map[string]struct{}, s string) string {
	// Check that s is in m, and if so pull the key out of m,
	// which is the canonical string.
	h := *(**hmap)(unsafe.Pointer(&m))
	i := any(m)
	t := *(**maptype)(unsafe.Pointer(&i))
	kp, _ := mapaccessK(t, h, noescape(unsafe.Pointer(&s)))
	if kp == nil {
		return ""
	}
	return *(*string)(kp)
}

//go:nosplit
func strinterncheckbytes(m map[string]struct{}, b []byte) string {
	ptr := unsafe.Pointer(&b[0])
	len := len(b)
	s := *(*string)(unsafe.Pointer(&stringStruct{ptr, len}))
	return strinterncheck(m, s)
}
