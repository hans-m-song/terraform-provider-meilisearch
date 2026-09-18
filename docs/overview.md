# Provider modernization

Research date: 2026-09-18. Status: M-01 implemented and verified; M-02 core index settings implemented and verified; remaining milestones proposed.

## Goal

Modernize the existing Go Terraform Plugin Framework provider and deliver declarative index settings, runtime instance controls, and task webhooks. Assess temporary credentials and operational workflows separately so their Terraform lifecycle matches their remote effects.

## Confirmed scope and modernization policy

- Confirmed by the user: manage the Meilisearch server API; exclude Cloud provisioning, server deployment, and startup configuration.
- Confirmed by the user: raise the Terraform minimum for the whole provider and prioritize current upstream behavior over legacy resource compatibility. Breaking schema/state changes are permissible; document their effects and upgrade paths.
- M-01 provider-wide supported minimum: Terraform 1.14+, covering the planned action, ephemeral, and write-only feature families. A higher minimum remains permissible if required by a later approved slice.
- Primary integration targets verified on 2026-09-18: Terraform v1.16.3 and Meilisearch v1.53.2; refresh pins before implementation. Older-server compatibility is optional and must not delay current functionality. Sources are recorded in feasibility.md.
- M-01 is validated against the official Meilisearch v1.53.2 Docker image. Older-server, edition/Cloud, and OpenTofu coverage are unclaimed; later features must establish their own capability gates.

M-01 modernization is complete. M-02 core settings ownership/import/reset policy is implemented and verified. Other later-feature contracts remain proposals.

## Existing architecture

`internal/provider/provider.go` constructs a shared Meilisearch SDK client and registers index/key resources and index/key/version data sources. Resources implement Framework lifecycle interfaces directly. Tests use protocol 6 provider factories and local-server acceptance tests. See [structure](structure.md) and [investigation](feasibility.md).

## Proposed architecture

```text
Terraform configuration
  |
  +-- resources: indexes, durable keys, settings, webhooks
  +-- ephemeral: temporary keys
  +-- actions: ingest documents, create snapshot, create dump
          |
     Framework adapters
          |
     narrow API operations + cancellable task waiter
          |
     Meilisearch Go SDK / bounded HTTP fallback
          |
     Meilisearch server
```

Use typed Terraform schemas. Prefer SDK operations; isolate a `net/http` fallback only where verified SDK behavior cannot express the API contract. Avoid a generic REST resource or broad abstraction layer. Share task completion, error classification, and secret-safe diagnostics where there is demonstrated reuse.

## Proposed decisions

1. Manage index settings in one separate `meilisearch_index_settings` resource per index. Never allow both inline settings and a separate resource to own the same fields.
2. Manage runtime experimental flags through an explicit singleton `meilisearch_experimental_features` resource. Keep network topology separate and defer it until its ownership model is validated.
3. Expose API-created webhooks as managed resources, imported by UUID. Represent startup-created immutable hooks through read-only access where supported.
4. Model durable API keys as managed resources, modernizing their schemas where appropriate. A separate ephemeral key may create a bounded-lifetime credential and revoke it when Terraform closes it.
5. Use Terraform actions for ingestion, snapshots, and dumps. They leave durable effects and do not have a normal read/update/delete lifecycle.
6. Do not promise backup download, off-site storage, scheduling, or restoration through the backup creation API.

Decision 1 is approved for M-02's eight core fields: initially omitted/null fields are unmanaged; removal or resource deletion resets previously owned fields through JSON null; UID import adopts supported core fields only. The durable-key portion of decision 4 was implemented in M-01. Other feature decisions require approval. Their reasoning and uncertainties are in [feasibility](feasibility.md).

## Workflow and gates

Implement the vertical slices in [roadmap](roadmap.md). For each slice: approve its contract, update task status, implement within ownership boundaries, verify failure cases and acceptance behavior, generate user documentation, and record verified results. Keep generated Registry documentation distinct from these design documents. No production API operations are part of this investigation.
