terraform {
  required_version = ">= 1.14.0"

  required_providers {
    meilisearch = {
      source = "hans-m-song/meilisearch"
    }
  }
}

provider "meilisearch" {
  host              = "http://localhost:7700"
  api_key           = "T35T-M45T3R-K3Y"
  operation_timeout = "5m"
}
