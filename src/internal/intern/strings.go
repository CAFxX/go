package intern

// String performs best-effort string interning of the provided string.
func String(s string) string {
	m := runtime_procPin()
	if is, ok := m[s]; ok {
		s = is
	} else {
		// Randomly add only a fraction of strings so that uncommon strings
		// are unlikely to end up in the interning tables.
		if fastrand()%128 == 0 {
			m[s] = s
		}
	}
	runtime_procUnpin()
	return s
}

// Bytes perform best-effort string interning of the provided byte slice.
func Bytes(b []byte) (s string) {
	m := runtime_procPin()
	if is, ok := m[string(b)]; ok {
		s = is
	} else {
		s = string(b)
		// Randomly add only a fraction of strings so that uncommon strings
		// are unlikely to end up in the interning tables.
		if fastrand()%128 == 0 {
			m[s] = s
		}
	}
	runtime_procUnpin()
	return s
}

// Implemented in runtime.
func runtime_procPin() map[string]string
func runtime_procUnpin()
func fastrand() uint32
