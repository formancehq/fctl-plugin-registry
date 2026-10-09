# Catalogue maintenance

## Central index: schema v2

```yaml
schemaVersion: 2
plugins:
  auth:
    releases:
      - serviceVersion: 2.5.2
        catalogue: https://github.com/formancehq/auth/releases/download/v2.5.2/fctl-plugin-catalogue.json
        sha256: "<64 lowercase hexadecimal characters>"
```

This is the entire contract. `plugins` maps a lowercase service identifier to a
`releases` sequence. Each reference contains exactly `serviceVersion`, `catalogue`
and `sha256`; embedded platform records and command manifests are forbidden.
`serviceVersion` is an exact semantic version without `v`, not a range or tag.
A service/version pair is unique. An empty plugins map or release sequence is
valid; null or missing required collections are not.

`catalogue` is an absolute public HTTPS URL, without credentials or a fragment.
`sha256` is the checksum of the **raw downloaded file bytes**. Reformatting JSON,
changing a trailing newline, or reordering properties changes this checksum.
Compute it on the downloaded asset, not on a parsed/reserialized document:

```sh
curl --fail --location --output catalogue.json \
  https://github.com/formancehq/auth/releases/download/v2.5.2/fctl-plugin-catalogue.json
shasum -a 256 catalogue.json
```

## Product catalogue: schema v1

The product release asset keeps its existing JSON contract:
`schemaVersion: 1`, `releases: [...]`. Each entry contains:

- `service`, `serviceVersion`, positive `revision`;
- `platform: {os, arch}` (`darwin`, `linux`, `windows`; `amd64`, `arm64`);
- `artifact: {registry, repository, digest}` (public HTTPS OCI origin,
  repository path and immutable `sha256:` manifest digest);
- `sha256` (checksum of the raw executable bytes);
- `manifest` (the public pluginsdk command, flag and form manifest).

Every entry must match the central service/version reference. A tuple of
service/version/revision/OS/architecture is unique. Product catalogues cannot
refer to another catalogue or central index. The manifest's name, service,
version, root and supported protocol must match its release. Unknown and duplicate
fields are rejected in both documents. YAML anchors, aliases, duplicate mapping
keys and multiple documents are rejected in the central index.

The validator anonymously retrieves OCI manifests, verifies their raw digests,
requires the plugin artifact type, empty JSON config and exactly one raw
executable layer, checks layer/config descriptors, then downloads each executable
and verifies its size and SHA-256. Catalogue files are limited to 8 MiB, OCI
manifests to 4 MiB and executables to 128 MiB. Bearer challenges must request only
`pull` on the advertised repository; redirects require HTTPS and do not forward
registry tokens across hosts.

## Publish and promote

1. Build the plugin in its product repository under `misc/fctl-plugin`.
2. Generate the command manifest from the product build and publish all native
   executable layers and OCI manifests publicly.
3. Publish the complete `fctl-plugin-catalogue.json` release asset.
4. Download that asset and compute its raw SHA-256.
5. Open a PR adding the exact service/version, asset URL and checksum.
6. Run `just pre-commit`, `just tests`, `just build` and `just validate`.
7. Merge only after registry validation succeeds and maintainers review the
   advertised versions. CI validates PRs, main and manual runs without secrets.

An existing executable or digest must never be replaced under the same release
tuple. A CLI fix increments the plugin revision. If the product republishes its
catalogue with that revision, update the central checksum in a reviewed PR;
existing target locks retain their pinned revision until explicit sync.

Maintainers must configure branch protection to require **Registry validation /
validate**. Workflow files alone do not activate branch protection. Checksums
ensure integrity and bind the central index to product assets; they do not replace
publisher trust or repository/release access controls.

## Host resolution

fctl selects the exact service/version reference, verifies the product catalogue
checksum **before parsing**, then selects the current platform and requested or
latest revision. Installation and target locks retain the existing contract.
Cached command manifests support help and completion offline. The central index
contains no command tree and no binaries. `FCTL_PLUGIN_CATALOGUE` and explicit
`--catalogue` sources can still refer directly to product schema v1 catalogues.
