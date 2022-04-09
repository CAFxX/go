package intern

// Implemented in runtime.

// String performs best-effort string interning of the provided string.
//
// Interning is only beneficial if we expect to attempt to intern the same
// string many times; otherwise it will just waste CPU and memory
// resources.
func String(s string) string

// Bytes perform best-effort string interning of the provided byte slice.
//
// Interning is only beneficial if we expect to attempt to intern the same
// string many times; otherwise it will just waste CPU and memory
// resources.
func Bytes(b []byte) (s string)
