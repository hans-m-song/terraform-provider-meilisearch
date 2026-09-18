# Terraform Provider for Meilisearch.

[![Release](https://img.shields.io/github/v/release/hans-m-song/terraform-provider-meilisearch)](https://github.com/hans-m-song/terraform-provider-meilisearch/releases)
[![Registry](https://img.shields.io/badge/registry-doc%40latest-lightgrey?logo=terraform)](https://registry.terraform.io/providers/hans-m-song/meilisearch/latest/docs)
[![License](https://img.shields.io/badge/license-Mozilla-blue.svg)](https://github.com/hans-m-song/terraform-provider-meilisearch/blob/main/LICENSE)

This Terraform provider implements resource management for Meilisearch.

The modernization changes in this workspace are unreleased. See [the roadmap](docs/roadmap.md) for delivery status and [migration notes](docs/migration.md) for behavior changes. The supported baseline is Terraform 1.14+; newer server API coverage takes priority over legacy resource compatibility.

## Overview

### Using the provider

To use this provider, you must install it and provide authentication credentials:

```hcl
terraform {
  required_version = ">= 1.14.0"

  required_providers {
    meilisearch = {
      source = "hans-m-song/meilisearch"
    }
  }
}

provider "meilisearch" {
  host = "http://localhost:7700"
  api_key = "T35T-M45T3R-K3Y"
}
```

Alternatively, you may use environment variables `MEILISEARCH_API_KEY` and / or `MEILISEARCH_HOST` for authentication.
Use an API key with permissions for the operations you manage. Credentials supplied in Terraform configuration can be marked sensitive and still be stored in state. `operation_timeout` accepts a positive Go duration and defaults to `"5m"`; it covers API requests and asynchronous task completion.

### Resources

- `meilisearch_key`: create and manage API keys for Meilisearch.
- `meilisearch_index`: create and manage an index in Meilisearch.

### Data sources

- `meilisearch_key`: read API keys for Meilisearch.
- `meilisearch_index`: read a Meilisearch index.
- `meilisearch_version`: read the server version.

## Development

_This template repository is built on the [Terraform Plugin Framework](https://github.com/hashicorp/terraform-plugin-framework)._

### Requirements

- [Terraform](https://www.terraform.io/downloads.html) >= 1.14
- [Go](https://golang.org/doc/install) >= 1.25.8 (preferred toolchain: 1.27.1)
- [Docker](https://docs.docker.com/engine/install/) for disposable acceptance servers
- `curl` and `jq` for synthetic acceptance-fixture setup
- [golangci-lint](https://golangci-lint.run/usage/install/) for development

### Building The Provider

1. Clone the repository
1. Enter the repository directory
1. Build the provider using the Go `install` command:

```shell
go install
```

### Adding Dependencies

This provider uses [Go modules](https://github.com/golang/go/wiki/Modules).
Please see the Go documentation for the most up to date information about using Go modules.

To add a new dependency `github.com/author/dependency` to your Terraform provider:

```shell
go get github.com/author/dependency
go mod tidy
```

Then commit the changes to `go.mod` and `go.sum`.

### Developing the Provider

If you wish to work on the provider, you'll first need [Go](http://www.golang.org) installed on your machine (see [Requirements](#requirements) above).

To compile the provider, run `go install`. This will build the provider and put the provider binary in the `$GOPATH/bin` directory.

Don't forget that if you want to use a local binary for a Terraform provider, you need to add the following block to your `~/.terraformrc` file:
```
provider_installation {
  dev_overrides {
      "registry.terraform.io/hans-m-song/meilisearch" = "<replace/with/your/gopath/bin>"
  }

  direct {}
}
```

before using it in a `.tf` file such as:

```
terraform {
  required_providers {
    meilisearch = {
      source = "hans-m-song/meilisearch"
    }
  }
}
```

To generate or update documentation, run `go generate`.

In order to run the full suite of Acceptance tests, run `make testacc`.

```shell
make testacc
```

Acceptance tests use `scripts/test-acceptance.sh` to start a disposable Meilisearch v1.53.2 server, wait for readiness, seed synthetic fixtures, run Terraform tests, and clean up that server. The harness does not reuse existing Meilisearch data or delete workspace Terraform state. See the script's usage for the exact endpoint and Terraform binary selection.

For unit tests and build checks:

```shell
go test ./...
go vet ./...
go build ./...
```

### Run linter

Install [golangci-lint](https://golangci-lint.run/usage/install/) and run the linter:

```shell
golangci-lint run
```

Or using Docker:

```shell
docker run --rm --volume "$(pwd):/app" --workdir /app golangci/golangci-lint:v2.13.2 golangci-lint run
```
