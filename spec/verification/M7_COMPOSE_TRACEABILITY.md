# M7 Compose Traceability

`internal/engine/compose.go` pins and validates the provider metadata, limits
the advertised command matrix, maps the project working directory and file
arguments through registered shares, and delegates lifecycle semantics to the
selected `podman compose` provider. Tusk does not parse Compose YAML or delete
resources outside the provider's project scope.

| Requirement | Evidence | Status |
|---|---|---|
| `CMP-001..004`, `CMP-011` | `ComposeArgs`, `RunCompose`, path mapping | Implemented; backend acceptance pending |
| `CMP-005`, `CMP-010` | explicit tested command subset and provider metadata | Implemented |
| `CMP-006..009`, `CMP-012` | forwarding/reconciler hooks and provider error pass-through | Implemented; real workflow pending |
| `AT-012`, `AT-013` | Compose build/up/volume/restart/down through Podman | **Pending**: no qualified backend |
