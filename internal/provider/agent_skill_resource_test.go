// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAccAgentSkillResource(t *testing.T) {
	mock := newMockServer(t)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccAgentSkillConfig(mock.URL, ""),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("agentops_agent_skill.test", "id"),
					resource.TestCheckResourceAttr("agentops_agent_skill.test", "agent_id", "agent_1"),
					resource.TestCheckResourceAttrPair("agentops_agent_skill.test", "skill_id", "agentops_skill.test", "id"),
					resource.TestCheckResourceAttr("agentops_agent_skill.test", "origin", "manual"),
				),
			},
			{
				ResourceName:      "agentops_agent_skill.test",
				ImportState:       true,
				ImportStateVerify: true,
				// origin and created_at are not carried by the skill's used_by list, the
				// only read path for a binding, so they cannot be recovered on import.
				ImportStateVerifyIgnore: []string{"origin", "created_at"},
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					rs := s.RootModule().Resources["agentops_agent_skill.test"]
					return rs.Primary.Attributes["agent_id"] + "/" + rs.Primary.Attributes["skill_id"], nil
				},
			},
			{
				// Setting a pin repins the binding in place.
				Config: testAccAgentSkillConfig(mock.URL, "sklv_pinned"),
				Check:  resource.TestCheckResourceAttr("agentops_agent_skill.test", "pin_version_id", "sklv_pinned"),
			},
		},
	})
}

func testAccAgentSkillConfig(endpoint, pin string) string {
	pinLine := ""
	if pin != "" {
		pinLine = fmt.Sprintf("\n  pin_version_id = %q", pin)
	}
	return mockProviderConfig(endpoint) + fmt.Sprintf(`
resource "agentops_skill" "test" {
  name    = "attachable"
  content = "# Attachable\n"
}

resource "agentops_agent_skill" "test" {
  agent_id = "agent_1"
  skill_id = agentops_skill.test.id%s
}
`, pinLine)
}
