# fctl plugin registry

The official catalogue of native service plugins for fctl v4. This repository
contains distribution metadata. Product repositories own their plugin source,
builds and releases; public OCI registries store the executables.

Catalogue URL:

```text
https://raw.githubusercontent.com/formancehq/fctl-plugin-registry/main/registry.yaml
```

Auth 2.5.2 revision 1 is available for Linux, macOS and Windows on amd64 and
arm64. Its six GHCR artifacts were downloaded anonymously and verified against
the product release catalogue before advertisement.

Auth uses external discovery in fctl v4. Ledger retains its embedded provider
until a matching native release is advertised.

See [catalogue maintenance](docs/catalogue.md) for the schema and publication
workflow.
