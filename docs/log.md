# Investigation log

## 2026-09-18 — Modernization feasibility

- Inspected go.mod, README, provider registration/configuration, existing index/key lifecycle implementations, resource acceptance tests, and GNUmakefile.
- No local AGENTS.md/CLAUDE.md was found in the working directory or three ancestors checked. Followed supplied session instructions.
- Verified current module targets via the public Go proxy; Framework v1.19.0, SDK v0.36.3, and current testing/docs require toolchain modernization (full set: Go at least 1.25.8).
- Documented asynchronous deletion/completion gaps and fragile missing-object handling from source inspection; no live failure reproduction occurred.
- Confirmed settings request fidelity and webhook redaction constraints in upstream API/SDK sources.
- Proposed managed settings/flags/webhooks, temporary ephemeral keys, and ingestion/backup actions. These are recommendations awaiting approval, not accepted architecture decisions.
- Identified inconsistency between the Meilisearch documentation index's key-value retention description and the GET key reference. Deferred any behavior claim until versioned server validation.
- Asked for server-API versus deployment/Cloud scope and compatibility preferences. No response had arrived when draft planning documents were created.
- Subsequently confirmed by the user: server API only; raise the Terraform minimum for the whole provider; prioritize current upstream functionality over compatibility with old resources. Revised the draft to a proposed provider-wide Terraform 1.14+ baseline and included deliberate legacy schema modernization with migration/re-import guidance in M-01.
- Verified stable primary integration targets: Terraform v1.16.3 and Meilisearch v1.53.2; older-server support is optional in the revised proposal.
- No source, dependencies, generated documentation, or running systems were changed. No tests were executed because implementation did not occur. No notification was sent because no implementation milestone completed.

Rejected in this proposed design: ephemeral ingestion and backup creation because persistent remote effects do not fit Terraform's planning-time temporary-resource lifecycle. See [evidence](feasibility.md) and [roadmap](roadmap.md).

## 2026-09-18 — M-01 authorized

- User confirmed beginning M-01 after review of the roadmap. Updated selected task statuses to in progress.
- Host tools reported Go 1.27.1 and Terraform 1.15.8. No environment values were inspected.
- Delegated dependency upgrades and bounded upstream contract research. Potentially secret-bearing repository configuration inspection remains pending explicit permission; isolated synthetic testing can proceed independently.
- Dependency worker upgraded six direct modules and tidied indirect dependencies; go build ./... passed with Go 1.27.1.
- Main added operation_timeout validation/configuration and providerData plumbing, context-aware data-source reads, meaningful IDs, RFC3339/null timestamps, and sensitivity for key data-source credentials. Provider unit checks passed.
- go generate ./... passed; verified tfplugindocs v0.25.0 removes only managed Registry paths and preserved planning documents.
- Removed destructive GNUmakefile acceptance cleanup and wired a forthcoming isolated harness. Independent tester owns its implementation and live validation.
- Initial lint attempt used a protected default cache and reported expected mid-edit unused helpers; redirected subsequent lint cache to the writable task cache. No lint success claimed yet.
- Explorer verified primary-key updates are asynchronous and restricted to empty indexes; optional/computed primary_key retains inferred/current remote values when omitted. Explicit null reset is not expressible by the SDK update struct.
- Explorer verified nullable key metadata removal is not faithfully expressible by SDK KeyUpdate omitempty fields. Added host/API-key/shared-http-client fields to private providerData for a bounded, credential-safe PATCH adapter; resource worker owns adapter and validation.
- Additional synthetic HTTP data-source tests passed for meaningful IDs, RFC3339/null responses, Configure mismatches, and deadline cancellation. Resource integration remains in progress; no live acceptance success claimed yet.
- Changed key data-source actions/indexes to unordered sets to match the modernized key-resource scope model; reran the provider/data-source unit subset successfully after this change. Live resource verification remains pending.
- Main checks passed on the current integration snapshot: go test ./... -count=1 without acceptance enabled, go vet ./..., go build ./..., disposable-harness shell syntax and help, and another documentation generation. Resource workers remain active; these checks do not establish final live acceptance or a completed milestone.
- Default-configuration golangci-lint 2.13.2 passed with zero issues after fixing response-body cleanup and a test conversion. Existing repository lint configuration remains unread; this is not a configured-CI lint claim.
- Resource worker completed its initial iteration. Main reviewed optional-only metadata ownership, regenerated documentation, and passed unit/vet/build checks after production freeze. Four unchecked assertions in main-owned tests were removed following worker feedback.
- Coverage review found missing resource-level failure gates despite implemented handling. Delegated deterministic synthetic HTTP tests for partial creation, populated-index rejection, missing-index reads, and task-level not-found deletion. Live acceptance and nullable expiry-removal semantics remain unresolved.
- Independent tester verified index/key acceptance, import, metadata removal, and no-op plans against Meilisearch 1.53.2 on Terraform 1.14.9, 1.15.8, and 1.16.3. Full acceptance remained partial because the server reports commit_date as "unknown"; main corrected the test to accept unavailable build metadata and clarified the schema description.
- Resource worker's deterministic fault tests passed for timeout identity retention, failed/canceled creation state removal, missing-index reads, populated-index rejection, and task-level missing-index deletion. Review requested broader mutation counting for the populated-index test.
- Selected optional-only expiry ownership to make omission/null faithful to the API's never-expiring creation contract. Removing a finite expiry must replace the immutable key; schema validation and a live replacement/no-op test are in progress. Production code and the full minimum/current acceptance matrix will be verified again after this change.

## 2026-09-18 — Final M-01 verification

- Full acceptance passed on Terraform 1.14.9 and 1.16.3 against Meilisearch 1.53.2 after expiry and version-date fixes. Independent tester also passed unit tests, vet, build, no-config lint, and diff checks; official Terraform SHA256 sums were checked before execution. Disposable state, CLIs, and containers were removed.
- Main validated examples/provider/provider.tf against a freshly built local provider on Terraform 1.15.8 using temporary development overrides. Validation passed with the expected development-override warning; no apply or server mutation occurred.
- Final explorer source audit confirmed generated docs match schemas, context/task/error handling is integrated, and no gratuitous feature scope or destructive cleanup was added. It identified a coverage boundary: null-primary-key data-source tests do not directly test omitted-primary-key resource creation. Delegated one targeted live create/no-op/import gate on baseline/current CLIs; milestone notification remains pending that result.
- Replaced stale handoff checkpoints with the current verified state. Documented UTC expiry import spelling and untested automatic legacy-state conversion. Existing configuration/automation inspection is deferred to M-08 pending permission; no release or publication is authorized.
- The targeted omitted-primary-key resource create/no-op/import test passed on Terraform 1.14.9 and 1.16.3, preserving null state and actual UID identity. Main reviewed the added test and final changed-file inventory; diff checks passed. This closes the final runtime coverage gate without production changes.
- Explorer's bounded audit closure confirmed the new omission test and final handoff/log resolve prior findings. The authorized M-01 notify-discord command completed successfully (exit 0), reporting T-01/T-02/T-03 and the documented limitations. No provider release or commit was performed.

## 2026-09-18 — Repository configuration inspection authorized

- User explicitly allowed inspection of Compose, test initialization, lint/release configuration, and GitHub workflows. Inspected with credential/personal-content omission; prohibited paths and environment values were not inspected.
- Configured lint passed with zero issues; initialization shell syntax passed. GoReleaser 2.18.1 configuration check failed solely on deprecated archives.format.
- Recorded obsolete CI CLI/server versions and port mismatch, persistent development-volume considerations, and a concrete repair proposal in automation-audit.md. No workflow/configuration changes or release operations were performed.

## 2026-09-18 — T-15 repair authorized

- User approved the proposed CI/release repairs. Delegated only test workflow and GoReleaser configuration changes; release signing and development Compose/data remain protected.
- Added stable T-15 to the roadmap and marked it in progress. Independent configured-lint/release checks and a touched-file audit are required before completion.
## 2026-09-18 — Ownership namespace confirmed

The user confirmed `hans-m-song` for both GitHub references and the Terraform Registry provider namespace. T-16 updates active source references and generated documentation, with provider-address migration guidance. Remote transfer, publication and existing user state are outside this change.

## 2026-09-18 — T-15/T-16 complete

- Independent T-15 checks passed YAML parsing, configured lint (zero issues), GoReleaser validation, harness syntax/help and diff checks. GitHub execution/signing/artifacts remain untested.
- T-16 docs generation and freshly built local-provider example validation passed on Terraform 1.15.8. Independent audit identified and then verified the additional CODEOWNERS update; only the migration command retains the historical namespace.
- The new acceptance harness remains untracked and must be included in the eventual coordinated commit. No commit, remote transfer or publication was performed.
- The authorized M-01 follow-up Discord notification completed successfully.

## 2026-09-18 — Commit and push authorized

Commit 1986202 was pushed to main and the approved v0.3.0 tag was pushed. The tag's Tests run passed both acceptance suites but failed cleanup of read-only module-cache directories. On 2026-09-18 the user approved T-17: upstream action updates, explicit Terraform setup for generation, cleanup repair, verification and commit/push. The release tag remains unchanged.

The original Release run 35330492505 subsequently reported success. Registry publication was not checked. Verified upstream action manifests use Node 24 and retain the needed configuration inputs; both workflows are being updated to the scaffolding repository's current SHA pins.

T-17's full local acceptance suite passed on installed Terraform 1.15.8 with Meilisearch 1.53.2 and GOCACHE/GOMODCACHE explicitly unset. The harness downloaded into its own cache, exited zero after cleanup, and left no acceptance temporary directory or container. The v0.3.0 commit target remains 1986202. Independent failure/shared-cache verification is pending.

Independent T-17 verification subsequently passed workflow YAML/pins/signing references, full mocked harness success and failure (exit 23 retained), shared-cache preservation, read-only cleanup (0/7 retained) and injected cleanup failure diagnostics. Main additionally verified cleanup failure maps success to exit 1 while retaining exit 7. The independent final scope audit found no defect. Verification artifacts are isolated under /Volumes/Data/tmp/t17-workflow-verification. The approved follow-up is ready for commit/push; remote CI outcome is checked afterward.

Source commit a488c6c was pushed to main. [GitHub Tests run 35332506179](https://github.com/hans-m-song/terraform-provider-meilisearch/actions/runs/35332506179) completed successfully, including both Terraform matrix acceptance jobs and cleanup. No release tag was moved or added. Registry publication remains unverified.

The authorized T-17 completion notification command succeeded. Final documentation records the verified remote result in a separate follow-up checkpoint.

## 2026-09-18 — M-02/T-04 approved

- Pinned Charabia source resolved normalization uncertainty: its non-lossy string pipeline also strips non-whitespace controls and applies Persian mappings globally. Root added a shared comparison-key helper used by validator, equality and spelling retention; focused transformation tests pass. Executor adds actual Terraform Unicode/no-op regression before final matrix freeze. Source citations and the superseded NFKD-only boundary are recorded in feasibility.md.

- Independent pre-freeze unit/vet/build checks passed, and official Terraform 1.14.9/1.16.3 archives were SHA256-verified. Configured lint found 17 production issues; main repaired checked type conversion, error casing and an always-nil error result. The next lint run reported only an unused WIP test helper. Acceptance remains pending test freeze. Main review requested acceptance setup inside the Framework PreCheck gate, remote reset/unmanaged-field checks and a real import-state assertion.

- Main integrated the worker draft, split schema/validators and value conversion from lifecycle, and made GET response decoding reject missing supported fields and invalid/null string-array elements. Focused settings-contract tests passed for strict decoding, exact null/empty payloads, accepted malformed/oversized responses, diagnostic sanitization and NFKD collision validation. Full lifecycle/acceptance verification remains pending.

- Main compiled the current draft and validated combined resource/data-source examples with Terraform 1.15.8 and a temporary local-provider override. Validation and formatting checks passed; this does not establish final acceptance behavior. Temporary binary/configuration/data directories were removed, and no apply occurred.
- Documentation generation passed for the current draft, adding resource/data-source pages and importing the new examples. Regeneration is required if schema descriptions change during review; completion and acceptance remain pending.

The user approved the core index-settings resource/data-source scope and field ownership/import policy. Initial omission/null is unmanaged; previously managed removal and resource deletion reset only owned fields through JSON null. UID import adopts eight core fields; advanced settings remain unmanaged. The provider will preflight existing indexes, await tasks and preserve documents. Implementation and verification are in progress; no feature completion claimed.

Main integrated constructor registrations and extended shared diagnostics through a private code/status error interface used by bounded adapters. Focused operations module tests passed, including wrapped adapter classification and suppression of raw error details. Settings constructor implementations and full package verification remain worker/tester gates.

Independent synthetic API probes on Meilisearch 1.53.2 confirmed that null resets match a fresh index's defaults. Stop words preserve case but GET returns canonically decomposed Unicode; synonyms preserve exact spellings, list order and duplicates; the tested ranking alias remains unchanged. Displayed/searchable lists deduplicate names while retaining first occurrence, and a wildcard collapses mixed lists to ["*"]. The worker is addressing canonical Unicode equality and redundant-list validation. All probe containers were removed; no existing server/data was used.

Source review clarified compatibility decomposition (NFKD), confirmed by a further synthetic probe (ligature, circled digit and compatibility letter normalize to ff/1/H). Ranking `attribute` is a distinct supported literal, not an alias; no conversion occurred. A settings-only key excluding indexes.create produced an asynchronous index_not_found failure for a missing index and left it absent. Document scoped-key mitigation and the separate GET/PATCH race limitation. Probe credentials/data existed only in disposable servers; no credential output was emitted.

The user reported completing signing-secret setup and authorized committing and pushing the verified workspace changes. Secret values were not inspected. Release tagging and provider publication were not requested. The pre-commit worktree inventory contains the intended source, test, documentation and automation files; no exported private-key file appears in that inventory.

- Final M-02 source/test snapshot is frozen. Documentation regeneration and Terraform 1.15.8 validation of the combined core-settings resource/data-source examples passed with a freshly built local provider and disposable development overrides. Independent unit/build/vet/lint and the full Terraform 1.14.9/1.16.3 acceptance matrix remain the completion gate.

- Independent final unit, vet, build and configured lint gates passed (t04-final3 logs). Live acceptance has not started: another container named t04-index-settings occupies harness port 17700. It has not been stopped or reused pending provenance. Audit also identified missing explicit owned drift READ/repair coverage; the test worker is adding a bounded regression before final matrix.

- Final explorer audit found that Update/Delete could reset an owned unsupported advanced filterable rule before post-PATCH decoding diagnosed it. Root added a pre-mutation settings read whenever PATCH includes filterableAttributes; Create/Update/Delete now reject an advanced remote value before mutation. This guard is best-effort across separate GET/PATCH requests. Worker is adding zero-mutation/ownership-retention regressions and explicit live/synthetic drift repair. Independent matrix will use an isolated copied harness on port 17702 to leave the unconfirmed 17700 container untouched.

- Drift/advanced guard regressions completed: deterministic READ/repair and live PreConfig mutation preserve unowned display settings; Create/Update/Delete reject advanced filterable mutations with zero PATCH and prior state ownership retained. Worker focused/full provider unit, configured lint (0 issues), and diff checks passed. Final generation and combined-example validation passed again after the guard fix. Snapshot is frozen for independent matrix on isolated port 17702.

- Explorer closed the final bounded source audit: no remaining concrete blocker in settings source, tests, generated docs and examples. The prior advanced-filterable reset finding is guarded at all three mutation sites, with zero-PATCH/retained-ownership regressions. Separate GET/PATCH requests retain the documented concurrent mutation limitation. Live normalization wording remains pending until the final matrix result.

- First isolated final acceptance run (Terraform 1.14.9) passed settings data-source/import and existing index/key/version tests, but lifecycle failed in deferred cleanup: its unconditional expected out-of-band displayed value was not present. Production cause is not established; deferred CheckDestroy can mask an earlier panic. Worker is changing that fixture expectation only after successful drift mutation; focused live diagnosis precedes the next matrix.

- Focused live lifecycle rerun exposed the original nil-pointer panic in a remote stop-word test helper: it captured the nil SDK client while the TestCase was constructed, before PreCheck assigned the client. This is a test fixture defect; root authorized non-network client initialization before helper construction while retaining all server setup inside gated PreCheck. No production change is indicated by this failure.

- M-02/T-04 completed on 2026-09-18. Independent unit (TF_ACC empty), vet, build and configured lint passed (t04-final4 logs); worker unit/lint checks also passed after the final nil-client fixture correction. Full final acceptance passed all 11 tests with exit 0 on Terraform 1.14.9 and 1.16.3 against Meilisearch 1.53.2 (t04-final5 acceptance logs). Only test-host/harness port differed in the isolated source copy; production remained identical. Disposable source, provider binary, CLI staging, server and generated state were removed; the pre-existing port 17700 container was untouched. Final source audit, documentation generation and Terraform 1.15.8 combined-example validation passed. Core reset/import/drift/unmanaged/document-preservation and Unicode no-op gates are closed. Changes remain uncommitted; no new push/tag/release was performed.

- Authorized M-02 completion notification sent through notify-discord; command exited 0.

- User authorized committing and pushing the verified M-02 changes on 2026-09-18; no new release tag requested.

- User authorized publication on 2026-09-18. Preparing v0.4.0 for the additive core index-settings feature and previously unreleased automation repairs. GitHub Tests run 35340100066 passed for b364d06. Release workflow and Registry ingestion must be verified after tag publication.
