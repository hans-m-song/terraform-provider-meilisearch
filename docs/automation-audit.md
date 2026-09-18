# Automation audit

Inspected on 2026-09-18 after explicit permission for repository Compose, initialization, lint/release configuration, and GitHub workflows. Credentials and personal information are omitted. The findings below describe the configuration before the subsequently approved T-15 repair. No production operations were performed.

## Verified findings

| Surface | Finding | Consequence |
| --- | --- | --- |
| `.github/workflows/test.yml` | Terraform matrix is 1.0–1.4; service image is Meilisearch 1.0 on port 7700 | CI does not exercise the supported baseline/current matrix or current port-17700 fixtures |
| `docker_compose/docker-compose.yml` | Meilisearch 1.7 with a persistent named volume | Development setup differs from the tested 1.53.2 server; changing its image requires a deliberate data-handling decision |
| `docker_compose/test_init.sh` | Legacy port-7700 initialization without readiness/task waits or reliable HTTP failure checks | Initialization is not equivalent to the disposable acceptance harness |
| `.golangci.yml` | Version-2 configuration | Configured golangci-lint passed with zero issues |
| `.goreleaser.yml` | Uses `archives.format` | GoReleaser 2.18.1 check exits 2 because the property is deprecated |
| `.github/workflows/release.yml` | Uses go.mod for Go setup and invokes release on version tags | No release was run; signing/upload behavior remains untested |

GoReleaser's replacement is `formats: ["zip"]`. [Official deprecation notice](https://goreleaser.com/resources/deprecations/#archivesformat).

## Approved repair slice

1. Replace the obsolete CI matrix with pinned Terraform 1.14.9/1.16.3 and run `scripts/test-acceptance.sh`; remove the duplicate legacy service/initialization path.
2. Pin the lint binary to the verified 2.13.2 target and retain the existing compatible lint configuration.
3. Update the archive property and require `goreleaser check` to pass without publishing or accessing signing credentials.
4. Decide whether to retire legacy initialization or retain it as documented development-only tooling. Modernize Compose only with a fresh-volume or explicitly approved migration policy; never reuse old data blindly or remove its volume.

Acceptance: configured lint and release validation pass; CI uses the same reproducible CLI/server matrix and isolated harness as local verification; existing development data is preserved. Workflow execution on GitHub, release signing, and the complete cross-platform artifact matrix remain separate verification gates.

T-15 was approved and completed on 2026-09-18. Independent YAML parsing, configured lint (zero issues), GoReleaser validation, harness syntax/help and diff checks passed. CI now uses the pinned CLI matrix and disposable harness; action SHAs and signing references are retained. GitHub execution, signing and artifact builds were not performed. Development Compose/data migration remains separate.

## Follow-up after first tagged run

On 2026-09-18, [Tests run 35330492474](https://github.com/hans-m-song/terraform-provider-meilisearch/actions/runs/35330492474) passed build/lint/generation and both acceptance suites, then failed harness cleanup on read-only Go module-cache directories. The previous local runs used external writable cache roots and did not exercise this boundary. [Release run 35330492505](https://github.com/hans-m-song/terraform-provider-meilisearch/actions/runs/35330492505) completed successfully; Registry publication remains unverified.

Approved T-17 updates action pins from HashiCorp's current [test](https://github.com/hashicorp/terraform-provider-scaffolding-framework/blob/main/.github/workflows/test.yml) and [release](https://github.com/hashicorp/terraform-provider-scaffolding-framework/blob/main/.github/workflows/release.yml) templates and adds explicit Terraform setup for generation. It retains this provider's verified CLI matrix and lint binary pin. Harness cleanup repair is separate: upstream does not use this disposable-server harness. Go's module cache is read-only by default. [Go module-cache reference](https://go.dev/ref/mod#module-cache).

T-17 local verification passed: independent workflow checks and mocked harness success/failure/cache-preservation cases; real full acceptance on Terraform 1.15.8/Meilisearch 1.53.2 with fresh owned caches; no leftover container or temporary acceptance directory. The tag remains unchanged. Remote execution of updated actions is checked after the authorized follow-up push.

The pushed source commit a488c6c passed [GitHub Tests run 35332506179](https://github.com/hans-m-song/terraform-provider-meilisearch/actions/runs/35332506179), including build/lint/generation and both acceptance/cleanup jobs. This closes the observed CI cleanup failure and updated test-action execution gates. The updated release-action pins have not yet been exercised by a new release.
