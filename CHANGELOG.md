## Unreleased

- Change GitHub links and the Terraform provider namespace to `hans-m-song`; existing state bindings require provider-address migration.
- Align CI with the supported Terraform matrix and disposable acceptance harness; update GoReleaser archive configuration.

- Upgrade Terraform Plugin Framework to v1.19.0 and Meilisearch Go SDK to v0.36.3; update protocol, logging, testing, and documentation dependencies.
- Require Go 1.25.8+ to build; establish Terraform 1.14+ as the provider-wide supported baseline.
- Add configurable operation deadlines and context-aware API calls.
- Wait for successful index creation/deletion tasks; reconcile missing remote objects and update primary keys in place on empty indexes.
- Represent key scopes as unordered sets and support explicit nullable metadata updates.
- Make key expiry optional; changing or removing a finite expiry replaces the immutable key.
- Replace placeholder identifiers with remote UIDs (and package version for the version data source); normalize index/key timestamps to RFC3339.
- Mark API-key data-source credentials sensitive. Managed keys and key data sources still store credentials in state.
- Preserve server-reported unavailable version build metadata as `unknown`.
- Replace destructive shared acceptance-test cleanup with a disposable, isolated test harness.

See docs/migration.md for deliberate behavior changes. This entry describes unreleased workspace changes; verification status is recorded in docs/roadmap.md.

## 0.0.1

FEATURES:
- Setup `meilisearch` provider.
- Add `meilisearch_key` data source.
- Add `meilisearch_key` resource.
