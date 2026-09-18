package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

func TestAccIndexResourcePrimaryKeyOmission(t *testing.T) {
	const uid = "99999999-0000-1111-2222-333333333333"

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig + `
resource "meilisearch_index" "omitted_primary_key" {
	  uid = "99999999-0000-1111-2222-333333333333"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("meilisearch_index.omitted_primary_key", "uid", uid),
					resource.TestCheckResourceAttr("meilisearch_index.omitted_primary_key", "id", uid),
					resource.TestCheckNoResourceAttr("meilisearch_index.omitted_primary_key", "primary_key"),
				),
			},
			{
				Config: providerConfig + `
resource "meilisearch_index" "omitted_primary_key" {
  uid = "99999999-0000-1111-2222-333333333333"
}
`,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("meilisearch_index.omitted_primary_key", "uid", uid),
					resource.TestCheckResourceAttr("meilisearch_index.omitted_primary_key", "id", uid),
					resource.TestCheckNoResourceAttr("meilisearch_index.omitted_primary_key", "primary_key"),
				),
			},
			{
				ResourceName:      "meilisearch_index.omitted_primary_key",
				ImportStateId:     uid,
				ImportState:       true,
				ImportStateVerify: true,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("meilisearch_index.omitted_primary_key", "uid", uid),
					resource.TestCheckResourceAttr("meilisearch_index.omitted_primary_key", "id", uid),
					resource.TestCheckNoResourceAttr("meilisearch_index.omitted_primary_key", "primary_key"),
				),
			},
		},
	})
}
