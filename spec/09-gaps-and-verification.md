# Gaps and Verification Plan

## High Priority Gaps

1. **Protocol security (still needs to be aligned)**: daemon request parsing is already safer, but needs to be closed for `jsonrpc` and non-JSON simulation cases with automated testing.
2. **Socket location inconsistency**: VM daemon default `/tusk/serial.sock`, while host target `~/.tusk/vm/serial.sock`.
3. **Mix of stub/pseudo-real**: `ImagePull`, `ImageList`, network, volume, inspect are still not end-to-end.
4. **Host-real container runtime not fully invoked**: `internal/container/spec.go` contains placeholders, `SaveSpec` always errors, rootfs extraction is not yet used consistently by CLI/daemon.
5. **Automated testing is not comprehensive**: only `cmd/tuskd/main_test.go` exists; coverage of other command/protocol has not been closed.

## Technical Verification Points

- `go test ./...` (compile + baseline check).
- `tusk init`, `tusk status`, `tusk start`, `tusk stop` (without running VM).
- `tusk pull <image>` and `tusk images` with internet connection.
- `tusk run alpine echo hi` when simulation daemon is active.
- `tusk compose up -f docker-compose.yml` on a simple image service.
- Simulate malformed JSON-RPC request to `/tusk/serial.sock` to ensure no panic in `tuskd`.

## Documentation Notes

- `docs/` and `CHANGELOG.md` often declare features complete; this specification maintains realistic implementation status with `done/partial/stub` labels.
