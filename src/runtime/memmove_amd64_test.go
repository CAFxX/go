// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build amd64

package runtime_test

import (
	"bytes"
	"internal/asan"
	"internal/msan"
	"internal/race"
	. "runtime"
	"sync/atomic"
	"testing"
	"unsafe"
)

const memmoveAMD64MaxTestSize = 64 << 20

func memmoveAMD64SkipSanitizers(t *testing.T) {
	t.Helper()
	if race.Enabled || asan.Enabled || msan.Enabled {
		t.Skip("skipping large memmove sweep under sanitizers")
	}
}

// TestMemmoveAMD64ZeroLengthNil verifies that a zero-length copy does not
// dereference either pointer, including when both are nil.
func TestMemmoveAMD64ZeroLengthNil(t *testing.T) {
	var b byte
	Memmove(nil, nil, 0)
	Memmove(unsafe.Pointer(&b), nil, 0)
	Memmove(nil, unsafe.Pointer(&b), 0)
}

// TestMemmoveAMD64WordAtomicity changes the source word concurrently while
// repeatedly copying it. A split source load could combine halves from
// different values. The test is intentionally racy and is disabled under
// the race detector; the destination is read with an atomic load.
func TestMemmoveAMD64WordAtomicity(t *testing.T) {
	if race.Enabled {
		t.Skip("skip intentional source race under the race detector")
	}

	const (
		valueA = uint64(0xaaaaaaaa55555555)
		valueB = uint64(0x55555555aaaaaaaa)
	)
	var src, dst uint64
	atomic.StoreUint64(&src, valueA)
	atomic.StoreUint64(&dst, valueA)

	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			select {
			case <-stop:
				return
			default:
			}
			atomic.StoreUint64(&src, valueA)
			atomic.StoreUint64(&src, valueB)
		}
	}()
	defer func() {
		close(stop)
		<-done
	}()

	for i := 0; i < 1<<18; i++ {
		Memmove(unsafe.Pointer(&dst), unsafe.Pointer(&src), unsafe.Sizeof(src))
		got := atomic.LoadUint64(&dst)
		if got != valueA && got != valueB {
			t.Fatalf("copied a torn word: got %#016x, want %#016x or %#016x", got, valueA, valueB)
		}
		if i&255 == 0 {
			Gosched()
		}
	}
}

// memmoveAMD64BoundarySizes returns sizes immediately below, at, and above
// powers-of-two boundaries used by common copy implementations, through
// the largest size exercised by this test suite.
func memmoveAMD64BoundarySizes() []int {
	boundaries := []int{
		16, 32, 64, 128, 256, 512, 1024, 2048, 4096, 8192,
		16 << 10, 32 << 10, 64 << 10, 128 << 10, 256 << 10,
		512 << 10, 1 << 20, 2 << 20, 4 << 20, 8 << 20,
		16 << 20, 32 << 20, memmoveAMD64MaxTestSize,
	}
	sizes := []int{0, 1, 2, 3, 7, 8, 9}
	seen := make(map[int]bool, len(boundaries)*3+len(sizes))
	for _, n := range sizes {
		seen[n] = true
	}
	for _, boundary := range boundaries {
		for _, delta := range []int{-1, 0, 1} {
			n := boundary + delta
			if n >= 0 && n <= memmoveAMD64MaxTestSize && !seen[n] {
				sizes = append(sizes, n)
				seen[n] = true
			}
		}
	}
	return sizes
}

// checkMemmoveAMD64Disjoint checks a disjoint copy and places canaries around
// the destination. Separate guard-page tests check the actual memory boundary.
func checkMemmoveAMD64Disjoint(t *testing.T, dstBuf, srcBuf []byte, dstOff, srcOff, n int) {
	t.Helper()
	const canary = byte(0xa7)
	const guard = 64

	left := dstOff - guard
	if left < 0 {
		left = 0
	}
	for i := left; i < dstOff; i++ {
		dstBuf[i] = canary
	}
	right := dstOff + n + guard
	if right > len(dstBuf) {
		right = len(dstBuf)
	}
	for i := dstOff + n; i < right; i++ {
		dstBuf[i] = canary
	}

	src := srcBuf[srcOff : srcOff+n]
	dst := dstBuf[dstOff : dstOff+n]
	copy(dst, src)
	if !bytes.Equal(dst, src) {
		t.Fatalf("copy mismatch: n=%d srcOff=%d dstOff=%d", n, srcOff, dstOff)
	}
	for i := left; i < dstOff; i++ {
		if dstBuf[i] != canary {
			t.Fatalf("copy wrote before destination: n=%d srcOff=%d dstOff=%d at %d", n, srcOff, dstOff, i)
		}
	}
	for i := dstOff + n; i < right; i++ {
		if dstBuf[i] != canary {
			t.Fatalf("copy wrote after destination: n=%d srcOff=%d dstOff=%d at %d", n, srcOff, dstOff, i)
		}
	}
}

// TestMemmoveAMD64SizeAlignmentSweep covers every size through 8192 bytes.
// For each size, both source and destination visit all 64-byte alignment
// residues. Selected boundary sizes additionally cover every alignment pair.
func TestMemmoveAMD64SizeAlignmentSweep(t *testing.T) {
	memmoveAMD64SkipSanitizers(t)
	if testing.Short() {
		t.Skip("skipping exhaustive memmove alignment sweep in short mode")
	}

	const maxSize = 8192
	src := make([]byte, maxSize+64)
	dst := make([]byte, maxSize+128)
	for i := range src {
		src[i] = byte(i*131 + i/17)
	}

	for n := 0; n <= maxSize; n++ {
		for residue := 0; residue < 64; residue++ {
			srcOff := residue
			dstOff := (residue*29 + n*17) & 63
			checkMemmoveAMD64Disjoint(t, dst, src, dstOff, srcOff, n)
		}
	}

	// The full 64 x 64 alignment cross-product is deliberately restricted
	// to representative sizes so the normal runtime test suite stays bounded.
	allPairsSizes := []int{
		0, 1, 2, 3, 4, 7, 8, 9, 15, 16, 17, 31, 32, 33,
		63, 64, 65, 127, 128, 129, 255, 256, 257, 511, 512,
		513, 1023, 1024, 1025, 2047, 2048, 2049, 4095, 4096,
		4097, 8191, 8192,
	}
	for _, n := range allPairsSizes {
		for srcOff := 0; srcOff < 64; srcOff++ {
			for dstOff := 0; dstOff < 64; dstOff++ {
				checkMemmoveAMD64Disjoint(t, dst, src, dstOff, srcOff, n)
			}
		}
	}
}

// TestMemmoveAMD64LargeBoundaries covers the large-copy cutovers and
// power-of-two boundaries up to 64 MiB at several independent alignments.
func TestMemmoveAMD64LargeBoundaries(t *testing.T) {
	memmoveAMD64SkipSanitizers(t)
	if testing.Short() {
		t.Skip("skipping large memmove boundary tests in short mode")
	}

	src := make([]byte, memmoveAMD64MaxTestSize+64)
	dst := make([]byte, memmoveAMD64MaxTestSize+128)
	for i := range src {
		src[i] = byte(i*131 + i/17)
	}

	for i, n := range memmoveAMD64BoundarySizes() {
		for j := 0; j < 3; j++ {
			srcOff := (i*13 + j*19) & 63
			dstOff := (i*31 + j*7 + 1) & 63
			checkMemmoveAMD64Disjoint(t, dst, src, dstOff, srcOff, n)
		}
	}
}

// TestMemmoveAMD64OverlapDistanceSweep checks both overlap directions for
// every byte distance 1..4096, plus the next 63 distances that exercise
// 4 KiB aliases with small nonzero page offsets.
func TestMemmoveAMD64OverlapDistanceSweep(t *testing.T) {
	memmoveAMD64SkipSanitizers(t)
	if testing.Short() {
		t.Skip("skipping exhaustive memmove overlap sweep in short mode")
	}

	const (
		copySize = 8192
		base     = 64
		maxDelta = 4096 + 63
	)
	buf := make([]byte, base+maxDelta+copySize+64)
	want := make([]byte, len(buf))

	for delta := 1; delta <= maxDelta; delta++ {
		for _, dstAfterSrc := range []bool{true, false} {
			for i := range buf {
				buf[i] = byte(i*37 + delta*11)
			}
			copy(want, buf)

			srcOff, dstOff := base, base+delta
			if !dstAfterSrc {
				srcOff, dstOff = base+delta, base
			}

			// Use a byte-wise reference in the direction that preserves the
			// original source when the ranges overlap.
			if dstOff > srcOff {
				for i := copySize - 1; i >= 0; i-- {
					want[dstOff+i] = want[srcOff+i]
				}
			} else {
				for i := 0; i < copySize; i++ {
					want[dstOff+i] = want[srcOff+i]
				}
			}

			copy(buf[dstOff:dstOff+copySize], buf[srcOff:srcOff+copySize])
			if !bytes.Equal(buf, want) {
				t.Fatalf("overlap copy mismatch: delta=%d dstAfterSrc=%t", delta, dstAfterSrc)
			}
		}
	}
}