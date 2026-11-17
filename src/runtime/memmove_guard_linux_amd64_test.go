// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build linux && amd64

package runtime_test

import (
	"bytes"
	"syscall"
	"testing"
)

func memmoveAMD64GuardedBuffer(t *testing.T, size int) []byte {
	t.Helper()
	page := syscall.Getpagesize()
	mapping, err := syscall.Mmap(-1, 0, size+2*page, syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_PRIVATE|syscall.MAP_ANON)
	if err != nil {
		t.Fatalf("Mmap: %v", err)
	}
	t.Cleanup(func() {
		if err := syscall.Munmap(mapping); err != nil {
			t.Errorf("Munmap: %v", err)
		}
	})
	if err := syscall.Mprotect(mapping[:page], syscall.PROT_NONE); err != nil {
		t.Fatalf("Mprotect leading guard page: %v", err)
	}
	if err := syscall.Mprotect(mapping[len(mapping)-page:], syscall.PROT_NONE); err != nil {
		t.Fatalf("Mprotect trailing guard page: %v", err)
	}
	return mapping[page : len(mapping)-page]
}

// TestMemmoveAMD64GuardPages places source and destination ranges directly
// against protected pages. Any access before or after the requested range
// faults instead of silently touching adjacent mapped memory.
func TestMemmoveAMD64GuardPages(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping memmove guard-page tests in short mode")
	}
	memmoveAMD64SkipSanitizers(t)

	src := memmoveAMD64GuardedBuffer(t, memmoveAMD64MaxTestSize)
	dst := memmoveAMD64GuardedBuffer(t, memmoveAMD64MaxTestSize)
	for i := range src {
		src[i] = byte(i*131 + i/17)
	}

	// Exhaust every small length at both protected boundaries and in both
	// source/destination boundary orientations.
	for n := 0; n <= 8192; n++ {
		copy(dst[:n], src[:n])
		if !bytes.Equal(dst[:n], src[:n]) {
			t.Fatalf("copy mismatch at leading guard: n=%d", n)
		}
		copy(dst[len(dst)-n:], src[len(src)-n:])
		if !bytes.Equal(dst[len(dst)-n:], src[len(src)-n:]) {
			t.Fatalf("copy mismatch at trailing guard: n=%d", n)
		}
		copy(dst[len(dst)-n:], src[:n])
		if !bytes.Equal(dst[len(dst)-n:], src[:n]) {
			t.Fatalf("copy mismatch at mixed guards (source first): n=%d", n)
		}
		copy(dst[:n], src[len(src)-n:])
		if !bytes.Equal(dst[:n], src[len(src)-n:]) {
			t.Fatalf("copy mismatch at mixed guards (destination first): n=%d", n)
		}
	}

	for _, n := range memmoveAMD64BoundarySizes() {
		if n <= 8192 {
			continue
		}
		// Both ranges begin immediately after a protected page.
		copy(dst[:n], src[:n])
		if !bytes.Equal(dst[:n], src[:n]) {
			t.Fatalf("copy mismatch at leading guard: n=%d", n)
		}

		// Both ranges end immediately before a protected page.
		copy(dst[len(dst)-n:], src[len(src)-n:])
		if !bytes.Equal(dst[len(dst)-n:], src[len(src)-n:]) {
			t.Fatalf("copy mismatch at trailing guard: n=%d", n)
		}

		// Exercise a source at the leading guard and a destination at the
		// trailing guard, then reverse the placement.
		copy(dst[len(dst)-n:], src[:n])
		if !bytes.Equal(dst[len(dst)-n:], src[:n]) {
			t.Fatalf("copy mismatch at mixed guards (source first): n=%d", n)
		}
		copy(dst[:n], src[len(src)-n:])
		if !bytes.Equal(dst[:n], src[len(src)-n:]) {
			t.Fatalf("copy mismatch at mixed guards (destination first): n=%d", n)
		}
	}
}