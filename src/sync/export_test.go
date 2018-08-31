// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sync

import "sync/atomic"

// Export for testing.
var Runtime_Semacquire = runtime_Semacquire
var Runtime_Semrelease = runtime_Semrelease
var Runtime_procPin = runtime_procPin
var Runtime_procUnpin = runtime_procUnpin

func (p *Pool) Shards() (total, empty int) {
        s := atomic.LoadUintptr(&p.localSize) // load-acquire
        l := p.local                             // load-consume
	for i := 0; i < int(s); i++ {
		pl := indexLocal(l, i)
		if pl.private == nil && len(pl.shared) == 0 {
			empty++
		} 
	}
	total = int(s)
	return
}
