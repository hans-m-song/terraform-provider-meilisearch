# Provider migration and adoption notes

The M-01 modernization changes are included in the v0.3.0 tag pushed on 2026-09-18. Core index settings are included in the v0.3.1 release. Upstream correctness takes priority over legacy compatibility. The release workflow completed successfully; Terraform Registry publication and installation of v0.3.1 were verified on 2026-09-18. Verification evidence and completion status are in [roadmap](roadmap.md).

## Core index settings adoption

`meilisearch_index_settings` binds to an existing index UID. Initially omitted or null fields remain unmanaged and appear as null in resource state; the matching data source reads server values. `managed_fields` records ownership independently of values. Removing a managed field sends JSON null to select the server default, then relinquishes ownership. Destroy resets only owned fields and retains the index and documents.

UID import adopts all eight supported fields. Review configuration before the first apply: omitted imported fields reset, including a null distinct attribute whose ownership was recorded during import. Advanced object-based filterable rules cause a diagnostic when owned or read through the data source; unrelated advanced settings remain untouched.

Use one settings resource per index. Preflight GET and PATCH are separate operations; use a settings-scoped provider alias whose key excludes `indexes.create` to prevent implicit creation during concurrent deletion. Required actions are `indexes.get`, `settings.get`, `settings.update` and `tasks.get`. See [the verified API boundary](feasibility.md).

## Runtime and build baseline

The provider address is now `hans-m-song/meilisearch`, as confirmed on 2026-09-18. Update required-provider sources and local development overrides. For an existing binding, securely back up state, then migrate its provider address:

```shell
terraform state replace-provider \
  registry.terraform.io/paulden/meilisearch \
  registry.terraform.io/hans-m-song/meilisearch
```

This command is migration guidance; it was not run against user state. It changes the provider binding, not remote object UIDs. GitHub ownership transfer and Terraform Registry publication are separate operations and have not been performed.

Use Terraform 1.14+ for all provider configuration. Add an explicit constraint to your root module:

```hcl
terraform {
  required_version = ">= 1.14.0"
}
```

The build requires Go 1.25.8+; go.mod selects Go 1.27.1 as the preferred toolchain. Primary server validation targets Meilisearch v1.53.2; older-server compatibility is not a release guarantee. Current upstream module pins are listed in go.mod.

The Terraform baseline is a support contract and a configuration constraint. Framework's TerraformVersion request field is explicitly intended for logging/analytics, rather than provider behavior gating. [Framework ConfigureRequest](https://github.com/hashicorp/terraform-plugin-framework/blob/v1.19.0/provider/configure.go).

## Implemented data-source and configuration changes

- Index/key `id` values are their actual remote UIDs; version `id` is the package version. Outputs depending on the literal `placeholder` must be updated.
- Index/key timestamps use RFC3339. An absent key expiry is null instead of a zero-time string. Version build metadata is passed through; commit_date can be `unknown` when the server build has no date.
- Key data-source `key` values are sensitive. Sensitivity hides normal display, but managed keys/data sources continue to persist the credential in Terraform state.
- Hosts must be absolute HTTP(S) URLs without embedded credentials, query parameters, or fragments.
- `operation_timeout` is a positive Go duration, defaulting to `5m`; adjust it for queued or expensive asynchronous work. Cancellation and deadlines apply to data-source API reads as well as resource operations.
- API diagnostics report codes/statuses and task identities without echoing raw request bodies, authorization headers, or SDK request dumps.

## Implemented resource changes

The full acceptance suite passed on Terraform 1.14.9 and 1.16.3 against Meilisearch 1.53.2 on 2026-09-18. Schema and lifecycle changes:

- Key `actions` and `indexes` are unordered sets in resources and data sources. Configuration may still use bracket syntax, but expressions using numeric indexing must change to set membership or an explicit sorted list.
- Resource `id` is the remote UID. Index `primary_key` is optional/computed: omission retains the remote value or permits server inference when documents arrive. A configured non-empty change updates an empty index in place; populated indexes are rejected without deleting documents. Clearing a primary key with JSON null is outside this provider contract because the SDK update type cannot represent it.
- Key expiry is optional. Omission/null creates a never-expiring key; changing or removing a finite expiry replaces the immutable key. Empty strings are invalid. Imported expiry values use the server's RFC3339 representation; use UTC (`Z`) timestamps in configuration before import to avoid timezone-spelling differences being treated as changes.
- Index creation and deletion wait for successful asynchronous task completion. Failed/canceled tasks return errors; a timed-out creation wait retains the accepted index identity for reconciliation.
- Key name/description updates use a bounded HTTP PATCH adapter because SDK `omitempty` fields cannot encode nullable metadata removal. Removing configured metadata updates it to null; remote clearing and a no-op follow-up plan are verified.

Deterministic HTTP tests also verify cancellation, partial-creation identity retention, failed/canceled task state removal, populated-index rejection without mutation, and missing-index reconciliation. Legacy state conversion and timezone-insensitive expiry imports are not verified. The UTC spelling limitation is inferred from the current string comparison in key_resource.go.

## Operational tooling

`make build` compiles packages without installing into a global Go binary directory. Use `go install .` explicitly if installation is desired.

`make testacc` uses a disposable server harness. The previous destructive `clean` target and acceptance prerequisite were removed; no shared Docker volume or workspace Terraform state cleanup is performed. The legacy development Compose files were inspected with permission; their Meilisearch 1.7 persistent volume is separate from the tested 1.53.2 environment. Do not treat changing that image as a tested data migration. See automation-audit.md.

For deliberate schema breaks that require re-import, retain your state backup securely and use the documented UID import into the modernized address. Re-import adopts the existing remote object; do not destroy/recreate a populated index just to change Terraform state representation. Do not expose existing state or credentials while seeking assistance.

Automatic conversion of legacy state has not been verified. If legacy state cannot be decoded, remove only the affected binding and import its existing remote UID, for example:

```shell
terraform state rm meilisearch_key.example
terraform import meilisearch_key.example API_KEY_UID
terraform plan
```

Use `meilisearch_index.example` and the index UID for indexes. Keep configured key scopes, expiry, and metadata aligned with the imported object; review the resulting plan before applying. Removing a state binding does not delete the remote object. The import path is exercised against newly generated synthetic state; existing user state is outside the test scope.
