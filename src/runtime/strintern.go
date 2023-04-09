package runtime

type strinterntable struct {
	// TODO: use sets instead to avoid storing string headers twice
	cur map[string]string
	old map[string]string // read-only, as it is shared across Ps
}

func (s *strinterntable) get(a string) string {
	if s.cur == nil {
		s.cur = map[string]string{}
	}
	if i, ok := s.cur[a]; ok {
		return i
	}
	if i, ok := s.old[a]; ok {
		s.cur[i] = i
		return i
	}
	if fastrandn(4) == 0 {
		s.cur[a] = a
	}
	return a
}

func (s *strinterntable) getbytes(a []byte) string {
	if s.cur == nil {
		s.cur = map[string]string{}
	}
	if i, ok := s.cur[string(a)]; ok {
		return i
	}
	if i, ok := s.old[string(a)]; ok {
		s.cur[i] = i
		return i
	}
	b := string(a)
	if fastrandn(4) == 0 {
		s.cur[b] = b
	}
	return b
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
	}
	for _, pp := range allp {
		pp.strinterntable.old = maxit
		pp.strinterntable.cur = nil
	}
}
