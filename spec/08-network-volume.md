# Network and Volume Specification

## Network Command Surface

- `tusk network` only prints `Network management not implemented yet`.
- `tusk volume` only prints `Volume management not implemented yet`.

## Status

- `runNetwork`: stub
- `runVolume`: stub

## Internal Implementation

- `internal/network/manager.go` provides realistic API (Create/List/Remove/AllocateIP/PortForward) but default values/actions are still dummy (print + placeholder ID).
- `internal/network/requestAddress` tries `net.ParseIP` on subnet string and manipulates the 3rd index, which is not valid for CIDR format containing `/24`.
- No CLI glue to actually call `NetworkCreate/List/Remove`.

## Implications

- Compose features that rely on network/volume will not have real isolation effect.
- Port mapping in container flags (`-p`) is discarded during `tusk run` parsing.
