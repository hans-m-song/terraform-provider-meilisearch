# Feasibility investigation

Verified on 2026-09-18. This is a source/API investigation, not a compiled upgrade or live-server validation. Names and lifecycle choices marked proposed remain subject to approval.

## Dependency baseline

| Module | Repository pin | Verified upgrade target |
| --- | --- | --- |
| terraform-plugin-framework | v1.14.1 | v1.19.0 |
| terraform-plugin-go | v0.26.0 | v0.31.0 |
| terraform-plugin-log | v0.9.0 | v0.11.0 |
| terraform-plugin-testing | v1.12.0 | v1.16.0 |
| terraform-plugin-docs | v0.21.0 | v0.25.0 |
| meilisearch-go | v0.31.0 | v0.36.3 |

Local pins are in [go.mod](../go.mod). Each target was checked against the corresponding public Go module proxy `@latest` endpoint. For example: [Framework metadata](https://proxy.golang.org/github.com/hashicorp/terraform-plugin-framework/@latest), [SDK metadata](https://proxy.golang.org/github.com/meilisearch/meilisearch-go/@latest), [testing metadata](https://proxy.golang.org/github.com/hashicorp/terraform-plugin-testing/@latest), [docs metadata](https://proxy.golang.org/github.com/hashicorp/terraform-plugin-docs/@latest), [log metadata](https://proxy.golang.org/github.com/hashicorp/terraform-plugin-log/@latest), [protocol metadata](https://proxy.golang.org/github.com/hashicorp/terraform-plugin-go/@latest). Recheck at implementation time; these are a dated baseline.

Framework v1.19.0 requires Go 1.25.0; the verified latest testing/docs modules require Go 1.25.8. Thus the full proposed toolchain update requires at least Go 1.25.8, beyond this repository's `go 1.23.0` and `toolchain go1.24.1`. [Framework release](https://github.com/hashicorp/terraform-plugin-framework/releases/tag/v1.19.0), [testing module](https://proxy.golang.org/github.com/hashicorp/terraform-plugin-testing/@v/v1.16.0.mod), [docs module](https://proxy.golang.org/github.com/hashicorp/terraform-plugin-docs/@v/v0.25.0.mod).

The provider already uses Framework and protocol 6; no SDKv2 migration is needed. SDK release notes identify network changes in v0.35.0 and JWT v5/Go 1.21 in v0.36.0. These do not establish that all existing provider calls are compatible: compilation and acceptance tests are still required. [SDK releases](https://github.com/meilisearch/meilisearch-go/releases).

## Existing correctness work

Direct source observations:

- `index_resource.go`: creation calls a non-context task waiter with a five-second polling interval. That interval is not a five-second operation timeout. Only `succeeded` populates the result; another returned terminal status has no explicit diagnostic before state is written.
- Index deletion submits a task without waiting. Terraform can consider deletion complete before the server completes it, which matters for replacement and dependent operations.
- Index/key reads detect absence through substrings in error messages. Configure type-assertion failures only log instead of returning diagnostics.
- Both resource types use `id = "placeholder"`. Replace these with meaningful remote identity or remove redundant IDs under an explicitly documented breaking schema change; legacy compatibility is not a priority.
- Existing index primary-key changes force replacement. Reassess against current upstream behavior, especially updates to an empty index versus one containing documents. Choose correct semantics and document any replacement/data implications rather than preserving legacy behavior automatically.
- README lists `meilisearch_api_key`; registration exposes `meilisearch_key`. Existing index/key acceptance tests cover creation, replacement or update, and import, but not the new features.

Evidence: [index resource](../internal/provider/index_resource.go), [key resource](../internal/provider/key_resource.go), [registration](../internal/provider/provider.go), [README](../README.md), [index tests](../internal/provider/index_resource_test.go), [key tests](../internal/provider/key_resource_test.go). These observations motivate M-01; they are not live reproductions.

## Lifecycle suitability

| Feature | API feasibility | Proposed Terraform model | Assessment |
| --- | --- | --- | --- |
| Index settings | High; readable settings and asynchronous updates/resets | Managed resource plus read-only data source | Strong fit; request fidelity is the main risk |
| Runtime experimental flags | High for API-exposed flags | Singleton managed resource plus data source | Suitable if ownership/reset policy is explicit |
| Network topology | API and SDK exist | Separate experimental resource | Conditional; cluster coordination deserves separate design |
| Startup instance configuration | Launch options/config files | Deployment provider/module | Outside the server API provider's natural boundary |
| Webhooks | Full UUID-based CRUD | Managed resource plus data source | Strong fit for editable API-created hooks |
| Durable API keys | Existing CRUD resource | Keep managed resource | Appropriate for application credentials |
| Temporary API keys | Create, expiry, delete | Ephemeral resource | Feasible with expiry fallback and cleanup contract |
| Document ingestion | Async addition/replacement/update | Action | Suitable for bounded seeding; bulk synchronization belongs in a pipeline |
| Snapshots | Async creation | Action | Suitable for triggering a server-side backup |
| Dumps | Async creation | Action | Suitable for triggering a portable server-side export |

The latter four model assessments are design inferences from remote effects and Terraform lifecycle semantics, not upstream recommendations specific to this provider.

Ephemeral resources require Terraform 1.10+, write-only managed-resource arguments require 1.11+, and actions require 1.14+. Framework upgrades alone do not establish the provider's minimum compatible Terraform CLI: test discovery and existing resources on the selected baseline. [Ephemeral resources](https://developer.hashicorp.com/terraform/plugin/framework/ephemeral-resources), [write-only arguments](https://developer.hashicorp.com/terraform/plugin/framework/resources/write-only-arguments), [action tutorial requirements](https://developer.hashicorp.com/terraform/tutorials/configuration-language/actions).

The user confirmed a provider-wide minimum increase and modernization over legacy compatibility. Proposed baseline: Terraform 1.14+ for the entire provider, with current stable Terraform also tested. Breaking resource schemas and state changes are allowed when justified by upstream correctness; migration or re-import instructions remain required, but backward-compatible shims are not mandatory. Server API only is confirmed; Cloud provisioning and startup controls are excluded.

Current stable release targets verified on 2026-09-18: Terraform v1.16.3 and Meilisearch v1.53.2. Use these as the primary integration targets, refreshing pins at implementation time; do not infer complete SDK support for this newer server release. Older-server support is optional and must not delay current API coverage. [Terraform release](https://github.com/hashicorp/terraform/releases/tag/v1.16.3), [Meilisearch release](https://github.com/meilisearch/meilisearch/releases/tag/v1.53.2).

## Index settings contract

Approved M-02 contract: `meilisearch_index_settings` identifies an existing index by UID and does not own its lifetime. Reference the index resource when Terraform also creates it, and preflight existence to avoid accidental creation by the settings endpoint. Updates do not replace the index. Implementation and acceptance verification are complete on Meilisearch 1.53.2.

The API distinguishes omitted fields, explicit null resets, and actual empty/false/zero values. Settings changes may reindex data. [Update all settings](https://www.meilisearch.com/docs/reference/api/settings/update-all-settings).

The SDK's aggregate `Settings` has `omitempty` on slices/maps/scalars, including `FacetSearch bool`, while its filterable-attribute field is string-only despite dedicated methods supporting mixed rules. A direct struct conversion cannot represent every required state. Webhook update maps likewise cannot represent null header removals. Use dedicated SDK methods where faithful; otherwise isolate exact API payloads. [SDK types](https://github.com/meilisearch/meilisearch-go/blob/v0.36.3/types.go), [SDK API interfaces](https://github.com/meilisearch/meilisearch-go/blob/v0.36.3/meilisearch_interface.go), [advanced filterable attributes](https://www.meilisearch.com/docs/reference/api/settings/update-filterableattributes).

Core ownership policy approved on 2026-09-18:

- Initially omitted field: unmanaged; preserve server value.
- Configured field: compare and reconcile drift.
- Previously managed field removed from configuration: reset that field, then relinquish ownership.
- Resource deletion: reset only fields it owns, never delete the index; skip reset if the parent index is already absent.
- Import: adopt the eight supported core fields through UID import; align configuration with the desired imported values before applying. Omitted imported fields reset on apply. Settings outside this scope remain unmanaged; advanced object-based filterable rules cause a diagnostic when importing or reading an owned filterable field, without changing the server.

Resets use explicit JSON null for the individual fields in a settings PATCH. The server selects its own defaults; the provider does not hardcode them or call reset-all for resource deletion. Omission preserves the current server value. Empty lists/maps are sent as `[]`/`{}` rather than null; their semantics are field-specific. In v1.53.2, empty stop-word and synonym settings reset internally and GET returns empty values. [Settings PATCH semantics](https://www.meilisearch.com/docs/reference/api/settings/update-all-settings), [versioned settings implementation](https://github.com/meilisearch/meilisearch/blob/v1.53.2/crates/milli/src/update/settings.rs).

Concurrency limitation: the API can create a missing index during settings PATCH. Preflight rejects an already-missing index, but a separate GET/PATCH cannot provide an atomic guarantee against concurrent or queued deletion. Meilisearch 1.53.2 derives implicit-creation permission from `indexes.create`; use a separate provider alias with a UID-scoped key granting `indexes.get`, `settings.get`, `settings.update` and `tasks.get`, excluding `indexes.create`, to prohibit that creation. A synthetic live probe confirmed a failed `index_not_found` task with the index still absent. Concurrent external deletion/recreation remains outside an atomic provider guarantee. [Settings route](https://github.com/meilisearch/meilisearch/blob/v1.53.2/crates/meilisearch/src/routes/indexes/settings.rs), [authorization filters](https://github.com/meilisearch/meilisearch/blob/v1.53.2/crates/meilisearch-auth/src/lib.rs).

Verified 1.53.2 normalization: stop words undergo compatibility decomposition (NFKD), preserving case; configured spelling may be retained only when equivalently normalized sets match the server. Synonyms persist raw keys/list order/duplicates. Searchable/displayed lists deduplicate first occurrences and collapse any wildcard list to ["*"]. Provider input validation rejects those redundant ordered-list forms to keep plans consistent. [Settings update implementation](https://github.com/meilisearch/meilisearch/blob/v1.53.2/crates/milli/src/update/settings.rs). These behaviors were also confirmed with disposable synthetic server probes; full provider acceptance passed on Terraform 1.14.9 and 1.16.3.

Before a PATCH changing filterable attributes, the provider reads the remote settings and rejects unsupported object-based rules without mutation; this read and PATCH are separate requests, so concurrent external changes cannot be excluded. [Provider mutation guard](../internal/provider/index_settings_resource.go).

Versioned Charabia source establishes additional non-lossy stop-word transformations: non-whitespace controls are removed, Persian character/digit/punctuation mappings apply globally to strings, and Swedish recomposition remains NFKD-equivalent. The adapter uses one comparison key for these transformations in drift comparison, spelling retention and duplicate validation; it does not lowercase or strip diacritics. Live acceptance verified case/diacritic preservation, Persian kaf/digits, control removal and Swedish recomposition with stable repeated plans. [String normalizer pipeline](https://github.com/meilisearch/charabia/blob/bc63b9860434f06cf78b51bb0eab00aaefb69f15/charabia/src/normalizer/mod.rs#L284-L300), [control normalization](https://github.com/meilisearch/charabia/blob/bc63b9860434f06cf78b51bb0eab00aaefb69f15/charabia/src/normalizer/control_char.rs), [Persian normalization](https://github.com/meilisearch/charabia/blob/bc63b9860434f06cf78b51bb0eab00aaefb69f15/charabia/src/normalizer/persian.rs).

Settings API responses are bounded to 1 MiB, so larger aggregate settings responses return an error.

The implemented managed_fields set records ownership independently of nullable values. Ordered fields such as ranking/searchable attributes remain lists. Choose sets only where server semantics establish order is irrelevant. Test normalized responses and a no-op second plan.

First slice: searchable/displayed/filterable/sortable attributes, ranking rules, stop words, synonyms, distinct attribute. Next: typo tolerance, pagination, faceting, tokenization, proximity precision, cutoff, localized attributes, prefix/facet search, and advanced filter rules. Then assess embedders/chat and newer server fields against a versioned coverage ledger.

SDK freshness does not imply complete server coverage. For example, current API documentation includes foreign keys absent from the inspected SDK bulk settings type. Track supported, deliberately deferred, experimental, and unavailable settings explicitly. Embedder secrets need a separate redaction/write-only design. [Current settings API](https://www.meilisearch.com/docs/reference/api/settings/update-all-settings), [embedder API](https://www.meilisearch.com/docs/reference/api/settings/update-embedders).

## Instance controls

There is no basis here for promising arbitrary startup configuration updates through a single `instance_settings` resource. Launch configuration belongs to the system running the server. [Launch configuration](https://www.meilisearch.com/docs/resources/self_hosting/configuration/overview).

Prefer a singleton `meilisearch_experimental_features`, identified by the configured instance, with typed optional flags and ownership limited to configured fields. Build fresh SDK request managers per operation to avoid sharing mutable flag builders. Unknown flags/version mismatches must produce actionable diagnostics rather than pretending a value was applied. [Experimental flag API](https://www.meilisearch.com/docs/reference/api/experimental-features/configure-experimental-features).

Network topology is a distinct control plane: topology changes can involve coordinated work and remote credentials. Defer until per-instance versus whole-cluster ownership, task completion, recovery, edition availability, and secret handling are specified. [Network API](https://www.meilisearch.com/docs/reference/api/experimental-features/configure-network-topology).

Runtime console-log configuration has an update endpoint, but this investigation did not establish a corresponding readable configuration endpoint. Treat it as a possible action, not a drift-managed resource, unless a read contract is verified. [Log target API](https://www.meilisearch.com/docs/reference/api/logs/update-target-of-the-console-logs).

## Webhooks

Proposed `meilisearch_webhook`: UUID identity, URL, headers, computed editability, UUID import, drift reconciliation, idempotent deletion. Reject attempted management of immutable hooks clearly. API docs expose CRUD and editability. [Create](https://www.meilisearch.com/docs/reference/api/webhooks/create-webhook), [update](https://www.meilisearch.com/docs/reference/api/webhooks/update-webhook).

Authorization headers are redacted when reading hooks. Never write the redacted response back as a credential. Tests must establish patch/removal behavior and retain or re-supply the original secret appropriately. A sensitive attribute still persists in state; optional write-only inputs with a non-secret revision trigger avoid this but cannot detect remote secret drift. Imports cannot recover redacted credentials. URLs can also contain secrets and must not be included indiscriminately in diagnostics. [Webhook limits and redaction](https://www.meilisearch.com/docs/resources/self_hosting/webhooks), [write-only behavior](https://developer.hashicorp.com/terraform/plugin/framework/resources/write-only-arguments).

The exact earliest server release and edition availability of editable webhook CRUD remain unverified. SDK webhook support was added in v0.33.1; that is not the server minimum. Resolve the server matrix before declaring compatibility.

## Ephemeral keys

Proposed ephemeral `meilisearch_key` exists alongside the managed resource of the same type in its separate Terraform namespace. `Open` creates a fresh UID, restrictive scopes, and a required bounded expiry; the result contains a sensitive credential. Private lifecycle data carries the UID needed for cleanup. `Close` revokes only the key created by that opening. Never revoke an existing key read by an ephemeral lookup.

Terraform can open ephemeral resources during planning and applying; optional Close/Renew methods serve their lifecycle. Cleanup cannot be assumed after a crash, so server expiry is mandatory for newly created temporary keys. This does not make credentials for a long-running application suitable for ephemeral creation. Values can only flow into permitted ephemeral contexts. [Open lifecycle](https://developer.hashicorp.com/terraform/plugin/framework/ephemeral-resources/open), [ephemeral contexts](https://developer.hashicorp.com/terraform/language/block/ephemeral).

Key updates expose name/description rather than expiry extension. There is therefore no verified in-place lease renewal design. Initially require an expiry long enough for the run, fail clearly on insufficient validity, and document long-run limits; do not promise transparent rotation. [Key updates](https://www.meilisearch.com/docs/reference/api/keys/update-api-key).

Alternative: an ephemeral read of an existing key avoids storing its value in Terraform, but creates no temporary remote credential and cannot safely delete it. Investigated docs are inconsistent: the documentation index describes key values as creation-only, while the specific GET reference includes the key in its response. M-01 key import verification on 2026-09-18 retrieved the credential successfully from Meilisearch 1.53.2; broader server behavior remains unclaimed. Lookup remains a possible later proposal, not an implemented ephemeral feature. [GET key reference](https://www.meilisearch.com/docs/reference/api/keys/get-api-key), [documentation index](https://www.meilisearch.com/docs/llms.txt), [M-01 verification](roadmap.md).

## Ingestion and backups

Actions represent remote side effects, support direct invocation or lifecycle triggers, and currently cannot modify resource state. Therefore do not advertise durable computed backup outputs as ordinary resource attributes. [Framework actions](https://developer.hashicorp.com/terraform/plugin/framework/actions).

Proposed ingestion action: require an existing index and explicit stable document IDs; choose replace versus partial-update semantics explicitly; support bounded JSON/NDJSON inputs; batch and await all submitted tasks. Define retries after an uncertain submission and partial completion. Repeated invocation must not delete unmentioned documents. Keep recurring synchronization, unbounded datasets, and pipeline checkpointing out of scope. [Document operations](https://www.meilisearch.com/docs/capabilities/indexing/how_to/add_and_update_documents), [large imports](https://www.meilisearch.com/docs/capabilities/indexing/how_to/import_large_datasets).

An ingestion ephemeral resource could technically make writes during Open, but would create lasting effects during planning and have no legitimate Close operation. Snapshots/dumps have the same lifecycle mismatch. Exclude these ephemeral designs. An ephemeral credential consumed by an operation is a separate, suitable concern; validate permitted action credential contexts before committing to that composition.

Snapshot/dump actions submit work and wait for success, reporting task identity and available metadata through diagnostics/progress. Files are created on the server; remote API reachability does not give the Terraform runner filesystem access. No normal backup artifact CRUD is established by these creation endpoints. Storage/retention and recovery remain operational responsibilities. [Snapshot creation](https://www.meilisearch.com/docs/reference/api/backups/create-snapshot), [dump creation](https://www.meilisearch.com/docs/reference/api/backups/create-dump).

Snapshots serve periodic recovery; dumps support migration. Do not turn backup creation into a restore operation: restoration needs a separate deployment/recovery workflow. Edition/Cloud endpoint availability and supported restore-version combinations must be validated before release. [Snapshot guide](https://www.meilisearch.com/docs/resources/self_hosting/data_backup/snapshots), [dump guide](https://www.meilisearch.com/docs/resources/self_hosting/data_backup/dumps).

## Remaining evidence gates

- Compile the upgraded dependency set and run acceptance tests; neither happened during research.
- Inspect test/deployment/release configuration only after permission to read potentially secret-bearing files.
- Pin minimum and current server versions, editions, and per-feature capability requirements.
- Resolve webhook null-removal and settings round-trip behavior with HTTP contract tests and live acceptance tests.
- Verify secret absence in plans/state for ephemeral/write-only features using synthetic fixtures.
- Confirm remaining ownership/lifecycle decisions before implementation; no dates or effort estimates are promised before these gates. Server-only scope and modernization priority are already confirmed.
