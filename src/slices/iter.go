// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package slices

import (
	"cmp"
	"iter"
	"unsafe"
)

// All returns an iterator over index-value pairs in the slice
// in the usual order.
func All[Slice ~[]E, E any](s Slice) iter.Seq2[int, E] {
	return func(yield func(int, E) bool) {
		for i, v := range s {
			if !yield(i, v) {
				return
			}
		}
	}
}

// Backward returns an iterator over index-value pairs in the slice,
// traversing it backward with descending indices.
func Backward[Slice ~[]E, E any](s Slice) iter.Seq2[int, E] {
	return func(yield func(int, E) bool) {
		for i := len(s) - 1; i >= 0; i-- {
			if !yield(i, s[i]) {
				return
			}
		}
	}
}

// Values returns an iterator that yields the slice elements in order.
func Values[Slice ~[]E, E any](s Slice) iter.Seq[E] {
	return func(yield func(E) bool) {
		for _, v := range s {
			if !yield(v) {
				return
			}
		}
	}
}

const oldimpl = false

// AppendSeq appends the values from seq to the slice and
// returns the extended slice.
// If seq is empty, the result preserves the nilness of s.
func AppendSeq[Slice ~[]E, E any](s Slice, seq iter.Seq[E]) Slice {
	if oldimpl {
		for v := range seq {
			s = append(s, v)
		}
		return s
	}

	var seg []Slice
	if cap(s) != 0 {
		seg = append(seg, s)
	}
	cnt := len(s)

	singleuse(seq)(func(v E) bool {
		cnt++
		if len(seg) == 0 || len(seg[len(seg)-1]) == cap(seg[len(seg)-1]) {
			seg = append(seg, Grow(Slice(nil), max(4, cnt*2)))
		}
		seg[len(seg)-1] = append(seg[len(seg)-1], v)
		return true
	})

	// Fast path: empty iter.Seq or single segment.
	if len(seg) == 0 {
		return s // preserve nilness
	} else if len(seg) == 1 {
		return seg[0] // already has all elements
	}

	// Fast path: if the last segment has enough capacity to hold all
	// elements, we reuse it and avoid an additional allocation.
	if last := seg[len(seg)-1]; cap(last) >= cnt {
		l := len(last)
		last = last[:cnt]
		copy(last[cnt-l:], last[:l])
		var start int
		for _, s := range seg[:len(seg)-1] {
			start += copy(last[start:], s)
		}
		return last
	}

	// General case: allocate a new slice and copy all segments into it.
	rs := Grow(Slice(nil), cnt)
	for _, s := range seg {
		rs = append(rs, s...)
	}
	return rs
}

func noescape[T any](p T) T {
	x := uintptr(unsafe.Pointer(&p))
	return *(*T)(unsafe.Pointer(x ^ 0))
}

func singleuse[E any](seq iter.Seq[E]) iter.Seq[E] {
	return func(yield func(E) bool) {
		if seq == nil {
			return
		}
		_seq := seq
		seq = nil
		_seq(noescape(yield))
	}
}

// Collect collects values from seq into a new slice and returns it.
// If seq is empty, the result is nil.
func Collect[E any](seq iter.Seq[E]) []E {
	return AppendSeq([]E(nil), seq)
}

// Sorted collects values from seq into a new slice, sorts the slice,
// and returns it.
// If seq is empty, the result is nil.
func Sorted[E cmp.Ordered](seq iter.Seq[E]) []E {
	s := Collect(seq)
	Sort(s)
	return s
}

// SortedFunc collects values from seq into a new slice, sorts the slice
// using the comparison function, and returns it.
// If seq is empty, the result is nil.
func SortedFunc[E any](seq iter.Seq[E], cmp func(E, E) int) []E {
	s := Collect(seq)
	SortFunc(s, cmp)
	return s
}

// SortedStableFunc collects values from seq into a new slice.
// It then sorts the slice while keeping the original order of equal elements,
// using the comparison function to compare elements.
// It returns the new slice.
// If seq is empty, the result is nil.
func SortedStableFunc[E any](seq iter.Seq[E], cmp func(E, E) int) []E {
	s := Collect(seq)
	SortStableFunc(s, cmp)
	return s
}

// Chunk returns an iterator over consecutive sub-slices of up to n elements of s.
// All but the last sub-slice will have size n.
// All sub-slices are clipped to have no capacity beyond the length.
// If s is empty, the sequence is empty: there is no empty slice in the sequence.
// Chunk panics if n is less than 1.
func Chunk[Slice ~[]E, E any](s Slice, n int) iter.Seq[Slice] {
	if n < 1 {
		panic("cannot be less than 1")
	}

	return func(yield func(Slice) bool) {
		for i := 0; i < len(s); i += n {
			// Clamp the last chunk to the slice bound as necessary.
			end := min(n, len(s[i:]))

			// Set the capacity of each chunk so that appending to a chunk does
			// not modify the original slice.
			if !yield(s[i : i+end : i+end]) {
				return
			}
		}
	}
}
