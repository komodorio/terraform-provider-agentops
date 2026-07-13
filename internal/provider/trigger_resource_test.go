// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccTriggerResource exercises a full create/read/update/import round-trip
// against a live AgentOps API. It runs only when TF_ACC=1; set AGENTOPS_API_KEY,
// AGENTOPS_ENDPOINT (for non-prod), and AGENTOPS_TEST_TARGET_ID to a valid target.
func TestAccTriggerResource(t *testing.T) {
	targetID := os.Getenv("AGENTOPS_TEST_TARGET_ID")

	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
			if targetID == "" {
				t.Skip("AGENTOPS_TEST_TARGET_ID not set")
			}
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccTriggerConfig(targetID, "acctest trigger", true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("agentops_trigger.test", "id"),
					resource.TestCheckResourceAttrSet("agentops_trigger.test", "token"),
					resource.TestCheckResourceAttr("agentops_trigger.test", "name", "acctest trigger"),
					resource.TestCheckResourceAttr("agentops_trigger.test", "is_enabled", "true"),
				),
			},
			{
				ResourceName:            "agentops_trigger.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"token", "signing_secret"},
			},
			{
				Config: testAccTriggerConfig(targetID, "acctest trigger renamed", false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("agentops_trigger.test", "name", "acctest trigger renamed"),
					resource.TestCheckResourceAttr("agentops_trigger.test", "is_enabled", "false"),
				),
			},
		},
	})
}

func testAccTriggerConfig(targetID, name string, enabled bool) string {
	return fmt.Sprintf(`
resource "agentops_trigger" "test" {
  name        = %q
  target_id   = %q
  target_type = "agent"
  is_enabled  = %t
}
`, name, targetID, enabled)
}
