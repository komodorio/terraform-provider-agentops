// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// TestAccHostedAgentResource covers full CRUD + composite import. The write-only
// spec fields (instructions, model, skills, ...) are preserved from config and
// ignored on import, since the API never returns them.
func TestAccHostedAgentResource(t *testing.T) {
	mock := newMockServer(t)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccHostedAgentConfig(mock.URL, "gpt-4o"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("agentops_hosted_agent.test", "id"),
					resource.TestCheckResourceAttr("agentops_hosted_agent.test", "customer", "acme"),
					resource.TestCheckResourceAttr("agentops_hosted_agent.test", "agent_id", "triage"),
					resource.TestCheckResourceAttr("agentops_hosted_agent.test", "model", "gpt-4o"),
					resource.TestCheckResourceAttr("agentops_hosted_agent.test", "skills.0.id", "search"),
					resource.TestCheckResourceAttr("agentops_hosted_agent.test", "image.tag", "v1"),
					resource.TestCheckResourceAttrSet("agentops_hosted_agent.test", "runtime_agent_id"),
					resource.TestCheckResourceAttr("agentops_hosted_agent.test", "repo_owner", "komodorio"),
				),
			},
			{
				ResourceName:      "agentops_hosted_agent.test",
				ImportState:       true,
				ImportStateVerify: true,
				// Write-only spec fields are not returned by the API, so they
				// cannot be verified on import.
				ImportStateVerifyIgnore: []string{
					"instructions", "credential_ref", "model", "display_name",
					"replica_count", "commit_message", "mcp_group_id",
					"capabilities", "skills", "mcp_servers", "image",
				},
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					rs := s.RootModule().Resources["agentops_hosted_agent.test"]
					return rs.Primary.Attributes["customer"] + "/" + rs.Primary.Attributes["agent_id"], nil
				},
			},
			{
				Config: testAccHostedAgentConfig(mock.URL, "gpt-4o-mini"),
				Check:  resource.TestCheckResourceAttr("agentops_hosted_agent.test", "model", "gpt-4o-mini"),
			},
		},
	})
}

func testAccHostedAgentConfig(endpoint, model string) string {
	return mockProviderConfig(endpoint) + fmt.Sprintf(`
resource "agentops_hosted_agent" "test" {
  customer       = "acme"
  agent_id       = "triage"
  instructions   = "Triage incoming alerts."
  credential_ref = "cred_1"
  model          = %q

  skills = [
    {
      id      = "search"
      content = "search skill body"
    },
  ]

  image = {
    repository = "komodorio/agent"
    tag        = "v1"
  }
}
`, model)
}
