// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccServiceAccountResource covers create/read/import and the ForceNew +
// list-based Read behaviour (no update or GET-by-id endpoint).
func TestAccServiceAccountResource(t *testing.T) {
	mock := newMockServer(t)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccServiceAccountConfig(mock.URL, "ci-bot"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("agentops_service_account.test", "id"),
					resource.TestCheckResourceAttr("agentops_service_account.test", "display_name", "ci-bot"),
					resource.TestCheckResourceAttr("agentops_service_account.test", "status", "active"),
				),
			},
			{
				ResourceName:            "agentops_service_account.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"role_ids"},
			},
			{
				// display_name is ForceNew -> renaming replaces the resource (new id).
				Config: testAccServiceAccountConfig(mock.URL, "ci-bot-renamed"),
				Check:  resource.TestCheckResourceAttr("agentops_service_account.test", "display_name", "ci-bot-renamed"),
			},
		},
	})
}

func testAccServiceAccountConfig(endpoint, name string) string {
	return mockProviderConfig(endpoint) + fmt.Sprintf(`
resource "agentops_service_account" "test" {
  display_name = %q
}
`, name)
}
