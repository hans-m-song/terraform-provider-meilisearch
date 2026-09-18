package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/meilisearch/meilisearch-go"
)

func TestAccKeyResourceMetadataRemoval(t *testing.T) {
	const uid = "77777777-8888-9999-0000-111111111111"
	client := meilisearch.New(testHost, meilisearch.WithAPIKey(testAPIKey))

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig + fmt.Sprintf(`
resource "meilisearch_key" "metadata" {
	  uid         = %q
	  name        = "metadata-name"
	  description = "metadata-description"
	  actions     = ["search"]
	  indexes     = ["test_index_1"]
	  expires_at  = "2042-04-02T00:42:42Z"
}
`, uid),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("meilisearch_key.metadata", "name", "metadata-name"),
					resource.TestCheckResourceAttr("meilisearch_key.metadata", "description", "metadata-description"),
				),
			},
			{
				Config: providerConfig + fmt.Sprintf(`
resource "meilisearch_key" "metadata" {
  uid        = %q
  actions    = ["search"]
  indexes    = ["test_index_1"]
  expires_at = "2042-04-02T00:42:42Z"
}
`, uid),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("meilisearch_key.metadata", "Update"),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckNoResourceAttr("meilisearch_key.metadata", "name"),
					resource.TestCheckNoResourceAttr("meilisearch_key.metadata", "description"),
					checkRemoteKeyMetadataCleared(client, uid),
				),
			},
			{
				Config: providerConfig + fmt.Sprintf(`
resource "meilisearch_key" "metadata" {
  uid        = %q
  actions    = ["search"]
  indexes    = ["test_index_1"]
  expires_at = "2042-04-02T00:42:42Z"
}
`, uid),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckNoResourceAttr("meilisearch_key.metadata", "name"),
					resource.TestCheckNoResourceAttr("meilisearch_key.metadata", "description"),
				),
			},
		},
	})
}

func checkRemoteKeyMetadataCleared(client meilisearch.ServiceManager, uid string) resource.TestCheckFunc {
	return func(_ *terraform.State) error {
		key, err := client.GetKey(uid)
		if err != nil {
			return fmt.Errorf("read key %s: %w", uid, err)
		}
		if key.Name != "" || key.Description != "" {
			return fmt.Errorf("remote key metadata name=%q description=%q, expected both empty", key.Name, key.Description)
		}

		return nil
	}
}
