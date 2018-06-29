// asmcheck

// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package codegen

// Check small call to memequal are replaced with moves+cmp.

type BoxType1 [1]byte
type BoxType2 [2]byte
type BoxType4 [4]byte
type BoxType8 [8]byte

func (t BoxType1) memeqsmall(s string) bool {
	// 386:-".*memequal"
	// amd64:-".*memequal"
	// arm64:-".*memequal"
	// s390x:-".*memequal"
	return string(t[:]) == s
}

func (t BoxType2) memeqsmall(s string) bool {
	// 386:-".*memequal"
	// amd64:-".*memequal"
	// arm64:-".*memequal"
	// s390x:-".*memequal"
	return string(t[:]) == s
}

func (t BoxType4) memeqsmall(s string) bool {
	// 386:-".*memequal"
	// amd64:-".*memequal"
	// arm64:-".*memequal"
	// s390x:-".*memequal"
	return string(t[:]) == s
}

func (t BoxType8) memeqsmall(s string) bool {
	// amd64:-".*memequal"
	// arm64:-".*memequal"
	// s390x:-".*memequal"
	return string(t[:]) == s
}
