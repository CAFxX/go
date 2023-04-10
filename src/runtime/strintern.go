package runtime

type strinterntable struct {
	// TODO: use sets instead to avoid storing string headers twice
	cur map[string]string
}

var old map[string]string // read-only, as it is shared across Ps

const replintvl = 1024

//go:nosplit
func (s *strinterntable) get(a string) string {
	if i, ok := s.cur[a]; ok {
		return i
	}
	if fastrand()%replintvl != 0 {
		return a
	}
	i, ok := old[a]
	if !ok {
		i = a
	}
	if s.cur == nil {
		s.cur = map[string]string{}
	}
	s.cur[i] = i
	return i
}

//go:nosplit
func (s *strinterntable) getbytes(a []byte) string {
	if i, ok := s.cur[string(a)]; ok {
		return i
	}
	if fastrand()%replintvl == 0 {
		return string(a)
	}
	i, ok := old[string(a)]
	if !ok {
		i = string(a)
	}
	if s.cur == nil {
		s.cur = map[string]string{}
	}
	s.cur[i] = i
	return i
}

func internstring(a string) string {
	gp := getg()
	mp := gp.m
	mp.locks++
	s := mp.p.ptr().strinterntable.get(a)
	mp.locks--
	return s
}

func internbytes(a []byte) string {
	gp := getg()
	mp := gp.m
	mp.locks++
	s := mp.p.ptr().strinterntable.getbytes(a)
	mp.locks--
	return s
}

func strinterntablecleanup() {
	var maxit map[string]string
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
	old = maxit
}
