// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package runtime

import (
	"internal/bytealg"
	"unsafe"
)

// The constant is known to the compiler.
// There is no fundamental theory behind this number.
const tmpStringBufSize = 32

type tmpBuf [tmpStringBufSize]byte

// concatstrings implements a Go string concatenation x+y+z+...
// The operands are passed in the slice a.
// If buf != nil, the compiler has determined that the result does not
// escape the calling function, so the string data can be stored in buf
// if small enough.
func concatstrings(buf *tmpBuf, a []string) string {
	idx := 0
	l := 0
	count := 0
	for i, x := range a {
		n := len(x)
		if n == 0 {
			continue
		}
		if l+n < l {
			throw("string concatenation too long")
		}
		l += n
		count++
		idx = i
	}
	if count == 0 {
		return ""
	}

	// If there is just one string and either it is not on the stack
	// or our result does not escape the calling frame (buf != nil),
	// then we can return that string directly.
	if count == 1 && (buf != nil || !stringDataOnStack(a[idx])) {
		return a[idx]
	}

	// TODO: find a way to intern rune encoding without a temporary buffer
	// or at least to avoid the second copy
	var s string
	var b []byte
	usingInternBuf := (buf == nil || l > len(buf)) && interningAllowed(l)
	if usingInternBuf {
		var _b internBuf // must not escape the stack
		b = _b[:l]
	} else {
		s, b = rawstringtmp(buf, l)
	}

	sz := 0
	for _, x := range a {
		copy(b[sz:], x)
		sz += len(x)
	}

	if usingInternBuf {
		return interntmp(b)
	}
	return s
}

func concatstring2(buf *tmpBuf, a [2]string) string {
	return concatstrings(buf, a[:])
}

func concatstring3(buf *tmpBuf, a [3]string) string {
	return concatstrings(buf, a[:])
}

func concatstring4(buf *tmpBuf, a [4]string) string {
	return concatstrings(buf, a[:])
}

func concatstring5(buf *tmpBuf, a [5]string) string {
	return concatstrings(buf, a[:])
}

// Buf is a fixed-size buffer for the result,
// it is not nil if the result does not escape.
func slicebytetostring(buf *tmpBuf, b []byte) (str string) {
	l := len(b)
	if l == 0 {
		// Turns out to be a relatively common case.
		// Consider that you want to parse out data between parens in "foo()bar",
		// you find the indices and convert the subslice to string.
		return ""
	}

	if raceenabled {
		racereadrangepc(unsafe.Pointer(&b[0]),
			uintptr(l),
			getcallerpc(),
			funcPC(slicebytetostring))
	}
	if msanenabled {
		msanread(unsafe.Pointer(&b[0]), uintptr(l))
	}

	if l == 1 {
		stringStructOf(&str).str = unsafe.Pointer(&staticbytes[b[0]])
		stringStructOf(&str).len = 1
		return
	}

	usingBuf := buf != nil && l <= len(buf)
	tryIntern := !usingBuf && interningAllowed(l)

	var idx, off, N uintptr
	if tryIntern {
		str, idx, off, N = stringIsInterned(slicebytetostringtmp(b))
		if str != "" {
			return
		}
	}

	var p unsafe.Pointer
	if usingBuf {
		p = unsafe.Pointer(buf)
	} else {
		p = mallocgc(uintptr(l), nil, false)
	}

	stringStructOf(&str).str = p
	stringStructOf(&str).len = l
	memmove(p, (*(*slice)(unsafe.Pointer(&b))).array, uintptr(l))

	if tryIntern {
		internString(str, idx, off, N)
	}

	return
}

// stringDataOnStack reports whether the string's data is
// stored on the current goroutine's stack.
func stringDataOnStack(s string) bool {
	ptr := uintptr(stringStructOf(&s).str)
	stk := getg().stack
	return stk.lo <= ptr && ptr < stk.hi
}

func rawstringtmp(buf *tmpBuf, l int) (s string, b []byte) {
	if buf != nil && l <= len(buf) {
		b = buf[:l]
		s = slicebytetostringtmp(b)
	} else {
		s, b = rawstring(l)
	}
	return
}

// slicebytetostringtmp returns a "string" referring to the actual []byte bytes.
//
// Callers need to ensure that the returned string will not be used after
// the calling goroutine modifies the original slice or synchronizes with
// another goroutine.
//
// The function is only called when instrumenting
// and otherwise intrinsified by the compiler.
//
// Some internal compiler optimizations use this function.
// - Used for m[string(k)] lookup where m is a string-keyed map and k is a []byte.
// - Used for "<"+string(b)+">" concatenation where b is []byte.
// - Used for string(b)=="foo" comparison where b is []byte.
func slicebytetostringtmp(b []byte) string {
	if raceenabled && len(b) > 0 {
		racereadrangepc(unsafe.Pointer(&b[0]),
			uintptr(len(b)),
			getcallerpc(),
			funcPC(slicebytetostringtmp))
	}
	if msanenabled && len(b) > 0 {
		msanread(unsafe.Pointer(&b[0]), uintptr(len(b)))
	}
	return *(*string)(unsafe.Pointer(&b))
}

func stringtoslicebyte(buf *tmpBuf, s string) []byte {
	var b []byte
	if buf != nil && len(s) <= len(buf) {
		*buf = tmpBuf{}
		b = buf[:len(s)]
	} else {
		b = rawbyteslice(len(s))
	}
	copy(b, s)
	return b
}

func stringtoslicerune(buf *[tmpStringBufSize]rune, s string) []rune {
	// two passes.
	// unlike slicerunetostring, no race because strings are immutable.
	n := 0
	for range s {
		n++
	}

	var a []rune
	if buf != nil && n <= len(buf) {
		*buf = [tmpStringBufSize]rune{}
		a = buf[:n]
	} else {
		a = rawruneslice(n)
	}

	n = 0
	for _, r := range s {
		a[n] = r
		n++
	}
	return a
}

func slicerunetostring(buf *tmpBuf, a []rune) string {
	if raceenabled && len(a) > 0 {
		racereadrangepc(unsafe.Pointer(&a[0]),
			uintptr(len(a))*unsafe.Sizeof(a[0]),
			getcallerpc(),
			funcPC(slicerunetostring))
	}
	if msanenabled && len(a) > 0 {
		msanread(unsafe.Pointer(&a[0]), uintptr(len(a))*unsafe.Sizeof(a[0]))
	}
	size1 := 0
	for _, r := range a {
		size1 += encodedrunebytes(r)
	}

	// TODO: find a way to intern rune encoding without a temporary buffer
	// or at least to avoid the second copy
	var s string
	var b []byte
	l := size1 + 3
	usingInternBuf := (buf == nil || l > len(buf)) && interningAllowed(l)
	if usingInternBuf {
		var _b internBuf // must not escape the stack
		b = _b[:]
	} else {
		s, b = rawstringtmp(buf, l)
	}

	size2 := 0
	for _, r := range a {
		// check for race
		if size2 >= size1 {
			break
		}
		size2 += encoderune(b[size2:], r)
	}

	if usingInternBuf {
		return interntmp(b[:size2])
	}
	return s[:size2]
}

type stringStruct struct {
	str unsafe.Pointer
	len int
}

// Variant with *byte pointer type for DWARF debugging.
type stringStructDWARF struct {
	str *byte
	len int
}

func stringStructOf(sp *string) *stringStruct {
	return (*stringStruct)(unsafe.Pointer(sp))
}

func intstring(buf *[4]byte, v int64) (s string) {
	if v >= 0 && v < runeSelf {
		stringStructOf(&s).str = unsafe.Pointer(&staticbytes[v])
		stringStructOf(&s).len = 1
		return
	}

	var b []byte
	if buf != nil {
		b = buf[:]
		s = slicebytetostringtmp(b)
	} else {
		s, b = rawstring(4)
	}
	if int64(rune(v)) != v {
		v = runeError
	}
	n := encoderune(b, rune(v))
	return s[:n]
}

// rawstring allocates storage for a new string. The returned
// string and byte slice both refer to the same storage.
// The storage is not zeroed. Callers should use
// b to set the string contents and then drop b.
func rawstring(size int) (s string, b []byte) {
	p := mallocgc(uintptr(size), nil, false)

	stringStructOf(&s).str = p
	stringStructOf(&s).len = size

	*(*slice)(unsafe.Pointer(&b)) = slice{p, size, size}

	return
}

// rawbyteslice allocates a new byte slice. The byte slice is not zeroed.
func rawbyteslice(size int) (b []byte) {
	cap := roundupsize(uintptr(size))
	p := mallocgc(cap, nil, false)
	if cap != uintptr(size) {
		memclrNoHeapPointers(add(p, uintptr(size)), cap-uintptr(size))
	}

	*(*slice)(unsafe.Pointer(&b)) = slice{p, size, int(cap)}
	return
}

// rawruneslice allocates a new rune slice. The rune slice is not zeroed.
func rawruneslice(size int) (b []rune) {
	if uintptr(size) > maxAlloc/4 {
		throw("out of memory")
	}
	mem := roundupsize(uintptr(size) * 4)
	p := mallocgc(mem, nil, false)
	if mem != uintptr(size)*4 {
		memclrNoHeapPointers(add(p, uintptr(size)*4), mem-uintptr(size)*4)
	}

	*(*slice)(unsafe.Pointer(&b)) = slice{p, size, int(mem / 4)}
	return
}

// used by cmd/cgo
func gobytes(p *byte, n int) (b []byte) {
	if n == 0 {
		return make([]byte, 0)
	}

	if n < 0 || uintptr(n) > maxAlloc {
		panic(errorString("gobytes: length out of range"))
	}

	bp := mallocgc(uintptr(n), nil, false)
	memmove(bp, unsafe.Pointer(p), uintptr(n))

	*(*slice)(unsafe.Pointer(&b)) = slice{bp, n, n}
	return
}

func gostring(p *byte) string {
	l := findnull(p)
	if l == 0 {
		return ""
	}
	s, b := rawstring(l)
	memmove(unsafe.Pointer(&b[0]), unsafe.Pointer(p), uintptr(l))
	return s
}

func gostringn(p *byte, l int) string {
	if l == 0 {
		return ""
	}
	s, b := rawstring(l)
	memmove(unsafe.Pointer(&b[0]), unsafe.Pointer(p), uintptr(l))
	return s
}

func index(s, t string) int {
	if len(t) == 0 {
		return 0
	}
	for i := 0; i < len(s); i++ {
		if s[i] == t[0] && hasPrefix(s[i:], t) {
			return i
		}
	}
	return -1
}

func contains(s, t string) bool {
	return index(s, t) >= 0
}

func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

const (
	maxUint = ^uint(0)
	maxInt  = int(maxUint >> 1)
)

// atoi parses an int from a string s.
// The bool result reports whether s is a number
// representable by a value of type int.
func atoi(s string) (int, bool) {
	if s == "" {
		return 0, false
	}

	neg := false
	if s[0] == '-' {
		neg = true
		s = s[1:]
	}

	un := uint(0)
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c < '0' || c > '9' {
			return 0, false
		}
		if un > maxUint/10 {
			// overflow
			return 0, false
		}
		un *= 10
		un1 := un + uint(c) - '0'
		if un1 < un {
			// overflow
			return 0, false
		}
		un = un1
	}

	if !neg && un > uint(maxInt) {
		return 0, false
	}
	if neg && un > uint(maxInt)+1 {
		return 0, false
	}

	n := int(un)
	if neg {
		n = -n
	}

	return n, true
}

// atoi32 is like atoi but for integers
// that fit into an int32.
func atoi32(s string) (int32, bool) {
	if n, ok := atoi(s); n == int(int32(n)) {
		return int32(n), ok
	}
	return 0, false
}

//go:nosplit
func findnull(s *byte) int {
	if s == nil {
		return 0
	}

	// Avoid IndexByteString on Plan 9 because it uses SSE instructions
	// on x86 machines, and those are classified as floating point instructions,
	// which are illegal in a note handler.
	if GOOS == "plan9" {
		p := (*[maxAlloc/2 - 1]byte)(unsafe.Pointer(s))
		l := 0
		for p[l] != 0 {
			l++
		}
		return l
	}

	// pageSize is the unit we scan at a time looking for NULL.
	// It must be the minimum page size for any architecture Go
	// runs on. It's okay (just a minor performance loss) if the
	// actual system page size is larger than this value.
	const pageSize = 4096

	offset := 0
	ptr := unsafe.Pointer(s)
	// IndexByteString uses wide reads, so we need to be careful
	// with page boundaries. Call IndexByteString on
	// [ptr, endOfPage) interval.
	safeLen := int(pageSize - uintptr(ptr)%pageSize)

	for {
		t := *(*string)(unsafe.Pointer(&stringStruct{ptr, safeLen}))
		// Check one page at a time.
		if i := bytealg.IndexByteString(t, 0); i != -1 {
			return offset + i
		}
		// Move to next page
		ptr = unsafe.Pointer(uintptr(ptr) + uintptr(safeLen))
		offset += safeLen
		safeLen = pageSize
	}
}

func findnullw(s *uint16) int {
	if s == nil {
		return 0
	}
	p := (*[maxAlloc/2/2 - 1]uint16)(unsafe.Pointer(s))
	l := 0
	for p[l] != 0 {
		l++
	}
	return l
}

//go:nosplit
func gostringnocopy(str *byte) string {
	ss := stringStruct{str: unsafe.Pointer(str), len: findnull(str)}
	s := *(*string)(unsafe.Pointer(&ss))
	return s
}

func gostringw(strw *uint16) string {
	str := (*[maxAlloc/2/2 - 1]uint16)(unsafe.Pointer(strw))
	n1 := 0
	for i := 0; str[i] != 0; i++ {
		n1 += encodedrunebytes(rune(str[i]))
	}
	s, b := rawstring(n1 + 4)
	n2 := 0
	for i := 0; str[i] != 0; i++ {
		// check for race
		if n2 >= n1 {
			break
		}
		n2 += encoderune(b[n2:], rune(str[i]))
	}
	b[n2] = 0 // for luck
	return s[:n2]
}

// String interning
// Experimental automatic string interning support based on per-P interning tables
// that are cleared during every GC. Since the tables are small-ish and they are
// fixed in size collisions are likely: when a collision happens the old string is
// replaced with the new one; this means that when checking for presence we have
// to compare the whole string in addition to the hash. To bound the resulting
// overhead automatic interning is only performed on small strings (<strInternMaxLen)
// and it is disabled automatically if the hit ratio is too low.
// TODO: Move most string interning to the concurrent GC mark phase.
// TODO: Expose APIs to perform on-demand interning.
// TODO: Tune strInternMaxLen and the size of the per-P tables.
// TODO: Remove the extra copy due to the use of internBuf.
// TODO: Evaluate alternatives to memhash.
// TODO: Once midstack inlining works deduplicate the checks for strInternMaxLen
//       in the functions above and allow the early return in stringIsInterned to
//       be inlined.
// TODO: Deal with table thrashing due to hash collisions of frequent strings with
//       infrequent ones (e.g. by storing a 1-3 bit "hit" counter per entry)
// TODO: "Compress" the interning tables. Currently every entry is a string, using 16
//       bytes per entry on 64 bit platforms. It should be possible to compress this
//       to 8 bytes (stuffing the length bits inside the byte pointer to the string)
//       or even less (if we have max N strings of len M, the maximum number of bytes
//       we need to address is N*M, so e.g. N=2^16 and M=64, we don't need more than
//       log2(2^16*64)=22 bit to address the strings, and log2(M)=6 bit to store the
//       the string length, i.e. we need 28 bits - that fit in 4 bytes). Since the
//       tables are cleared during GC and anyway they are not supposed to keep the
//       strings alive, compressing the tables won't even require the GC to be aware
//       of the "compressed pointers".
// TODO: Store a computed string hash in the string itself, and use that to speed up
//       table lookup.

const (
	strInternTableMaxLen = 16 // (arbitrary) maximum size of per-P table (power of 2)
	strInternTableMinLen = 8  // (arbitrary) minimum size of per-P table (power of 2)
	strInternMaxLen      = 64 // (arbitrary) maximum length of string to be considered for interning
	strInternProbes      = 2  // (arbitrary) maximum number of alternative slots to visit
)

var strInternTableLen = strInternTableMinLen

type internBuf [strInternMaxLen]byte

func stringIsInterned(s string) (string, uintptr, uintptr, uintptr) {
	procPin()
	_p_ := getg().m.p.ptr()

	if _p_.strInternTable == nil {
		_p_.strInternTable = make([]string, interntablesize())
	}

	ps := (*stringStruct)(noescape(unsafe.Pointer(&s)))
	hash := memhash(ps.str, _p_.strInternSeed, uintptr(ps.len))
	N := uintptr(len(_p_.strInternTable))
	idx, off := hashReduce(hash, N)

	for i, _idx := 0, idx; i < strInternProbes; i, _idx = i+1, hashNext(_idx, off, N) {
		is := _p_.strInternTable[_idx]
		pis := (*stringStruct)(unsafe.Pointer(&is))
		if pis.len == ps.len && memequal(pis.str, ps.str, uintptr(ps.len)) {
			_p_.strInternHits++
			procUnpin()
			return is, idx, off, N
		}
	}

	procUnpin()
	return "", idx, off, N
}

func internString(s string, idx, off, N uintptr) {
	// Note that because we're not pinned, we may be writing to the table of
	// a different P or otherwise with a different seed (because GC happened
	// in the meanwhile): while writes in such a situation will degrade the
	// effectiveness of the per-P table, they don't pose correctness issues
	// because stringIsInterned needs to check the strings for equality.
	// Additionally, since it is possible for the interning table to change
	// size, we simply skip updating the table if it's clear the size of the
	// table has changed.
	procPin()
	_p_ := getg().m.p.ptr()
	for i, _idx := 0, idx; i < strInternProbes; i, _idx = i+1, hashNext(_idx, off, N) {
		if _idx >= uintptr(len(_p_.strInternTable)) {
			// the size of the table has clearly changed: skip updating it
			goto unpin
		}
		if _p_.strInternTable[_idx] == "" {
			_p_.strInternTable[_idx] = s
			_p_.strInternCount++
			goto unpin
		}
	}
	_p_.strInternTable[idx] = s
	_p_.strInternEvicts++
unpin:
	procUnpin()
}

func interntmp(b []byte) string {
	s, idx, off, N := stringIsInterned(slicebytetostringtmp(b))
	if s == "" {
		var nb []byte
		s, nb = rawstring(len(b))
		copy(nb, b)
		internString(s, idx, off, N)
	}
	return s
}

func interningAllowed(l int) bool {
	if debug.stringintern == 0 {
		return false
	}
	if l > strInternMaxLen || l <= 0 {
		return false
	}
	if _p_ := getg().m.p.ptr(); _p_.strInternHits < _p_.strInternEvicts {
		return false
	}
	return true
}

func hashReduce(h uintptr, N uintptr) (idx, off uintptr) {
	//h = (h * 11400714819323198485) ^ ((h >> 32) * 11400714819323198485)
	off = ((h / N) % N) | 1
	//off = uintptr(fastreduce(h, uint32(N))) | 1
	idx = hashNext(h, off, N)
	return
}

func hashNext(idx, off, N uintptr) uintptr {
	return (idx + off) % N
	//return uintptr(fastreduce(idx+off, uint32(N)))
}

func fastreduce(h uintptr, N uint32) uint32 {
	_h := (uint64(h) * 11400714819323198485) >> 32
	return uint32((_h * uint64(N)) >> 32)
}

// This function is called with the world stopped, at the beginning of a garbage collection.
// It must not allocate and probably should not call any runtime functions.
//go:nosplit
func clearinterningtables() {
	var hits, evicts, count uint

	for _, p := range allp {
		evicts += p.strInternEvicts
		hits += p.strInternHits
		count += p.strInternCount
		p.strInternTable = nil
		p.strInternSeed = internseed()
		p.strInternEvicts = 0
		p.strInternHits = 0
		p.strInternCount = 0
	}

	if debug.gctrace > 0 {
		l := uint(len(allp)) * interntablesize()
		printlock()
		print("intern")
		if evicts > 0 {
			print(" hit/evict=", 100*hits/evicts, "%")
		}
		print(" count/len=", 100*count/l, "%")
		print(" hits=", hits)
		print(" evicts=", evicts)
		print(" count=", count)
		print(" len=", l)
		printnl()
		printunlock()
	}

	if evicts > hits/100 {
		strInternTableLen++
	} else if count < uint(interntablesize()/10) {
		strInternTableLen--
	}
	strInternTableLen = clamp(strInternTableLen, strInternTableMinLen, strInternTableMaxLen)
}

func internmem() uint64 {
	c := uint64(interntablesize())
	lock(&allpLock)
	np := uint64(len(allp))
	unlock(&allpLock)
	return c * np * uint64(unsafe.Sizeof(stringStruct{}))
}

func interntablesize() uint {
	return 1 << uint(strInternTableLen)
}

func clamp(n, min, max int) int {
	if min > max {
		panic("clamp: min greater than max")
	} else if n < min {
		n = min
	} else if n > max {
		n = max
	}
	return n
}

func internseed() uintptr {
	if unsafe.Sizeof(uintptr(0)) <= unsafe.Sizeof(uint32(0)) {
		return uintptr(fastrand())
	}
	return (uintptr(fastrand()) << 32) | uintptr(fastrand())
}
