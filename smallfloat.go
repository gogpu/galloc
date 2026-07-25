// Copyright (c) 2025 Andrey Kolkov and GoGPU Contributors
// SPDX-License-Identifier: MIT
//
// Based on OffsetAllocator by Sebastian Aaltonen
// https://github.com/sebbbi/OffsetAllocator

package galloc

import "math/bits"

// SmallFloat constants.
// Bin sizes follow a floating point (exponent + mantissa) distribution
// (piecewise linear log approximation). This ensures that for each size class
// the average overhead percentage stays the same — max 12.5% waste.
const (
	mantissaBits  = 3
	mantissaValue = 1 << mantissaBits // 8
	mantissaMask  = mantissaValue - 1 // 7
)

// uintToFloatRoundUp converts an unsigned integer size to a SmallFloat bin
// index, rounding up. This is used when searching for a bin that is guaranteed
// to hold an allocation of the given size.
func uintToFloatRoundUp(size uint32) uint32 {
	var exp uint32
	var mantissa uint32

	if size < mantissaValue {
		// Denorm: 0..(mantissaValue-1)
		mantissa = size
	} else {
		// Normalized: Hidden high bit always 1. Not stored. Just like float.
		leadingZeros := uint32(bits.LeadingZeros32(size))
		highestSetBit := 31 - leadingZeros

		mantissaStartBit := highestSetBit - mantissaBits
		exp = mantissaStartBit + 1
		mantissa = (size >> mantissaStartBit) & mantissaMask

		lowBitsMask := uint32((1 << mantissaStartBit) - 1)

		// Round up!
		if (size & lowBitsMask) != 0 {
			mantissa++
		}
	}

	// + allows mantissa->exp overflow for round up
	return (exp << mantissaBits) + mantissa
}

// uintToFloatRoundDown converts an unsigned integer size to a SmallFloat bin
// index, rounding down. This is used when inserting a free node into a bin —
// the bin index must be <= the actual size so that any allocation from that
// bin is guaranteed to fit.
func uintToFloatRoundDown(size uint32) uint32 {
	var exp uint32
	var mantissa uint32

	if size < mantissaValue {
		// Denorm: 0..(mantissaValue-1)
		mantissa = size
	} else {
		// Normalized: Hidden high bit always 1. Not stored. Just like float.
		leadingZeros := uint32(bits.LeadingZeros32(size))
		highestSetBit := 31 - leadingZeros

		mantissaStartBit := highestSetBit - mantissaBits
		exp = mantissaStartBit + 1
		mantissa = (size >> mantissaStartBit) & mantissaMask
	}

	return (exp << mantissaBits) | mantissa
}

// floatToUint converts a SmallFloat bin index back to an unsigned integer size.
// This is the minimum size that maps to this bin.
func floatToUint(floatValue uint32) uint32 {
	exponent := floatValue >> mantissaBits
	mantissa := floatValue & mantissaMask
	if exponent == 0 {
		// Denorms
		return mantissa
	}
	return (mantissa | mantissaValue) << (exponent - 1)
}

// MinAllocatorSize returns the minimum allocator size needed to hold an object
// of the given size. Due to SmallFloat bin rounding, the allocator size may
// need to be slightly larger than the requested object size.
func MinAllocatorSize(neededObjectSize uint32) uint32 {
	return floatToUint(uintToFloatRoundUp(neededObjectSize))
}
