# Tusk

Tusk is being rebuilt as a Podman-first runtime bridge for Termux. A Smart
Engine discovers, qualifies, benchmarks, and ranks proot and QEMU execution
backends before selecting one for real Podman workloads.

The canonical product and system contract is:

[`spec/specs/000.TUSK_REBUILD_SPEC.md`](spec/specs/000.TUSK_REBUILD_SPEC.md)

## Milestone status

| Milestone | Status |
|---|---|
| M0 Canonical specification and reset | Complete |
| M1 Host foundation | Complete |
| M2 Probe and qualification framework | Complete |
| M3 Proot candidates | Not started |

M1 includes:

- versioned configuration and atomic state writes;
- device discovery and fingerprinting;
- backend candidate registry, preflight, and explanation;
- versioned conformance and benchmark contracts;
- deterministic evidence-based balanced scoring policy;
- signed release manifest verification;
- resumable, checksum-verified artifact downloads;
- process-wide lifecycle locking;
- Linux race tests and Android ARM64 cross-build CI.

No backend is marked qualified yet. Podman forwarding will only be exposed after
the real conformance gates in the canonical spec pass.

## Development

Go is the primary implementation language for the host CLI and Smart Engine.
POSIX shell is reserved for a minimal installer and runtime image provisioning.

```sh
go test ./...
go run ./cmd/tusk version
go run ./cmd/tusk device --json
go run ./cmd/tusk engine candidates --json
go run ./cmd/tusk engine explain proot-native
```

Run the complete M1 verification gate with:

```sh
make check
```
