# galloc

[![CI](https://github.com/gogpu/galloc/actions/workflows/ci.yml/badge.svg)](https://github.com/gogpu/galloc/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/gogpu/galloc.svg)](https://pkg.go.dev/github.com/gogpu/galloc)
[![codecov](https://codecov.io/gh/gogpu/galloc/branch/main/graph/badge.svg)](https://codecov.io/gh/gogpu/galloc)
[![OpenSSF Best Practices](https://www.bestpractices.dev/projects/13786/badge)](https://www.bestpractices.dev/projects/13786)
[![Go Version](https://img.shields.io/github/go-mod/go-version/gogpu/galloc)](https://github.com/gogpu/galloc/blob/main/go.mod)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

**O(1) offset allocator for GPU memory sub-allocation.** Pure Go, zero CGO, zero dependencies.

A Go port of Sebastian Aaltonen's [OffsetAllocator](https://github.com/sebbbi/OffsetAllocator) (MIT, 1,051 stars C++). Manages a single contiguous range of offsets (e.g., a GPU heap or buffer) and hands out sub-regions with O(1) allocation, O(1) free, and automatic neighbor coalescing.

## Features

- **O(1) allocate and free** -- two-level bitfield bin lookup, no linear scans
- **Automatic coalescing** -- adjacent free regions merge on free via doubly-linked neighbor list
- **Zero heap allocations** on hot path -- all node storage pre-allocated in `New()`
- **256 size classes** -- SmallFloat encoding (3-bit mantissa + 5-bit exponent) covers the full 32-bit range with at most 12.5% overhead per allocation
- **Thread-safe wrapper** -- `SyncAllocator` adds `sync.Mutex` for concurrent use
- **Pure Go** -- no unsafe, no CGO, no assembly, stdlib only
- **Go 1.25+** -- matches gogpu ecosystem requirements

## Installation

```bash
go get github.com/gogpu/galloc
```

## Quick Start

```go
package main

import (
    "fmt"
    "github.com/gogpu/galloc"
)

func main() {
    // Manage a 64MB GPU heap with up to 4096 allocations.
    a := galloc.New(64*1024*1024, 4096)

    // Allocate 256 bytes.
    alloc := a.Allocate(256)
    if alloc.Failed() {
        panic("out of memory")
    }
    fmt.Printf("Offset: %d\n", alloc.Offset) // 0

    // Check free space.
    report := a.StorageReport()
    fmt.Printf("Free: %d, Largest: %d\n", report.TotalFreeSpace, report.LargestFreeRegion)

    // Free the allocation.
    a.Free(alloc)
}
```

## Aligned Allocation

GPU APIs require buffer offsets to be aligned (e.g., 256 bytes for Vulkan uniform buffers). `AllocateAligned` guarantees the returned offset is a multiple of the given alignment:

```go
// Vulkan uniform buffer: 256-byte alignment.
alloc := a.AllocateAligned(4096, 256)
fmt.Printf("Offset: %d (aligned: %v)\n", alloc.Offset, alloc.Offset%256 == 0)

// DX12 texture data: 512-byte alignment.
tex := a.AllocateAligned(65536, 512)

// Alignment of 0 or 1 is equivalent to Allocate (zero overhead).
plain := a.AllocateAligned(100, 1)

a.Free(alloc)
a.Free(tex)
a.Free(plain)
```

Alignment must be a power of two. The implementation over-allocates by up to `alignment - 1` bytes; these padding bytes are reclaimed when the allocation is freed.

## Algorithm

The allocator is based on [TLSF](http://www.gii.upv.es/tlsf/) (Two-Level Segregated Fit) principles adapted for offset-only allocation (no actual memory management).

### SmallFloat Binning

Sizes are mapped to 256 bins using a custom floating-point encoding with a 3-bit mantissa and 5-bit exponent:

```
Bin index = (exponent << 3) | mantissa

Sizes 0-7:   exact (denormalized, exponent = 0)
Sizes 8+:    normalized with hidden high bit (like IEEE 754)

Examples:
  Size     1 -> bin   1 (exact)
  Size     8 -> bin   8 (exact)
  Size  1024 -> bin  64
  Size 65536 -> bin 112
```

The maximum overhead per allocation is 12.5% (1/8), determined by the 3-bit mantissa.

### Two-Level Bitfield

Free bins are tracked with a two-level bitfield for O(1) lookup:

```
usedBinsTop: uint32        -- 32 bits, one per top-level group
usedBins[32]: uint8        -- 8 bits each, one per leaf bin within a group
                              Total: 32 * 8 = 256 leaf bins
```

Finding the best-fit bin requires at most two `TrailingZeros` intrinsics.

### Neighbor Coalescing

Every node maintains doubly-linked `neighborPrev`/`neighborNext` pointers to spatially adjacent regions. When a region is freed, the allocator checks both neighbors and merges contiguous free regions into a single larger block, eliminating fragmentation.

### Node Pool

All nodes are pre-allocated in `New()`. A freelist stack provides O(1) node allocation and deallocation without any heap interaction on the hot path.

## Thread Safety

`Allocator` is **not** safe for concurrent use. For multi-goroutine scenarios, use `SyncAllocator`:

```go
a := galloc.NewSync(64*1024*1024, 4096)
// Safe to call from multiple goroutines:
alloc := a.Allocate(256)
a.Free(alloc)
```

## Use Cases

- **GPU buffer sub-allocation** -- manage a single `VkBuffer` or `ID3D12Resource` heap
- **Ring buffer management** -- track free regions in a staging belt
- **Memory pool offset tracking** -- any scenario requiring contiguous range sub-allocation

## Credits

Based on [OffsetAllocator](https://github.com/sebbbi/OffsetAllocator) by Sebastian Aaltonen. Also informed by [offset-allocator](https://github.com/pcwalton/offset-allocator) (Rust port by Patrick Walton).

## License

MIT License. See [LICENSE](LICENSE).
