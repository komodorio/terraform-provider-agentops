// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccGrantResource(t *testing.T) {
	mock := newMockServer(t)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccGrantConfig(mock.URL, "role_1"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("agentops_grant.test", "id"),
					resource.TestCheckResourceAttr("agentops_grant.test", "grant_kind", "role"),
					resource.TestCheckResourceAttr("agentops_grant.test", "resource_type", "agent"),
					resource.TestCheckResourceAttr("agentops_grant.test", "subject", `{"id":"prn_1","kind":"principal"}`),
				),
			},
			{
				ResourceName:      "agentops_grant.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: testAccGrantConfig(mock.URL, "role_2"),
				Check:  resource.TestCheckResourceAttr("agentops_grant.test", "role_id", "role_2"),
			},
		},
	})
}

func testAccGrantConfig(endpoint, roleID string) string {
	return mockProviderConfig(endpoint) + fmt.Sprintf(`
resource "agentops_grant" "test" {
  grant_kind    = "role"
  resource_id   = "agent_1"
  resource_type = "agent"
  role_id       = %q
  subject       = jsonencode({ id = "prn_1", kind = "principal" })
}
`, roleID)
}
