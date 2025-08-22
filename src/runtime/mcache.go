// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package runtime

import (
	"internal/runtime/atomic"
	"internal/runtime/gc"
	"internal/runtime/sys"
	"unsafe"
)

// Per-thread (in Go, per-P) cache for small objects.
// This includes a small object cache and local allocation stats.
// No locking needed because it is per-thread (per-P).
//
// mcaches are allocated from non-GC'd memory, so any heap pointers
// must be specially handled.
type mcache struct {
	_ sys.NotInHeap

	// The following members are accessed on every malloc,
	// so they are grouped here for better caching.
	nextSample  int64   // trigger heap sample after allocating this many bytes
	memProfRate int     // cached mem profile rate, used to detect changes
	scanAlloc   uintptr // bytes of scannable heap allocated

	// Allocator cache for tiny objects w/o pointers.
	// See "Tiny allocator" comment in malloc.go.

	// tiny points to the beginning of the current tiny block, or
	// nil if there is no current tiny block.
	//
	// tiny is a heap pointer. Since mcache is in non-GC'd memory,
	// we handle it by clearing it in releaseAll during mark
	// termination.
	//
	// tinyAllocs is the number of tiny allocations performed
	// by the P that owns this mcache.
	tiny       uintptr
	tinyoffset uintptr
	tinyAllocs uintptr

	// The rest is not accessed on every malloc.

	// alloc contains spans to allocate from, indexed by spanClass.
	alloc [numSpanClasses]*mspan

	// TODO(thepudds): better to interleave alloc and reusableScan/reusableNoscan so that
	// a single malloc call can often access both in the same cache line for a given spanClass.
	// It's not interleaved right now in part to have slightly smaller diff, and might be
	// negligible effect on current microbenchmarks.
	// TODO(thepudds): currently very wasteful to have numSpanClasses of reusableScan and
	// reusableNoscan. Could just cast as needed instead of having two arrays, or could use
	// two arrays of size _NumSizeClasses, or ___.

	// reusableNoscan contains linked lists of reusable noscan heap objects, indexed by spanClass.
	// The next pointers are stored in the first word of the heap objects.
	reusableNoscan [numSpanClasses]gclinkptr

	// reusableScan contains reusableLinks forming chunked linked lists of reusable scan objects,
	// indexed by spanClass.
	reusableScan [numSpanClasses]uintptr

	stackcache [_NumStackOrders]stackfreelist

	reusableLinkCache uintptr // free list of *reusableLink. (We don't use fixalloc.free).

	// flushGen indicates the sweepgen during which this mcache
	// was last flushed. If flushGen != mheap_.sweepgen, the spans
	// in this mcache are stale and need to the flushed so they
	// can be swept. This is done in acquirep.
	flushGen atomic.Uint32
}

// A gclink is a node in a linked list of blocks, like mlink,
// but it is opaque to the garbage collector.
// The GC does not trace the pointers during collection,
// and the compiler does not emit write barriers for assignments
// of gclinkptr values. Code should store references to gclinks
// as gclinkptr, not as *gclink.
type gclink struct {
	next gclinkptr
}

// A gclinkptr is a pointer to a gclink, but it is opaque
// to the garbage collector.
type gclinkptr uintptr

// ptr returns the *gclink form of p.
// The result should be used for accessing fields, not stored
// in other data structures.
func (p gclinkptr) ptr() *gclink {
	return (*gclink)(unsafe.Pointer(p))
}

type stackfreelist struct {
	list gclinkptr // linked list of free stacks
	size uintptr   // total size of stacks in list
}

// dummy mspan that contains no free objects.
var emptymspan mspan

func allocmcache() *mcache {
	var c *mcache
	systemstack(func() {
		lock(&mheap_.lock)
		c = (*mcache)(mheap_.cachealloc.alloc())
		c.flushGen.Store(mheap_.sweepgen)
		unlock(&mheap_.lock)
	})
	for i := range c.alloc {
		c.alloc[i] = &emptymspan
	}
	c.nextSample = nextSample()

	return c
}

// freemcache releases resources associated with this
// mcache and puts the object onto a free list.
//
// In some cases there is no way to simply release
// resources, such as statistics, so donate them to
// a different mcache (the recipient).
func freemcache(c *mcache) {
	systemstack(func() {
		c.releaseAll()
		stackcache_clear(c)

		// NOTE(rsc,rlh): If gcworkbuffree comes back, we need to coordinate
		// with the stealing of gcworkbufs during garbage collection to avoid
		// a race where the workbuf is double-freed.
		// gcworkbuffree(c.gcworkbuf)
		lock(&mheap_.lock)
		mheap_.cachealloc.free(unsafe.Pointer(c))
		unlock(&mheap_.lock)
	})
}

// getMCache is a convenience function which tries to obtain an mcache.
//
// Returns nil if we're not bootstrapping or we don't have a P. The caller's
// P must not change, so we must be in a non-preemptible state.
func getMCache(mp *m) *mcache {
	// Grab the mcache, since that's where stats live.
	pp := mp.p.ptr()
	var c *mcache
	if pp == nil {
		// We will be called without a P while bootstrapping,
		// in which case we use mcache0, which is set in mallocinit.
		// mcache0 is cleared when bootstrapping is complete,
		// by procresize.
		c = mcache0
	} else {
		c = pp.mcache
	}
	return c
}

// refill acquires a new span of span class spc for c. This span will
// have at least one free object. The current span in c must be full.
//
// Must run in a non-preemptible context since otherwise the owner of
// c could change.
func (c *mcache) refill(spc spanClass) {
	// println("refill", spc, "c", c, "mheap_.sweepgen", mheap_.sweepgen)

	// Return the current cached span to the central lists.
	s := c.alloc[spc]

	if s.allocCount != s.nelems {
		throw("refill of span with free space remaining")
	}

	// TODO(thepudds): we might be able to allow mallocgcTiny to reuse 16 byte objects from spc==5,
	// but for now, just clear our reusable objects for tinySpanClass. Similarly, we might have
	// reusable pointers on a noheader scan span if we left reusable objects that did not have
	// matching heap bits and then needed to refill on the normal allocation path.
	// TODO(thepudds): this passes tests, but might be better to just allow the refill to happen
	// without complaining, rather than clearing. Also, we currently only partially clear (not clearing next),
	// and only partially check whether to complain (not checking next), which means we are effectively
	// already allowing refills to proceed with non-empty reusable links.
	if spc == tinySpanClass || (!spc.noscan() && heapBitsInSpan(s.elemsize)) {
		if c.reusableScan[spc] != 0 {
			rl := (*reusableLink)(unsafe.Pointer(c.reusableScan[spc]))
			rl.len = 0
		}
		c.reusableNoscan[spc] = 0
	}
	if c.reusableScan[spc] != 0 {
		rl := (*reusableLink)(unsafe.Pointer(c.reusableScan[spc]))
		if rl.len > 0 {
			throw("refill of span with reusable pointers remaining on pointer slice")
		}
	}
	if c.reusableNoscan[spc] != 0 {
		throw("refill of span with reusable pointers remaining on pointer free list")
	}

	if s != &emptymspan {
		// Mark this span as no longer cached.
		if s.sweepgen != mheap_.sweepgen+3 {
			throw("bad sweepgen in refill")
		}
		mheap_.central[spc].mcentral.uncacheSpan(s)

		// Count up how many slots were used and record it.
		stats := memstats.heapStats.acquire()
		slotsUsed := int64(s.allocCount) - int64(s.allocCountBeforeCache)
		atomic.Xadd64(&stats.smallAllocCount[spc.sizeclass()], slotsUsed)

		// Flush tinyAllocs.
		if spc == tinySpanClass {
			atomic.Xadd64(&stats.tinyAllocCount, int64(c.tinyAllocs))
			c.tinyAllocs = 0
		}
		memstats.heapStats.release()

		// Count the allocs in inconsistent, internal stats.
		bytesAllocated := slotsUsed * int64(s.elemsize)
		gcController.totalAlloc.Add(bytesAllocated)

		// Clear the second allocCount just to be safe.
		s.allocCountBeforeCache = 0
	}

	// Get a new cached span from the central lists.
	s = mheap_.central[spc].mcentral.cacheSpan()
	if s == nil {
		throw("out of memory")
	}

	if s.allocCount == s.nelems {
		throw("span has no free space")
	}

	// Indicate that this span is cached and prevent asynchronous
	// sweeping in the next sweep phase.
	s.sweepgen = mheap_.sweepgen + 3

	// Store the current alloc count for accounting later.
	s.allocCountBeforeCache = s.allocCount

	// Update heapLive and flush scanAlloc.
	//
	// We have not yet allocated anything new into the span, but we
	// assume that all of its slots will get used, so this makes
	// heapLive an overestimate.
	//
	// When the span gets uncached, we'll fix up this overestimate
	// if necessary (see releaseAll).
	//
	// We pick an overestimate here because an underestimate leads
	// the pacer to believe that it's in better shape than it is,
	// which appears to lead to more memory used. See #53738 for
	// more details.
	usedBytes := uintptr(s.allocCount) * s.elemsize
	gcController.update(int64(s.npages*pageSize)-int64(usedBytes), int64(c.scanAlloc))
	c.scanAlloc = 0

	c.alloc[spc] = s
}

// allocLarge allocates a span for a large object.
func (c *mcache) allocLarge(size uintptr, noscan bool) *mspan {
	if size+pageSize < size {
		throw("out of memory")
	}
	npages := size >> gc.PageShift
	if size&pageMask != 0 {
		npages++
	}

	// Deduct credit for this span allocation and sweep if
	// necessary. mHeap_Alloc will also sweep npages, so this only
	// pays the debt down to npage pages.
	deductSweepCredit(npages*pageSize, npages)

	spc := makeSpanClass(0, noscan)
	s := mheap_.alloc(npages, spc)
	if s == nil {
		throw("out of memory")
	}

	// Count the alloc in consistent, external stats.
	stats := memstats.heapStats.acquire()
	atomic.Xadd64(&stats.largeAlloc, int64(npages*pageSize))
	atomic.Xadd64(&stats.largeAllocCount, 1)
	memstats.heapStats.release()

	// Count the alloc in inconsistent, internal stats.
	gcController.totalAlloc.Add(int64(npages * pageSize))

	// Update heapLive.
	gcController.update(int64(s.npages*pageSize), 0)

	// Put the large span in the mcentral swept list so that it's
	// visible to the background sweeper.
	mheap_.central[spc].mcentral.fullSwept(mheap_.sweepgen).push(s)

	// Adjust s.limit down to the object-containing part of the span.
	//
	// This is just to create a slightly tighter bound on the limit.
	// It's totally OK if the garbage collector, in particular
	// conservative scanning, can temporarily observes an inflated
	// limit. It will simply mark the whole object or just skip it
	// since we're in the mark phase anyway.
	s.limit = s.base() + size
	s.initHeapBits()
	return s
}

func (c *mcache) releaseAll() {
	// Take this opportunity to flush scanAlloc.
	scanAlloc := int64(c.scanAlloc)
	c.scanAlloc = 0

	sg := mheap_.sweepgen
	dHeapLive := int64(0)
	for i := range c.alloc {
		s := c.alloc[i]
		if s != &emptymspan {
			slotsUsed := int64(s.allocCount) - int64(s.allocCountBeforeCache)
			s.allocCountBeforeCache = 0

			// Adjust smallAllocCount for whatever was allocated.
			stats := memstats.heapStats.acquire()
			atomic.Xadd64(&stats.smallAllocCount[spanClass(i).sizeclass()], slotsUsed)
			memstats.heapStats.release()

			// Adjust the actual allocs in inconsistent, internal stats.
			// We assumed earlier that the full span gets allocated.
			gcController.totalAlloc.Add(slotsUsed * int64(s.elemsize))

			if s.sweepgen != sg+1 {
				// refill conservatively counted unallocated slots in gcController.heapLive.
				// Undo this.
				//
				// If this span was cached before sweep, then gcController.heapLive was totally
				// recomputed since caching this span, so we don't do this for stale spans.
				dHeapLive -= int64(s.nelems-s.allocCount) * int64(s.elemsize)
			}

			// Release the span to the mcentral.
			mheap_.central[i].mcentral.uncacheSpan(s)
			c.alloc[i] = &emptymspan
		}
	}
	// Clear tinyalloc pool.
	c.tiny = 0
	c.tinyoffset = 0

	// Flush tinyAllocs.
	stats := memstats.heapStats.acquire()
	atomic.Xadd64(&stats.tinyAllocCount, int64(c.tinyAllocs))
	c.tinyAllocs = 0
	memstats.heapStats.release()

	// Clear the reusable linked lists.
	// For noscan objects, the nodes of the linked lists are the reusable heap objects themselves,
	// so we can simply clear the linked list head pointers.
	// TODO(thepudds): consider having debug logging of a non-empty reusable lists getting cleared,
	// maybe based on the existing debugReusableLog.
	clear(c.reusableNoscan[:])

	// For scan objects, we have chunked linked lists. We save the list nodes in c.reusableLinkCache.
	// TODO(thepudds): we currently do not enforce a max count, which simplifies current perf testing, but we need
	// a limit or a different mechanism. This will be tackled in a later CL.
	// TODO(thepudds): could consider tracking tail pointers if we end up with longer list lengths,
	// or maybe we end up with a completely different storage scheme.
	for i := range c.reusableScan {
		if c.reusableScan[i] != 0 {
			// Prepend the list in c.reusableScan[i] to c.reusableLinkCache.
			// First, find the tail. Note we don't clear or reset each link,
			// and instead do that when pulling from the cache.
			rl := (*reusableLink)(unsafe.Pointer(c.reusableScan[i]))
			for rl.next != 0 {
				rl = (*reusableLink)(unsafe.Pointer(rl.next))
			}
			rl.next = c.reusableLinkCache
			c.reusableLinkCache = c.reusableScan[i]
			c.reusableScan[i] = 0
		}
	}

	// Update heapLive and heapScan.
	gcController.update(dHeapLive, scanAlloc)
}

// prepareForSweep flushes c if the system has entered a new sweep phase
// since c was populated. This must happen between the sweep phase
// starting and the first allocation from c.
func (c *mcache) prepareForSweep() {
	// Alternatively, instead of making sure we do this on every P
	// between starting the world and allocating on that P, we
	// could leave allocate-black on, allow allocation to continue
	// as usual, use a ragged barrier at the beginning of sweep to
	// ensure all cached spans are swept, and then disable
	// allocate-black. However, with this approach it's difficult
	// to avoid spilling mark bits into the *next* GC cycle.
	sg := mheap_.sweepgen
	flushGen := c.flushGen.Load()
	if flushGen == sg {
		return
	} else if flushGen != sg-2 {
		println("bad flushGen", flushGen, "in prepareForSweep; sweepgen", sg)
		throw("bad flushGen")
	}
	c.releaseAll()
	stackcache_clear(c)
	c.flushGen.Store(mheap_.sweepgen) // Synchronizes with gcStart
}

// TODO(thepudds): for tracking the set of reusable objects, we currently have two parallel approaches:
//
// 1. For noscan objects: a linked list of heap object, with the next pointers stored
//    in the first word of the formerly live heap objects.
//
// 2. For scan objects: a chunked linked list, with N pointers to heap objects per chunk
//    and the chunks linked together.
//
// The first approach is simpler and likely better performance. Importantly, it also uses
// zero extra memory, and we do not need any cap on how many noscan objects we track.
//
// However, we don't use the first approach for scan objects because a linked list for
// an mspan with types that mostly have user pointers in the first slot
// might have a performance hiccup where the GC could examine one of the objects
// in the free list (for example via a queued pointer during mark, or maybe an unlucky conservative scan),
// and then the GC might then walk and mark the ~complete list of objects
// in the free list by following the next pointers from that (otherwise dead) object,
// while thinking the next pointers are valid user pointers.
//
// One alternative I might try for scan objects is storing the next pointer at the end of the allocation slot
// when there is room, which is likely often the case, especially once you get past the smallest size classes.
// For example, if the user allocates a 1024-byte user object, that ends up in the 1152-byte size class
// (due to the existing type header) with ~120 bytes otherwise unused at the end of the allocation slot.
// A drawback of this is it is not a complete solution because some heap objects do not have spare space,
// so we might still need a chunked list or similar. (We could bump up a size class when needed,
// but that is probably too wasteful. Separately, instead of a footer, when there is naturally
// enough spare space, we could consider sliding the start of the user object an extra 8 bytes
// so that the runtime can then use the second word of the allocation slot for the next pointer,
// which might have better performance than a footer, but might be more complex).
//
// One idea from Michael K. is possibly setting the type header to nil for scan objects > 512 bytes,
// and then use the second word of the heap object for the next pointer. If done properly, the nil
// type header could mean the GC does not interpret the next pointer value as a user pointer.
// The GC will currently throw if it finds an object with a nil type header, though
// we likely could make not complain about that. If we did this approach, we would still need
// a separate solution for <= 512 byte scan objects.

// reusableLink is a node in an chunked linked list of reusable objects,
// used for scan objects.
type reusableLink struct {
	// TODO(thepudds): should this include sys.NotInHeap?

	// next points to the next reusableLink in the list.
	next uintptr
	// len is the number of reusable objects in this link.
	len int32
	// rest is the number of links in the list following this one,
	// used to track max number of links.
	rest int32
	// ptrs contains pointers to reusable objects.
	// The pointer is to the base of the heap object.
	// TODO(thepudds): maybe aim for 64 bytes or 128 bytes (2 cache lines if aligned).
	ptrs [6]uintptr
}

func (c *mcache) allocReusableLink() uintptr {
	var rl *reusableLink
	if c.reusableLinkCache != 0 {
		// Pop from the free list of reusableLinks.
		rl = (*reusableLink)(unsafe.Pointer(c.reusableLinkCache))
		c.reusableLinkCache = rl.next
	} else {
		systemstack(func() {
			lock(&mheap_.reusableLinkLock)
			rl = (*reusableLink)(mheap_.reusableLinkAlloc.alloc())
			unlock(&mheap_.reusableLinkLock)
		})
	}
	// TODO(thepudds): do a general review -- double-check we conservatively clearing next pointers (e.g., after popping).
	rl.next = 0
	rl.len = 0
	rl.rest = 0
	return uintptr(unsafe.Pointer(rl))
}

// addReusableScan adds an object pointer to a chunked linked list of reusable pointers
// for a span class.
func (c *mcache) addReusableScan(spc spanClass, ptr uintptr) {
	if !runtimeFreegcEnabled {
		return
	}

	rl := (*reusableLink)(unsafe.Pointer(c.reusableScan[spc]))
	if rl == nil || rl.len == int32(len(rl.ptrs)) {
		var linkCount int32
		if rl != nil {
			linkCount = rl.rest + 1
			if linkCount == 4 {
				// Currently allow at most 4 links per span class.
				// We are full, so drop this reusable ptr.
				// TODO(thepudds): pick a max. Maybe a max per mcache instead or in addition to a max per span class?
				return
			}
		}
		// Prepend a new link.
		new := (*reusableLink)(unsafe.Pointer(c.allocReusableLink()))
		new.next = uintptr(unsafe.Pointer(rl))
		c.reusableScan[spc] = uintptr(unsafe.Pointer(new))

		new.rest = linkCount
		rl = new
	}
	rl.ptrs[rl.len] = ptr
	rl.len++
}

// addReusableNoscan adds a noscan object pointer to the reusable pointer free list
// for a span class.
func (c *mcache) addReusableNoscan(spc spanClass, ptr uintptr) {
	if !runtimeFreegcEnabled {
		return
	}

	// Add to the reusable pointers free list.
	v := gclinkptr(ptr)
	v.ptr().next = c.reusableNoscan[spc]
	c.reusableNoscan[spc] = v
}

// hasReusableScan reports whether there is a reusable object available for
// a scan spc.
func (c *mcache) hasReusableScan(spc spanClass) bool {
	if !runtimeFreegcEnabled || c.reusableScan[spc] == 0 {
		return false
	}
	rl := (*reusableLink)(unsafe.Pointer(c.reusableScan[spc]))
	return rl.len > 0 || rl.next != 0
}

// hasReusableNoscan reports whether there is a reusable object available for
// a noscan spc.
func (c *mcache) hasReusableNoscan(spc spanClass) bool {
	if !runtimeFreegcEnabled {
		return false
	}
	return c.reusableNoscan[spc] != 0
}

// reusableLink returns a reusableLink containing reusable pointers for the span class.
// It must only be called after hasReusableScan has returned true.
func (c *mcache) reusableLink(spc spanClass) *reusableLink {
	rl := (*reusableLink)(unsafe.Pointer(c.reusableScan[spc]))
	if rl.len == 0 {
		if rl.next == 0 {
			throw("nonEmptyReusableLink: no reusableLink with reusable objects")
		}

		// Make the next link head of the list.
		next := (*reusableLink)(unsafe.Pointer(rl.next))
		c.reusableScan[spc] = uintptr(unsafe.Pointer(next))
		if next.len != int32(len(next.ptrs)) {
			throw("nextReusableScan: next reusable link is not full")
		}

		// Prepend the empty link to reusableLinkCache.
		rl.next = c.reusableLinkCache
		c.reusableLinkCache = uintptr(unsafe.Pointer(rl))

		rl = next
	}
	return rl
}
