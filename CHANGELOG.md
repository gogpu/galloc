# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Changed

- **CI** follows the standard workflow of the GoGPU modules. The tests run with the race detector on Linux, macOS and Windows with Go oldstable and stable, and again at the `go.mod` minimum. CI also builds and tests for 386, runs golangci-lint, gofmt and `go mod tidy` checks, the two fuzz targets and govulncheck. A single required `Test` gate covers all of them. Before, CI tested only Go 1.25 and had no race, 32-bit or minimum-version runs.

## [0.2.1] - 2026-08-11

### Fixed

- **Honor allocation limits at boundary cases** (#2, @besmpl) — `New(1024, 1).Allocate(1)` panicked because the initial free region consumed the only allocation node. Node pool now sized at `2*maxAllocs` (math proof: K allocations → at most K+1 free regions → 2K nodes). Same bug exists in the upstream C++ [OffsetAllocator](https://github.com/sebbbi/OffsetAllocator).
- **Track active allocations separately** — `allocCount` field enforces `maxAllocs` limit independently from structural node pool, preventing fragmented layouts from exhausting nodes before reaching the advertised limit.
- **Reject aligned overflow** — `AllocateAligned` with near-`MaxUint32` size no longer wraps `size + alignment - 1`.

### Added

- 7 regression tests: single-slot, zero-capacity, fragmented-capacity, reuse, accounting, overflow, metadata-exhaustion.

## [0.2.0] - 2026-07-27

### Added

- `AllocateAligned(size, alignment)` — per-allocation aligned sub-allocation (TLSF-style over-allocate)
- `SyncAllocator.AllocateAligned` — thread-safe aligned allocation wrapper
- GPU alignment support: Vulkan (256), DX12 (4–65536), Metal (64–256)
- Alignment=0 or 1 fast-paths to `Allocate` with zero overhead
- Non-power-of-2 alignment panics (programmer error, same as `sync.Pool`)
- Fuzz testing for aligned allocation (`FuzzAllocAligned`)
- Benchmarks for aligned allocation (~50ns/op, 0 allocs/op)

## [0.1.0] - 2026-07-25

### Added

- O(1) offset allocator (`Allocator`) with two-level bitfield bin lookup
- SmallFloat encoding (3-bit mantissa, 5-bit exponent) for 256 size classes
- Automatic neighbor coalescing on free via doubly-linked neighbor list
- Zero heap allocations on hot path — all node storage pre-allocated
- Thread-safe wrapper (`SyncAllocator`) with `sync.Mutex`
- `StorageReport` for free space introspection
- `Reset` to clear all allocations
- `MinAllocatorSize` helper for minimum allocator sizing
- Double-free detection via panic
- Comprehensive test suite (22 tests + fuzz testing)
- Benchmarks: ~40ns allocate, ~40ns free, 0 allocs/op
- Enterprise CI: 3 OS, lint, formatting, fuzz, Codecov OIDC
