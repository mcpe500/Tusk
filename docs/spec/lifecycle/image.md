# Image Lifecycle

## Scope
This specification documents the status of image management features (`tusk pull`, `tusk images`) and the contract used between client and daemon.

## Status Recommendations

| Command | Host Handler | Status | Implementation Facts | Notes |
|---------|--------------|--------|----------------------|-------|
| `tusk pull` | `runPull` | done | Calls `image.Puller` for registry auth, fetches manifest, config, and layer, then stores them in local store. | `ImagePull` in daemon also exists but is not used for real pull from this command. |
| `tusk images` | `runImages` | partial | Reads the local filesystem `~/.tusk/images/blobs/sha256` and `~/.tusk/images/manifests/*.json`, then displays a short list. | Only shows a rough list; the repository/tag columns do not come from a valid index (`TODO` is marked in code). |

## `tusk pull` Details
1. CLI: `runPull(ref)`
   - `ref` can be `image:tag`, `image@sha256:...`, or `docker://...`
2. `image.New(...).Init()` creates the store directory if it does not exist.
3. `puller.Pull(ctx, ref)`:
   - parse reference to registry/name/tag
   - fetch token from Docker auth endpoint
   - fetch manifest
   - fetch blob config and layer
   - save blob via `Store.SaveBlob()`
   - save manifest via `Store.SaveManifest()`

### Local Store Structure (brief)
- `~/.tusk/images/blobs/<algo>/<digest>` : blob content (zip layer/config)
- `~/.tusk/images/manifests/<sha256:..>.json` : manifest per pulled digest
- `~/.tusk/images/index` : placeholder directory for index/lookup (not yet fully used)

## `tusk images` Details
1. Reads the contents of `~/.tusk/images/blobs/sha256`.
2. Counts the number of blobs as the number of items in the list.
3. Iterates manifests to display `REPOSITORY TAG DIGEST` columns, defaulting to `local`/`latest` if the tag is unknown.
4. Displays total blob count.

## Remaining Limitations
- Tag lookup from index is not yet correct (TODO comment in `runImages`: `TODO: lookup tag from index`).
- There is no `tusk rmi` command in `cmd/tusk/main.go` to remove images.
- `ImageList` in daemon is only a stub returning empty and is not used by the CLI at this time.

## Class Notes
- `internal/image/store.go`: manifest/blob storage operations + layer extraction, and ref parser.
- `internal/image/store.go: Puller.Pull`: real pull HTTP + auth implementation.
- `cmd/tusk/main.go: runPull/runImages`: host CLI path.

## Relation to Daemon
For `tusk pull`, the CLI does not rely on the `ImagePull` RPC; the operation runs directly in the host `internal/image` package.
Conversely, the `tuskd` daemon responds to `ImagePull` as a stub (`{status: pulled}`) without data transfer.
