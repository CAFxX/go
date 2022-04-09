package intern_test

import (
	"internal/intern"
	"strconv"
	"testing"
)

func BenchmarkStrings(b *testing.B) {
	var s string
	for i := 0; i < b.N; i++ {
		s = intern.String("hello world")
	}
	_ = s
}

func BenchmarkBytesCommon(b *testing.B) {
	var buf [64]byte
	var s string
	for i := 0; i < b.N; i++ {
		_buf := strconv.AppendInt(buf[:0], int64(1_000_000_000+(i%100)), 10)
		s = intern.Bytes(_buf)
	}
	_ = s
}

func BenchmarkBytesUncommon(b *testing.B) {
	var buf [64]byte
	var s string
	for i := 0; i < b.N; i++ {
		_buf := strconv.AppendInt(buf[:0], int64(1_000_000_000+(i%1_000_000_000)), 10)
		s = intern.Bytes(_buf)
	}
	_ = s
}

func BenchmarkBytesNoIntern(b *testing.B) {
	var buf [64]byte
	var s string
	for i := 0; i < b.N; i++ {
		_buf := strconv.AppendInt(buf[:0], int64(1_000_000_000+(i%1_000_000_000)), 10)
		s = string(_buf)
	}
	_ = s
}
