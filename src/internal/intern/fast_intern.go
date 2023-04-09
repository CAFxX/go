package intern

import (
	_ "runtime"
	_ "unsafe"
)

//go:linkname runtime_internstring runtime.internstring
func runtime_internstring(string) string

//go:linkname runtime_internbytes runtime.internbytes
func runtime_internbytes([]byte) string

// String performs best-effort string interning.
func String(a string) string {
	return runtime_internstring(a)
}

// Bytes performs best-effort byte slice interning.
func Bytes(a []byte) string {
	return runtime_internbytes(a)
}
