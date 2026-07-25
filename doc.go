// Copyright (c) 2025 Andrey Kolkov and GoGPU Contributors
// SPDX-License-Identifier: MIT
//
// Based on OffsetAllocator by Sebastian Aaltonen
// https://github.com/sebbbi/OffsetAllocator

// Package galloc provides an O(1) offset allocator for GPU memory sub-allocation.
//
// This is a Pure Go port of Sebastian Aaltonen's OffsetAllocator
// (https://github.com/sebbbi/OffsetAllocator, MIT, 1,051 stars). It manages a
// single contiguous range of offsets (e.g. a GPU heap or buffer) and hands out
// sub-regions in O(1) time with O(1) free and automatic neighbor coalescing.
//
// # Algorithm
//
// The allocator uses a two-level bitfield bin system inspired by TLSF
// (Two-Level Segregated Fit) memory allocators. Bin sizes follow a SmallFloat
// encoding with a 3-bit mantissa and 5-bit exponent, yielding 256 size classes
// that cover the full 32-bit range with at most 12.5% overhead per allocation.
//
// Allocation searches the two-level bitfield for the smallest bin that fits the
// request. Free regions within each bin are stored in a doubly-linked free list.
// When a region is freed, the allocator checks its spatial neighbors (tracked
// via a doubly-linked neighbor list) and coalesces adjacent free regions.
//
// All operations (Allocate, Free) are O(1) with respect to the number of
// existing allocations. The hot path performs zero heap allocations — all node
// storage is pre-allocated in [New].
//
// # Thread Safety
//
// [Allocator] is NOT safe for concurrent use. Use [SyncAllocator] for a
// thread-safe wrapper that adds a sync.Mutex.
//
// # Usage
//
//	a := galloc.New(1024*1024, 1024) // 1 MB range, up to 1024 allocations
//	alloc := a.Allocate(256)
//	if alloc.Failed() {
//	    // no space
//	}
//	// use alloc.Offset as the sub-allocation offset
//	a.Free(alloc)
package galloc
