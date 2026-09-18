package provider

import (
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

const (
	testHost   = "http://localhost:17700"
	testAPIKey = "T35T-M45T3R-K3Y"

	providerConfig = `
provider "meilisearch" {
  host     = "` + testHost + `"
  api_key  = "` + testAPIKey + `"
}
`
)

var (
	testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
		"meilisearch": providerserver.NewProtocol6WithError(New("dev")()),
	}
)
