// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccChannelResource covers full CRUD + import, the JSON `config` attribute,
// labels, and the active/paused delivery lifecycle via pause/resume.
func TestAccChannelResource(t *testing.T) {
	mock := newMockServer(t)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccChannelConfig(mock.URL, "paused"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("agentops_channel.test", "id"),
					resource.TestCheckResourceAttr("agentops_channel.test", "channel_provider", "slack"),
					resource.TestCheckResourceAttr("agentops_channel.test", "connector", "bot"),
					resource.TestCheckResourceAttr("agentops_channel.test", "display_name", "alerts"),
					resource.TestCheckResourceAttr("agentops_channel.test", "status", "paused"),
					resource.TestCheckResourceAttr("agentops_channel.test", "config", `{"team":"platform"}`),
					resource.TestCheckResourceAttr("agentops_channel.test", "labels.env", "prod"),
					resource.TestCheckResourceAttrSet("agentops_channel.test", "slug"),
				),
			},
			{
				ResourceName:            "agentops_channel.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"app_token"},
			},
			{
				Config: testAccChannelConfig(mock.URL, "active"),
				Check:  resource.TestCheckResourceAttr("agentops_channel.test", "status", "active"),
			},
		},
	})
}

func testAccChannelConfig(endpoint, status string) string {
	return mockProviderConfig(endpoint) + fmt.Sprintf(`
resource "agentops_channel" "test" {
  channel_provider = "slack"
  connector        = "bot"
  display_name     = "alerts"
  status       = %q
  app_token    = "xoxb-secret"
  config       = jsonencode({ team = "platform" })
  labels = {
    env = "prod"
  }
}
`, status)
}
