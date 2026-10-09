# fctl plugin registry

The official release index for fctl v4. Product repositories own plugin source,
builds, command manifests and release catalogues. This repository advertises
immutable references to those public catalogues. It does not execute plugins,
build product binaries, or require Cloud/GitHub credentials.

Repository profile: validation CLI. The executable is a repository maintenance
utility, not a deployed service or distributed product. It uses Go's flag package
and standard HTTP client; Cobra, Fx, go-libs and GoReleaser are not needed for this
profile. Maintainers should revisit this exception if the validator becomes a
published service or CLI.

Official URL:

```text
https://raw.githubusercontent.com/formancehq/fctl-plugin-registry/main/registry.yaml
```

Auth 2.5.2 revision 1 remains available for Linux, macOS and Windows on amd64 and
arm64. The index references the original public release catalogue without
changing its bytes or its six OCI artifacts. fctl must support central schema v2
before promotion; schema v1 product catalogues remain unchanged.

## Validate a change

The committed Nix lock pins Go 1.26, Just and golangci-lint. Local and CI checks
use identical recipes:

```sh
nix develop --impure --command just pre-commit
nix develop --impure --command just tests
nix develop --impure --command just build
nix develop --impure --command just validate
```

`just validate` downloads every referenced catalogue and every advertised raw
executable anonymously, checking SHA-256 at each boundary. It never runs downloaded
code. The command has a ten-minute overall deadline, bounded HTTP requests and
bounded downloads. Network failures fail the promotion gate.

`just validate-local` checks only index structure without network access. It does
not replace full validation. Tests use synthetic in-process HTTP fixtures, run
with the race detector and enforce at least 80% statement coverage. CI uploads
its coverage report as an artifact without a Codecov token.

See [catalogue maintenance](docs/catalogue.md) for the contract and release process.
