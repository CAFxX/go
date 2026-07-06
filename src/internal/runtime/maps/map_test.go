// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package maps_test

import (
	"fmt"
	"internal/abi"
	"internal/runtime/maps"
	"math"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"
	"unsafe"
)

func TestCtrlSize(t *testing.T) {
	cs := unsafe.Sizeof(maps.CtrlGroup(0))
	if cs != abi.MapGroupSlots {
		t.Errorf("ctrlGroup size got %d want abi.MapGroupSlots %d", cs, abi.MapGroupSlots)
	}
}

func TestMapPut(t *testing.T) {
	m, typ := maps.NewTestMap[uint32, uint64](8)

	key := uint32(0)
	elem := uint64(256 + 0)

	for i := 0; i < 31; i++ {
		key += 1
		elem += 1
		m.Put(typ, unsafe.Pointer(&key), unsafe.Pointer(&elem))

		if maps.DebugLog {
			fmt.Printf("After put %d: %v\n", key, m)
		}
	}

	if m.Used() != 31 {
		t.Errorf("Used() used got %d want 31", m.Used())
	}

	key = uint32(0)
	elem = uint64(256 + 0)

	for i := 0; i < 31; i++ {
		key += 1
		elem += 1
		got, ok := m.Get(typ, unsafe.Pointer(&key))
		if !ok {
			t.Errorf("Get(%d) got ok false want true", key)
		}
		gotElem := *(*uint64)(got)
		if gotElem != elem {
			t.Errorf("Get(%d) got elem %d want %d", key, gotElem, elem)
		}
	}
}

func TestSmallMapGrow(t *testing.T) {
	m, typ := maps.NewTestMap[uint32, uint64](8)

	key := uint32(0)
	elem := uint64(256 + 0)

	for i := 0; i < 8; i++ {
		key += 1
		elem += 1
		m.Put(typ, unsafe.Pointer(&key), unsafe.Pointer(&elem))

		if maps.DebugLog {
			fmt.Printf("After put %d: %v\n", key, m)
		}
	}

	if m.TableCount() != 0 {
		t.Errorf("TableCount() got %d want 0", m.TableCount())
	}

	key = uint32(0)
	elem = uint64(256 + 10)

	for i := 0; i < 8; i++ {
		key += 1
		elem += 1
		m.Put(typ, unsafe.Pointer(&key), unsafe.Pointer(&elem))

		if maps.DebugLog {
			fmt.Printf("After put %d: %v\n", key, m)
		}
	}

	if m.TableCount() != 0 {
		t.Errorf("TableCount() got %d want 0", m.TableCount())
	}

	key += 1
	elem += 1
	m.Put(typ, unsafe.Pointer(&key), unsafe.Pointer(&elem))

	if maps.DebugLog {
		fmt.Printf("After put %d: %v\n", key, m)
	}

	if m.TableCount() != 1 {
		t.Errorf("TableCount() got %d want 1", m.TableCount())
	}
}

// Grow enough to cause a table split.
func TestMapSplit(t *testing.T) {
	m, typ := maps.NewTestMap[uint32, uint64](0)

	key := uint32(0)
	elem := uint64(256 + 0)

	for i := 0; i < 2*maps.MaxTableCapacity; i++ {
		key += 1
		elem += 1
		m.Put(typ, unsafe.Pointer(&key), unsafe.Pointer(&elem))

		if maps.DebugLog {
			fmt.Printf("After put %d: %v\n", key, m)
		}
	}

	if m.Used() != 2*maps.MaxTableCapacity {
		t.Errorf("Used() used got %d want 31", m.Used())
	}

	key = uint32(0)
	elem = uint64(256 + 0)

	for i := 0; i < 2*maps.MaxTableCapacity; i++ {
		key += 1
		elem += 1
		got, ok := m.Get(typ, unsafe.Pointer(&key))
		if !ok {
			t.Errorf("Get(%d) got ok false want true", key)
		}
		gotElem := *(*uint64)(got)
		if gotElem != elem {
			t.Errorf("Get(%d) got elem %d want %d", key, gotElem, elem)
		}
	}
}

func TestMapDelete(t *testing.T) {
	m, typ := maps.NewTestMap[uint32, uint64](32)

	key := uint32(0)
	elem := uint64(256 + 0)

	for i := 0; i < 31; i++ {
		key += 1
		elem += 1
		m.Put(typ, unsafe.Pointer(&key), unsafe.Pointer(&elem))

		if maps.DebugLog {
			fmt.Printf("After put %d: %v\n", key, m)
		}
	}

	key = uint32(0)
	elem = uint64(256 + 0)

	for i := 0; i < 31; i++ {
		key += 1
		m.Delete(typ, unsafe.Pointer(&key))
	}

	if m.Used() != 0 {
		t.Errorf("Used() used got %d want 0", m.Used())
	}

	key = uint32(0)
	elem = uint64(256 + 0)

	for i := 0; i < 31; i++ {
		key += 1
		elem += 1
		_, ok := m.Get(typ, unsafe.Pointer(&key))
		if ok {
			t.Errorf("Get(%d) got ok true want false", key)
		}
	}
}

func TestTableClear(t *testing.T) {
	m, typ := maps.NewTestMap[uint32, uint64](32)

	key := uint32(0)
	elem := uint64(256 + 0)

	for i := 0; i < 31; i++ {
		key += 1
		elem += 1
		m.Put(typ, unsafe.Pointer(&key), unsafe.Pointer(&elem))

		if maps.DebugLog {
			fmt.Printf("After put %d: %v\n", key, m)
		}
	}

	m.Clear(typ)

	if m.Used() != 0 {
		t.Errorf("Clear() used got %d want 0", m.Used())
	}

	key = uint32(0)
	elem = uint64(256 + 0)

	for i := 0; i < 31; i++ {
		key += 1
		elem += 1
		_, ok := m.Get(typ, unsafe.Pointer(&key))
		if ok {
			t.Errorf("Get(%d) got ok true want false", key)
		}
	}
}

// +0.0 and -0.0 compare equal, but we must still must update the key slot when
// overwriting.
func TestTableKeyUpdate(t *testing.T) {
	m, typ := maps.NewTestMap[float64, uint64](8)

	zero := float64(0.0)
	negZero := math.Copysign(zero, -1.0)
	elem := uint64(0)

	m.Put(typ, unsafe.Pointer(&zero), unsafe.Pointer(&elem))
	if maps.DebugLog {
		fmt.Printf("After put %f: %v\n", zero, m)
	}

	elem = 1
	m.Put(typ, unsafe.Pointer(&negZero), unsafe.Pointer(&elem))
	if maps.DebugLog {
		fmt.Printf("After put %f: %v\n", negZero, m)
	}

	if m.Used() != 1 {
		t.Errorf("Used() used got %d want 1", m.Used())
	}

	it := new(maps.Iter)
	it.Init(typ, m)
	it.Next()
	keyPtr, elemPtr := it.Key(), it.Elem()
	if keyPtr == nil {
		t.Fatal("it.Key() got nil want key")
	}

	key := *(*float64)(keyPtr)
	elem = *(*uint64)(elemPtr)
	if math.Copysign(1.0, key) > 0 {
		t.Errorf("map key %f has positive sign", key)
	}
	if elem != 1 {
		t.Errorf("map elem got %d want 1", elem)
	}
}

// Put should reuse a deleted slot rather than consuming an empty slot.
func TestTablePutDelete(t *testing.T) {
	// Put will reuse the first deleted slot it encounters.
	//
	// This is awkward to test because Delete will only install ctrlDeleted
	// if the group is full, otherwise it goes straight to empty.
	//
	// So first we must add to the table continuously until we happen to
	// fill a group.

	// Avoid small maps, they have no tables.
	m, typ := maps.NewTestMap[uint32, uint32](16)

	key := uint32(0)
	elem := uint32(256 + 0)

	for {
		key += 1
		elem += 1

		m.Put(typ, unsafe.Pointer(&key), unsafe.Pointer(&elem))

		// Normally a Put that fills a group would fill it with the
		// inserted key, so why search the whole map for a potentially
		// different key in a full group?
		//
		// Put may grow/split a table. Initial construction of the new
		// table(s) could result in a full group consisting of
		// arbitrary keys.
		fullKeyPtr := m.KeyFromFullGroup(typ)
		if fullKeyPtr != nil {
			// Found a full group.
			key = *(*uint32)(fullKeyPtr)
			elem = 256 + key
			break
		}
	}

	// Key is in a full group. Deleting it will result in a ctrlDeleted
	// slot.
	m.Delete(typ, unsafe.Pointer(&key))

	// Re-insert key. This should reuse the deleted slot rather than
	// consuming space.
	tabWant := m.TableFor(typ, unsafe.Pointer(&key))
	growthLeftWant := tabWant.GrowthLeft()

	m.Put(typ, unsafe.Pointer(&key), unsafe.Pointer(&elem))

	tabGot := m.TableFor(typ, unsafe.Pointer(&key))
	growthLeftGot := tabGot.GrowthLeft()

	if tabGot != tabWant {
		// There shouldn't be a grow, as replacing a deleted slot
		// doesn't require more space.
		t.Errorf("Put(%d) grew table got %v want %v map %v", key, tabGot, tabWant, m)
	}

	if growthLeftGot != growthLeftWant {
		t.Errorf("GrowthLeft got %d want %d: map %v tab %v", growthLeftGot, growthLeftWant, m, tabGot)
	}
}

func TestTableIteration(t *testing.T) {
	m, typ := maps.NewTestMap[uint32, uint64](8)

	key := uint32(0)
	elem := uint64(256 + 0)

	for i := 0; i < 31; i++ {
		key += 1
		elem += 1
		m.Put(typ, unsafe.Pointer(&key), unsafe.Pointer(&elem))

		if maps.DebugLog {
			fmt.Printf("After put %d: %v\n", key, m)
		}
	}

	got := make(map[uint32]uint64)

	it := new(maps.Iter)
	it.Init(typ, m)
	for {
		it.Next()
		keyPtr, elemPtr := it.Key(), it.Elem()
		if keyPtr == nil {
			break
		}

		key := *(*uint32)(keyPtr)
		elem := *(*uint64)(elemPtr)
		got[key] = elem
	}

	if len(got) != 31 {
		t.Errorf("Iteration got %d entries, want 31: %+v", len(got), got)
	}

	key = uint32(0)
	elem = uint64(256 + 0)

	for i := 0; i < 31; i++ {
		key += 1
		elem += 1
		gotElem, ok := got[key]
		if !ok {
			t.Errorf("Iteration missing key %d", key)
			continue
		}
		if gotElem != elem {
			t.Errorf("Iteration key %d got elem %d want %d", key, gotElem, elem)
		}
	}
}

// Deleted keys shouldn't be visible in iteration.
func TestTableIterationDelete(t *testing.T) {
	m, typ := maps.NewTestMap[uint32, uint64](8)

	key := uint32(0)
	elem := uint64(256 + 0)

	for i := 0; i < 31; i++ {
		key += 1
		elem += 1
		m.Put(typ, unsafe.Pointer(&key), unsafe.Pointer(&elem))

		if maps.DebugLog {
			fmt.Printf("After put %d: %v\n", key, m)
		}
	}

	got := make(map[uint32]uint64)
	first := true
	deletedKey := uint32(1)
	it := new(maps.Iter)
	it.Init(typ, m)
	for {
		it.Next()
		keyPtr, elemPtr := it.Key(), it.Elem()
		if keyPtr == nil {
			break
		}

		key := *(*uint32)(keyPtr)
		elem := *(*uint64)(elemPtr)
		got[key] = elem

		if first {
			first = false

			// If the key we intended to delete was the one we just
			// saw, pick another to delete.
			if key == deletedKey {
				deletedKey++
			}
			m.Delete(typ, unsafe.Pointer(&deletedKey))
		}
	}

	if len(got) != 30 {
		t.Errorf("Iteration got %d entries, want 30: %+v", len(got), got)
	}

	key = uint32(0)
	elem = uint64(256 + 0)

	for i := 0; i < 31; i++ {
		key += 1
		elem += 1

		wantOK := true
		if key == deletedKey {
			wantOK = false
		}

		gotElem, gotOK := got[key]
		if gotOK != wantOK {
			t.Errorf("Iteration key %d got ok %v want ok %v", key, gotOK, wantOK)
			continue
		}
		if wantOK && gotElem != elem {
			t.Errorf("Iteration key %d got elem %d want %d", key, gotElem, elem)
		}
	}
}

// Deleted keys shouldn't be visible in iteration even after a grow.
func TestTableIterationGrowDelete(t *testing.T) {
	m, typ := maps.NewTestMap[uint32, uint64](8)

	key := uint32(0)
	elem := uint64(256 + 0)

	for i := 0; i < 31; i++ {
		key += 1
		elem += 1
		m.Put(typ, unsafe.Pointer(&key), unsafe.Pointer(&elem))

		if maps.DebugLog {
			fmt.Printf("After put %d: %v\n", key, m)
		}
	}

	got := make(map[uint32]uint64)
	first := true
	deletedKey := uint32(1)
	it := new(maps.Iter)
	it.Init(typ, m)
	for {
		it.Next()
		keyPtr, elemPtr := it.Key(), it.Elem()
		if keyPtr == nil {
			break
		}

		key := *(*uint32)(keyPtr)
		elem := *(*uint64)(elemPtr)
		got[key] = elem

		if first {
			first = false

			// If the key we intended to delete was the one we just
			// saw, pick another to delete.
			if key == deletedKey {
				deletedKey++
			}

			// Double the number of elements to force a grow.
			key := uint32(32)
			elem := uint64(256 + 32)

			for i := 0; i < 31; i++ {
				key += 1
				elem += 1
				m.Put(typ, unsafe.Pointer(&key), unsafe.Pointer(&elem))

				if maps.DebugLog {
					fmt.Printf("After put %d: %v\n", key, m)
				}
			}

			// Then delete from the grown map.
			m.Delete(typ, unsafe.Pointer(&deletedKey))
		}
	}

	// Don't check length: the number of new elements we'll see is
	// unspecified.

	// Check values only of the original pre-iteration entries.
	key = uint32(0)
	elem = uint64(256 + 0)

	for i := 0; i < 31; i++ {
		key += 1
		elem += 1

		wantOK := true
		if key == deletedKey {
			wantOK = false
		}

		gotElem, gotOK := got[key]
		if gotOK != wantOK {
			t.Errorf("Iteration key %d got ok %v want ok %v", key, gotOK, wantOK)
			continue
		}
		if wantOK && gotElem != elem {
			t.Errorf("Iteration key %d got elem %d want %d", key, gotElem, elem)
		}
	}
}

func testTableIterationGrowDuplicate(t *testing.T, grow int) {
	m, typ := maps.NewTestMap[uint32, uint64](8)

	key := uint32(0)
	elem := uint64(256 + 0)

	for i := 0; i < 31; i++ {
		key += 1
		elem += 1
		m.Put(typ, unsafe.Pointer(&key), unsafe.Pointer(&elem))

		if maps.DebugLog {
			fmt.Printf("After put %d: %v\n", key, m)
		}
	}

	got := make(map[uint32]uint64)
	it := new(maps.Iter)
	it.Init(typ, m)
	for i := 0; ; i++ {
		it.Next()
		keyPtr, elemPtr := it.Key(), it.Elem()
		if keyPtr == nil {
			break
		}

		key := *(*uint32)(keyPtr)
		elem := *(*uint64)(elemPtr)
		if elem != 256+uint64(key) {
			t.Errorf("iteration got key %d elem %d want elem %d", key, elem, 256+uint64(key))
		}
		if _, ok := got[key]; ok {
			t.Errorf("iteration got key %d more than once", key)
		}
		got[key] = elem

		// Grow halfway through iteration.
		if i == 16 {
			key := uint32(32)
			elem := uint64(256 + 32)

			for i := 0; i < grow; i++ {
				key += 1
				elem += 1
				m.Put(typ, unsafe.Pointer(&key), unsafe.Pointer(&elem))

				if maps.DebugLog {
					fmt.Printf("After put %d: %v\n", key, m)
				}
			}
		}
	}

	// Don't check length: the number of new elements we'll see is
	// unspecified.
}

// Grow should not allow duplicate keys to appear.
func TestTableIterationGrowDuplicate(t *testing.T) {
	// Small grow, only enough to cause table grow.
	t.Run("grow", func(t *testing.T) { testTableIterationGrowDuplicate(t, 32) })

	// Large grow, to cause table split.
	t.Run("split", func(t *testing.T) { testTableIterationGrowDuplicate(t, 2*maps.MaxTableCapacity) })
}

func TestAlignUpPow2(t *testing.T) {
	tests := []struct {
		in       uint64
		want     uint64
		overflow bool
	}{
		{
			in:   0,
			want: 0,
		},
		{
			in:   3,
			want: 4,
		},
		{
			in:   4,
			want: 4,
		},
		{
			in:   1 << 63,
			want: 1 << 63,
		},
		{
			in:   (1 << 63) - 1,
			want: 1 << 63,
		},
		{
			in:       (1 << 63) + 1,
			overflow: true,
		},
	}

	for _, tc := range tests {
		got, overflow := maps.AlignUpPow2(tc.in)
		if got != tc.want {
			t.Errorf("alignUpPow2(%d) got %d, want %d", tc.in, got, tc.want)
		}
		if overflow != tc.overflow {
			t.Errorf("alignUpPow2(%d) got overflow %v, want %v", tc.in, overflow, tc.overflow)
		}
	}
}

// Verify that a map with zero-size slot is safe to use.
func TestMapZeroSizeSlot(t *testing.T) {
	m, typ := maps.NewTestMap[struct{}, struct{}](16)

	key := struct{}{}
	elem := struct{}{}

	m.Put(typ, unsafe.Pointer(&key), unsafe.Pointer(&elem))

	if maps.DebugLog {
		fmt.Printf("After put %d: %v\n", key, m)
	}

	got, ok := m.Get(typ, unsafe.Pointer(&key))
	if !ok {
		t.Errorf("Get(%d) got ok false want true", key)
	}
	gotElem := *(*struct{})(got)
	if gotElem != elem {
		t.Errorf("Get(%d) got elem %d want %d", key, gotElem, elem)
	}

	tab := m.TableFor(typ, unsafe.Pointer(&key))
	start := tab.GroupsStart()
	length := tab.GroupsLength()
	end := unsafe.Pointer(uintptr(start) + length*typ.GroupSize - 1) // inclusive to ensure we have a valid pointer
	if uintptr(got) < uintptr(start) || uintptr(got) > uintptr(end) {
		t.Errorf("elem address outside groups allocation; got %p want [%p, %p]", got, start, end)
	}
}

func TestMapIndirect(t *testing.T) {
	type big [abi.MapMaxKeyBytes + abi.MapMaxElemBytes]byte

	m, typ := maps.NewTestMap[big, big](8)

	key := big{}
	elem := big{}
	elem[0] = 128

	for i := 0; i < 31; i++ {
		key[0] += 1
		elem[0] += 1
		m.Put(typ, unsafe.Pointer(&key), unsafe.Pointer(&elem))

		if maps.DebugLog {
			fmt.Printf("After put %v: %v\n", key, m)
		}
	}

	if m.Used() != 31 {
		t.Errorf("Used() used got %d want 31", m.Used())
	}

	key = big{}
	elem = big{}
	elem[0] = 128

	for i := 0; i < 31; i++ {
		key[0] += 1
		elem[0] += 1
		got, ok := m.Get(typ, unsafe.Pointer(&key))
		if !ok {
			t.Errorf("Get(%v) got ok false want true", key)
		}
		gotElem := *(*big)(got)
		if gotElem != elem {
			t.Errorf("Get(%v) got elem %v want %v", key, gotElem, elem)
		}
	}
}

// Delete should clear element. See https://go.dev/issue/25936.
func TestMapDeleteClear(t *testing.T) {
	m, typ := maps.NewTestMap[int64, int64](8)

	key := int64(0)
	elem := int64(128)

	m.Put(typ, unsafe.Pointer(&key), unsafe.Pointer(&elem))

	if maps.DebugLog {
		fmt.Printf("After put %d: %v\n", key, m)
	}

	got, ok := m.Get(typ, unsafe.Pointer(&key))
	if !ok {
		t.Errorf("Get(%d) got ok false want true", key)
	}
	gotElem := *(*int64)(got)
	if gotElem != elem {
		t.Errorf("Get(%d) got elem %d want %d", key, gotElem, elem)
	}

	m.Delete(typ, unsafe.Pointer(&key))

	gotElem = *(*int64)(got)
	if gotElem != 0 {
		t.Errorf("Delete(%d) failed to clear element. got %d want 0", key, gotElem)
	}
}

var alwaysFalse bool
var escapeSink any

func escape[T any](x T) T {
	if alwaysFalse {
		escapeSink = x
	}
	return x
}

const (
	belowMax = abi.MapGroupSlots * 3 / 2                                          // 1.5 * group max = 2 groups @ 75%
	atMax    = (2 * abi.MapGroupSlots * maps.MaxAvgGroupLoad) / abi.MapGroupSlots // 2 groups at 7/8 full.
)

func TestTableGroupCount(t *testing.T) {
	// Test that maps of different sizes have the right number of
	// tables/groups.

	type mapCount struct {
		tables int
		groups uint64
	}

	type mapCase struct {
		initialLit  mapCount
		initialHint mapCount
		after       mapCount
	}

	var testCases = []struct {
		n      int     // n is the number of map elements
		escape mapCase // expected values for escaping map
	}{
		{
			n: -(1 << 30),
			escape: mapCase{
				initialLit:  mapCount{0, 0},
				initialHint: mapCount{0, 0},
				after:       mapCount{0, 0},
			},
		},
		{
			n: -1,
			escape: mapCase{
				initialLit:  mapCount{0, 0},
				initialHint: mapCount{0, 0},
				after:       mapCount{0, 0},
			},
		},
		{
			n: 0,
			escape: mapCase{
				initialLit:  mapCount{0, 0},
				initialHint: mapCount{0, 0},
				after:       mapCount{0, 0},
			},
		},
		{
			n: 1,
			escape: mapCase{
				initialLit:  mapCount{0, 0},
				initialHint: mapCount{0, 0},
				after:       mapCount{0, 1},
			},
		},
		{
			n: abi.MapGroupSlots,
			escape: mapCase{
				initialLit:  mapCount{0, 0},
				initialHint: mapCount{0, 0},
				after:       mapCount{0, 1},
			},
		},
		{
			n: abi.MapGroupSlots + 1,
			escape: mapCase{
				initialLit:  mapCount{0, 0},
				initialHint: mapCount{1, 2},
				after:       mapCount{1, 2},
			},
		},
		{
			n: belowMax, // 1.5 group max = 2 groups @ 75%
			escape: mapCase{
				initialLit:  mapCount{0, 0},
				initialHint: mapCount{1, 2},
				after:       mapCount{1, 2},
			},
		},
		{
			n: atMax, // 2 groups at max
			escape: mapCase{
				initialLit:  mapCount{0, 0},
				initialHint: mapCount{1, 2},
				after:       mapCount{1, 2},
			},
		},
		{
			n: atMax + 1, // 2 groups at max + 1 -> grow to 4 groups
			escape: mapCase{
				initialLit:  mapCount{0, 0},
				initialHint: mapCount{1, 4},
				after:       mapCount{1, 4},
			},
		},
		{
			n: 2 * belowMax, // 3 * group max = 4 groups @75%
			escape: mapCase{
				initialLit:  mapCount{0, 0},
				initialHint: mapCount{1, 4},
				after:       mapCount{1, 4},
			},
		},
		{
			n: 2*atMax + 1, // 4 groups at max + 1 -> grow to 8 groups
			escape: mapCase{
				initialLit:  mapCount{0, 0},
				initialHint: mapCount{1, 8},
				after:       mapCount{1, 8},
			},
		},
	}

	testMap := func(t *testing.T, m map[int]int, n int, initial, after mapCount) {
		mm := *(**maps.Map)(unsafe.Pointer(&m))

		gotTab := mm.TableCount()
		if gotTab != initial.tables {
			t.Errorf("initial TableCount got %d want %d", gotTab, initial.tables)
		}

		gotGroup := mm.GroupCount()
		if gotGroup != initial.groups {
			t.Errorf("initial GroupCount got %d want %d", gotGroup, initial.groups)
		}

		for i := 0; i < n; i++ {
			m[i] = i
		}

		gotTab = mm.TableCount()
		if gotTab != after.tables {
			t.Errorf("after TableCount got %d want %d", gotTab, after.tables)
		}

		gotGroup = mm.GroupCount()
		if gotGroup != after.groups {
			t.Errorf("after GroupCount got %d want %d", gotGroup, after.groups)
		}
	}

	t.Run("mapliteral", func(t *testing.T) {
		for _, tc := range testCases {
			t.Run(fmt.Sprintf("n=%d", tc.n), func(t *testing.T) {
				t.Run("escape", func(t *testing.T) {
					m := escape(map[int]int{})
					testMap(t, m, tc.n, tc.escape.initialLit, tc.escape.after)
				})
			})
		}
	})
	t.Run("nohint", func(t *testing.T) {
		for _, tc := range testCases {
			t.Run(fmt.Sprintf("n=%d", tc.n), func(t *testing.T) {
				t.Run("escape", func(t *testing.T) {
					m := escape(make(map[int]int))
					testMap(t, m, tc.n, tc.escape.initialLit, tc.escape.after)
				})
			})
		}
	})
	t.Run("makemap", func(t *testing.T) {
		for _, tc := range testCases {
			t.Run(fmt.Sprintf("n=%d", tc.n), func(t *testing.T) {
				t.Run("escape", func(t *testing.T) {
					m := escape(make(map[int]int, tc.n))
					testMap(t, m, tc.n, tc.escape.initialHint, tc.escape.after)
				})
			})
		}
	})
	t.Run("makemap64", func(t *testing.T) {
		for _, tc := range testCases {
			t.Run(fmt.Sprintf("n=%d", tc.n), func(t *testing.T) {
				t.Run("escape", func(t *testing.T) {
					m := escape(make(map[int]int, int64(tc.n)))
					testMap(t, m, tc.n, tc.escape.initialHint, tc.escape.after)
				})
			})
		}
	})
}

func TestTombstoneGrow(t *testing.T) {
	tableSizes := []int{16, 32, 64, 128, 256}
	for _, tableSize := range tableSizes {
		for _, load := range []string{"low", "mid", "high"} {
			capacity := tableSize * 7 / 8
			var initialElems int
			switch load {
			case "low":
				initialElems = capacity / 8
			case "mid":
				initialElems = capacity / 2
			case "high":
				initialElems = capacity
			}
			t.Run(fmt.Sprintf("tableSize=%d/elems=%d/load=%0.3f", tableSize, initialElems, float64(initialElems)/float64(tableSize)), func(t *testing.T) {
				allocs := testing.AllocsPerRun(1, func() {
					// Fill the map with elements.
					m := make(map[int]int, capacity)
					for i := range initialElems {
						m[i] = i
					}

					// This is the heart of our test.
					// Loop over the map repeatedly, deleting a key then adding a not-yet-seen key
					// while keeping the map at a ~constant number of elements (+/-1).
					nextKey := initialElems
					for range 100000 {
						for k := range m {
							delete(m, k)
							break
						}
						m[nextKey] = nextKey
						nextKey++
						if len(m) != initialElems {
							t.Fatal("len(m) should remain constant")
						}
					}
				})

				// The make has 4 allocs (map, directory, table, groups).
				// Each growth has 2 allocs (table, groups).
				// We allow two growths if we start full, 1 otherwise.
				// Fail (somewhat arbitrarily) if there are more than that.
				allowed := float64(4 + 1*2)
				if initialElems == capacity {
					allowed += 2
				}
				if allocs > allowed {
					t.Fatalf("got %v allocations, allowed %v", allocs, allowed)
				}
			})
		}
	}
}

// fillMap inserts keys [0,n) with value==key.
func fillMap(m *maps.Map, typ *abi.MapType, n int) {
	for i := range n {
		key, elem := i, i
		m.Put(typ, unsafe.Pointer(&key), unsafe.Pointer(&elem))
	}
}

func deleteRange(m *maps.Map, typ *abi.MapType, lo, hi int) {
	for i := lo; i < hi; i++ {
		key := i
		m.Delete(typ, unsafe.Pointer(&key))
	}
}

// TestDeleteShrink checks that a table which stays sparse for
// ShrinkAgeCycles GC cycles is rebuilt smaller on a subsequent delete,
// that the shrink does not happen before it has aged, and that it is
// gated by shrinkEnabled.
func TestDeleteShrink(t *testing.T) {
	const n = 512

	newSparseMap := func() (*maps.Map, *abi.MapType, uint64) {
		// No size hint, so the tables carry no shrink floor and grow
		// organically to a single multi-group table.
		m, typ := maps.NewTestMap[int, int](0)
		fillMap(m, typ, n)
		full := m.TotalCapacity()
		if full == 0 {
			t.Fatal("map did not grow to a table")
		}
		return m, typ, full
	}

	// Deterministic GC-cycle clock.
	var cycle uint32
	maps.SetGCCycleForTest(func() uint32 { return cycle })
	defer maps.ResetGCCycleForTest()

	t.Run("shrinks after aging", func(t *testing.T) {
		cycle = 0
		m, typ, full := newSparseMap()

		// Drain to well under 1/4 load at a fixed cycle. The delete that
		// crosses the threshold records the stamp; none of these deletes
		// should shrink, because no GC cycle has elapsed.
		deleteRange(m, typ, 0, n-8)
		if got := m.TotalCapacity(); got != full {
			t.Fatalf("capacity changed before aging: got %d, want %d", got, full)
		}

		// Age past the threshold; the next qualifying delete shrinks.
		cycle += maps.ShrinkAgeCycles
		deleteRange(m, typ, n-8, n-7)
		if got := m.TotalCapacity(); got >= full {
			t.Fatalf("capacity did not shrink: got %d, want < %d", got, full)
		}

		// Surviving entries must still be present.
		for i := n - 7; i < n; i++ {
			key := i
			if _, ok := m.Get(typ, unsafe.Pointer(&key)); !ok {
				t.Fatalf("missing key %d after shrink", i)
			}
		}
	})

	t.Run("no shrink when disabled", func(t *testing.T) {
		cycle = 0
		m, typ, full := newSparseMap()
		maps.SetShrinkEnabledForTest(false)
		defer maps.SetShrinkEnabledForTest(true)

		deleteRange(m, typ, 0, n-8)
		cycle += maps.ShrinkAgeCycles + 1
		deleteRange(m, typ, n-8, n-1)
		if got := m.TotalCapacity(); got != full {
			t.Fatalf("capacity shrank with shrinking disabled: got %d, want %d", got, full)
		}
	})

	t.Run("respects the size-hint floor", func(t *testing.T) {
		// A presized map carries a per-table floor (log2 of its initial
		// capacity); an unhinted map carries none. Draining both must not
		// take the hinted one below its floor (R6).
		cycle = 0
		hinted, htyp := maps.NewTestMap[int, int](n)
		fillMap(hinted, htyp, n)
		floor := hinted.TotalCapacity()

		unhinted, utyp, _ := newSparseMap()

		deleteRange(hinted, htyp, 0, n-4)
		deleteRange(unhinted, utyp, 0, n-4)
		cycle += maps.ShrinkAgeCycles
		deleteRange(hinted, htyp, n-4, n-3)
		deleteRange(unhinted, utyp, n-4, n-3)

		// The unhinted map shrinks toward its few survivors; the hinted one
		// is floored and must stay at its presized capacity.
		if got := hinted.TotalCapacity(); got != floor {
			t.Fatalf("hinted map shrank below its floor: got %d, want %d", got, floor)
		}
		if uc := unhinted.TotalCapacity(); uc >= floor {
			t.Fatalf("unhinted map did not shrink past the hinted floor: got %d, floor %d", uc, floor)
		}
	})
}

// TestCloneIndirectNoAlias checks that Clone's rebuild path (taken when the
// source has tombstones) deep-copies indirect (>128B) keys and elems rather
// than aliasing the source's per-entry storage. Aliasing would let a write to
// the clone mutate the source.
func TestCloneIndirectNoAlias(t *testing.T) {
	type big [200]byte // >128B => stored indirectly
	src, typ := maps.NewTestMap[int, big](0)
	const n = 300
	for i := range n {
		key, elem := i, big{byte(i)}
		src.Put(typ, unsafe.Pointer(&key), unsafe.Pointer(&elem))
	}
	for i := range n / 2 {
		key := i
		src.Delete(typ, unsafe.Pointer(&key)) // leave tombstones -> rebuild path
	}

	clone := src.Clone(typ)

	// Overwrite a surviving key's element in the clone only.
	key := n - 1
	newElem := big{0xAA}
	clone.Put(typ, unsafe.Pointer(&key), unsafe.Pointer(&newElem))

	got, ok := src.Get(typ, unsafe.Pointer(&key))
	if !ok {
		t.Fatalf("source lost key %d", key)
	}
	if (*big)(got)[0] != byte(key) {
		t.Fatalf("clone write aliased source: source[%d][0] = %#x, want %#x",
			key, (*big)(got)[0], byte(key))
	}
}

// TestCloneStaleSourceIndex checks that Clone does not depend on the source
// tables' index fields: a background shrink install can mark them stale
// (index = -1) while a Clone is suspended at a stop-the-world, and the clone
// must still come out with correct indices (growing or iterating a clone
// built from stale-marked tables would otherwise corrupt its directory).
func TestCloneStaleSourceIndex(t *testing.T) {
	src, typ := maps.NewTestMap[int, int](0)
	const n = 4 * maps.MaxTableCapacity // several tables, multi-entry directory
	fillMap(src, typ, n)
	// Delete a few entries to give some tables tombstones (so they take the
	// rebuild path) while keeping the map dense enough that Clone keeps the
	// per-table copy rather than rebuilding the whole directory.
	const deleted = maps.MaxTableCapacity / 4
	deleteRange(src, typ, 0, deleted)
	if src.TableCount() == 0 {
		t.Fatal("map has no directory")
	}

	// Simulate the concurrent-install state.
	src.MarkTablesStaleForTest()

	clone := src.Clone(typ)

	// The clone's directory invariant must hold regardless of the source's
	// index fields: replacing a table with a bad recorded index would write
	// outside the directory.
	if i := clone.BadTableIndexForTest(); i >= 0 {
		t.Fatalf("clone table at slot %d has a wrong index", i)
	}

	// Iterate the clone: a table cloned with index -1 would derail the
	// iterator's directory arithmetic.
	var it maps.Iter
	it.Init(typ, clone)
	got := 0
	for it.Next(); it.Key() != nil; it.Next() {
		got++
	}
	if want := n - deleted; got != want {
		t.Fatalf("clone iteration returned %d entries, want %d", got, want)
	}

	// Grow the clone well past its capacity: replacing a table whose index
	// is -1 would write outside the directory.
	for i := n; i < 3*n; i++ {
		key, elem := i, i
		clone.Put(typ, unsafe.Pointer(&key), unsafe.Pointer(&elem))
	}
	for i := n; i < 3*n; i++ {
		key := i
		if _, ok := clone.Get(typ, unsafe.Pointer(&key)); !ok {
			t.Fatalf("missing key %d after growing the clone", i)
		}
	}
}

// TestCloneCompact checks that Clone compacts: a sparse or presized source
// yields a smaller, entry-preserving, independent clone, while a dense
// source is copied faithfully.
func TestCloneCompact(t *testing.T) {
	present := func(t *testing.T, m *maps.Map, typ *abi.MapType, lo, hi int) {
		t.Helper()
		for i := lo; i < hi; i++ {
			key := i
			if _, ok := m.Get(typ, unsafe.Pointer(&key)); !ok {
				t.Fatalf("missing key %d", i)
			}
		}
	}

	t.Run("sparse source is compacted and unhinted", func(t *testing.T) {
		// A large size hint gives the source a capacity floor; filling
		// only a few entries leaves it sparse.
		src, typ := maps.NewTestMap[int, int](512)
		fillMap(src, typ, 10)
		srcCap := src.TotalCapacity()

		clone := src.Clone(typ)
		if got := clone.TotalCapacity(); got >= srcCap {
			t.Fatalf("clone not compacted: got %d, source %d", got, srcCap)
		}
		present(t, clone, typ, 0, 10)
		present(t, src, typ, 0, 10) // source unchanged
		if got := src.TotalCapacity(); got != srcCap {
			t.Fatalf("source capacity changed: got %d, want %d", got, srcCap)
		}

		// The clone carries no floor, so it is itself shrinkable: a
		// second clone of the (already tight) clone does not grow it.
		if got := clone.Clone(typ).TotalCapacity(); got > clone.TotalCapacity() {
			t.Fatalf("re-clone grew: got %d, want <= %d", got, clone.TotalCapacity())
		}
	})

	t.Run("clone does not inherit background-shrink state", func(t *testing.T) {
		src, typ := maps.NewTestMap[int, int](0)
		const n = 512
		fillMap(src, typ, n)
		// Drain below the shrink threshold; the crossing delete marks the
		// source as enqueued for background shrinking.
		for i := 0; i < n-8; i++ {
			key := i
			src.Delete(typ, unsafe.Pointer(&key))
		}
		if !src.Enqueued() {
			t.Skip("source not enqueued (background shrinking disabled?)")
		}
		// The clone is a fresh map no queue entry refers to: it must not
		// inherit the enqueued bit, or it could never be re-enqueued.
		if clone := src.Clone(typ); clone.Enqueued() {
			t.Fatal("clone inherited the enqueued bit from its source")
		}
	})

	t.Run("dense source copied faithfully", func(t *testing.T) {
		src, typ := maps.NewTestMap[int, int](0) // organic growth, no floor
		const n = 400
		fillMap(src, typ, n)

		clone := src.Clone(typ)
		present(t, clone, typ, 0, n)

		// Independence: deleting from the clone must not affect the source.
		key := 0
		clone.Delete(typ, unsafe.Pointer(&key))
		if _, ok := clone.Get(typ, unsafe.Pointer(&key)); ok {
			t.Fatal("key 0 not deleted from clone")
		}
		if _, ok := src.Get(typ, unsafe.Pointer(&key)); !ok {
			t.Fatal("deleting from clone affected source")
		}
	})
}

// skipIfBgShrinkDisabled skips the test when GODEBUG disables background map
// shrinking. GODEBUG is last-setting-wins, so scan the whole list.
func skipIfBgShrinkDisabled(t *testing.T) {
	t.Helper()
	enabled := true
	for _, kv := range strings.Split(os.Getenv("GODEBUG"), ",") {
		switch kv {
		case "mapbgshrink=0":
			enabled = false
		case "mapbgshrink=1":
			enabled = true
		}
	}
	if !enabled {
		t.Skip("background map shrinking disabled by GODEBUG")
	}
}

// waitForBackgroundShrink drives GC cycles until m's total capacity drops
// below full, failing the test if it does not happen within a generous
// deadline. The background pipeline advances at most two phases per cycle
// (one at the worker, one at mark termination); the sleeps let the
// concurrent worker get scheduled.
func waitForBackgroundShrink(t *testing.T, m *maps.Map, full uint64) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for m.TotalCapacity() == full {
		if time.Now().After(deadline) {
			t.Fatalf("map did not shrink in the background: capacity still %d", full)
		}
		runtime.GC()
		time.Sleep(10 * time.Millisecond)
	}
	if shrunk := m.TotalCapacity(); shrunk >= full {
		t.Fatalf("capacity did not decrease: got %d, want < %d", shrunk, full)
	}
}

// TestBackgroundShrink checks the end-to-end idle-map pipeline: a heap map
// that is drained and then left idle is rebuilt smaller by the background
// worker within a few GC cycles, with no further operations on the map.
func TestBackgroundShrink(t *testing.T) {
	skipIfBgShrinkDisabled(t)

	// Pin the delete-path shrink clock so the drain below never triggers
	// synchronous shrinking; the background worker must be what shrinks.
	maps.SetGCCycleForTest(func() uint32 { return 0 })
	defer maps.ResetGCCycleForTest()

	m, typ := maps.NewTestMap[int, int](0)
	const n = 4 * maps.MaxTableCapacity
	fillMap(m, typ, n)
	full := m.TotalCapacity()
	if full == 0 {
		t.Fatal("map did not grow to tables")
	}

	// Drain fast, leaving a few entries. The last delete enqueues the map
	// for background shrinking.
	for i := 0; i < n-8; i++ {
		key := i
		m.Delete(typ, unsafe.Pointer(&key))
	}
	if got := m.TotalCapacity(); got != full {
		t.Fatalf("capacity changed during fast drain: got %d, want %d", got, full)
	}

	waitForBackgroundShrink(t, m, full)

	// The survivors must still be there, and the map must still work.
	for i := n - 8; i < n; i++ {
		key := i
		if _, ok := m.Get(typ, unsafe.Pointer(&key)); !ok {
			t.Fatalf("missing key %d after background shrink", i)
		}
	}
	for i := 0; i < 64; i++ {
		key, elem := i, i
		m.Put(typ, unsafe.Pointer(&key), unsafe.Pointer(&elem))
	}
	for i := 0; i < 64; i++ {
		key := i
		if _, ok := m.Get(typ, unsafe.Pointer(&key)); !ok {
			t.Fatalf("missing key %d after post-shrink inserts", i)
		}
	}
}

// TestBackgroundShrinkAfterClear checks the clear-then-idle path
// (go.dev/issue/70617): a map that is cleared keeps its tables, and the
// background worker reclaims them once the map goes idle.
func TestBackgroundShrinkAfterClear(t *testing.T) {
	skipIfBgShrinkDisabled(t)

	m, typ := maps.NewTestMap[int, int](0)
	const n = 4 * maps.MaxTableCapacity
	fillMap(m, typ, n)
	full := m.TotalCapacity()
	if full == 0 {
		t.Fatal("map did not grow to tables")
	}

	m.Clear(typ)
	if !m.Enqueued() {
		t.Fatal("Clear did not enqueue the map for background shrinking")
	}
	// Clear keeps the tables, so the capacity is unchanged until the worker
	// runs.
	if got := m.TotalCapacity(); got != full {
		t.Fatalf("Clear changed capacity: got %d, want %d", got, full)
	}

	waitForBackgroundShrink(t, m, full)

	// The map must still be empty and usable.
	for i := 0; i < 64; i++ {
		key, elem := i, i
		m.Put(typ, unsafe.Pointer(&key), unsafe.Pointer(&elem))
	}
	for i := 0; i < 64; i++ {
		key := i
		if _, ok := m.Get(typ, unsafe.Pointer(&key)); !ok {
			t.Fatalf("missing key %d after clear+shrink+insert", i)
		}
	}
}

// TestBackgroundShrinkStress hammers maps with writes while the background
// shrink pipeline races against them under GC pressure. String keys make torn
// reads consequential (a mis-read string header would fault), so this
// exercises the crash-safety of the concurrent snapshot and the
// discard-on-write validation. Run with -race in CI for the full effect.
func TestBackgroundShrinkStress(t *testing.T) {
	if testing.Short() {
		t.Skip("stress test")
	}

	const (
		nMaps    = 8
		nKeys    = 2048
		duration = 2 * time.Second
	)

	type mt struct {
		m   *maps.Map
		typ *abi.MapType
	}
	ms := make([]mt, nMaps)
	keys := make([]string, nKeys)
	for i := range keys {
		keys[i] = fmt.Sprintf("key-%d-with-some-length-%d", i, i*i)
	}
	for i := range ms {
		m, typ := maps.NewTestMap[string, int](0)
		ms[i] = mt{m, typ}
	}

	stop := make(chan struct{})
	done := make(chan struct{}, nMaps+1)

	// Writers: fill and drain their map repeatedly. Every drain enqueues
	// the map; most snapshots race a subsequent refill and must be
	// discarded, not crash.
	for i := range ms {
		go func(x mt) {
			defer func() { done <- struct{}{} }()
			for {
				select {
				case <-stop:
					return
				default:
				}
				for k := range nKeys {
					elem := k
					x.m.Put(x.typ, unsafe.Pointer(&keys[k]), unsafe.Pointer(&elem))
				}
				for k := range nKeys - 4 {
					x.m.Delete(x.typ, unsafe.Pointer(&keys[k]))
				}
			}
		}(ms[i])
	}

	// GC pressure: keep cycles coming so the worker and STW hooks run.
	go func() {
		defer func() { done <- struct{}{} }()
		for {
			select {
			case <-stop:
				return
			default:
			}
			runtime.GC()
			time.Sleep(time.Millisecond)
		}
	}()

	time.Sleep(duration)
	close(stop)
	for range nMaps + 1 {
		<-done
	}

	// Every map must still hold exactly its surviving entries.
	for i, x := range ms {
		for k := nKeys - 4; k < nKeys; k++ {
			if _, ok := x.m.Get(x.typ, unsafe.Pointer(&keys[k])); !ok {
				t.Fatalf("map %d missing surviving key %q", i, keys[k])
			}
		}
	}
}

// TestCloneCompactDirectory checks that cloning a map whose directory grew
// via splits and then drained produces a clone with a smaller directory.
func TestCloneCompactDirectory(t *testing.T) {
	// Keep the source's split structure while draining: the sync shrink
	// would otherwise resize tables (never the directory) as we delete.
	maps.SetShrinkEnabledForTest(false)
	defer maps.SetShrinkEnabledForTest(true)

	m, typ := maps.NewTestMap[int, int](0)
	const n = 4 * maps.MaxTableCapacity
	fillMap(m, typ, n)
	srcDir := m.TableCount()
	if srcDir < 2 {
		t.Fatalf("source did not split the directory: TableCount=%d", srcDir)
	}

	// Drain to a handful of entries.
	const keep = 4
	for i := 0; i < n-keep; i++ {
		key := i
		m.Delete(typ, unsafe.Pointer(&key))
	}

	clone := m.Clone(typ)
	if clone.TableCount() >= srcDir {
		t.Fatalf("clone directory not compacted: clone TableCount=%d, source=%d",
			clone.TableCount(), srcDir)
	}
	for i := n - keep; i < n; i++ {
		key := i
		if _, ok := clone.Get(typ, unsafe.Pointer(&key)); !ok {
			t.Fatalf("clone missing key %d", i)
		}
	}
	// Source is unchanged (still the large directory).
	if m.TableCount() != srcDir {
		t.Fatalf("source directory changed: got %d want %d", m.TableCount(), srcDir)
	}
}

// BenchmarkMapDeleteChurn measures the delete path's steady-state cost,
// including the shrink-eligibility hook, on a map held near full load so it
// never actually shrinks (the hook's non-shrinking overhead).
func BenchmarkMapDeleteChurn(b *testing.B) {
	m, typ := maps.NewTestMap[int, int](0)
	const n = 1024
	fillMap(m, typ, n)
	b.ReportAllocs()
	i := 0
	for b.Loop() {
		key := i & (n - 1)
		m.Delete(typ, unsafe.Pointer(&key))
		elem := key
		m.Put(typ, unsafe.Pointer(&key), unsafe.Pointer(&elem))
		i++
	}
}

// BenchmarkMapDrainRefill measures a full drain (which shrinks the tables once
// eligible) followed by a refill (which regrows them): the churn cost the
// shrink floor and background backoff are meant to bound.
func BenchmarkMapDrainRefill(b *testing.B) {
	const n = 1024
	var cycle uint32
	maps.SetGCCycleForTest(func() uint32 { return cycle })
	defer maps.ResetGCCycleForTest()
	m, typ := maps.NewTestMap[int, int](0)
	b.ReportAllocs()
	for b.Loop() {
		fillMap(m, typ, n)
		cycle += maps.ShrinkAgeCycles + 1
		deleteRange(m, typ, 0, n)
	}
}
