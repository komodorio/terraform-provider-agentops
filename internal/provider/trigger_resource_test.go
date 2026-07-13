// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccTriggerResource drives a full create/read/update/import/destroy cycle
// against the in-process mock server. It requires TF_ACC=1 (set in CI) but no
// live backend, so it runs deterministically without secrets.
func TestAccTriggerResource(t *testing.T) {
	mock := newMockServer(t)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{ // Create
				Config: testAccTriggerConfig(mock.URL, "acctest trigger", true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("agentops_trigger.test", "id"),
					resource.TestCheckResourceAttrSet("agentops_trigger.test", "token"),
					resource.TestCheckResourceAttr("agentops_trigger.test", "name", "acctest trigger"),
					resource.TestCheckResourceAttr("agentops_trigger.test", "target_id", "agent_1"),
					resource.TestCheckResourceAttr("agentops_trigger.test", "is_enabled", "true"),
				),
			},
			{ // Import
				ResourceName:            "agentops_trigger.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"token", "signing_secret"},
			},
			{ // Update: rename + disable
				Config: testAccTriggerConfig(mock.URL, "acctest trigger renamed", false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("agentops_trigger.test", "name", "acctest trigger renamed"),
					resource.TestCheckResourceAttr("agentops_trigger.test", "is_enabled", "false"),
				),
			},
		},
	})
}

func testAccTriggerConfig(endpoint, name string, enabled bool) string {
	return mockProviderConfig(endpoint) + fmt.Sprintf(`
resource "agentops_trigger" "test" {
  name        = %q
  target_id   = "agent_1"
  target_type = "agent"
  is_enabled  = %t
}
`, name, enabled)
}
