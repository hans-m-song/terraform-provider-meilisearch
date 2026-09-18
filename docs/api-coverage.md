# API coverage ledger

Baseline researched 2026-09-18: Meilisearch v1.53.2, Go SDK v0.36.3, Framework v1.19.0. Source-level coverage is not live acceptance evidence. Refresh against current tagged releases before each slice.

| API family | Terraform surface | Status / gate |
| --- | --- | --- |
| Index metadata | `meilisearch_index` resource/data source | M-01 implemented; UID, optional primary key, timestamps, async completion |
| API keys | `meilisearch_key` resource/data source | M-01 implemented; set scopes, nullable metadata, optional immutable expiry |
| Version | `meilisearch_version` data source | M-01 implemented; package-version identity, raw build metadata |
| Core search settings | `meilisearch_index_settings` | Implemented M-02; verified on Meilisearch 1.53.2 |
| Advanced search settings | Same settings owner | Proposed M-03; empty/false/zero/null fidelity required |
| Mixed filter rules | Typed settings field | SDK dedicated methods support richer types than bulk Settings; verify live round trip |
| Foreign keys and newest settings | Versioned settings extension | Current API/SDK gap; bounded HTTP fallback or upstream SDK change needed |
| Embedders/chat settings | Versioned settings extension | Proposed M-03; source/edition/secret contracts required |
| Experimental runtime flags | Singleton resource/data source | Proposed M-04; managed-field ownership and flag-version ledger required |
| Network topology | Separate experimental resource | Deferred; cluster ownership/recovery design required |
| Runtime log updates | Possible action | Deferred; readable configuration not established |
| Editable task webhooks | UUID resource/data source | Proposed M-05; redact-aware credentials and null header removals required |
| Temporary API-key creation | Ephemeral `meilisearch_key` | Conditional M-06; planning-time writes, cleanup, and expiry contract |
| Document addition/update | Ingestion action | Conditional M-07; bounded seed data only |
| Snapshot/dump creation | Separate actions | Conditional M-07; server-side artifacts only |
| Server startup controls / Cloud provisioning | None | Excluded by user-confirmed server-API-only scope |

Sources: [current settings API](https://www.meilisearch.com/docs/reference/api/settings/update-all-settings), [SDK types](https://github.com/meilisearch/meilisearch-go/blob/v0.36.3/types.go), [SDK operations](https://github.com/meilisearch/meilisearch-go/blob/v0.36.3/meilisearch_interface.go), [runtime flags](https://www.meilisearch.com/docs/reference/api/experimental-features/configure-experimental-features), [webhook API](https://www.meilisearch.com/docs/reference/api/webhooks/create-webhook). Specific uncertainties and supporting evidence are in [feasibility](feasibility.md).
