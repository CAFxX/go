package runtime

// This file contains the string interning implementation.
// String interning is performed in a best-effort manner:
// the runtime does not guarantee that all strings will be interned.
//
// The runtime keeps a per-P table, and a global table.
//
// The per-P table is used to hold the strings interned since the last GC.
// The per-P table can only be used by the P that owns it.
// During GC, the largest per-P table is promoted to be the global table,
// while all other per-P tables, and the previous global table, are dropped.
//
// The global table is read-only, as it is accessed concurrently by all Ps
// without synchronization.
//
// In case of a miss on the per-P table, the runtime will randomly check
// if the string is present in the global table. If it is, the string in
// the global table is added to the per-P table. If it is not, the string
// is added to the per-P table.
//
// How frequently the runtime checks the global table and adds the string
// to the per-P table is controlled by the replintvl constant. Higher values
// mean that strings are added to the per-P table less frequently: this
// is done also to prevent uncommon strings from being added to the
// tables.
//
// If interning is not used for two GC cycles, no memory is used for
// interning (aside from P+1 pointers for the table roots, where P is the
// number of Ps). The biggest downside is that a string that is interned
// can remain alive for one GC cycle more than it would otherwise.

// TODO:
// - add a way to disable interning
// - better protection against pathological uses (e.g. as the miss ratio
//   increases, the runtime should skip interning more often)
// - use a dedicated set implementation that allows to precompute the hash
//   of the string and use it in both the per-P and global tables (to avoid
//   hashing the string more than once)

import "unsafe"

type strinterntable struct {
	cur  map[string]struct{} // per-P table: used only by the owning P
	skip uint32
}

// global table: read-only, as it is shared across Ps
var strinterntableold map[string]struct{}

// replintvl controls how often misses lead to adding a string
// to the per-P table (on average once every replintvl misses).
const replintvl = 128

// TODO: inline this
//
//go:nosplit
func (s *strinterntable) get(a string) string {
	if i := strinterncheck(s.cur, a); i != "" {
		return i
	}
	if s.skip > 0 {
		s.skip--
		return a
	}
	s.skip = fastrand() % (replintvl * 2)
	i := strinterncheck(strinterntableold, a)
	if i == "" {
		i = a
	}
	if s.cur == nil {
		s.cur = make(map[string]struct{}, len(strinterntableold))
	}
	s.cur[i] = struct{}{}
	return i
}

// TODO: inline this
//
//go:nosplit
func (s *strinterntable) getbytes(a []byte) string {
	if i := strinterncheckbytes(s.cur, a); i != "" {
		return i
	}
	if s.skip > 0 {
		s.skip--
		return string(a)
	}
	s.skip = fastrand() % (replintvl * 2)
	i := strinterncheckbytes(strinterntableold, a)
	if i == "" {
		i = string(a)
	}
	if s.cur == nil {
		s.cur = make(map[string]struct{}, len(strinterntableold))
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
	// The M must be pinned while accessing the interning tables;
	// this prevents garbage collection (and, crucially, the
	// strinterntablecleanup) from running while any P is accessing
	// the interning tables.
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
	// The M must be pinned while accessing the interning tables;
	// this prevents garbage collection (and, crucially, the
	// strinterntablecleanup) from running while any P is accessing
	// the interning tables.
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
	// the G running on that M can not be preempted, and until all
	// running Gs are preempted the world can not be stopped).
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

// TODO: inline this
//
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

// TODO: inline this
//
//go:nosplit
func strinterncheckbytes(m map[string]struct{}, b []byte) string {
	hdr := (*slice)(unsafe.Pointer(&b))
	s := *(*string)(unsafe.Pointer(&stringStruct{hdr.array, hdr.len}))
	return strinterncheck(m, s)
}
