// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccRoleResource(t *testing.T) {
	mock := newMockServer(t)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccRoleConfig(mock.URL, "deployer"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("agentops_role.test", "id"),
					resource.TestCheckResourceAttr("agentops_role.test", "name", "deployer"),
					resource.TestCheckResourceAttr("agentops_role.test", "builtin", "false"),
				),
			},
			{
				ResourceName:      "agentops_role.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: testAccRoleConfig(mock.URL, "deployer-v2"),
				Check:  resource.TestCheckResourceAttr("agentops_role.test", "name", "deployer-v2"),
			},
		},
	})
}

func testAccRoleConfig(endpoint, name string) string {
	return mockProviderConfig(endpoint) + fmt.Sprintf(`
resource "agentops_role" "test" {
  name        = %q
  description = "managed by tf"
}
`, name)
}
