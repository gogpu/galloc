# Roadmap

## v0.1.0 — Initial Release (current)

- [x] Core O(1) allocator (SmallFloat binning, two-level bitfield)
- [x] Automatic neighbor coalescing on free
- [x] Thread-safe `SyncAllocator` wrapper
- [x] Zero heap allocations on hot path
- [x] Enterprise CI (3 OS, lint, formatting, fuzz, Codecov)
- [x] 22 tests, 6 benchmarks, fuzz testing

## v0.2.0 — Alignment + Integration

- [ ] **FEAT-001: `AllocateAligned(size, alignment)` — per-allocation alignment** (ADR-001)
- [ ] Integration with `wgpu` Vulkan memory pools (replace BuddyAllocator)
- [ ] Integration with `wgpu` DX12 descriptor heaps
- [ ] Extended statistics (per-bin occupancy, fragmentation metrics)
- [ ] `AllocationInfo` with bin index and actual allocated size

## v0.3.0 — Optimization

- [ ] Trim approach for aligned allocation (zero-waste prefix reclaim)
- [ ] Benchmark suite vs BuddyAllocator (side-by-side comparison)
- [ ] Memory layout optimization (cache-friendly node pool)
- [ ] `usedBins [8]uint32` variant (fewer type casts on hot path)

## v1.0.0 — Stable API

- [ ] API freeze
- [ ] Production benchmarks from wgpu + Project Neon consumers
- [ ] awesome-go listing
