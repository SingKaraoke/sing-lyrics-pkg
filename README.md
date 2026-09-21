# sing-lyrics-pkg

Shared Go module for the [Sing!](https://github.com/SingKaraoke) lyrics backend:
config loading, Postgres access, R2/S3 object storage, Meilisearch search,
device-key auth, and TTML parsing/validation. Consumed by
[sing-lyrics-api](https://github.com/SingKaraoke/sing-lyrics-api),
[sing-lyrics-upload-api](https://github.com/SingKaraoke/sing-lyrics-upload-api),
and [sing-lyrics-tools](https://github.com/SingKaraoke/sing-lyrics-tools).

```
auth/          device-key registration/signing (ed25519)
config/        env var loading
db/            Postgres access helpers (pgx)
models/        shared domain types (Track, Lyrics, ...)
objectstorage/ Cloudflare R2 (S3-compatible) client, multi-account support
search/        Meilisearch client + indexing helpers
ttml/          TTML lyrics parsing/validation
```

## Versioning

This repo is tagged with semver (`v0.1.0`, `v0.2.0`, ...). Consumers pin an exact
version in their own `go.mod`:

```
require github.com/SingKaraoke/sing-lyrics-pkg v0.1.0
```

Bump the minor version for additive changes, patch for fixes, and call out breaking
changes clearly in the tag/release notes since multiple repos depend on this one.

## Development

```bash
go build ./...
go vet ./...
go test ./...
```
