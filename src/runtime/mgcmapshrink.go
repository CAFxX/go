// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Background map shrinking.
//
// A swiss-table map (internal/runtime/maps) that grows large and is then
// drained keeps its backing arrays. The delete path shrinks a map that keeps
// receiving deletes (see internal/runtime/maps.table.shrink), but a map that
// drains (or is cleared) and then goes idle has no further operation to
// piggyback on. This file drives an idle-map shrinker.
//
// When a delete or a Clear leaves a map with reclaimable state, the map is
// enqueued here (once; maps.Map.enqueued dedupes). A background worker, woken
// at the start of each GC cycle, processes entries that have been enqueued for
// at least mapShrinkAgeCycles GC cycles, and only if the map has not been
// written since (its writeSeq is unchanged and even). Because a live map
// cannot be mutated safely from another goroutine, the rebuild is committed
// inside the mark-termination stop-the-world, with all the real work done
// concurrently:
//
//	cycle N   (concurrent mark): worker snapshots the oversized tables
//	cycle N   (mark termination): validate writeSeq unchanged (O(1))
//	cycle N+1 (concurrent):       worker builds replacement tables
//	cycle N+1 (mark termination): revalidate (O(1)); publish the directory
//
// The stop-the-world is what makes the writeSeq validation sound. Mutators
// update writeSeq and the map with plain stores (the design adds no atomics
// or fences to the write path), so on weak-memory machines a concurrent
// observer could see a stale writeSeq alongside newer map contents; no
// concurrent read of writeSeq can prove idleness. Stopping the world is a
// total synchronization point: every mutator is paused at a safepoint with
// its stores visible, so writeSeq unchanged-and-even, observed inside the
// stop-the-world, does prove that no write overlapped the snapshot.
//
// That schedule is nominal, not an invariant: each stop-the-world only
// advances jobs whose concurrent phase has completed and been relinked into
// the queue (jobs the worker holds detached mid-pass are simply picked up at
// a later stop-the-world), and each transition revalidates writeSeq against
// the original snapshot baseline, so a slow worker or an exhausted budget
// costs cycles of latency, never correctness.
//
// The concurrent snapshot may race mutators; it is crash-safe by construction
// and its output is discarded by the validation if anything was written (see
// internal/runtime/maps/shrink.go for the safety argument). The
// stop-the-world work never reads or copies map entries, never allocates, and
// never resolves weak handles (blocked during mark termination): per map it
// compares writeSeq and, for an install, publishes a pre-built replacement
// directory with a single pointer store plus one header store per replaced
// table, all charged against a per-stop-the-world budget.
//
// While jobs are in flight the worker re-samples their writeSeq roughly every
// 100ms. A change discards the job early (the map is active; requeue and
// wait). The re-sampling also bounds the window in which the 32-bit writeSeq
// could wrap all the way around between two observations of the same address:
// while the worker is live the window is one resample interval (a wrap would
// need ~4.3e10 writes per second to one map), and if GC cycles stop arriving
// entirely (GOGC=off with no runtime.GC) the resample cap discards the job -
// dropping its snapshot and baseline - before an unbounded no-observation
// window can open. A wrap is therefore never observable, without widening the
// counter.
//
// Race detector: the worker deliberately reads map memory that mutators may
// be writing, but this is invisible to TSan and can never produce a report.
// This file and internal/runtime/maps are runtime packages, compiled without
// sanitizer instrumentation (NoInstrument, cmd/internal/objabi/pkgspecial.go),
// and the pipeline makes no race.Read/race.Write annotations, so none of its
// accesses enter TSan's happens-before graph. The annotations at the map API
// entry points are untouched, so genuine user races (concurrent map ops from
// multiple goroutines) are still reported exactly as before.
//
// The queue lock is a leaf: enqueue allocates its node before taking the lock,
// and wakeups ready the worker after dropping it, so no other lock or
// allocation sits underneath.

package runtime

import (
	"internal/runtime/atomic"
	"unsafe"
)

// mapShrinkAgeCycles is the number of completed GC cycles a map must remain
// idle-and-enqueued before the worker acts on it. It matches the delete-path
// shrink age (internal/runtime/maps.shrinkAgeCycles).
const mapShrinkAgeCycles = 4

// mapShrinkCommitBudget bounds the work done by a single mark-termination
// hook, so background shrinking cannot lengthen a stop-the-world unboundedly.
// It is charged one unit per job examined plus one per table replaced by an
// install (each replaced table costs one header store). Jobs past the budget
// are left in place and advance at a later stop-the-world.
const mapShrinkCommitBudget = 512

// mapShrinkResampleNS is how often the worker re-samples in-flight jobs.
const mapShrinkResampleNS = 100e6 // 100ms

// mapShrinkMaxResamples bounds how many times a job may be re-sampled within
// one pipeline phase before the worker gives up and discards it (dropping the
// snapshot, re-baselining to pending). The cap only bites when mark
// terminations have stopped arriving (e.g. GOGC=off with no runtime.GC): a
// phase otherwise advances at the next mark termination, and sysmon forces a
// GC cycle every forcegcperiod (2 minutes), so the cap must comfortably
// exceed one forced-GC period of resamples - 6 minutes here - or idle
// programs whose only GCs are forced ones could never complete the pipeline.
const mapShrinkMaxResamples = 3 * (2 * 60e9) / mapShrinkResampleNS

// Backoff: a map that is shrunk and then quickly refilled and re-enqueued is
// oscillating; shrinking it again soon just churns. After committing a shrink
// the map is recorded here, and if it re-enqueues within mapShrinkRecentGC
// cycles it must stay idle 2x, then 4x, mapShrinkAgeCycles before being shrunk
// again. The record set is a small fixed ring (about 4KB).
const (
	mapShrinkRecent    = 256 // recently-shrunk maps kept for backoff
	mapShrinkRecentGC  = 64  // cycles a record stays eligible for backoff
	mapShrinkMaxAgeMul = 4   // ceiling on the idleness multiplier
)

// mapShrinkRecentEntry records one recently-shrunk map. The address is only
// ever compared, never dereferenced: the map may be dead, and the address may
// even have been reused by an unrelated allocation, in which case the new
// object inherits the backoff multiplier - a bounded, harmless extra delay.
type mapShrinkRecentEntry struct {
	addr uintptr // shrunk map's address (0 == empty slot)
	gc   uint32  // numgc when it was shrunk
	mul  uint8   // idleness multiplier to apply if it re-enqueues
}

// Pipeline phases. The worker advances pending -> snapshotted and
// validated -> built (concurrent); the mark-termination hook advances
// snapshotted -> validated and built -> installed/dropped (world stopped).
type mapShrinkPhase uint8

const (
	mapShrinkPending     mapShrinkPhase = iota // enqueued; awaiting age, idleness and snapshot
	mapShrinkNoop                              // nothing to shrink; STW clears the dedupe bit and drops
	mapShrinkSnapshotted                       // snapshot taken; awaiting STW validation
	mapShrinkValidated                         // snapshot proven consistent; awaiting build
	mapShrinkBuilt                             // replacements built; awaiting STW install
)

// mapShrinkJob tracks one map moving through the pipeline. Jobs form
// intrusive singly linked lists so that enqueue never allocates while holding
// the queue lock (keeping the lock a leaf). The pointer fields are ordinary
// GC-visible references: while a job holds strong/snap, the map and the
// snapshot (and everything they reference) stay alive.
type mapShrinkJob struct {
	next    *mapShrinkJob
	weak    *atomic.Uintptr // weak handle to the *maps.Map
	strong  unsafe.Pointer  // strong *maps.Map ref while in flight (worker sets; STW hook consumes)
	typ     unsafe.Pointer  // *abi.MapType, captured at enqueue (Map does not store it)
	snap    unsafe.Pointer  // *maps.shrinkHandle while in flight
	seqSnap uint32          // maps.Map.writeSeq the pipeline state is based on
	gcSnap  uint32          // numgc at (re-)baseline, for age gating
	spins   uint16          // re-samples in the current phase (see mapShrinkMaxResamples)
	ageMul  uint8           // idleness multiplier from backoff (>= 1; 1 means none)
	phase   mapShrinkPhase
}

// discard reverts an in-flight job to pending, dropping its snapshot and
// strong reference (so the map can die) and re-baselining its age.
func (j *mapShrinkJob) discard(seq uint32) {
	j.strong = nil
	j.snap = nil
	j.seqSnap = seq
	j.gcSnap = memstats.numgc
	j.spins = 0
	j.phase = mapShrinkPending
}

var mapShrinkWork struct {
	lock mutex // leaf; guards pending and parked

	// pending holds jobs in the mapShrinkPending phase: enqueued by
	// mutators (under the lock), consumed and refilled by the worker, and
	// pushed to by the mark-termination hook when it discards a job (world
	// stopped, so no lock is needed there).
	pending *mapShrinkJob

	// inflight holds jobs in all other phases. It is owned by the worker;
	// the mark-termination hook (world stopped: worker and mutators are
	// paused at safepoints, and runtime mutex holders cannot be paused
	// mid-critical-section) may also walk and modify it without the lock.
	inflight *mapShrinkJob

	g      *g
	parked bool

	// recent records recently-shrunk maps for backoff. Written only by the
	// mark-termination hook (world stopped). Read at enqueue by mutators;
	// a mutator can be preempted mid-scan across a commit and resume over
	// rewritten entries, so a read may mix fields of different records.
	// That is fine: the result only stretches an aging delay, and every
	// field read is a single word.
	recent     [mapShrinkRecent]mapShrinkRecentEntry
	recentNext int
}

// mapShrinkBackoffMul returns the idleness multiplier to apply to a map at
// addr that is (re-)enqueueing at cycle now: the highest fresh value recorded
// at its past shrinks, or 1 (no backoff). The ring may hold several entries
// for addr, since records are appended without dedup.
func mapShrinkBackoffMul(addr uintptr, now uint32) uint8 {
	mul := uint8(1)
	for i := range mapShrinkWork.recent {
		if e := &mapShrinkWork.recent[i]; e.addr == addr && now-e.gc <= mapShrinkRecentGC && e.mul > mul {
			mul = e.mul
		}
	}
	return mul
}

// mapShrinkRecordShrunk records that the map at addr was just shrunk while
// carrying idleness multiplier mul, so a quick re-enqueue backs off to 2*mul
// (capped). Append-only: called from the mark-termination hook (world
// stopped), where a dedup scan would be stop-the-world work for no benefit.
func mapShrinkRecordShrunk(addr uintptr, mul uint8) {
	next := mul * 2
	if next > mapShrinkMaxAgeMul {
		next = mapShrinkMaxAgeMul
	}
	mapShrinkWork.recent[mapShrinkWork.recentNext] = mapShrinkRecentEntry{addr, memstats.numgc, next}
	mapShrinkWork.recentNext = (mapShrinkWork.recentNext + 1) % mapShrinkRecent
}

// maps_enqueueShrink adds m (an *internal/runtime/maps.Map of type typ) to the
// background shrink queue. seq is m.writeSeq as of the enqueueing delete or
// Clear. The caller has already set m.enqueued to dedupe, so this runs at most
// once per drain episode per map.
//
//go:linknamestd maps_enqueueShrink internal/runtime/maps.enqueueShrink
func maps_enqueueShrink(m, typ unsafe.Pointer, seq uint32) {
	if debug.mapbgshrink == 0 {
		return
	}
	// Only a heap-allocated map can be weakly referenced and outlive this
	// call. A stack-allocated map (one the compiler proved non-escaping)
	// dies with its frame, so it never needs background shrinking; skip it.
	// m.enqueued may stay set on such a map, which is harmless: it is gone
	// when the frame returns.
	if spanOfHeap(uintptr(m)) == nil {
		return
	}
	// Allocate the node and register the weak handle before taking the
	// leaf lock.
	job := &mapShrinkJob{
		weak:    getOrAddWeakHandle(m),
		typ:     typ,
		seqSnap: seq,
		gcSnap:  memstats.numgc,
		ageMul:  mapShrinkBackoffMul(uintptr(m), memstats.numgc),
		phase:   mapShrinkPending,
	}
	lock(&mapShrinkWork.lock)
	job.next = mapShrinkWork.pending
	mapShrinkWork.pending = job
	unlock(&mapShrinkWork.lock)
}

// gcMapShrinkWake wakes the background shrink worker. Called from gcStart, so
// the worker runs concurrently with the mark phase. goready is called without
// the leaf lock held. If the worker is in its re-sampling sleep rather than
// parked, this is a no-op; the worker notices the new cycle itself.
func gcMapShrinkWake() {
	if debug.mapbgshrink == 0 {
		return
	}
	lock(&mapShrinkWork.lock)
	gp := mapShrinkWork.g
	wake := mapShrinkWork.parked && gp != nil
	if wake {
		mapShrinkWork.parked = false
	}
	unlock(&mapShrinkWork.lock)
	if wake {
		goready(gp, 0)
	}
}

func bgmapshrink(c chan int) {
	mapShrinkWork.g = getg()

	// mapShrinkWork.lock is initialized in gcinit, before any map delete
	// could enqueue.
	lock(&mapShrinkWork.lock)
	mapShrinkWork.parked = true
	c <- 1
	goparkunlock(&mapShrinkWork.lock, waitReasonGCMapShrinkWait, traceBlockSystemGoroutine, 1)

	for {
		// A full pass processes pending jobs (aging, idleness, snapshot)
		// and in-flight ones. While jobs remain in flight, re-sample them
		// every mapShrinkResampleNS - touching only the in-flight list, so
		// a large pending queue costs nothing here - and take another full
		// pass whenever a GC cycle completes (a mark termination has
		// advanced in-flight phases, and pending jobs age by GC cycles, so
		// there is nothing new to do for them in between).
		gcAtPass := memstats.numgc
		inflight := mapShrinkWorkerPass(true)
		for inflight && memstats.numgc == gcAtPass {
			timeSleep(mapShrinkResampleNS)
			inflight = mapShrinkWorkerPass(false)
		}
		if inflight {
			continue // a cycle completed; take a full pass
		}

		// Nothing in flight. Pending jobs, if any, cannot become
		// actionable before a further GC cycle, and gcStart wakes us
		// every cycle; a wakeup that fired while we were busy costs at
		// most that one cycle of latency.
		lock(&mapShrinkWork.lock)
		mapShrinkWork.parked = true
		goparkunlock(&mapShrinkWork.lock, waitReasonGCMapShrinkWait, traceBlockSystemGoroutine, 1)
	}
}

// mapShrinkWorkerPass runs one concurrent pass and reports whether any job is
// left in flight. A full pass detaches and processes both lists; a resample
// pass touches only the in-flight list. Weak-handle resolution and all
// allocation happen only here, never in the STW hook.
func mapShrinkWorkerPass(full bool) (inflight bool) {
	lock(&mapShrinkWork.lock)
	job := mapShrinkWork.inflight
	mapShrinkWork.inflight = nil
	if full {
		j := mapShrinkWork.pending
		mapShrinkWork.pending = nil
		// Process the in-flight jobs first, then the pending ones.
		if job == nil {
			job = j
		} else {
			t := job
			for t.next != nil {
				t = t.next
			}
			t.next = j
		}
	}
	unlock(&mapShrinkWork.lock)

	now := memstats.numgc
	var pendHead, pendTail, flightHead, flightTail *mapShrinkJob
	relink := func(j *mapShrinkJob) {
		j.next = nil
		if j.phase == mapShrinkPending {
			if pendTail == nil {
				pendHead, pendTail = j, j
			} else {
				pendTail.next = j
				pendTail = j
			}
			return
		}
		inflight = true
		if flightTail == nil {
			flightHead, flightTail = j, j
		} else {
			flightTail.next = j
			flightTail = j
		}
	}

	for job != nil {
		j := job
		job = job.next

		switch j.phase {
		case mapShrinkPending:
			// Age gate: leave young entries alone. Backoff stretches the
			// wait for maps that were recently shrunk and refilled.
			if now-j.gcSnap < mapShrinkAgeCycles*uint32(j.ageMul) {
				relink(j)
				continue
			}
			// Resolve the weak handle. May briefly park if a mark
			// termination is in progress; fine here (not the STW hook,
			// no lock held).
			strong := internal_weak_runtime_makeStrongFromWeak(unsafe.Pointer(j.weak))
			if strong == nil {
				continue // map died; drop
			}
			// Idleness gate: unchanged and not mid-write since the last
			// baseline.
			seq := mapShrinkReadSeq(strong)
			if seq&1 != 0 || seq != j.seqSnap {
				j.discard(seq) // re-baseline; wait for idleness again
				relink(j)
				continue
			}
			// Aged and idle: snapshot concurrently. Racing mutators can
			// make the snapshot fail or produce garbage; garbage is
			// discarded by the STW validation.
			snap, ok := mapShrinkSnapshot(strong, j.typ, seq)
			if !ok {
				j.discard(mapShrinkReadSeq(strong))
				relink(j)
				continue
			}
			j.strong = strong
			j.spins = 0
			if snap == nil {
				// Nothing needs shrinking (e.g. the delete-path shrink
				// already handled it). Have the STW hook clear the
				// dedupe bit and drop the job.
				j.phase = mapShrinkNoop
			} else {
				j.snap = snap
				j.phase = mapShrinkSnapshotted
			}
			relink(j)
		case mapShrinkValidated:
			// The snapshot was proven consistent at the last mark
			// termination. Re-hash it into right-sized tables. Reads only
			// the snapshot, never the live map.
			//
			// TODO(cafxx): this rebuild and the snapshot copy above both
			// touch every live entry while running concurrently with user
			// goroutines. Should we yield with goschedIfBusy() every few
			// hundred entries (as the background sweeper does, see
			// mgcsweep.go) so shrinking a very large map cannot delay
			// runnable user work? goschedIfBusy is cheap when there is idle
			// time, so it may be worth doing unconditionally, or only when
			// GOMAXPROCS==1, where the worker shares the sole P with the
			// mutator. The per-entry loops live in internal/runtime/maps
			// (shrinkBuild, shrinkSnapshot), so goschedIfBusy would need to
			// be pushed there like the other maps_* helpers.
			mapShrinkBuild(j.typ, j.snap)
			if seq := mapShrinkReadSeq(j.strong); seq != j.seqSnap {
				j.discard(seq) // map woke up during the build
			} else {
				j.phase = mapShrinkBuilt
				j.spins = 0
			}
			relink(j)
		case mapShrinkNoop, mapShrinkSnapshotted, mapShrinkBuilt:
			// Waiting on the next STW. Re-sample so an awakened map is
			// discarded early; these frequent same-address samples also
			// keep the seq-comparison windows too short to wrap.
			if seq := mapShrinkReadSeq(j.strong); seq != j.seqSnap {
				j.discard(seq)
			} else if j.spins++; j.spins >= mapShrinkMaxResamples {
				// Mark terminations have stopped arriving entirely
				// (e.g. GOGC=off with no runtime.GC; a forced GC would
				// have advanced the phase long ago). Stop pinning the
				// snapshot: re-baseline to pending, so a later GC cycle
				// restarts the pipeline.
				j.discard(j.seqSnap)
			}
			relink(j)
		default:
			relink(j)
		}
	}

	lock(&mapShrinkWork.lock)
	if pendHead != nil {
		pendTail.next = mapShrinkWork.pending
		mapShrinkWork.pending = pendHead
	}
	if flightHead != nil {
		flightTail.next = mapShrinkWork.inflight
		mapShrinkWork.inflight = flightHead
	}
	unlock(&mapShrinkWork.lock)
	return inflight
}

// gcMapShrinkCommit runs inside the mark-termination stop-the-world, after the
// write barrier is off (setGCPhase(_GCoff)). The world is stopped: mutators
// and the worker are paused, so it may walk the in-flight list and touch map
// memory without locks. Its work is bounded by mapShrinkCommitBudget, charged
// per job examined plus per table replaced; jobs past the budget stay in
// place for a later stop-the-world. It never reads or copies map entries,
// never allocates, and never resolves weak handles (blocked here).
func gcMapShrinkCommit() {
	if debug.mapbgshrink == 0 || mapShrinkWork.inflight == nil {
		return
	}
	budget := mapShrinkCommitBudget

	job := mapShrinkWork.inflight
	mapShrinkWork.inflight = nil
	var head, tail *mapShrinkJob
	relink := func(j *mapShrinkJob) {
		j.next = nil
		if tail == nil {
			head, tail = j, j
		} else {
			tail.next = j
			tail = j
		}
	}
	// Discarded jobs go back to the pending list; the world is stopped, so
	// pushing without the lock cannot race an enqueue.
	toPending := func(j *mapShrinkJob) {
		j.next = mapShrinkWork.pending
		mapShrinkWork.pending = j
	}

	for job != nil {
		j := job
		job = job.next

		if budget <= 0 {
			// Out of budget: keep this job and all the rest in flight,
			// wholesale, for a later stop-the-world.
			relink(j)
			tail.next = job
			mapShrinkWork.inflight = head
			return
		}
		budget--

		// Jobs the worker holds detached mid-pass are not in this list;
		// they are simply picked up at a later STW.
		switch j.phase {
		case mapShrinkNoop:
			if seq := mapShrinkReadSeq(j.strong); seq != j.seqSnap {
				j.discard(seq) // state changed; reassess later
				toPending(j)
				continue
			}
			// Map is idle and needs nothing: clear the dedupe bit so a
			// later delete can re-enqueue it, and drop the job.
			mapShrinkClearEnqueued(j.strong)
		case mapShrinkSnapshotted:
			// The world is stopped, so this comparison is authoritative:
			// writeSeq unchanged and even proves no write overlapped the
			// snapshot, making it a consistent image of the map (see the
			// file comment).
			if seq := mapShrinkReadSeq(j.strong); seq&1 != 0 || seq != j.seqSnap {
				j.discard(seq)
				toPending(j)
				continue
			}
			j.phase = mapShrinkValidated
			j.spins = 0
			relink(j)
		case mapShrinkValidated:
			// Awaiting its concurrent build (the worker may be behind the
			// nominal schedule). Re-check idleness here too, so that even
			// with a starved worker every in-flight job's writeSeq is
			// compared at least once per GC cycle, and an awakened map's
			// snapshot memory is released promptly.
			if seq := mapShrinkReadSeq(j.strong); seq&1 != 0 || seq != j.seqSnap {
				j.discard(seq)
				toPending(j)
				continue
			}
			relink(j)
		case mapShrinkBuilt:
			if seq := mapShrinkReadSeq(j.strong); seq&1 != 0 || seq != j.seqSnap {
				j.discard(seq)
				toPending(j)
				continue
			}
			// Unchanged since the validated snapshot: the rebuilt tables
			// hold exactly the map's contents, and the pre-built
			// directory is publishable with one pointer store (this also
			// clears the dedupe bit). Drop the job, charging the budget
			// for the per-table header stores.
			tables := mapShrinkInstall(j.strong, j.snap)
			budget -= tables
			if tables > 0 {
				mapShrinkRecordShrunk(uintptr(j.strong), j.ageMul)
			}
		default:
			relink(j)
		}
	}

	if head != nil {
		tail.next = mapShrinkWork.inflight
		mapShrinkWork.inflight = head
	}
}

// maps_heapObjSize reports the number of bytes from p to the end of p's heap
// allocation, or 0 if p is not a heap pointer. The concurrent snapshot uses it
// to bound every read to the referenced allocation, whatever racing mutators
// do to the map's structure.
//
//go:linknamestd maps_heapObjSize internal/runtime/maps.heapObjSize
func maps_heapObjSize(p unsafe.Pointer) uintptr {
	base, s, _ := findObject(uintptr(p), 0, 0)
	if s == nil {
		return 0
	}
	return s.elemsize - (uintptr(p) - base)
}

// Implemented in internal/runtime/maps and reached by linkname.
//
//go:linknamestd mapShrinkReadSeq internal/runtime/maps.shrinkReadSeq
func mapShrinkReadSeq(m unsafe.Pointer) uint32

//go:linknamestd mapShrinkSnapshot internal/runtime/maps.shrinkSnapshot
func mapShrinkSnapshot(m, typ unsafe.Pointer, seq uint32) (unsafe.Pointer, bool)

//go:linknamestd mapShrinkBuild internal/runtime/maps.shrinkBuild
func mapShrinkBuild(typ, snap unsafe.Pointer)

//go:linknamestd mapShrinkInstall internal/runtime/maps.shrinkInstall
func mapShrinkInstall(m, snap unsafe.Pointer) int

//go:linknamestd mapShrinkClearEnqueued internal/runtime/maps.shrinkClearEnqueued
func mapShrinkClearEnqueued(m unsafe.Pointer)
