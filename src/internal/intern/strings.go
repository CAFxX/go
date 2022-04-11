package intern

// String performs best-effort string interning of the provided string.
func String(s string) string {
	m, seed := runtime_procPin()
	h := runtime_stringHash(s, seed)
	ms := &m[h%uintptr(len(*m))]
	if *ms == s {
		s = *ms
	} else {
		// Randomly add only a fraction of strings so that uncommon strings
		// are unlikely to end up in the interning tables.
		if fastrand()%128 == 0 {
			*ms = s
		}
	}
	runtime_procUnpin()
	return s
}

// Bytes perform best-effort string interning of the provided byte slice.
func Bytes(b []byte) (s string) {
	m, seed := runtime_procPin()
	h := runtime_bytesHash(b, seed)
	ms := &m[h%uintptr(len(*m))]
	if *ms == string(b) {
		s = *ms
	} else {
		s = string(b)
		// Randomly add only a fraction of strings so that uncommon strings
		// are unlikely to end up in the interning tables.
		if fastrand()%128 == 0 {
			*ms = s
		}
	}
	runtime_procUnpin()
	return s
}

// Implemented in runtime.
func runtime_procPin() (*[1024]string, uintptr)
func runtime_procUnpin()
func runtime_stringHash(string, uintptr) uintptr
func runtime_bytesHash([]byte, uintptr) uintptr
func fastrand() uint32
