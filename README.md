# fctl plugin registry

The official catalogue of native service plugins for fctl v4. This repository
contains distribution metadata. Product repositories own their plugin source,
builds and releases; public OCI registries store the executables.

Catalogue URL:

```text
https://raw.githubusercontent.com/formancehq/fctl-plugin-registry/main/registry.yaml
```

The catalogue starts empty. No executable is advertised until its public artifact
has been published and verified. The v4 registry integration keeps embedded
providers for service versions without an advertised native release. External
discovery currently applies to Ledger.

See [catalogue maintenance](docs/catalogue.md) for the schema and publication
workflow.
