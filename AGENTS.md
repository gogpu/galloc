# galloc -- AI Assistant Guide

## What is galloc?

galloc is a Pure Go O(1) offset allocator for GPU memory sub-allocation. It is
part of the [GoGPU](https://github.com/gogpu) ecosystem.

It manages a contiguous range of offsets (e.g., a GPU heap or buffer) and hands
out sub-regions with O(1) allocation, O(1) free, and automatic neighbor
coalescing. Zero CGO, zero dependencies, zero heap allocations on the hot path.

Based on Sebastian Aaltonen's
[OffsetAllocator](https://github.com/sebbbi/OffsetAllocator) (C++, MIT).

## Quick Start

```go
a := galloc.New(64*1024*1024, 4096) // 64 MB, up to 4096 allocations
alloc := a.Allocate(256)
if alloc.Failed() {
    // handle OOM
}
// use alloc.Offset
a.Free(alloc)
```

For concurrent use: `galloc.NewSync(size, maxAllocs)`.

## Architecture

```
galloc.go       -- Allocator core: Allocate, Free, coalescing, bin management
smallfloat.go   -- SmallFloat uint-to-bin encoding (256 bins, 3-bit mantissa)
sync.go         -- SyncAllocator (mutex-wrapped Allocator)
doc.go          -- Package documentation
```

### Key Data Structures

- **Two-level bitfield** (`usedBinsTop` uint32 + `usedBins[32]` uint8) -- O(1) bin lookup via `bits.TrailingZeros`
- **Node pool** (`[]node`, pre-allocated) -- freelist stack, zero heap allocs
- **Neighbor list** (doubly-linked per node) -- spatial adjacency for coalescing
- **Bin list** (singly-linked per bin) -- free regions within same size class

### Key Constants

- `NoSpace = 0xFFFFFFFF` -- sentinel for failed allocation
- `numLeafBins = 256` -- total size classes
- Max overhead per allocation: 12.5% (1/8, from 3-bit mantissa)

## When to Use galloc

- GPU buffer sub-allocation (VkBuffer, ID3D12Resource heaps)
- Ring buffer offset management (staging belts)
- Any contiguous-range sub-allocation needing O(1) performance

## Ecosystem

galloc is consumed by [gogpu/wgpu](https://github.com/gogpu/wgpu) HAL backends
for GPU memory management. It has zero dependencies and can be used standalone.

| Repo | Relationship |
|------|-------------|
| [wgpu](https://github.com/gogpu/wgpu) | Consumer -- HAL memory sub-allocation |
| [gogpu](https://github.com/gogpu/gogpu) | Parent ecosystem |

## Development

```bash
go build ./...                   # Build
go test ./...                    # Test
go test -bench=. -benchmem       # Benchmark
go test -fuzz=FuzzAllocFree      # Fuzz
golangci-lint run --timeout=5m   # Lint
```
