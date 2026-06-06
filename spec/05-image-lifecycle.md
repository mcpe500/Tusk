# Image Lifecycle Specification

## Scope

- `tusk pull`
- `tusk images`
- Store path and manifest index

## Feature Status

| Command/Flow | Status | Evidence |
|---|---|---|
| `tusk pull <ref>` | done | `image.NewPuller().Pull(...)` downloads token, manifest, config, layer, then `SaveBlob/SaveManifest` |
| `tusk images` | partial | scan `~/.tusk/images/blobs/sha256` + manifest listing |
| `Resolve manifest by ref` | partial | `Store.ResolveManifestRef` reads index `~/.tusk/images/index/*.txt` |
| `Get manifest by digest` | done | `GetManifest` |
| `Get blob` | done | `GetBlob` + storage by `algo/digest` |

## `tusk pull` Details

1. `runPull` calls `store.Init()`.
2. `Puller.Pull(ctx, ref)` does:
   - parse reference,
   - fetch token,
   - fetch manifest,
   - fetch blob config,
   - fetch all layers,
   - save blob + manifest locally.

## `tusk images` Details

1. Count blobs in `images/blobs/sha256`.
2. Read `images/manifests/*.json` and print short `digest`.
3. Tags are not currently retrieved from index metadata.

## Known Shortcomings

- `runImages` does not return accurate `REPOSITORY`/`TAG` fields; `tag` currently defaults to `latest`.
- `manifest` index is not used to consistently display user-friendly mapping.
- No `tusk rmi` command.
