package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/meilisearch/meilisearch-go"
)

func TestAccKeyResourceExpiryRemoval(t *testing.T) {
	const uid = "88888888-9999-0000-1111-222222222222"
	client := meilisearch.New(testHost, meilisearch.WithAPIKey(testAPIKey))

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig + fmt.Sprintf(`
resource "meilisearch_key" "expiry" {
  uid         = %q
  name        = "expiry-name"
  description = "expiry-description"
  actions     = ["search"]
  indexes     = ["test_index_1"]
  expires_at  = "2042-04-02T00:42:42Z"
}
`, uid),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("meilisearch_key.expiry", "expires_at", "2042-04-02T00:42:42Z"),
				),
			},
			{
				Config: providerConfig + fmt.Sprintf(`
resource "meilisearch_key" "expiry" {
  uid         = %q
  name        = "expiry-name"
  description = "expiry-description"
  actions     = ["search"]
  indexes     = ["test_index_1"]
}
`, uid),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("meilisearch_key.expiry", "Replace"),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckNoResourceAttr("meilisearch_key.expiry", "expires_at"),
					checkRemoteKeyExpiryCleared(client, uid),
				),
			},
			{
				Config: providerConfig + fmt.Sprintf(`
resource "meilisearch_key" "expiry" {
  uid         = %q
  name        = "expiry-name"
  description = "expiry-description"
  actions     = ["search"]
  indexes     = ["test_index_1"]
}
`, uid),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckNoResourceAttr("meilisearch_key.expiry", "expires_at"),
				),
			},
		},
	})
}

func checkRemoteKeyExpiryCleared(client meilisearch.ServiceManager, uid string) resource.TestCheckFunc {
	return func(_ *terraform.State) error {
		key, err := client.GetKey(uid)
		if err != nil {
			return fmt.Errorf("read key %s: %w", uid, err)
		}
		if !key.ExpiresAt.IsZero() {
			return fmt.Errorf("remote key expires_at=%s, expected no expiry", key.ExpiresAt.Format("2006-01-02T15:04:05Z07:00"))
		}

		return nil
	}
}
