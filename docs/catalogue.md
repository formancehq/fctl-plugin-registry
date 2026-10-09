# Catalogue maintenance

`registry.yaml` is a YAML document with `schemaVersion: 1` and a `releases`
sequence. An empty sequence is valid. Do not add placeholder releases.

## Release fields

Each release uses the same JSON field names as the fctl distribution contract:

| Field | Meaning |
| --- | --- |
| `service` | Service identity, initially `ledger` |
| `serviceVersion` | Exact version returned by the service's `/_info`, without the `v` prefix |
| `revision` | Positive plugin revision, independent of the service version |
| `platform.os` | `darwin`, `linux` or `windows` |
| `platform.arch` | `amd64` or `arm64` |
| `artifact.registry` | Public HTTPS OCI registry origin |
| `artifact.repository` | OCI repository path |
| `artifact.digest` | Immutable OCI manifest digest, with the `sha256:` prefix |
| `sha256` | SHA-256 of the raw executable bytes |
| `manifest` | Complete SDK command and form manifest generated from the executable |

The manifest includes the plugin name, service, exact version, protocol version
and command tree. Preserve JSON field names when converting publisher output to
YAML, and quote values that YAML could interpret as booleans or numbers. A tuple
of service, service version, revision, OS and architecture must be unique.

## Publish a release

1. Build the plugin in its product repository for each supported platform.
2. Generate its command manifest from the actual executable.
3. Publish native executable layers and OCI manifests to a public registry.
4. Verify anonymous download access, executable SHA-256 and OCI digests.
5. Open a pull request adding the generated release records to `registry.yaml`.

A CLI-only fix increments `revision` for the same service version. Add new
records rather than changing the bytes or digests of an existing release.
Product release automation should submit catalogue updates after publication;
this repository does not build or publish product executables.

## Host discovery

The fctl v4 registry integration reads this URL automatically for a new Ledger
target. It selects the exact deployed service version and current platform,
then installs the executable and saves a target lock with its manifest.

Until a matching release is advertised, new targets keep the embedded Ledger
provider. New targets also keep it when the official catalogue is temporarily
unreachable; malformed catalogue metadata is an error. An already installed external plugin retains its revision. A service
version change requires a matching catalogue release; it must not silently
replace an installed external plugin with an embedded provider.

`fctl plugins sync` explicitly resolves the newest plugin revision for the exact
service version. An absent release is an error for explicit sync. Help and shell
completion use cached metadata without a registry request or plugin process.

The `--catalogue` flag overrides the source for explicit sync. The
`FCTL_PLUGIN_CATALOGUE` environment variable overrides discovery and sync. A
previous target's catalogue remains available for later version changes. The
host accepts YAML and JSON catalogues.

Checksums are verified against this catalogue. They do not independently verify
the publisher: maintainers must protect catalogue and artifact publication.
