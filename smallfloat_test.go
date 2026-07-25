// Copyright (c) 2025 Andrey Kolkov and GoGPU Contributors
// SPDX-License-Identifier: MIT

package galloc

import "testing"

func TestSmallFloatRoundTrip(t *testing.T) {
	// Denorms, exp=1, and exp=2 + mantissa=0 are all precise.
	// With 3-bit mantissa (8 values), the first 17 integers (0..16) are exact.
	const preciseNumberCount = 17
	for i := uint32(0); i < preciseNumberCount; i++ {
		up := uintToFloatRoundUp(i)
		down := uintToFloatRoundDown(i)
		if up != i {
			t.Errorf("uintToFloatRoundUp(%d) = %d, want %d", i, up, i)
		}
		if down != i {
			t.Errorf("uintToFloatRoundDown(%d) = %d, want %d", i, down, i)
		}
	}

	// float->uint->float conversion must be precise for all bin indices.
	// Values < 240 (240 maps to 4G which overflows uint32).
	for i := uint32(0); i < 240; i++ {
		v := floatToUint(i)
		up := uintToFloatRoundUp(v)
		down := uintToFloatRoundDown(v)
		if up != i {
			t.Errorf("roundtrip up: floatToUint(%d)=%d, uintToFloatRoundUp(%d)=%d, want %d", i, v, v, up, i)
		}
		if down != i {
			t.Errorf("roundtrip down: floatToUint(%d)=%d, uintToFloatRoundDown(%d)=%d, want %d", i, v, v, down, i)
		}
	}
}

func TestSmallFloatKnownValues(t *testing.T) {
	// Known value pairs from the C++ / Rust test suites.
	tests := []struct {
		number uint32
		up     uint32
		down   uint32
	}{
		{17, 17, 16},
		{118, 39, 38},
		{1024, 64, 64},
		{65536, 112, 112},
		{529445, 137, 136},
		{1048575, 144, 143},
	}

	for _, tt := range tests {
		up := uintToFloatRoundUp(tt.number)
		down := uintToFloatRoundDown(tt.number)
		if up != tt.up {
			t.Errorf("uintToFloatRoundUp(%d) = %d, want %d", tt.number, up, tt.up)
		}
		if down != tt.down {
			t.Errorf("uintToFloatRoundDown(%d) = %d, want %d", tt.number, down, tt.down)
		}
	}
}

func TestSmallFloatBoundaries(t *testing.T) {
	// Size 0 should map to bin 0.
	if v := uintToFloatRoundUp(0); v != 0 {
		t.Errorf("uintToFloatRoundUp(0) = %d, want 0", v)
	}
	if v := uintToFloatRoundDown(0); v != 0 {
		t.Errorf("uintToFloatRoundDown(0) = %d, want 0", v)
	}

	// Size 1 should map to bin 1.
	if v := uintToFloatRoundUp(1); v != 1 {
		t.Errorf("uintToFloatRoundUp(1) = %d, want 1", v)
	}
	if v := uintToFloatRoundDown(1); v != 1 {
		t.Errorf("uintToFloatRoundDown(1) = %d, want 1", v)
	}

	// mantissaValue (8) should be the first normalized value.
	if v := uintToFloatRoundUp(mantissaValue); v != mantissaValue {
		t.Errorf("uintToFloatRoundUp(%d) = %d, want %d", mantissaValue, v, mantissaValue)
	}

	// floatToUint(0) should be 0.
	if v := floatToUint(0); v != 0 {
		t.Errorf("floatToUint(0) = %d, want 0", v)
	}

	// Verify that round-up is always >= the original value.
	testValues := []uint32{1, 7, 8, 9, 15, 16, 17, 100, 1000, 10000, 100000, 1000000}
	for _, size := range testValues {
		binIndex := uintToFloatRoundUp(size)
		binSize := floatToUint(binIndex)
		if binSize < size {
			t.Errorf("floatToUint(uintToFloatRoundUp(%d)) = %d, which is less than %d", size, binSize, size)
		}
	}

	// Verify that round-down bin size is always <= the original value.
	for _, size := range testValues {
		binIndex := uintToFloatRoundDown(size)
		binSize := floatToUint(binIndex)
		if binSize > size {
			t.Errorf("floatToUint(uintToFloatRoundDown(%d)) = %d, which is greater than %d", size, binSize, size)
		}
	}
}

func TestMinAllocatorSize(t *testing.T) {
	// From the Rust ext::min_allocator_size tests. Log-distributed sizes.
	testSizes := []uint32{
		0, 1, 2, 3, 4, 5, 8, 17, 23, 36, 51, 68, 87, 151, 165, 167,
		201, 223, 306, 346, 394, 411, 806, 969, 1404, 1798, 2236, 4281,
		4745, 13989, 21095, 26594, 27146, 29679, 144685, 153878, 495127,
		727999, 1377073, 9440387, 41994490, 68520116,
	}

	for _, size := range testSizes {
		allocatorSize := MinAllocatorSize(size)
		a := New(allocatorSize, 128)
		alloc := a.Allocate(size)
		if alloc.Failed() {
			t.Errorf("MinAllocatorSize(%d) = %d, but Allocate(%d) failed", size, allocatorSize, size)
		}
	}
}
