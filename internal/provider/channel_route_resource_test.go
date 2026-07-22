// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// TestAccChannelRouteResource covers full CRUD + composite import for a route
// attached to a channel.
func TestAccChannelRouteResource(t *testing.T) {
	mock := newMockServer(t)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccChannelRouteConfig(mock.URL, 10),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("agentops_channel_route.test", "id"),
					resource.TestCheckResourceAttrPair("agentops_channel_route.test", "channel_id", "agentops_channel.test", "id"),
					resource.TestCheckResourceAttr("agentops_channel_route.test", "rule_type", "keyword"),
					resource.TestCheckResourceAttr("agentops_channel_route.test", "target_type", "agent"),
					resource.TestCheckResourceAttr("agentops_channel_route.test", "target_id", "agent_1"),
					resource.TestCheckResourceAttr("agentops_channel_route.test", "priority", "10"),
					resource.TestCheckResourceAttr("agentops_channel_route.test", "is_enabled", "true"),
					resource.TestCheckResourceAttr("agentops_channel_route.test", "match", `{"keyword":"deploy"}`),
				),
			},
			{
				ResourceName:      "agentops_channel_route.test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					rs := s.RootModule().Resources["agentops_channel_route.test"]
					return rs.Primary.Attributes["channel_id"] + "/" + rs.Primary.Attributes["id"], nil
				},
			},
			{
				Config: testAccChannelRouteConfig(mock.URL, 20),
				Check:  resource.TestCheckResourceAttr("agentops_channel_route.test", "priority", "20"),
			},
		},
	})
}

func testAccChannelRouteConfig(endpoint string, priority int) string {
	return mockProviderConfig(endpoint) + fmt.Sprintf(`
resource "agentops_channel" "test" {
  channel_provider = "slack"
  connector        = "bot"
  display_name     = "alerts"
}

resource "agentops_channel_route" "test" {
  channel_id  = agentops_channel.test.id
  rule_type   = "keyword"
  target_type = "agent"
  target_id   = "agent_1"
  priority    = %d
  match       = jsonencode({ keyword = "deploy" })
}
`, priority)
}
