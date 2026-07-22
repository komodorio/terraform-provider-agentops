// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccGraderConfigResource covers full CRUD + import. Read resolves the
// config via the list endpoint (there is no get-by-id).
func TestAccGraderConfigResource(t *testing.T) {
	mock := newMockServer(t)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccGraderConfigConfig(mock.URL, 25),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("agentops_grader_config.test", "id"),
					resource.TestCheckResourceAttr("agentops_grader_config.test", "target_agent_id", "agent_target"),
					resource.TestCheckResourceAttr("agentops_grader_config.test", "grader_agent_id", "agent_grader"),
					resource.TestCheckResourceAttr("agentops_grader_config.test", "sample_rate", "25"),
					resource.TestCheckResourceAttr("agentops_grader_config.test", "guidelines", "Be strict."),
					resource.TestCheckResourceAttr("agentops_grader_config.test", "runs_seen", "0"),
				),
			},
			{
				ResourceName:      "agentops_grader_config.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: testAccGraderConfigConfig(mock.URL, 50),
				Check:  resource.TestCheckResourceAttr("agentops_grader_config.test", "sample_rate", "50"),
			},
		},
	})
}

func testAccGraderConfigConfig(endpoint string, sampleRate int) string {
	return mockProviderConfig(endpoint) + fmt.Sprintf(`
resource "agentops_grader_config" "test" {
  target_agent_id = "agent_target"
  grader_agent_id = "agent_grader"
  guidelines      = "Be strict."
  sample_rate     = %d
}
`, sampleRate)
}
