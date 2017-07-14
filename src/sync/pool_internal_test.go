// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sync

import "unsafe"

// PoolLocalSize is the size of poolLocal: it's defined here because tests in
// pool_test.go are in a different package and therefore can't access
// poolLocal{}.
//
// Note that this constant is only defined during testing.
const PoolLocalSize = unsafe.Sizeof(poolLocal{})
