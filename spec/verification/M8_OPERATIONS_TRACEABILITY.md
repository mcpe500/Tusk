# M8 Operations and Release Traceability

| Requirement | Evidence | Status |
|---|---|---|
| `UPD-001..007` | `UpdatePlan`, signed manifest verification, transactional system replacement | Implemented; release artifact/device validation pending |
| `BKP-001..005` | `CreateBackup`, checksum metadata, safe extraction, transactional restore | Implemented; backend quiescence integration pending |
| `UN-001..006` | `Uninstall`, explicit `--purge-data`, workspace/share preservation | Implemented |
| `SEC-004`, `SEC-006`, `SEC-010`, `SEC-011` | manifest verifier, redaction, safe archive target, secure temporary files | Implemented; security review pending |
| `OBS-002..007` | doctor, acceptance report, diagnostic bundle | Implemented |
| `QLT-004`, `QLT-006`, `QLT-009..012` | atomic state, make check, fail-closed operations, no simulation path | Implemented at host level |
| `AT-001..036` | acceptance matrix | **Pending**: no qualified backend/device evidence |

`acceptance status` does not infer success from unit tests or mock executors.
