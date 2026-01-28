# PodResourceCalculator - Improvement Tasks

## Completed ✅

- [x] Update dependencies to Kubernetes 1.34 (client-go v0.34.3)
- [x] Update logrus to v1.9.4
- [x] Update excelize to v2.10.0
- [x] Fix path traversal vulnerability in output filename
- [x] Fix path traversal vulnerability in kubeconfig path
- [x] Add context timeout (30s) for Kubernetes API calls
- [x] Add namespace input validation (RFC 1123)
- [x] Add ROUND() function to H1 and L1 cells (2 decimals)

## High Priority

### Code Quality
- [x] Add `make lint` target with golangci-lint
  - [x] Add to Makefile
  - [x] Configure .golangci.yml
  - [x] Run and fix any issues
  
- [x] Remove duplicate `createNodeSheet()` function
  - [x] Delete `createNodeSheet()` (lines ~700-750)
  - [x] Use only `createNodeSheetFromData()`
  - [x] Update any callers
  
- [x] Add unit tests
  - [x] Test `validatePath()` function
  - [x] Test `validateNamespace()` function
  - [x] Test resource calculation logic
  - [x] Add `make test` verification to CI

## Medium Priority

### Code Organization
- [ ] Split `generateExcel()` function (400+ lines) - **Deferred**
  - Note: Function is complex with many interdependencies
  - Requires careful refactoring to avoid breaking changes
  - Recommend: Extract smaller helper functions incrementally
  - Current structure is functional, refactoring is optimization only

- [x] Define constants for magic numbers
  - [x] `ProcessingBatchSize = 50`
  - [x] `MemoryLogInterval = 500`
  - [x] `HighEfficiency = 80`
  - [x] `MediumEfficiency = 60`
  - [x] `LowEfficiency = 40`
  - [x] `DefaultAPITimeout = 30 * time.Second`
  - [x] Chart dimension constants
  - [x] Provisioning threshold constants

- [x] Improve error handling
  - [x] Log warnings for `SetCellStyle()` errors
  - [x] Add error context to all error returns
  - [x] Use consistent error wrapping pattern

## Low Priority

### Performance Optimizations
- [x] Remove redundant cluster totals calculation - **Not Actually Redundant**
  - Note: Pre-calculation is needed for per-container percentage calculations
  - Cannot be eliminated without two-pass approach (would hurt performance)
  - Current implementation is optimal for single-pass processing

- [x] Fix duplicate node tracking updates
  - Consolidated node total updates to single write per pod
  - Removed duplicate node lookup in container loop
  - Performance improvement: fewer map operations

### Feature Enhancements
- [ ] Add QoS class column to Resources sheet
  - Guaranteed (requests == limits)
  - Burstable (requests < limits)
  - BestEffort (no requests/limits)

- [ ] Track additional resource types
  - Ephemeral-storage requests/limits
  - GPU requests/limits (nvidia.com/gpu)
  - Custom resources

- [ ] Add pod metadata columns
  - Pod age (creation timestamp)
  - Pod restart count
  - Last restart time

- [ ] Expand pod status filter
  - Include Failed pods with warning indicator
  - Include Unknown pods with warning indicator
  - Add status summary to Insights sheet

### Documentation
- [ ] Add inline comments to complex functions
  - Document efficiency calculation logic
  - Document chart scaling algorithm
  - Document aggregation logic

- [ ] Create CONTRIBUTING.md
  - Code style guidelines
  - Testing requirements
  - PR process

- [ ] Add examples to README
  - Example output screenshots
  - Common use cases
  - Troubleshooting guide

## Future Considerations

### Advanced Features
- [ ] Add command-line flag for API timeout duration
- [ ] Support multiple output formats (CSV, JSON)
- [ ] Add filtering by labels/annotations
- [ ] Add node capacity comparison
- [ ] Generate recommendations based on historical data
- [ ] Add Prometheus metrics export option

### CI/CD
- [ ] Add GitHub Actions workflow
  - Run tests on PR
  - Run linters on PR
  - Build multi-platform binaries
  - Create releases automatically

- [ ] Add pre-commit hooks
  - Run `go fmt`
  - Run `go vet`
  - Run `golangci-lint`

## Quick Wins (Easy to implement)

Priority order for quick improvements:
1. Add `make lint` target (5 minutes)
2. Define constants for magic numbers (5 minutes)
3. Remove duplicate `createNodeSheet()` (10 minutes)
4. Add unit tests for validation functions (20 minutes)

## Notes

- All security issues have been addressed
- Code is production-ready as-is
- Remaining tasks are optimizations and enhancements
- No breaking changes should be introduced without major version bump
