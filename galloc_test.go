// Copyright (c) 2025 Andrey Kolkov and GoGPU Contributors
// SPDX-License-Identifier: MIT

package galloc

import (
	"sync"
	"testing"
)

const (
	testSize256MB = 1024 * 1024 * 256
	testMaxAllocs = 1024
)

func TestAllocateBasic(t *testing.T) {
	a := New(testSize256MB, testMaxAllocs)
	alloc := a.Allocate(1337)
	if alloc.Failed() {
		t.Fatal("Allocate(1337) failed")
	}
	if alloc.Offset != 0 {
		t.Errorf("first allocation offset = %d, want 0", alloc.Offset)
	}
	a.Free(alloc)
}

func TestAllocateMultiple(t *testing.T) {
	a := New(testSize256MB, testMaxAllocs)

	// Zero-size allocation at offset 0.
	alloc0 := a.Allocate(0)
	if alloc0.Failed() {
		t.Fatal("Allocate(0) failed")
	}
	if alloc0.Offset != 0 {
		t.Errorf("alloc0.Offset = %d, want 0", alloc0.Offset)
	}

	alloc1 := a.Allocate(1)
	if alloc1.Failed() {
		t.Fatal("Allocate(1) failed")
	}
	if alloc1.Offset != 0 {
		t.Errorf("alloc1.Offset = %d, want 0", alloc1.Offset)
	}

	alloc123 := a.Allocate(123)
	if alloc123.Failed() {
		t.Fatal("Allocate(123) failed")
	}
	if alloc123.Offset != 1 {
		t.Errorf("alloc123.Offset = %d, want 1", alloc123.Offset)
	}

	alloc1234 := a.Allocate(1234)
	if alloc1234.Failed() {
		t.Fatal("Allocate(1234) failed")
	}
	if alloc1234.Offset != 124 {
		t.Errorf("alloc1234.Offset = %d, want 124", alloc1234.Offset)
	}

	a.Free(alloc0)
	a.Free(alloc1)
	a.Free(alloc123)
	a.Free(alloc1234)

	// After freeing all, the entire range should be available as one region.
	validateAll := a.Allocate(testSize256MB)
	if validateAll.Failed() {
		t.Fatal("full re-allocation after free-all failed")
	}
	if validateAll.Offset != 0 {
		t.Errorf("full re-allocation offset = %d, want 0", validateAll.Offset)
	}
	a.Free(validateAll)
}

func TestFreeAndRealloc(t *testing.T) {
	a := New(testSize256MB, testMaxAllocs)

	alloc := a.Allocate(1337)
	if alloc.Offset != 0 {
		t.Errorf("first alloc offset = %d, want 0", alloc.Offset)
	}
	a.Free(alloc)

	alloc2 := a.Allocate(1337)
	if alloc2.Offset != 0 {
		t.Errorf("realloc offset = %d, want 0", alloc2.Offset)
	}
	a.Free(alloc2)

	// Validate no fragmentation.
	validateAll := a.Allocate(testSize256MB)
	if validateAll.Failed() {
		t.Fatal("full re-allocation failed")
	}
	if validateAll.Offset != 0 {
		t.Errorf("full re-allocation offset = %d, want 0", validateAll.Offset)
	}
	a.Free(validateAll)
}

func TestCoalesceTwoBlocks(t *testing.T) {
	a := New(testSize256MB, testMaxAllocs)

	allocA := a.Allocate(1024)
	allocB := a.Allocate(3456)

	if allocA.Offset != 0 {
		t.Errorf("allocA.Offset = %d, want 0", allocA.Offset)
	}
	if allocB.Offset != 1024 {
		t.Errorf("allocB.Offset = %d, want 1024", allocB.Offset)
	}

	a.Free(allocA)

	// Reuse A's slot exactly.
	allocC := a.Allocate(1024)
	if allocC.Offset != 0 {
		t.Errorf("allocC.Offset = %d, want 0", allocC.Offset)
	}

	a.Free(allocC)
	a.Free(allocB)

	// After freeing all, verify clean state.
	validateAll := a.Allocate(testSize256MB)
	if validateAll.Failed() {
		t.Fatal("full re-allocation failed")
	}
	if validateAll.Offset != 0 {
		t.Errorf("full re-allocation offset = %d, want 0", validateAll.Offset)
	}
	a.Free(validateAll)
}

func TestCoalesceThreeBlocks(t *testing.T) {
	a := New(testSize256MB, testMaxAllocs)

	allocA := a.Allocate(1024)
	allocB := a.Allocate(1024)
	allocC := a.Allocate(1024)

	if allocA.Offset != 0 {
		t.Errorf("allocA.Offset = %d, want 0", allocA.Offset)
	}
	if allocB.Offset != 1024 {
		t.Errorf("allocB.Offset = %d, want 1024", allocB.Offset)
	}
	if allocC.Offset != 2048 {
		t.Errorf("allocC.Offset = %d, want 2048", allocC.Offset)
	}

	// Free middle first, then neighbors. All three should coalesce.
	a.Free(allocB)
	a.Free(allocA)
	a.Free(allocC)

	// After coalescing all three, a 3072-byte allocation should succeed at offset 0.
	alloc3 := a.Allocate(3072)
	if alloc3.Failed() {
		t.Fatal("3072-byte allocation after three-way coalesce failed")
	}
	if alloc3.Offset != 0 {
		t.Errorf("coalesced allocation offset = %d, want 0", alloc3.Offset)
	}
	a.Free(alloc3)
}

func TestFragmentationResistance(t *testing.T) {
	a := New(testSize256MB, testMaxAllocs)

	// Allocate 256 x 1MB chunks.
	allocs := make([]Allocation, 256)
	for i := range allocs {
		allocs[i] = a.Allocate(1024 * 1024)
		if allocs[i].Failed() {
			t.Fatalf("alloc[%d] failed", i)
		}
		if allocs[i].Offset != uint32(i)*1024*1024 {
			t.Errorf("alloc[%d].Offset = %d, want %d", i, allocs[i].Offset, uint32(i)*1024*1024)
		}
	}

	report := a.StorageReport()
	if report.TotalFreeSpace != 0 {
		t.Errorf("TotalFreeSpace = %d, want 0", report.TotalFreeSpace)
	}

	// Free four random slots.
	a.Free(allocs[243])
	a.Free(allocs[5])
	a.Free(allocs[123])
	a.Free(allocs[95])

	// Free four contiguous slots (allocator must merge).
	a.Free(allocs[151])
	a.Free(allocs[152])
	a.Free(allocs[153])
	a.Free(allocs[154])

	// Reallocate the four random slots (1MB each).
	allocs[243] = a.Allocate(1024 * 1024)
	allocs[5] = a.Allocate(1024 * 1024)
	allocs[123] = a.Allocate(1024 * 1024)
	allocs[95] = a.Allocate(1024 * 1024)

	// Allocate 4MB in the contiguous hole.
	allocs[151] = a.Allocate(1024 * 1024 * 4)

	if allocs[243].Failed() || allocs[5].Failed() || allocs[123].Failed() || allocs[95].Failed() || allocs[151].Failed() {
		t.Fatal("re-allocation in fragmented space failed")
	}

	// Free everything except the contiguous range we replaced.
	for i, alloc := range allocs {
		if i >= 152 && i <= 154 {
			continue // these slots are now part of the 4MB block at allocs[151]
		}
		a.Free(alloc)
	}

	report2 := a.StorageReport()
	if report2.TotalFreeSpace != testSize256MB {
		t.Errorf("TotalFreeSpace = %d, want %d", report2.TotalFreeSpace, testSize256MB)
	}
	if report2.LargestFreeRegion != testSize256MB {
		t.Errorf("LargestFreeRegion = %d, want %d", report2.LargestFreeRegion, testSize256MB)
	}

	// Validate clean: allocate entire range.
	validateAll := a.Allocate(testSize256MB)
	if validateAll.Failed() {
		t.Fatal("full re-allocation after defrag failed")
	}
	if validateAll.Offset != 0 {
		t.Errorf("full re-allocation offset = %d, want 0", validateAll.Offset)
	}
	a.Free(validateAll)
}

func TestAllocateWhenFull(t *testing.T) {
	a := New(1024, 256)

	// Fill completely.
	alloc := a.Allocate(1024)
	if alloc.Failed() {
		t.Fatal("Allocate(1024) failed on empty allocator")
	}

	// Should fail gracefully, no panic.
	alloc2 := a.Allocate(1)
	if !alloc2.Failed() {
		t.Error("Allocate(1) should have failed when allocator is full")
	}

	a.Free(alloc)
}

func TestAllocateLargerThanTotal(t *testing.T) {
	a := New(1024, 256)

	alloc := a.Allocate(2048)
	if !alloc.Failed() {
		t.Error("Allocate(2048) should have failed on a 1024-size allocator")
	}
}

func TestMaxAllocsExhaustion(t *testing.T) {
	const maxAllocs = 8
	a := New(1024, maxAllocs)

	// Each allocation consumes a node. The initial free region also consumes
	// one node, plus the remainder from each split. With maxAllocs=8, we can
	// make fewer than 8 simultaneous allocations because of remainder nodes.
	var allocs []Allocation
	for i := 0; i < 100; i++ {
		alloc := a.Allocate(1)
		if alloc.Failed() {
			break
		}
		allocs = append(allocs, alloc)
	}

	if len(allocs) == 0 {
		t.Fatal("expected at least one allocation to succeed")
	}

	// We should have hit the limit before 100.
	if len(allocs) >= 100 {
		t.Error("expected maxAllocs exhaustion before 100 allocations")
	}

	// Verify we can't allocate more.
	extra := a.Allocate(1)
	if !extra.Failed() {
		t.Error("allocation should fail after maxAllocs exhaustion")
	}

	// Free all and verify recovery.
	for _, alloc := range allocs {
		a.Free(alloc)
	}

	recovered := a.Allocate(1024)
	if recovered.Failed() {
		t.Error("allocation should succeed after freeing all")
	}
	a.Free(recovered)
}

func TestStorageReport(t *testing.T) {
	a := New(1024, 256)

	report := a.StorageReport()
	if report.TotalFreeSpace != 1024 {
		t.Errorf("initial TotalFreeSpace = %d, want 1024", report.TotalFreeSpace)
	}
	if report.LargestFreeRegion != 1024 {
		t.Errorf("initial LargestFreeRegion = %d, want 1024", report.LargestFreeRegion)
	}

	alloc := a.Allocate(256)
	report = a.StorageReport()
	if report.TotalFreeSpace != 768 {
		t.Errorf("after 256 alloc TotalFreeSpace = %d, want 768", report.TotalFreeSpace)
	}

	a.Free(alloc)
	report = a.StorageReport()
	if report.TotalFreeSpace != 1024 {
		t.Errorf("after free TotalFreeSpace = %d, want 1024", report.TotalFreeSpace)
	}
}

func TestReset(t *testing.T) {
	a := New(1024, 256)

	// Make some allocations.
	a.Allocate(100)
	a.Allocate(200)
	a.Allocate(300)

	// Reset should return to initial state.
	a.Reset()

	report := a.StorageReport()
	if report.TotalFreeSpace != 1024 {
		t.Errorf("after Reset TotalFreeSpace = %d, want 1024", report.TotalFreeSpace)
	}
	if report.LargestFreeRegion != 1024 {
		t.Errorf("after Reset LargestFreeRegion = %d, want 1024", report.LargestFreeRegion)
	}

	// Should be able to allocate the full range.
	alloc := a.Allocate(1024)
	if alloc.Failed() {
		t.Fatal("allocation after Reset failed")
	}
	if alloc.Offset != 0 {
		t.Errorf("allocation after Reset offset = %d, want 0", alloc.Offset)
	}
	a.Free(alloc)
}

func TestAllocationSize(t *testing.T) {
	a := New(testSize256MB, testMaxAllocs)

	sizes := []uint32{1, 42, 1337, 65536, 1024 * 1024}
	for _, size := range sizes {
		alloc := a.Allocate(size)
		if alloc.Failed() {
			t.Fatalf("Allocate(%d) failed", size)
		}
		got := a.AllocationSize(alloc)
		if got != size {
			t.Errorf("AllocationSize for Allocate(%d) = %d, want %d", size, got, size)
		}
		a.Free(alloc)
	}

	// Failed allocation returns 0.
	failedAlloc := Allocation{Offset: NoSpace, Metadata: NoSpace}
	if s := a.AllocationSize(failedAlloc); s != 0 {
		t.Errorf("AllocationSize for failed alloc = %d, want 0", s)
	}
}

func TestAllocationFailed(t *testing.T) {
	good := Allocation{Offset: 0, Metadata: 0}
	if good.Failed() {
		t.Error("Allocation{Offset:0} should not be Failed()")
	}

	bad := Allocation{Offset: NoSpace, Metadata: NoSpace}
	if !bad.Failed() {
		t.Error("Allocation{Offset:NoSpace} should be Failed()")
	}
}

func TestDoubleFreePanics(t *testing.T) {
	a := New(1024, 256)
	alloc := a.Allocate(128)
	a.Free(alloc)

	defer func() {
		if r := recover(); r == nil {
			t.Error("double Free should panic")
		}
	}()

	a.Free(alloc) // should panic
}

func TestFreeNoSpaceAllocationIsNoop(t *testing.T) {
	a := New(1024, 256)

	// Freeing a failed allocation should be a no-op, not panic.
	failedAlloc := Allocation{Offset: NoSpace, Metadata: NoSpace}
	a.Free(failedAlloc) // should not panic

	report := a.StorageReport()
	if report.TotalFreeSpace != 1024 {
		t.Errorf("TotalFreeSpace = %d, want 1024", report.TotalFreeSpace)
	}
}

func TestReuseComplex(t *testing.T) {
	a := New(testSize256MB, testMaxAllocs)

	// From the Rust/C++ "reuse_complex" test.
	allocA := a.Allocate(1024)
	if allocA.Offset != 0 {
		t.Errorf("allocA.Offset = %d, want 0", allocA.Offset)
	}

	allocB := a.Allocate(3456)
	if allocB.Offset != 1024 {
		t.Errorf("allocB.Offset = %d, want 1024", allocB.Offset)
	}

	a.Free(allocA)

	// C doesn't fit in A's bin, goes to end.
	allocC := a.Allocate(2345)
	if allocC.Offset != 1024+3456 {
		t.Errorf("allocC.Offset = %d, want %d", allocC.Offset, 1024+3456)
	}

	// D fits in A's freed slot.
	allocD := a.Allocate(456)
	if allocD.Offset != 0 {
		t.Errorf("allocD.Offset = %d, want 0", allocD.Offset)
	}

	// E uses remainder after D.
	allocE := a.Allocate(512)
	if allocE.Offset != 456 {
		t.Errorf("allocE.Offset = %d, want 456", allocE.Offset)
	}

	report := a.StorageReport()
	expectedFree := uint32(testSize256MB - 3456 - 2345 - 456 - 512)
	if report.TotalFreeSpace != expectedFree {
		t.Errorf("TotalFreeSpace = %d, want %d", report.TotalFreeSpace, expectedFree)
	}
	if report.LargestFreeRegion == report.TotalFreeSpace {
		t.Error("LargestFreeRegion should not equal TotalFreeSpace (fragmented)")
	}

	a.Free(allocC)
	a.Free(allocD)
	a.Free(allocB)
	a.Free(allocE)

	// Validate clean.
	validateAll := a.Allocate(testSize256MB)
	if validateAll.Failed() {
		t.Fatal("full re-allocation failed")
	}
	if validateAll.Offset != 0 {
		t.Errorf("full re-allocation offset = %d, want 0", validateAll.Offset)
	}
	a.Free(validateAll)
}

func TestSyncAllocatorConcurrent(t *testing.T) {
	s := NewSync(1024*1024, 4096)
	const goroutines = 8
	const opsPerGoroutine = 256

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		go func() {
			defer wg.Done()
			for i := 0; i < opsPerGoroutine; i++ {
				alloc := s.Allocate(64)
				if !alloc.Failed() {
					_ = s.AllocationSize(alloc)
					_ = s.StorageReport()
					s.Free(alloc)
				}
			}
		}()
	}
	wg.Wait()

	report := s.StorageReport()
	if report.TotalFreeSpace != 1024*1024 {
		t.Errorf("after concurrent ops TotalFreeSpace = %d, want %d", report.TotalFreeSpace, 1024*1024)
	}
}

func TestSyncAllocator(t *testing.T) {
	s := NewSync(1024, 256)

	alloc := s.Allocate(128)
	if alloc.Failed() {
		t.Fatal("SyncAllocator.Allocate failed")
	}

	size := s.AllocationSize(alloc)
	if size != 128 {
		t.Errorf("SyncAllocator.AllocationSize = %d, want 128", size)
	}

	report := s.StorageReport()
	if report.TotalFreeSpace != 896 {
		t.Errorf("SyncAllocator.StorageReport.TotalFreeSpace = %d, want 896", report.TotalFreeSpace)
	}

	s.Free(alloc)

	report = s.StorageReport()
	if report.TotalFreeSpace != 1024 {
		t.Errorf("after free TotalFreeSpace = %d, want 1024", report.TotalFreeSpace)
	}

	s.Reset()
	report = s.StorageReport()
	if report.TotalFreeSpace != 1024 {
		t.Errorf("after Reset TotalFreeSpace = %d, want 1024", report.TotalFreeSpace)
	}
}
