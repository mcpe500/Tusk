# M2 Probe and Qualification Traceability

This record maps the M2 implementation to the canonical requirements. A
candidate is qualified only after the real adapter path supplies successful
Podman execution, lifecycle restart, reliability, and all benchmark metrics.
Unit test executors never create release qualification evidence.

| Requirement | Implementation | Verification |
|---|---|---|
| `ENG-003..006` | `internal/engine/qualification.go` | `qualification_test.go`; non-zero commands and missing lifecycle fail closed |
| `ENG-007` | versioned contracts, digest-pinned probe, timeout, repetitions, warmup | `contracts_test.go`, `benchmark_test.go` |
| `ENG-008..010` | `score.go`, `fingerprint.go` | `score_test.go`, `fingerprint_test.go` |
| `ENG-011..012` | `tune.go`, CLI tune report | `tune_test.go`; CLI smoke output includes rejection and raw metrics |
| `ENG-013..014` | evidence fingerprint includes device, OS, Termux, Tusk, backend binary, probe, and policy | `fingerprint_test.go`, `tune_test.go` |
| `ENG-021..022` | storage/download/thermal budgets, bounded commands, atomic journal resume | `run_test.go`, `reliability_test.go`, `qualification_test.go` |
| `AT-002` | every discovered candidate is persisted with attempted or explicit rejection evidence | `tune_test.go` |
| `AT-034..035` | deterministic ranking, cache invalidation, interruption journal | `score_test.go`, `tune_test.go` |

The real-device adapter acceptance gates remain intentionally unavailable until
M3/M4 provide proot and QEMU adapters. No unsupported backend is reported as
qualified by this milestone.
