package runtime

import (
	"runtime/internal/math"
	"unsafe"
)

func InternString(s string) string {
	mp := getg().m
	mp.locks++
	s = mp.p.ptr().internStringTable.get(s)
	mp.locks--
	return s
}

func InternBytes(b []byte) string {
	mp := getg().m
	mp.locks++
	s := mp.p.ptr().internStringTable.getBytes(b)
	mp.locks--
	if s == "" {
		s = string(b)
	}
	return s
}

type internStringTable struct {
	table []string
	seed  uint32
	state internStringTableState
}

type internStringTableState uint8

const (
	internStringTableEmpty internStringTableState = iota
	internStringTableUsed
	internStringTableDirty
)

const replacementInterval = 16

func (t *internStringTable) get(s string) string {
	if len(t.table) == 0 {
		return s
	}
	if t.state == internStringTableDirty {
		t.reset(-1)
	}
	h := stringHash(s, uintptr(t.seed))
	i := mulhi(h, uintptr(len(t.table)))
	switch t.table[i] {
	case s:
		return t.table[i]
	default:
		if fastrandn(replacementInterval) != 0 {
			return s
		}
		fallthrough
	case "":
		t.table[i] = s
		t.state = internStringTableUsed
		return s
	}
}

func (t *internStringTable) getBytes(b []byte) string {
	if len(t.table) == 0 {
		return ""
	}
	if t.state == internStringTableDirty {
		t.reset(-1)
	}
	h := bytesHash(b, uintptr(t.seed))
	i := mulhi(h, uintptr(len(t.table)))
	switch t.table[i] {
	case slicebytetostringtmp(&b[0], len(b)):
		return t.table[i]
	default:
		if fastrandn(replacementInterval) != 0 {
			return ""
		}
		fallthrough
	case "":
		s := string(b)
		t.table[i] = s
		t.state = internStringTableUsed
		return s
	}
}

func mulhi(a, b uintptr) uintptr {
	if unsafe.Sizeof(uintptr(0)) == 8 {
		hi, _ := math.Mul64(uint64(a), uint64(b))
		return uintptr(hi)
	}
	return uintptr((uint64(a) * uint64(b)) >> 32)
}

func (t *internStringTable) reset(n int) {
	t.seed = fastrand()
	t.state = internStringTableEmpty
	if n >= 0 && len(t.table) != n {
		t.table = make([]string, n)
		return
	}
	for i := range t.table {
		t.table[i] = ""
	}
}

func internstringtablecleanup() {
	for _, p := range allp {
		t := &p.internStringTable
		switch t.state {
		case internStringTableEmpty:
			// The table is unused and contains no strings, so there's nothing to do.
		case internStringTableUsed:
			// The table contains some string. Mark it dirty, so that either the next
			// InternString operation (or, in the worst case, the next GC cycle) will
			// reset it to the empty state.
			// We could simplify this and always reset the table here, but this would
			// increase GC stop-the-world latency, that is undesirable. Instead, we
			// simply mark the table as dirty, and expect the next InternString call
			// to do it while the world is running.
			t.state = internStringTableDirty
		case internStringTableDirty:
			// This can happen only if a table was used before the previous GC cycle,
			// was marked as dirty in the previous GC cycle, and then no InternString
			// operation was performed between the previous GC cycle and the current
			// one. As such it should happen very rarely.
			// We have to do this to guarantee that eventually all strings are GCed,
			// even in case the user suddenly stops calling InternString.
			t.reset(-1)
		default:
			throw("illegal state")
		}
	}
}
