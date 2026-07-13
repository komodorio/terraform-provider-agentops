// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccWorkflowResource(t *testing.T) {
	mock := newMockServer(t)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccWorkflowConfig(mock.URL, "triage"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("agentops_workflow.test", "id"),
					resource.TestCheckResourceAttr("agentops_workflow.test", "name", "triage"),
					resource.TestCheckResourceAttr("agentops_workflow.test", "is_enabled", "true"),
					resource.TestCheckResourceAttr("agentops_workflow.test", "labels.team", "sre"),
				),
			},
			{
				ResourceName:      "agentops_workflow.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: testAccWorkflowConfig(mock.URL, "triage-v2"),
				Check:  resource.TestCheckResourceAttr("agentops_workflow.test", "name", "triage-v2"),
			},
		},
	})
}

func testAccWorkflowConfig(endpoint, name string) string {
	return mockProviderConfig(endpoint) + fmt.Sprintf(`
resource "agentops_workflow" "test" {
  name   = %q
  labels = { team = "sre" }
}
`, name)
}
