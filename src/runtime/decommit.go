//go:build !darwin && !linux && !freebsd && !netbsd && !openbsd && !aix
// +build !darwin,!linux,!freebsd,!netbsd,!openbsd,!aix

package runtime

import _ "unsafe"

//go:linkname decommitRangeNet net.runtime_decommitRange
func decommitRangeNet(p []byte) {}

//go:linkname decommitRangeOs os.runtime_decommitRange
func decommitRangeOs(p []byte) {}

//go:linkname decommitUnusedStackNet net.runtime_decommitUnusedStack
func decommitUnusedStackNet() {}

//go:linkname decommitUnusedStackOs os.runtime_decommitUnusedStack
func decommitUnusedStackOs() {}
