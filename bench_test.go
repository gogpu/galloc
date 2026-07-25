// Copyright (c) 2025 Andrey Kolkov and GoGPU Contributors
// SPDX-License-Identifier: MIT

package galloc

import "testing"

func BenchmarkAllocate(b *testing.B) {
	// Measure single Allocate call. Reset each iteration to avoid exhaustion.
	a := New(1024*1024*256, 1024)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		alloc := a.Allocate(256)
		a.Free(alloc)
	}
	// Note: alloc+free cycle to keep the allocator usable across iterations.
	// The alloc cost dominates since Free is measured separately.
}

func BenchmarkFree(b *testing.B) {
	// Measure single Free (with coalescing against one neighbor).
	// Avoid StopTimer/StartTimer in inner loop — their syscall overhead
	// dominates and produces misleading results (~16µs instead of ~25ns).
	a := New(1024*1024*256, 2048)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		alloc := a.Allocate(64)
		a.Free(alloc)
	}
}

func BenchmarkFreeBatch(b *testing.B) {
	// Measure batch Free with full coalescing (512 blocks → 1 merged region).
	// Setup cost is amortized by running the entire batch as one b.N iteration.
	const batchSize = 512
	const maxAllocs = 2048
	a := New(1024*1024, maxAllocs)
	allocs := make([]Allocation, batchSize)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		a.Reset()
		for j := range allocs {
			allocs[j] = a.Allocate(64)
		}
		for j := range allocs {
			a.Free(allocs[j])
		}
	}
}

func BenchmarkAllocFree(b *testing.B) {
	// Alloc+Free cycle — measures steady-state overhead.
	a := New(1024*1024*256, 1024)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		alloc := a.Allocate(256)
		a.Free(alloc)
	}
}

func BenchmarkAllocMany(b *testing.B) {
	// Fill an allocator to near capacity, then free everything.
	const maxAllocs = 4096
	a := New(1024*1024, maxAllocs)
	allocs := make([]Allocation, 0, 256)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		allocs = allocs[:0]
		a.Reset()
		for {
			alloc := a.Allocate(64)
			if alloc.Failed() {
				break
			}
			allocs = append(allocs, alloc)
		}
		for _, alloc := range allocs {
			a.Free(alloc)
		}
	}
}

func BenchmarkFragmented(b *testing.B) {
	// Interleaved alloc/free pattern to stress coalescing.
	const maxAllocs = 4096
	a := New(1024*1024, maxAllocs)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		a.Reset()

		// Allocate 256 blocks.
		var allocs [256]Allocation
		for j := range allocs {
			allocs[j] = a.Allocate(128)
		}

		// Free every other block (creates fragmentation).
		for j := 0; j < 256; j += 2 {
			a.Free(allocs[j])
		}

		// Re-allocate in the gaps.
		for j := 0; j < 256; j += 2 {
			allocs[j] = a.Allocate(128)
		}

		// Free all.
		for j := range allocs {
			a.Free(allocs[j])
		}
	}
}
