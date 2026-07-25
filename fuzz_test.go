// Copyright (c) 2025 Andrey Kolkov and GoGPU Contributors
// SPDX-License-Identifier: MIT

package galloc

import "testing"

func FuzzAllocFree(f *testing.F) {
	// Seed corpus.
	f.Add(uint32(1024), uint8(10), uint16(64))
	f.Add(uint32(65536), uint8(50), uint16(256))
	f.Add(uint32(1), uint8(1), uint16(1))
	f.Add(uint32(1024*1024), uint8(100), uint16(1024))

	f.Fuzz(func(t *testing.T, totalSize uint32, numOps uint8, allocSize uint16) {
		if totalSize == 0 || totalSize > 1024*1024*16 {
			return // skip degenerate or excessively large inputs
		}

		maxAllocs := uint32(numOps)*2 + 16
		if maxAllocs > 65536 {
			maxAllocs = 65536
		}

		a := New(totalSize, maxAllocs)

		// Perform a sequence of alloc/free operations. No panics or corruption.
		var live []Allocation
		for i := uint8(0); i < numOps; i++ {
			size := uint32(allocSize)
			if size == 0 {
				size = 1
			}
			alloc := a.Allocate(size)
			if !alloc.Failed() {
				live = append(live, alloc)
			}

			// Free half of live allocations periodically.
			if len(live) > 4 && i%3 == 0 {
				half := len(live) / 2
				for _, alloc := range live[:half] {
					a.Free(alloc)
				}
				live = live[half:]
			}
		}

		// Free all remaining.
		for _, alloc := range live {
			a.Free(alloc)
		}

		// After freeing everything, the report should show totalSize as free.
		report := a.StorageReport()
		if report.TotalFreeSpace != totalSize {
			t.Errorf("after freeing all: TotalFreeSpace = %d, want %d", report.TotalFreeSpace, totalSize)
		}
	})
}
