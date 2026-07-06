// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package maps

import (
	"internal/abi"
	"internal/goarch"
	"internal/runtime/atomic"
	"unsafe"
)

// The background idle-map shrinker lives in the runtime (runtime/mgcmapshrink.go),
// which owns the queue, the worker goroutine and the GC hooks. This file is the
// maps-side implementation of the pipeline steps the runtime drives.
//
// A map that drains (or is cleared) and then goes idle keeps its oversized
// tables. The runtime enqueues such a map (see the delete and Clear paths)
// and, once it has been idle for several GC cycles, shrinks it through a
// pipeline that bounds the stop-the-world work per map:
//
//	worker (concurrent): snapshot the live entries of oversized tables
//	STW N:               validate the snapshot against writeSeq (O(1))
//	worker (concurrent): re-hash the snapshot into right-sized tables
//	STW N+1:             revalidate (O(1)) and publish the pre-built directory
//
// Validation must happen inside a stop-the-world: mutators update writeSeq
// with plain stores, so no concurrent read of it can prove idleness (see the
// runtime file's comment for the full memory-model argument).
//
// The snapshot runs concurrently with mutators that may be writing the map, so
// it must be crash-safe under races even though its output may be garbage
// (garbage is discarded by the STW validation). Crash safety rests on three
// properties:
//
//  1. Everything the worker holds stays alive. The GC is precise and
//     non-moving, so a pointer loaded into a GC-visible slot (a typed local or
//     field) pins its referent, and holding a groups array (a heap-typed
//     object) transitively pins the keys and elements it references.
//
//  2. Pointer loads and copies never tear. Aligned pointer-sized loads are
//     atomic on all supported platforms, and typedmemmove copies pointer words
//     indivisibly, so a pointer read from a live, properly typed allocation is
//     always a valid-or-nil pointer, never a blend of two.
//
//  3. Bounds never leave the allocation. A racing grow can make pairs like
//     (dirPtr, dirLen) or (groups.data, lengthMask) mutually inconsistent, and
//     wider-than-pointer fields (used, lengthMask on 32-bit) can tear
//     outright, so every iteration bound is clamped to the referenced
//     allocation's actual size (heapObjSize), cross-checked against the
//     table's capacity, and the snapshot fails softly if a clamp binds or a
//     buffer would overflow. In the idle case the clamps never bind; in the
//     racing case reads stay in-bounds (safe, possibly meaningless) and
//     validation discards.
//
// The snapshot buffers are typed arrays of the map's key and element types, so
// the GC scans them: the copied entries keep their referents alive between the
// copy and the adoption of the rebuilt tables, no matter what happens to the
// source map in the meantime.

// bgShrinkEnabled mirrors GODEBUG=mapbgshrink and is set by the runtime in
// parsedebugvars. It gates whether drained maps are enqueued for background
// shrinking.
//
//go:linknamestd bgShrinkEnabled
var bgShrinkEnabled bool

// enqueueShrink adds m (of type typ) to the runtime's background shrink queue,
// recording seq as the write-sequence snapshot. Implemented in the runtime and
// pushed here (see runtime.maps_enqueueShrink).
//
//go:linknamestd enqueueShrink
func enqueueShrink(m, typ unsafe.Pointer, seq uint32)

// heapObjSize reports the number of bytes from p to the end of p's heap
// allocation, or 0 if p is not a heap pointer. Implemented in the runtime and
// pushed here (see runtime.maps_heapObjSize).
//
//go:linknamestd heapObjSize
func heapObjSize(p unsafe.Pointer) uintptr

// shrinkReadSeq returns m.writeSeq with an atomic load. The background worker
// and the stop-the-world hooks use it to tell whether a map has been (or is
// being) written.
//
//go:linknamestd shrinkReadSeq
func shrinkReadSeq(m unsafe.Pointer) uint32 {
	return atomic.Load(&(*Map)(m).writeSeq)
}

// shrinkClearEnqueued clears m.enqueued so a later delete can re-enqueue the
// map. Called only from the mark-termination stop-the-world (for a map that
// turned out to need no shrinking), where it cannot race the delete path that
// sets the bit.
//
//go:linknamestd shrinkClearEnqueued
func shrinkClearEnqueued(m unsafe.Pointer) {
	(*Map)(m).enqueued = false
}

// shrinkTab records one oversized table's snapshot within a shrinkHandle: the
// source table (to confirm it is still installed at commit), its groups as
// captured and bounds-validated by the sizing pass (the copy pass reads only
// these, so it cannot race a mutator into different values), where its live
// entries sit in the shared key/elem buffers, and later the rebuilt
// replacement.
type shrinkTab struct {
	old        *table
	nt         *table
	groups     groupsReference
	index      int
	localDepth uint8
	floor      uint8
	oldCap     uint16
	start      int
	n          int
}

// shrinkHandle is the maps-side snapshot handle. The runtime holds it as an
// unsafe.Pointer in its job (a pointer field, so it and everything it
// references stay live). keys and elems are GC-scanned arrays of the map's key
// and elem types.
type shrinkHandle struct {
	seed  uintptr
	keys  unsafe.Pointer // *[cap]Key
	elems unsafe.Pointer // *[cap]Elem
	cap   int
	tabs  []shrinkTab

	// dirPtr and dirLen identify the directory the snapshot was taken
	// from (dirPtr also keeps it alive). newDir is the fully built
	// replacement directory, assembled concurrently by shrinkBuild so
	// that installing it is a single pointer store at the stop-the-world.
	dirPtr unsafe.Pointer
	dirLen int
	newDir []*table
}

// shrinkSnapshot runs in the concurrent worker. It copies the live entries of
// m's oversized tables into freshly allocated, typed buffers. seq is
// m.writeSeq, sampled by the caller while even; the snapshot is only
// meaningful if writeSeq is still seq at the next stop-the-world (the caller
// validates).
//
// It returns (handle, true) with a non-nil handle if there is work to do,
// (nil, true) if the map needs no shrinking (the caller should clear the
// enqueued bit at the next STW and drop the job), and (nil, false) if a race
// was detected (the caller should re-baseline and retry later).
//
// It may race with mutators; see the crash-safety notes in the file comment.
//
//go:linknamestd shrinkSnapshot
func shrinkSnapshot(m, typ unsafe.Pointer, seq uint32) (unsafe.Pointer, bool) {
	mp := (*Map)(m)
	mt := (*abi.MapType)(typ)

	// Load the directory reference. Local copies are GC-visible, keeping
	// the (possibly stale) directory alive while we walk it.
	dirLen := mp.dirLen
	dirPtr := mp.dirPtr
	if dirPtr == nil || dirLen <= 0 {
		// Small (single-group) map: already minimal.
		return nil, true
	}
	// Clamp the directory length to its allocation (property 3).
	if uintptr(dirLen)*goarch.PtrSize > heapObjSize(dirPtr) {
		return nil, false
	}

	h := &shrinkHandle{seed: mp.seed, dirPtr: dirPtr, dirLen: dirLen}

	// Sizing pass: find the tables that need shrinking, validating each
	// one's bounds, and size the snapshot buffers by their live entries
	// only (a map can be large while only a corner of it is shrinkable).
	// The copy pass below reads only the values captured and validated
	// here.
	entries := uint64(0)
	var lastTab *table
	for i := 0; i < dirLen; i++ {
		t := *(**table)(unsafe.Pointer(uintptr(dirPtr) + uintptr(i)*goarch.PtrSize))
		if t == nil {
			return nil, false // racing directory growth
		}
		if t == lastTab {
			continue // duplicate directory entry for the same table
		}
		lastTab = t

		// Local copies keep the groups allocation alive; clamp its
		// length to the allocation before touching any group.
		gdata := t.groups.data
		lengthMask := t.groups.lengthMask
		capacity := t.capacity
		tUsed := t.used
		if gdata == nil {
			return nil, false
		}
		if (lengthMask+1)*uint64(mt.GroupSize) > uint64(heapObjSize(gdata)) {
			return nil, false
		}
		if uint64(capacity) != (lengthMask+1)*abi.MapGroupSlots {
			return nil, false // (data, mask, capacity) torn
		}
		if uint64(tUsed) > uint64(capacity) {
			return nil, false // torn read
		}

		if !t.needsShrink() {
			continue
		}
		entries += uint64(tUsed)
		h.tabs = append(h.tabs, shrinkTab{
			old:        t,
			groups:     groupsReference{data: gdata, lengthMask: lengthMask},
			index:      t.index,
			localDepth: t.localDepth,
			floor:      t.minCapLog2(),
			oldCap:     capacity,
		})
	}
	if len(h.tabs) == 0 {
		return nil, true // nothing needs shrinking
	}

	h.cap = int(entries)
	if entries > 0 {
		h.keys = newarray(mt.Key, int(entries))
		h.elems = newarray(mt.Elem, int(entries))
	}

	// Copy pass: extract each recorded table's live entries.
	off := 0
	for ti := range h.tabs {
		st := &h.tabs[ti]
		st.start = off
		for gi := uint64(0); gi <= st.groups.lengthMask; gi++ {
			g := st.groups.group(mt, gi)
			for j := uintptr(0); j < abi.MapGroupSlots; j++ {
				if (g.ctrls().get(j) & ctrlEmpty) == ctrlEmpty {
					continue // empty or deleted
				}
				if off >= h.cap {
					return nil, false // more entries than counted: racing
				}
				kSrc := g.key(mt, j)
				if mt.IndirectKey() {
					kSrc = *((*unsafe.Pointer)(kSrc))
					if kSrc == nil {
						return nil, false // mid-delete
					}
				}
				eSrc := g.elem(mt, j)
				if mt.IndirectElem() {
					eSrc = *((*unsafe.Pointer)(eSrc))
					if eSrc == nil {
						return nil, false // mid-delete
					}
				}
				typedmemmove(mt.Key, shrinkElemAt(h.keys, mt.Key, off), kSrc)
				typedmemmove(mt.Elem, shrinkElemAt(h.elems, mt.Elem, off), eSrc)
				off++
			}
		}
		st.n = off - st.start
	}

	// Early discard: if the map was written while we copied, the snapshot
	// is garbage. (This is advisory; the authoritative check is the one at
	// the stop-the-world.)
	if atomic.Load(&mp.writeSeq) != seq {
		return nil, false
	}
	return unsafe.Pointer(h), true
}

// shrinkBuild runs in the concurrent worker, after the snapshot was validated
// at a stop-the-world. For each snapshotted table it builds a right-sized
// replacement by re-hashing the copied entries; it then assembles a complete
// replacement directory, so the stop-the-world install is a single pointer
// store. It reads only the snapshot and the held (validated) old directory,
// never the live map's structure.
//
//go:linknamestd shrinkBuild
func shrinkBuild(typ, handle unsafe.Pointer) {
	mt := (*abi.MapType)(typ)
	h := (*shrinkHandle)(handle)
	for ti := range h.tabs {
		st := &h.tabs[ti]
		nt := newTable(mt, uint64(shrinkTargetCap(st)), st.index, st.localDepth)
		nt.setMinCapLog2(st.floor)
		for j := st.start; j < st.start+st.n; j++ {
			key := shrinkElemAt(h.keys, mt.Key, j)
			elem := shrinkElemAt(h.elems, mt.Elem, j)
			hash := mt.Hasher(key, h.seed)
			if mt.IndirectKey() {
				// Clone, not alias: otherwise the rebuilt table would
				// point into the shared snapshot arrays, pinning them
				// whole and defeating the shrink.
				key = cloneIndirectValue(mt.Key, key)
			}
			if mt.IndirectElem() {
				elem = cloneIndirectValue(mt.Elem, elem)
			}
			nt.uncheckedPutSlot(mt, hash, key, elem)
		}
		st.nt = nt
	}

	// Assemble the replacement directory: the old directory's entries, with
	// each shrunk table's (contiguous) run of entries pointing at its
	// replacement. The old directory is held by the handle and was
	// validated at the last stop-the-world; if the map changes before the
	// install, the commit revalidation discards all of this.
	newDir := make([]*table, h.dirLen)
	for i := range newDir {
		newDir[i] = *(**table)(unsafe.Pointer(uintptr(h.dirPtr) + uintptr(i)*goarch.PtrSize))
	}
	for ti := range h.tabs {
		st := &h.tabs[ti]
		if st.index < 0 || st.index >= h.dirLen {
			continue // racing snapshot garbage; commit will discard
		}
		for i := st.index; i < h.dirLen && newDir[i] == st.old; i++ {
			newDir[i] = st.nt
		}
	}
	h.newDir = newDir
}

// shrinkInstall runs inside the mark-termination stop-the-world, after the
// caller revalidated that the map is unchanged since the snapshot. It
// publishes the pre-built directory with a single pointer store, marks the
// replaced tables stale (their entries are re-looked-up by iterators, exactly
// as after a mutator-driven grow), and clears the enqueued bit. It reads no
// entries and allocates nothing. It returns the number of tables replaced -
// one header store each, the unit the caller's stop-the-world budget is
// charged in - or 0 if the map was not touched.
//
//go:linknamestd shrinkInstall
func shrinkInstall(m, handle unsafe.Pointer) int {
	mp := (*Map)(m)
	h := (*shrinkHandle)(handle)
	// The map is unchanged since the snapshot (writeSeq-validated by the
	// caller under stop-the-world), so its directory must still be the one
	// the replacement was derived from; confirm defensively.
	if mp.dirPtr != h.dirPtr || mp.dirLen != h.dirLen || h.newDir == nil {
		mp.enqueued = false
		return 0
	}
	mp.dirPtr = unsafe.Pointer(&h.newDir[0])
	tables := 0
	for ti := range h.tabs {
		st := &h.tabs[ti]
		if st.nt != nil {
			st.old.index = -1 // stale: iterators re-look-up its entries
			tables++
		}
	}
	// The snapshot covered every table with tombstones (needsShrink includes
	// tombstones() > 0) and all were rebuilt tombstone-free, so the idle map
	// is now quiescent (R7).
	mp.tombstonePossible = false
	mp.enqueued = false
	return tables
}

// needsShrink reports whether the table has reclaimable state: tombstones to
// drop, or capacity above what its live entries need (down to its floor).
func (t *table) needsShrink() bool {
	return t.tombstones() > 0 || uint64(t.capacity) > t.rightSizedCapacity()
}

// shrinkTargetCap is the capacity for a rebuilt table: enough for its snapshot
// entries at load <= 1/2, never below the floor and never above the old
// capacity (a same-size rebuild just purges tombstones).
func shrinkTargetCap(st *shrinkTab) uint16 {
	c := rightSizedFor(uint64(st.n), uint64(1)<<st.floor)
	if c > uint64(st.oldCap) {
		c = uint64(st.oldCap)
	}
	return uint16(c)
}

// shrinkElemAt returns a pointer to the i-th element of a typed array.
func shrinkElemAt(base unsafe.Pointer, elemType *abi.Type, i int) unsafe.Pointer {
	return unsafe.Pointer(uintptr(base) + uintptr(i)*elemType.Size_)
}
