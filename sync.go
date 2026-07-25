// Copyright (c) 2025 Andrey Kolkov and GoGPU Contributors
// SPDX-License-Identifier: MIT
//
// Based on OffsetAllocator by Sebastian Aaltonen
// https://github.com/sebbbi/OffsetAllocator

package galloc

import "sync"

// SyncAllocator is a thread-safe wrapper around [Allocator].
// All methods are protected by a sync.Mutex.
type SyncAllocator struct {
	mu sync.Mutex
	a  *Allocator
}

// NewSync creates a thread-safe allocator that manages a contiguous range of
// size units with room for at most maxAllocs simultaneous allocations.
func NewSync(size, maxAllocs uint32) *SyncAllocator {
	return &SyncAllocator{a: New(size, maxAllocs)}
}

// Allocate reserves a contiguous region of the given size.
// See [Allocator.Allocate] for details.
func (s *SyncAllocator) Allocate(size uint32) Allocation {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.a.Allocate(size)
}

// Free releases a previously-made allocation.
// See [Allocator.Free] for details.
func (s *SyncAllocator) Free(alloc Allocation) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.a.Free(alloc)
}

// AllocationSize returns the size stored for an allocation.
// See [Allocator.AllocationSize] for details.
func (s *SyncAllocator) AllocationSize(alloc Allocation) uint32 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.a.AllocationSize(alloc)
}

// StorageReport returns a summary of the allocator's free space.
// See [Allocator.StorageReport] for details.
func (s *SyncAllocator) StorageReport() StorageReport {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.a.StorageReport()
}

// Reset clears all allocations and returns the allocator to its initial state.
// See [Allocator.Reset] for details.
func (s *SyncAllocator) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.a.Reset()
}
