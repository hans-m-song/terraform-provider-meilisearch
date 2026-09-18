# Modernization roadmap

Prepared 2026-09-18. Status: M-01 implemented and verified; later milestones proposed. Confirmed scope: server API only. Confirmed policy: raise the provider-wide Terraform minimum and prioritize current upstream functionality over legacy compatibility. M-01 baseline: Terraform 1.14+. Milestone and task IDs are immutable; removed tasks retain their IDs and a disposition.

## Research checklist

- [x] Inspect current dependencies, registration, resource lifecycles, and acceptance coverage.
- [x] Verify dated upstream dependency targets and toolchain constraints.
- [x] Assess index settings, runtime controls, webhooks, temporary keys, ingestion, and backups.
- [x] Record API/SDK mismatches, lifecycle recommendations, and unresolved evidence.
- [x] Define vertical slices, dependencies, ownership, and verification gates.
- [x] Confirm server API scope and modernization-over-compatibility policy.
- [x] Approve the M-01 baseline and existing-resource modernization boundary.
- [ ] Approve later-feature lifecycle/ownership and server/edition contracts before their implementation.
- [x] Authorize M-01 implementation on 2026-09-18.

See [feasibility](feasibility.md) for evidence, [overview](overview.md) for proposed policy, and [structure](structure.md) for boundaries.

Primary integration targets as verified on 2026-09-18: Terraform v1.16.3 and Meilisearch v1.53.2. Recheck before later slices. Provider-wide supported minimum is Terraform 1.14+; supporting older Meilisearch servers is optional and must not delay current API coverage.

## Deliverable sequence

```text
M-01 reliable upgraded indexes and keys
  |
  +--> M-02 core index settings --> M-03 extended settings
  |
  +--> M-04 runtime flags
  |
  +--> M-05 editable webhooks
  |
  +--> M-06 temporary credentials [conditional]
  |
  +--> M-07 ingestion and backup actions [conditional]
              |
              v
        M-08 verified release
```

Sequence for delivery: M-01, M-02, M-03, M-04, M-05, then approved optional features and M-08. Separate features can be developed in parallel after shared API/task boundaries stabilize. M-03 secret inputs depend on the chosen write-only contract; they do not require ephemeral key creation.

Common roles: main agent owns scope, integration, and durable documentation; `executor_light` implements bounded tasks; `tester` verifies independently; reusable `explorer` investigates bounded supplementary context. Delegation begins only after implementation approval. Workers do not own shared registration/dependency changes without coordination.

## M-01 — Existing resources work reliably on current dependencies

Status: complete on 2026-09-18. User-verifiable outcome: modernized index/key resources use current upstream APIs, import correctly, and produce a no-op follow-up plan. Breaking changes have an explicit migration or re-import path on the approved CLI/server matrix.

Verification: full acceptance suite passed on Terraform 1.14.9 and 1.16.3 against official Meilisearch v1.53.2; an added targeted omitted-primary-key resource create/no-op/import test also passed on both targets. Index/key resource tests passed on Terraform 1.15.8 before the final expiry refinement. Unit tests, vet, build, default-configuration golangci-lint, documentation generation, provider-example validation, and diff checks passed. Deterministic SDK/HTTP tests cover cancellation, failed/canceled creation, partial-create identity, populated-index rejection, and missing-object reconciliation. See [migration](migration.md) and [handoff](handoff.md).

Limits: automatic legacy-state conversion is untested; expiry imports require matching UTC spelling; older server/edition/OpenTofu compatibility is unclaimed. Authorized configuration inspection and T-15 repairs are complete, with configured lint and release configuration checks passing. GitHub execution/signing/cross-platform artifacts remain M-08 release gates.

### T-01 — Approve compatibility and API coverage contracts

Status: complete for M-01; later-feature decisions remain in their feature tasks.

- Goal/scope: finalize the M-01 provider-wide Terraform 1.14+ baseline, current-server test matrix, and deliberate existing-resource schema decisions; establish a per-setting API/SDK coverage ledger. Record later-feature ownership/delete and edition/version proposals for approval in their feature tasks. Server-only scope and modernization priority are settled.
- Constraints: no legacy shims solely to retain old resources; avoid gratuitous renames; document deliberate breaks; no assumed OpenTofu guarantee.
- Ownership/roles: main agent decides with user; explorer supplies version/edition evidence.
- Dependencies: this investigation.
- Acceptance: confirmed decision record and pinned test matrix; every requested feature categorized as supported, conditional, or excluded.
- Validation gate: supported tagged API/source documentation checked against the matrix.
- Blockers: none for M-01. Earliest webhook server/edition support and later-feature ownership approvals remain gates of their respective feature tasks.
- Parallel boundary: read-only research may run independently; shared contracts must settle before dependent implementation.

### T-02 — Upgrade dependencies and build/test toolchain

Status: complete for M-01. Direct module targets upgraded; build, meaningful tests, lint, and documentation generation passed. Repository configuration inspection is complete; T-15 addresses the identified CI/release gaps.

- Goal/scope: upgrade the six direct upstream modules to reverified targets, choose a supported Go toolchain at least 1.25.8 for the currently verified tool set, tidy modules, adapt changed SDK signatures, and update build/lint/docs tooling.
- Constraints: changes target current upstream APIs and tools; any schema/state break is deliberate and documented, not incidental.
- Ownership/roles: executor_light owns dependency/tool changes; main agent integrates; tester verifies.
- Dependencies: T-01. Repository configuration inspection permission was subsequently granted and the audit completed.
- Acceptance: build, meaningful tests, lint, and documentation generation pass; modernized examples validate; dependency changes and any required legacy configuration changes explained.
- Validation gate: module consistency, SDK compile checks, and acceptance coverage on approved CLI/server baselines.
- Blockers: none for the verified M-01 local toolchain. T-15 repairs the identified CI/release configuration gaps; development data migration is separate.
- Parallel boundary: exclusive ownership of go.mod/go.sum and shared automation; feature workers wait for stabilized dependencies.

### T-03 — Correct existing remote-operation lifecycle

Status: complete on 2026-09-18. Shared deadlines/error helpers, data-source modernization, index/key lifecycle, nullable key metadata PATCH, optional immutable expiry, and disposable-server acceptance are verified.

- Goal/scope: cancellable requests and task waits, configurable operation deadlines, terminal-task failure diagnostics, typed not-found classification, idempotent deletion, Configure diagnostics, meaningful resource identity, current key/index schemas, primary-key semantics verified against upstream, and acceptance-test cleanup/readiness improvements.
- Constraints: prioritize correctness over legacy state; document schema changes and migration/re-import; retain remote identity on recoverable partial creation failure so a failed wait does not silently orphan an index. Do not force destructive replacement merely to simplify schema modernization.
- Ownership/roles: executor_light owns API/task helpers and existing lifecycle changes; tester owns verification; main agent coordinates registration/configuration.
- Dependencies: T-02.
- Acceptance: index create/delete waits for actual completion; failed/canceled tasks fail accurately; timeouts honor cancellation; externally missing keys/indexes reconcile correctly; import and no-op plans pass.
- Validation gate: deterministic HTTP/task tests plus existing live acceptance tests, including delayed deletion/replacement and failure cases.
- Blockers: none for the documented M-01 contract. Timezone-insensitive expiry import and automatic legacy-state conversion are not claimed.
- Parallel boundary: shared task/error contract must stabilize before async feature implementations.

### T-15 — Align repository CI and release configuration

Status: complete on 2026-09-18; approved repairs passed independent configuration verification.

- Goal/scope: pin CI Terraform 1.14.9/1.16.3 and lint 2.13.2; use the disposable acceptance harness; replace deprecated GoReleaser archive configuration.
- Constraints: preserve release credentials/signing and existing development volumes; no publishing, GitHub execution, or Compose data migration.
- Ownership/roles: executor_light owns test workflow and GoReleaser configuration; main owns integration/docs; tester verifies independently; explorer audits touched files.
- Dependencies: T-02/T-03 and the completed, authorized repository configuration inspection.
- Acceptance: configured lint and goreleaser check pass; CI uses the verified matrix/harness; old test service/bootstrap removed; credential references unchanged.
- Verification gates: workflow source validation, configured lint, release config check, shell syntax and diff checks. GitHub execution/signing/artifact builds remain release gates.
- Blockers: none for local configuration verification; GitHub execution and release signing remain separate gates.
- Parallel boundaries: exclusive ownership of .github/workflows/test.yml and .goreleaser.yml; no provider/dependency edits.

### T-16 — Adopt the confirmed provider ownership namespace

Status: complete on 2026-09-18; confirmed namespace applied, including CODEOWNERS, generated docs and migration guidance.

- Goal/scope: update active ownership links, provider address, examples and generated documentation; document state address migration.
- Constraints: no remote ownership transfer, publication, existing-state inspection or mutation.
- Ownership/roles: main owns address/reference edits and documentation; independent audit checks the final surface.
- Dependencies: confirmed namespace and T-02 documentation toolchain.
- Acceptance: active references use the confirmed namespace; generated docs and local example validation pass.
- Verification gates: generation, reference search, build/example validation and diff checks.
- Blockers: none for repository references; remote transfer/publication were not performed.
- Parallel boundaries: README.md, main.go, examples/provider/provider.tf, .github/CODEOWNERS and docs; separate from T-15 configuration files.

## M-02 — Core index settings can be managed declaratively

Status: proposed. Outcome: create an index, configure its search behavior, change settings without replacement, import settings, and reconcile remote drift.

### T-04 — Implement core settings resource and read access

- Goal/scope: `meilisearch_index_settings` and corresponding data source; searchable/displayed/filterable/sortable attributes, ranking rules, stop words, synonyms, and distinct attribute; existing-index validation and documented ownership.
- Constraints: one settings owner per index; faithful empty/null/value serialization; resource deletion resets owned fields only; index deletion remains separate; do not silently create an index.
- Ownership/roles: executor_light owns settings schema/conversion/lifecycle and examples; main agent integrates registration/docs; tester verifies.
- Dependencies: T-01, T-03.
- Acceptance: CRUD/import, managed-field reset/removal, out-of-band drift repair, preservation of unmanaged settings, and no-op second plan pass; index documents survive settings-resource deletion.
- Validation gate: payload tests for omission/null/empty values and live acceptance with documents and long-running settings tasks.
- Blockers: approval of ownership/import contract; SDK payload fidelity needs validation.
- Parallel boundary: settings files exclusive; may run alongside flags/webhooks after shared helpers stabilize.

## M-03 — Extended index settings have explicit coverage

Status: proposed. Outcome: configure advanced search behavior without perpetual diffs; published coverage distinguishes latest API fields from SDK coverage.

### T-05 — Add non-secret advanced settings

- Goal/scope: typo tolerance, pagination, faceting, dictionary/separator tokens, proximity precision, search cutoff, localization, prefix/facet search, advanced filter rules, and a decision for newer settings such as foreign keys.
- Constraints: typed schemas, order-aware collections, per-feature server gates; never route mixed filter rules through the SDK's string-only bulk field.
- Ownership/roles: executor_light owns extension and coverage ledger; tester verifies.
- Dependencies: T-04.
- Acceptance: supported fields round-trip; false, zero, empty, nested partial updates, and resets work; unsupported capabilities fail before mutation where determinable.
- Validation gate: HTTP serialization tests and acceptance on minimum/current supported servers; no-op plans after normalized responses.
- Blockers: API coverage differs from SDK; minimum release for each setting must be established.
- Parallel boundary: same settings schema is shared with T-06; coordinate rather than edit it simultaneously.

### T-06 — Assess and implement approved embedder/chat settings

- Goal/scope: versioned embedder/chat schemas; redact secrets; select sensitive versus optional write-only input design and explicit revision triggers; record deliberately deferred sources/fields.
- Constraints: do not claim remote secret drift detection; do not overwrite redacted secrets; write-only paths require Terraform 1.11+; configuration changes may cause costly reindexing.
- Ownership/roles: main agent approves secret contract; executor_light implements bounded extension; tester verifies synthetic secrets.
- Dependencies: T-05 and approved secret-input/version policy.
- Acceptance: approved configurations apply and reconcile non-secret drift; secret changes use an explicit update signal; secrets absent from artifacts on write-only paths; imports document unrecoverable values.
- Validation gate: synthetic plan/state inspection and live round-trip/failure tests; unsupported fields listed explicitly.
- Blockers: secret-read behavior and server/edition matrix unresolved.
- Parallel boundary: do not overlap T-05 schema edits; secret contract can inform T-08 independently.

## M-04 — Runtime instance flags are declarative

Status: proposed. Outcome: adopt one instance's experimental-feature settings and reconcile only the selected flags.

### T-07 — Manage runtime experimental features

- Goal/scope: singleton resource/data source for typed API-exposed flags, import/adoption, field ownership, explicit false values, and documented removal/destroy policy.
- Constraints: startup settings and Cloud provisioning are excluded by confirmed scope; no shared mutable SDK flag builder; no blanket reset of unowned flags.
- Ownership/roles: executor_light owns flags implementation; tester verifies; main agent approves singleton policy.
- Dependencies: T-01, T-03.
- Acceptance: adoption, flag update, drift repair, unmanaged preservation, import, no-op plan, and unsupported-flag diagnostics pass.
- Validation gate: request contracts and minimum/current server acceptance tests; test disappearance/stabilization of experimental flags.
- Blockers: flag/version matrix and ownership policy approval.
- Parallel boundary: independent feature files; provider registration remains coordinated.

Network topology and runtime log updates are deferred candidates, not implicit deliverables. Revisit topology only with an approved cluster ownership/recovery design; treat log updates as an action unless readable state is established.

## M-05 — Task webhooks can be managed safely

Status: proposed. Outcome: create, update, import, and remove an editable webhook without corrupting redacted authorization credentials.

### T-08 — Implement webhook lifecycle and read access

- Goal/scope: UUID resource/data source, URL/header management, editability, UUID import, typed missing-object handling, and exact header removal semantics.
- Constraints: never persist redacted responses as configured secrets; do not manage immutable startup-created hooks; optional write-only secret path requires 1.11+ and explicit update signal.
- Ownership/roles: executor_light owns webhook files/payload adapter and examples; main agent approves secret contract; tester verifies.
- Dependencies: T-01, T-03, approved redaction policy.
- Acceptance: CRUD/import, URL drift, header addition/change/removal, auth-header preservation, missing deletion, immutable-hook diagnostics, and no-op follow-up plans pass.
- Validation gate: synthetic secret/artifact checks and live callback tests; verify server minimum/edition support, not SDK version alone.
- Blockers: null-removal cannot be assumed expressible through current SDK map types; docs contain inconsistent examples; server matrix unresolved.
- Parallel boundary: independent of settings/flags; bounded HTTP fallback and provider registration coordinated.

## M-06 — Temporary credentials avoid persisted key values

Status: conditional; requires lifecycle approval. Outcome: acquire a temporary scoped key for a Terraform run; normal closure revokes it and expiry limits crash exposure.

### T-09 — Prototype and validate temporary-key lifecycle

- Goal/scope: disposable-server prototype for Open/Close and private UID data, bounded expiry, fresh identity per open, permitted consumption contexts, and long-run validity limits.
- Constraints: Terraform 1.10+; planning can create keys; existing application keys must never be revoked; no unsupported in-place expiry renewal promise.
- Ownership/roles: executor_light owns bounded prototype; tester verifies; main agent decides whether to proceed.
- Dependencies: T-01, T-03 and explicit approval of planning-time key creation.
- Acceptance: secrets excluded from saved artifacts; normal Close revokes the correct UID; interrupted run loses validity by expiry; repeated opens do not collide; insufficient lifetime fails clearly.
- Validation gate: plan/apply/close tests and expiry/crash simulation using disposable synthetic credentials.
- Blockers: permitted composition with intended consumers and long-run policy not yet proven.
- Parallel boundary: credential feature files independent; provider ephemeral registration/data plumbing is coordinated.

### T-10 — Release approved ephemeral key support

- Goal/scope: production ephemeral `meilisearch_key`, validation, documented scopes/expiry/cleanup limits, examples, and version requirements.
- Constraints: durable keys retain managed-resource semantics, with modernized schema permitted; ephemeral existing-key lookup is a separate optional design and must not inherit creation cleanup.
- Ownership/roles: executor_light implements; tester verifies independently; main agent updates verified docs.
- Dependencies: successful T-09 and user go/no-go decision.
- Acceptance: all approved lifecycle scenarios pass; managed key regression tests pass; invalid consumption contexts explained.
- Validation gate: CLI acceptance matrix, synthetic secret artifact verification, and full key suite.
- Blockers: T-09 results. Existing-key lookup is separately deferred; M-01 verified credential retrieval on 1.53.2, but an ephemeral lookup contract and broader server support require their own validation if requested.
- Parallel boundary: independent from operations after configuration interfaces settle.

## M-07 — Bounded operational workflows use actions

Status: conditional; requires Terraform action/version approval. Outcome: explicitly ingest seed data or create a backup, with completion/failure visible to the operator.

### T-11 — Add bounded document-ingestion action

- Goal/scope: action for explicit replace/partial-update ingestion into an existing index; stable IDs; bounded JSON/NDJSON input, payload batching, all-task completion, and actionable partial-failure reports.
- Constraints: Terraform 1.14+; never ingest during plan; preserve documents not submitted; no synchronization pipeline or document deletion-on-destroy; explicitly define replay and uncertain-submit behavior.
- Ownership/roles: executor_light owns action/input/batching files; tester verifies; main agent integrates actions registration and configuration.
- Dependencies: T-01, T-03; action-input persistence and permitted ephemeral credential contexts verified before promising secret-safe composition.
- Acceptance: direct invocation and approved triggers work; repeat ingestion with stable IDs preserves intended data; partial failures and cancellation surface accurately; no plan-time document writes.
- Validation gate: batching/request tests, document-content acceptance checks, failure injection, and trigger-order tests after settings application.
- Blockers: concrete provider-wide CLI baseline and action lifecycle approval; payload format/size and input-secret contract need a bounded specification.
- Parallel boundary: independent action files; input/API helpers only shared with proven reuse.

### T-12 — Add snapshot and dump actions

- Goal/scope: separate actions submitting snapshot/dump tasks, configurable deadlines, success/failure reporting, and available task/dump metadata in progress diagnostics.
- Constraints: Terraform 1.14+; server-side artifacts only; no fabricated downloadable/local path, durable computed state output, retention, scheduler, or restore support.
- Ownership/roles: executor_light owns backup actions/examples; tester verifies in disposable server filesystem; main agent owns docs/integration.
- Dependencies: T-01, T-03; approved edition/server matrix.
- Acceptance: direct invocation completes; failed tasks/timeouts diagnose accurately; disposable server artifact exists after success; no backup creation during plan.
- Validation gate: task contract tests and live snapshot/dump creation; verify metadata format and endpoint availability on approved targets.
- Blockers: Community/Enterprise/Cloud availability not validated; filesystem validation requires authorized disposable setup.
- Parallel boundary: backup files independent of ingestion; shared registration coordinated.

## M-08 — Publish a verified compatibility contract

Status: proposed. Outcome: release-ready provider with accurate examples, version gates, migration guidance, and verified feature coverage.

### T-13 — Verify integrated features and documented migration

- Goal/scope: provider-wide minimum/current CLI-server matrix; modernized state/import and documented legacy migration or re-import; settings-before-ingestion ordering; independent flag/webhook ownership; optional ephemeral and action suites. Review semantic RFC3339 expiry comparison before claiming timezone-insensitive imports; M-01 documents UTC-spelling alignment.
- Constraints: test only approved optional features; production credentials/data excluded; no claimed OpenTofu support without its explicit test matrix.
- Ownership/roles: tester owns independent verification; main agent integrates fixes and records results.
- Dependencies: all selected feature tasks; deferred optional features explicitly excluded from release claims.
- Acceptance: meaningful acceptance suites pass; no-op plans and import round trips pass; documented migration/re-import works without unintended remote deletion; failure/timeout/security contracts verified. Unchanged legacy addresses/schemas are not an acceptance requirement.
- Validation gate: pinned disposable environments and reproducible evidence for all compatibility claims.
- Blockers: selected release scope and GitHub/signing/artifact verification. Repository automation inspection permission is granted and its audit complete.
- Parallel boundary: freeze shared schemas during integrated verification except coordinated fixes.

### T-14 — Finalize documentation and release preparation

- Goal/scope: generated Registry docs, approved examples, README name correction, changelog, coverage/version ledger, migration notes, and verified handoff.
- Constraints: distinguish supported server APIs from deployment controls; no unsupported secret-drift/backup-output/renewal claims; publication is a separately authorized action.
- Ownership/roles: main agent owns durable docs/release review; bounded generated/example edits may be delegated; tester checks published examples.
- Dependencies: T-13.
- Acceptance: documentation generation succeeds without overwriting design docs; every approved feature has examples and requirements; deferred fields/features and operational limits are explicit.
- Validation gate: examples validated on their documented minimums; release artifact/manifest checks and final documentation diff review.
- Blockers: no release work is authorized yet.
- Parallel boundary: main agent serializes durable decision/handoff updates; generated docs can be produced after schema freeze.

## Excluded or deferred approaches

- Ephemeral ingestion, snapshots, and dumps: rejected as unsuitable lifecycle designs; they can leave lasting writes during planning.
- Bulk document synchronization and backup scheduling/retention: external pipeline/operations concerns.
- Startup instance configuration and Cloud provisioning: excluded by confirmed server-API-only scope.
- Network topology: separate high-complexity control plane; needs a new approved slice.
- Tenant-token generation and ephemeral lookup of existing keys: possible future work, not required by the current feature request.
- Legacy schema compatibility shims: not required. Meaningful IDs and current primary-key behavior are part of T-03, with breaking changes documented.
