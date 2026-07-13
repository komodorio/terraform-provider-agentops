// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccMCPGatewayServerResource(t *testing.T) {
	mock := newMockServer(t)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccServerConfig(mock.URL, "https://mcp.example.com"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("agentops_mcp_gateway_server.test", "id"),
					resource.TestCheckResourceAttr("agentops_mcp_gateway_server.test", "name", "docs-mcp"),
					resource.TestCheckResourceAttr("agentops_mcp_gateway_server.test", "url", "https://mcp.example.com"),
				),
			},
			{
				ResourceName:      "agentops_mcp_gateway_server.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: testAccServerConfig(mock.URL, "https://mcp2.example.com"),
				Check:  resource.TestCheckResourceAttr("agentops_mcp_gateway_server.test", "url", "https://mcp2.example.com"),
			},
		},
	})
}

func testAccServerConfig(endpoint, url string) string {
	return mockProviderConfig(endpoint) + fmt.Sprintf(`
resource "agentops_mcp_gateway_server" "test" {
  name = "docs-mcp"
  url  = %q
}
`, url)
}
