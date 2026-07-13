// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccMCPGatewayGroupResource(t *testing.T) {
	mock := newMockServer(t)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccGroupConfig(mock.URL, "core-tools"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("agentops_mcp_gateway_group.test", "id"),
					resource.TestCheckResourceAttr("agentops_mcp_gateway_group.test", "name", "core-tools"),
					resource.TestCheckResourceAttr("agentops_mcp_gateway_group.test", "member_server_ids.0", "srv_1"),
				),
			},
			{
				ResourceName:      "agentops_mcp_gateway_group.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: testAccGroupConfig(mock.URL, "core-tools-v2"),
				Check:  resource.TestCheckResourceAttr("agentops_mcp_gateway_group.test", "name", "core-tools-v2"),
			},
		},
	})
}

func testAccGroupConfig(endpoint, name string) string {
	return mockProviderConfig(endpoint) + fmt.Sprintf(`
resource "agentops_mcp_gateway_group" "test" {
  name              = %q
  member_server_ids = ["srv_1"]
}
`, name)
}
