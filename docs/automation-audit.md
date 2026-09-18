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
