// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAccKnowledgeBaseAgentResource(t *testing.T) {
	mock := newMockServer(t)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccKBAgentConfig(mock.URL),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("agentops_knowledge_base_agent.test", "id"),
					resource.TestCheckResourceAttrSet("agentops_knowledge_base_agent.test", "kb_id"),
					resource.TestCheckResourceAttr("agentops_knowledge_base_agent.test", "agent_id", "agent_1"),
				),
			},
			{
				ResourceName:      "agentops_knowledge_base_agent.test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					rs := s.RootModule().Resources["agentops_knowledge_base_agent.test"]
					return rs.Primary.Attributes["kb_id"] + "/" + rs.Primary.Attributes["agent_id"], nil
				},
			},
		},
	})
}

func testAccKBAgentConfig(endpoint string) string {
	return mockProviderConfig(endpoint) + `
resource "agentops_knowledge_base" "test" {
  name = "kb-for-agent"
}

resource "agentops_knowledge_base_agent" "test" {
  kb_id    = agentops_knowledge_base.test.id
  agent_id = "agent_1"
}
`
}
