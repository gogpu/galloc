// Copyright (c) 2025 Andrey Kolkov and GoGPU Contributors
// SPDX-License-Identifier: MIT
//
// Based on OffsetAllocator by Sebastian Aaltonen
// https://github.com/sebbbi/OffsetAllocator

package galloc

import "math/bits"

// Constants for the two-level binning structure.
const (
	numTopBins        = 32
	binsPerLeaf       = 8
	topBinsIndexShift = 3
	leafBinsIndexMask = 0x7
	numLeafBins       = numTopBins * binsPerLeaf // 256
)

// NoSpace is the sentinel value indicating a failed allocation.
const NoSpace = 0xFFFFFFFF

// nodeUnused is the sentinel for unlinked node indices (equivalent to C++ Node::unused).
const nodeUnused = 0xFFFFFFFF

// Allocation represents a sub-region returned by [Allocator.Allocate].
//
// Offset is the starting position within the managed range.
// Metadata is an opaque internal handle required by [Allocator.Free] and
// [Allocator.AllocationSize]. Do not interpret or modify it.
type Allocation struct {
	Offset   uint32
	Metadata uint32
}

// Failed reports whether the allocation failed (no contiguous space available).
func (a Allocation) Failed() bool {
	return a.Offset == NoSpace
}

// StorageReport summarizes the free space in an [Allocator].
type StorageReport struct {
	TotalFreeSpace    uint32
	LargestFreeRegion uint32
}

// node is a single entry in the allocator's node pool. Each node represents
// either an active allocation or a free region. Nodes participate in two
// independent linked lists:
//
//   - Bin list: singly-linked list of free regions within the same size bin.
//   - Neighbor list: doubly-linked list of spatially adjacent regions (used
//     for coalescing on free).
type node struct {
	dataOffset   uint32
	dataSize     uint32
	binListPrev  uint32
	binListNext  uint32
	neighborPrev uint32
	neighborNext uint32
	used         bool
}

// Allocator manages a contiguous range of [0, size) and sub-allocates regions
// from it in O(1) time. All internal storage is pre-allocated; Allocate and
// Free perform zero heap allocations.
//
// Allocator is NOT safe for concurrent use from multiple goroutines.
// Use [SyncAllocator] for a thread-safe wrapper.
type Allocator struct {
	size      uint32
	maxAllocs uint32

	freeStorage uint32
	usedBinsTop uint32
	usedBins    [numTopBins]uint8
	binIndices  [numLeafBins]uint32

	nodes      []node
	freeNodes  []uint32
	freeOffset uint32
}

// New creates an Allocator that manages a contiguous range of size units
// with room for at most maxAllocs simultaneous allocations.
//
// All internal storage is allocated up front. Subsequent Allocate and Free
// calls perform zero heap allocations.
func New(size, maxAllocs uint32) *Allocator {
	a := &Allocator{
		size:      size,
		maxAllocs: maxAllocs,
	}
	a.nodes = make([]node, maxAllocs)
	a.freeNodes = make([]uint32, maxAllocs)
	a.reset()
	return a
}

// reset reinitializes the allocator to its initial state: a single free
// region spanning the entire managed range.
func (a *Allocator) reset() {
	a.freeStorage = 0
	a.usedBinsTop = 0
	a.freeOffset = a.maxAllocs - 1

	for i := range a.usedBins {
		a.usedBins[i] = 0
	}
	for i := range a.binIndices {
		a.binIndices[i] = nodeUnused
	}

	// Reset all nodes to zero state.
	for i := range a.nodes {
		a.nodes[i] = node{}
	}

	// Freelist is a stack. Nodes in inverse order so that index 0 pops first.
	for i := uint32(0); i < a.maxAllocs; i++ {
		a.freeNodes[i] = a.maxAllocs - i - 1
	}

	// Start state: whole storage as one big node.
	// The algorithm will split remainders and push them back as smaller nodes.
	a.insertNodeIntoBin(a.size, 0)
}

// Reset clears all allocations and returns the allocator to its initial state.
func (a *Allocator) Reset() {
	a.reset()
}

// Allocate reserves a contiguous region of the given size and returns an
// [Allocation] describing it. If there is not enough contiguous space, or the
// maximum number of allocations has been reached, the returned Allocation will
// have Offset == [NoSpace] (check with [Allocation.Failed]).
//
// A size of 0 is treated as a valid allocation of zero bytes at offset 0
// (matching the C++ original behavior).
func (a *Allocator) Allocate(size uint32) Allocation {
	fail := Allocation{Offset: NoSpace, Metadata: NoSpace}

	// Out of node slots?
	if a.freeOffset == 0 {
		return fail
	}

	// Round up to bin index to ensure that alloc >= bin.
	// Gives us the minimum bin index that fits the requested size.
	minBinIndex := uintToFloatRoundUp(size)

	minTopBinIndex := minBinIndex >> topBinsIndexShift
	minLeafBinIndex := minBinIndex & leafBinsIndexMask

	topBinIndex := minTopBinIndex
	leafBinIndex := uint32(NoSpace)

	// If the top bin exists, scan its leaf bin. This can fail.
	if (a.usedBinsTop & (1 << topBinIndex)) != 0 {
		leafBinIndex = findLowestSetBitAfter(uint32(a.usedBins[topBinIndex]), minLeafBinIndex)
	}

	// If we didn't find space in the current top bin, search from the next one.
	if leafBinIndex == NoSpace {
		topBinIndex = findLowestSetBitAfter(a.usedBinsTop, minTopBinIndex+1)
		if topBinIndex == NoSpace {
			return fail
		}
		// All leaf bins in this top bin fit the alloc, since the top bin was
		// rounded up. Start leaf search from bit 0.
		// This search can't fail since at least one leaf bit is set.
		leafBinIndex = uint32(bits.TrailingZeros32(uint32(a.usedBins[topBinIndex]))) //nolint:gosec // TrailingZeros32 returns [0,32], safe for uint32
	}

	binIndex := (topBinIndex << topBinsIndexShift) | leafBinIndex

	// Pop the top node of the bin. Bin top = node.next.
	nodeIndex := a.binIndices[binIndex]
	nd := &a.nodes[nodeIndex]
	nodeTotalSize := nd.dataSize
	nd.dataSize = size
	nd.used = true
	a.binIndices[binIndex] = nd.binListNext
	if nd.binListNext != nodeUnused {
		a.nodes[nd.binListNext].binListPrev = nodeUnused
	}
	a.freeStorage -= nodeTotalSize

	// Bin empty?
	if a.binIndices[binIndex] == nodeUnused {
		// Remove the leaf bin mask bit.
		a.usedBins[topBinIndex] &= ^(1 << leafBinIndex)

		// All leaf bins empty?
		if a.usedBins[topBinIndex] == 0 {
			// Remove the top bin mask bit.
			a.usedBinsTop &= ^(1 << topBinIndex)
		}
	}

	// Push back the remainder to a lower bin.
	remainderSize := nodeTotalSize - size
	if remainderSize > 0 {
		newNodeIndex := a.insertNodeIntoBin(remainderSize, nd.dataOffset+size)

		// Link nodes next to each other so that we can merge them later
		// if both are free. Update the old next neighbor to point to the
		// new node (which is now in the middle).
		if nd.neighborNext != nodeUnused {
			a.nodes[nd.neighborNext].neighborPrev = newNodeIndex
		}
		a.nodes[newNodeIndex].neighborPrev = nodeIndex
		a.nodes[newNodeIndex].neighborNext = nd.neighborNext
		nd.neighborNext = newNodeIndex
	}

	return Allocation{
		Offset:   nd.dataOffset,
		Metadata: nodeIndex,
	}
}

// AllocateAligned reserves a contiguous region of the given size at an offset
// that is a multiple of alignment. Alignment must be a power of two; passing a
// non-power-of-two value (other than 0) will panic. Alignment of 0 or 1 is
// equivalent to [Allocate].
//
// The implementation over-allocates by up to alignment-1 bytes to guarantee an
// aligned offset within the block. These padding bytes are reclaimed when the
// allocation is freed.
//
// A size of 0 is treated as a valid allocation (matching [Allocate] behavior).
func (a *Allocator) AllocateAligned(size, alignment uint32) Allocation {
	if alignment <= 1 {
		return a.Allocate(size)
	}
	if alignment&(alignment-1) != 0 {
		panic("galloc: alignment must be a power of two")
	}
	padded := size + alignment - 1
	alloc := a.Allocate(padded)
	if alloc.Failed() {
		return alloc
	}
	alloc.Offset = (alloc.Offset + alignment - 1) &^ (alignment - 1)
	return alloc
}

// Free releases a previously-made allocation, returning its space to the pool.
// Adjacent free regions are automatically coalesced.
//
// Passing a failed allocation (Offset == NoSpace) is a no-op. Freeing the same
// allocation twice will panic.
func (a *Allocator) Free(alloc Allocation) {
	if alloc.Metadata == NoSpace {
		return
	}

	nodeIndex := alloc.Metadata
	nd := &a.nodes[nodeIndex]

	// Double-free check.
	if !nd.used {
		panic("galloc: double free detected")
	}

	offset := nd.dataOffset
	size := nd.dataSize

	// Merge with previous neighbor if it is free.
	if nd.neighborPrev != nodeUnused && !a.nodes[nd.neighborPrev].used {
		prevNode := &a.nodes[nd.neighborPrev]
		offset = prevNode.dataOffset
		size += prevNode.dataSize

		a.removeNodeFromBin(nd.neighborPrev)

		nd.neighborPrev = prevNode.neighborPrev
	}

	// Merge with next neighbor if it is free.
	if nd.neighborNext != nodeUnused && !a.nodes[nd.neighborNext].used {
		nextNode := &a.nodes[nd.neighborNext]
		size += nextNode.dataSize

		a.removeNodeFromBin(nd.neighborNext)

		nd.neighborNext = nextNode.neighborNext
	}

	neighborNext := nd.neighborNext
	neighborPrev := nd.neighborPrev

	// Return the old node to the freelist.
	a.freeOffset++
	a.freeNodes[a.freeOffset] = nodeIndex

	// Insert a (possibly combined) free node into the appropriate bin.
	combinedNodeIndex := a.insertNodeIntoBin(size, offset)

	// Connect neighbors with the new combined node.
	if neighborNext != nodeUnused {
		a.nodes[combinedNodeIndex].neighborNext = neighborNext
		a.nodes[neighborNext].neighborPrev = combinedNodeIndex
	}
	if neighborPrev != nodeUnused {
		a.nodes[combinedNodeIndex].neighborPrev = neighborPrev
		a.nodes[neighborPrev].neighborNext = combinedNodeIndex
	}
}

// AllocationSize returns the size stored for an allocation. This is the size
// that was passed to Allocate, not the bin-rounded size.
func (a *Allocator) AllocationSize(alloc Allocation) uint32 {
	if alloc.Metadata == NoSpace {
		return 0
	}
	return a.nodes[alloc.Metadata].dataSize
}

// StorageReport returns a summary of the allocator's free space, including
// the total free space and the largest single contiguous free region.
func (a *Allocator) StorageReport() StorageReport {
	var largestFreeRegion uint32
	var freeStorage uint32

	// Out of node slots? -> report zero free space.
	if a.freeOffset > 0 {
		freeStorage = a.freeStorage
		if a.usedBinsTop != 0 {
			topBinIndex := 31 - uint32(bits.LeadingZeros32(a.usedBinsTop))                    //nolint:gosec // LeadingZeros32 returns [0,32], safe for uint32
			leafBinIndex := 31 - uint32(bits.LeadingZeros32(uint32(a.usedBins[topBinIndex]))) //nolint:gosec // LeadingZeros32 returns [0,32], safe for uint32
			largestFreeRegion = floatToUint((topBinIndex << topBinsIndexShift) | leafBinIndex)
		}
	}

	return StorageReport{
		TotalFreeSpace:    freeStorage,
		LargestFreeRegion: largestFreeRegion,
	}
}

// insertNodeIntoBin takes a free node from the freelist, initializes it with
// the given size and offset, and inserts it at the head of the bin's linked
// list. Returns the index of the inserted node.
func (a *Allocator) insertNodeIntoBin(size, dataOffset uint32) uint32 {
	// Round down to bin index to ensure that bin >= alloc.
	binIndex := uintToFloatRoundDown(size)

	topBinIndex := binIndex >> topBinsIndexShift
	leafBinIndex := binIndex & leafBinsIndexMask

	// Bin was empty before? Set the mask bits.
	if a.binIndices[binIndex] == nodeUnused {
		a.usedBins[topBinIndex] |= 1 << leafBinIndex
		a.usedBinsTop |= 1 << topBinIndex
	}

	// Take a freelist node and insert on top of the bin linked list.
	topNodeIndex := a.binIndices[binIndex]
	nodeIndex := a.freeNodes[a.freeOffset]
	a.freeOffset--

	a.nodes[nodeIndex] = node{
		dataOffset:   dataOffset,
		dataSize:     size,
		binListNext:  topNodeIndex,
		binListPrev:  nodeUnused,
		neighborPrev: nodeUnused,
		neighborNext: nodeUnused,
	}
	if topNodeIndex != nodeUnused {
		a.nodes[topNodeIndex].binListPrev = nodeIndex
	}
	a.binIndices[binIndex] = nodeIndex

	a.freeStorage += size

	return nodeIndex
}

// removeNodeFromBin unlinks a node from its bin's linked list and returns it
// to the freelist.
func (a *Allocator) removeNodeFromBin(nodeIndex uint32) {
	nd := a.nodes[nodeIndex]

	if nd.binListPrev != nodeUnused {
		// Easy case: we have a previous node. Just unlink from the middle.
		a.nodes[nd.binListPrev].binListNext = nd.binListNext
		if nd.binListNext != nodeUnused {
			a.nodes[nd.binListNext].binListPrev = nd.binListPrev
		}
	} else {
		// Hard case: we are the head of the bin. Must find the bin.
		binIndex := uintToFloatRoundDown(nd.dataSize)

		topBinIndex := binIndex >> topBinsIndexShift
		leafBinIndex := binIndex & leafBinsIndexMask

		a.binIndices[binIndex] = nd.binListNext
		if nd.binListNext != nodeUnused {
			a.nodes[nd.binListNext].binListPrev = nodeUnused
		}

		// Bin empty?
		if a.binIndices[binIndex] == nodeUnused {
			// Remove the leaf bin mask bit.
			a.usedBins[topBinIndex] &= ^(1 << leafBinIndex)

			// All leaf bins empty?
			if a.usedBins[topBinIndex] == 0 {
				// Remove the top bin mask bit.
				a.usedBinsTop &= ^(1 << topBinIndex)
			}
		}
	}

	// Return the node to the freelist.
	a.freeOffset++
	a.freeNodes[a.freeOffset] = nodeIndex

	a.freeStorage -= nd.dataSize
}

// findLowestSetBitAfter returns the index of the lowest set bit at or above
// startBitIndex in bitMask. Returns NoSpace if no such bit exists.
func findLowestSetBitAfter(bitMask, startBitIndex uint32) uint32 {
	maskBeforeStartIndex := (uint32(1) << startBitIndex) - 1
	maskAfterStartIndex := ^maskBeforeStartIndex
	bitsAfter := bitMask & maskAfterStartIndex
	if bitsAfter == 0 {
		return NoSpace
	}
	return uint32(bits.TrailingZeros32(bitsAfter))
}
