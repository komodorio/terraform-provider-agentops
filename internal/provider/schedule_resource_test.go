// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccScheduleResource covers full CRUD + import and exercises the JSON
// `input` attribute (jsontypes.Normalized) round-trip against the mock server.
func TestAccScheduleResource(t *testing.T) {
	mock := newMockServer(t)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccScheduleConfig(mock.URL, "0 9 * * *"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("agentops_schedule.test", "id"),
					resource.TestCheckResourceAttr("agentops_schedule.test", "agent_id", "agent_1"),
					resource.TestCheckResourceAttr("agentops_schedule.test", "cron_expr", "0 9 * * *"),
					resource.TestCheckResourceAttr("agentops_schedule.test", "is_enabled", "true"),
					resource.TestCheckResourceAttr("agentops_schedule.test", "input", `{"key":"value"}`),
				),
			},
			{
				ResourceName:      "agentops_schedule.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: testAccScheduleConfig(mock.URL, "0 18 * * *"),
				Check:  resource.TestCheckResourceAttr("agentops_schedule.test", "cron_expr", "0 18 * * *"),
			},
		},
	})
}

func testAccScheduleConfig(endpoint, cron string) string {
	return mockProviderConfig(endpoint) + fmt.Sprintf(`
resource "agentops_schedule" "test" {
  agent_id  = "agent_1"
  cron_expr = %q
  input     = jsonencode({ key = "value" })
}
`, cron)
}
