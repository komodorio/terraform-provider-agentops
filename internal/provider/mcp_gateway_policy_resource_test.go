// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccMCPGatewayPolicyResource covers create/update/import and exercises the
// JSON `document` attribute plus the list-based Read (this endpoint has no
// GET-by-id).
func TestAccMCPGatewayPolicyResource(t *testing.T) {
	mock := newMockServer(t)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccMCPPolicyConfig(mock.URL, true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("agentops_mcp_gateway_policy.test", "id"),
					resource.TestCheckResourceAttrSet("agentops_mcp_gateway_policy.test", "name"),
					resource.TestCheckResourceAttr("agentops_mcp_gateway_policy.test", "enabled", "true"),
					resource.TestCheckResourceAttr("agentops_mcp_gateway_policy.test", "document", `{"description":"test policy"}`),
				),
			},
			{
				ResourceName:      "agentops_mcp_gateway_policy.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: testAccMCPPolicyConfig(mock.URL, false),
				Check:  resource.TestCheckResourceAttr("agentops_mcp_gateway_policy.test", "enabled", "false"),
			},
		},
	})
}

func testAccMCPPolicyConfig(endpoint string, enabled bool) string {
	return mockProviderConfig(endpoint) + fmt.Sprintf(`
resource "agentops_mcp_gateway_policy" "test" {
  enabled  = %t
  document = jsonencode({ description = "test policy" })
}
`, enabled)
}
