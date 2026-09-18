package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccKeyDataSource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Read testing
			{
				Config: providerConfig + `
data "meilisearch_key" "test" {
	uid = "11111111-2222-3333-4444-555555555555"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					// Verify all attributes are set
					resource.TestCheckResourceAttr("data.meilisearch_key.test", "uid", "11111111-2222-3333-4444-555555555555"),
					resource.TestCheckResourceAttr("data.meilisearch_key.test", "name", "test_api_key"),
					resource.TestCheckResourceAttr("data.meilisearch_key.test", "description", "Test API key"),
					resource.TestCheckResourceAttr("data.meilisearch_key.test", "expires_at", "2042-04-02T00:42:42Z"),
					// Verifiy number and values of actions
					resource.TestCheckResourceAttr("data.meilisearch_key.test", "actions.#", "1"),
					resource.TestCheckTypeSetElemAttr("data.meilisearch_key.test", "actions.*", "documents.add"),
					// Verifiy number and values of indexes
					resource.TestCheckResourceAttr("data.meilisearch_key.test", "indexes.#", "2"),
					resource.TestCheckTypeSetElemAttr("data.meilisearch_key.test", "indexes.*", "products"),
					resource.TestCheckTypeSetElemAttr("data.meilisearch_key.test", "indexes.*", "users"),
					resource.TestCheckResourceAttr("data.meilisearch_key.test", "id", "11111111-2222-3333-4444-555555555555"),
				),
			},
		},
	})
}
