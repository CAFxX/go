// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build amd64

package runtime_test

import (
	"fmt"
	"testing"
	"unsafe"
)

var memmoveAMD64BenchmarkSink byte
var memmoveAMD64BenchmarkBufferSink []byte

// BenchmarkMemmoveAMD64FixedSizes is the fixed-size baseline. The length is
// passed through the benchmark closure so the measured copy uses the dynamic
// size path rather than a source-level fixed-size struct assignment.
func BenchmarkMemmoveAMD64FixedSizes(b *testing.B) {
	sizes := []int{
		0, 1, 2, 3, 4, 7, 8, 9, 15, 16, 17, 31, 32, 33, 63, 64, 65,
		127, 128, 129, 255, 256, 257, 511, 512, 513, 1023, 1024, 1025,
		2047, 2048, 2049, 4095, 4096, 4097, 8191, 8192, 8193,
		16 << 10, 64 << 10, 256 << 10, 1 << 20, 4 << 20, 16 << 20,
		64 << 20,
	}
	for _, n := range sizes {
		n := n
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			src := make([]byte, n+64)
			dst := make([]byte, n+128)
			for i := range src {
				src[i] = byte(i*131 + i/17)
			}
			b.SetBytes(int64(n))
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				copy(dst[63:63+n], src[1:1+n])
			}
			b.StopTimer()
			if n != 0 {
				memmoveAMD64BenchmarkSink ^= dst[63+n-1]
			}
			memmoveAMD64BenchmarkBufferSink = dst
		})
	}
}

// BenchmarkMemmoveAMD64Latency models a copy followed immediately by a load
// from the copied region. BenchmarkMemmoveAMD64Throughput issues independent
// copies before consuming their results, exposing more memory-level parallelism.
type memmoveAMD64Header struct {
	ptr unsafe.Pointer
	len uintptr
	cap uintptr
}

//go:noinline
func memmoveAMD64ConsumeHeader(h *memmoveAMD64Header) uintptr {
	x := h.len + h.cap
	if h.ptr != nil {
		x++
	}
	return x
}

// BenchmarkMemmoveAMD64HeaderReadAfterCopy models a pointer/len/cap header
// being copied into stack storage and immediately consumed. The noinline
// consumer forces real destination loads without making the header escape.
func BenchmarkMemmoveAMD64HeaderReadAfterCopy(b *testing.B) {
	backing := make([]byte, 64)
	src := memmoveAMD64Header{ptr: unsafe.Pointer(&backing[0]), len: 17, cap: 64}
	var dst memmoveAMD64Header
	var sink uintptr
	b.SetBytes(int64(unsafe.Sizeof(src)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dst = src
		sink += memmoveAMD64ConsumeHeader(&dst)
	}
	b.StopTimer()
	memmoveAMD64HeaderBenchmarkSink ^= sink
}

var memmoveAMD64HeaderBenchmarkSink uintptr

func BenchmarkMemmoveAMD64Latency(b *testing.B) {
	for _, n := range []int{8, 16, 32, 64, 128, 256} {
		n := n
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			src := make([]byte, n)
			dst := make([]byte, n)
			for i := range src {
				src[i] = byte(i*31 + 7)
			}
			var sink byte
			b.SetBytes(int64(n))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				copy(dst, src)
				sink ^= dst[i&(n-1)]
			}
			b.StopTimer()
			memmoveAMD64BenchmarkSink ^= sink
			memmoveAMD64BenchmarkBufferSink = dst
		})
	}
}

func BenchmarkMemmoveAMD64Throughput(b *testing.B) {
	for _, n := range []int{16, 64, 256, 4096} {
		n := n
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			const independentCopies = 8
			src := make([]byte, independentCopies*n)
			dst := make([]byte, independentCopies*n)
			for i := range src {
				src[i] = byte(i*31 + 7)
			}
			b.SetBytes(int64(independentCopies * n))
			b.ResetTimer()
			var sink byte
			for i := 0; i < b.N; i++ {
				for j := 0; j < independentCopies; j++ {
					off := j * n
					copy(dst[off:off+n], src[off:off+n])
				}
				for j := 0; j < independentCopies; j++ {
					sink ^= dst[(j+1)*n-1]
				}
			}
			b.StopTimer()
			memmoveAMD64BenchmarkSink ^= sink
			memmoveAMD64BenchmarkBufferSink = dst
		})
	}
}

// BenchmarkMemmoveAMD64AlignmentSweep rotates through all source and
// destination 64-byte alignment residues for each representative copy size.
func BenchmarkMemmoveAMD64AlignmentSweep(b *testing.B) {
	for _, n := range []int{8, 16, 32, 64, 128, 256, 4096, 65536} {
		n := n
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			src := make([]byte, n+64)
			dst := make([]byte, n+64)
			for i := range src {
				src[i] = byte(i*31 + 7)
			}
			b.SetBytes(int64(n))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				srcOff := i & 63
				dstOff := (i*29 + 11) & 63
				copy(dst[dstOff:dstOff+n], src[srcOff:srcOff+n])
			}
			b.StopTimer()
			if b.N > 0 {
				lastDstOff := (((b.N - 1) * 29) + 11) & 63
				memmoveAMD64BenchmarkSink ^= dst[lastDstOff+n/2]
			}
			memmoveAMD64BenchmarkBufferSink = dst
		})
	}
}

// BenchmarkMemmoveAMD64SizeEntropy separates monomorphic, patterned,
// intra-size-bucket-random, and cross-bucket-random dynamic lengths.
func BenchmarkMemmoveAMD64SizeEntropy(b *testing.B) {
	const streamLen = 256
	pattern := []int{8, 16, 32, 64, 128, 256, 512, 1024}
	streams := []struct {
		name  string
		sizes []int
	}{
		{name: "monomorphic", sizes: make([]int, streamLen)},
		{name: "patterned", sizes: make([]int, streamLen)},
		{name: "small-random", sizes: make([]int, streamLen)},
		{name: "medium-random", sizes: make([]int, streamLen)},
		{name: "intra-bucket-random", sizes: make([]int, streamLen)},
		{name: "cross-bucket-random", sizes: make([]int, streamLen)},
	}
	for i := 0; i < streamLen; i++ {
		streams[0].sizes[i] = 256
		streams[1].sizes[i] = pattern[i%len(pattern)]
	}
	state := uint32(0x9e3779b9)
	for i := 0; i < streamLen; i++ {
		state ^= state << 13
		state ^= state >> 17
		state ^= state << 5
		streams[2].sizes[i] = int(state % 129)
		streams[3].sizes[i] = 128 + int(state%3969)
		streams[4].sizes[i] = 256 + int(state&255)
		streams[5].sizes[i] = int(state & 65535)
	}

	for _, stream := range streams {
		stream := stream
		b.Run(stream.name, func(b *testing.B) {
			maxSize, total := 0, 0
			for _, n := range stream.sizes {
				if n > maxSize {
					maxSize = n
				}
				total += n
			}
			src := make([]byte, maxSize+64)
			dst := make([]byte, maxSize+64)
			for i := range src {
				src[i] = byte(i*37 + 11)
			}
			avgSize := total / len(stream.sizes)
			b.SetBytes(int64(avgSize))
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				n := stream.sizes[i&(streamLen-1)]
				srcOff := (i * 13) & 63
				dstOff := (i*29 + 7) & 63
				copy(dst[dstOff:dstOff+n], src[srcOff:srcOff+n])
			}
			b.StopTimer()
			if b.N > 0 {
				lastDstOff := (((b.N - 1) * 29) + 7) & 63
				lastSrcOff := ((b.N - 1) * 13) & 63
				lastSize := stream.sizes[(b.N-1)&(streamLen-1)]
				if lastSize != 0 {
					memmoveAMD64BenchmarkSink ^= dst[lastDstOff+lastSize-1] ^ src[lastSrcOff]
				}
			}
			memmoveAMD64BenchmarkBufferSink = dst
		})
	}
}

// BenchmarkMemmoveAMD64OverlapAnd4KAlias measures both overlap directions
// and address deltas around 4 KiB, including the exact 4 KiB alias case.
func BenchmarkMemmoveAMD64OverlapAnd4KAlias(b *testing.B) {
	deltas := []int{-4097, -4096, -4095, -64, -8, -1, 0, 1, 8, 64, 4095, 4096, 4097}
	for _, n := range []int{1024, 4096, 8192} {
		n := n
		for _, delta := range deltas {
			delta := delta
			b.Run(fmt.Sprintf("n_%d/delta_%+d", n, delta), func(b *testing.B) {
				absDelta := delta
				if absDelta < 0 {
					absDelta = -absDelta
				}
				buf := make([]byte, n+absDelta+128)
				for i := range buf {
					buf[i] = byte(i*37 + 11)
				}
				srcOff, dstOff := 64, 64+delta
				if delta < 0 {
					srcOff, dstOff = 64+absDelta, 64
				}
				b.SetBytes(int64(n))
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					copy(buf[dstOff:dstOff+n], buf[srcOff:srcOff+n])
				}
				b.StopTimer()
				memmoveAMD64BenchmarkSink ^= buf[dstOff+n-1]
				memmoveAMD64BenchmarkBufferSink = buf
			})
		}
	}
}

// BenchmarkMemmoveAMD64WorkingSet sweeps the bytes resident in each source
// and destination allocation. The combined active footprint is twice n.
func BenchmarkMemmoveAMD64WorkingSet(b *testing.B) {
	for _, n := range []int{
		2 << 10, 4 << 10, 8 << 10, 16 << 10, 24 << 10, 32 << 10, 48 << 10, 64 << 10, 96 << 10,
		128 << 10, 192 << 10, 256 << 10, 384 << 10, 512 << 10, 768 << 10,
		1 << 20, 2 << 20, 3 << 20, 4 << 20, 6 << 20, 8 << 20, 12 << 20,
		16 << 20, 24 << 20, 32 << 20, 48 << 20, 64 << 20,
	} {
		n := n
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			src := make([]byte, n)
			dst := make([]byte, n)
			for i := range src {
				src[i] = byte(i*131 + i/17)
			}
			b.SetBytes(int64(n))
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				copy(dst, src)
			}
			b.StopTimer()
			memmoveAMD64BenchmarkSink ^= dst[len(dst)-1]
			memmoveAMD64BenchmarkBufferSink = dst
		})
	}
}