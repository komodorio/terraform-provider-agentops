// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccIntegrationConnectionResource(t *testing.T) {
	mock := newMockServer(t)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccConnectionConfig(mock.URL, "GitHub"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("agentops_integration_connection.test", "id"),
					resource.TestCheckResourceAttrSet("agentops_integration_connection.test", "auth_config_key"),
					resource.TestCheckResourceAttr("agentops_integration_connection.test", "provider_key", "github"),
					resource.TestCheckResourceAttr("agentops_integration_connection.test", "display_name", "GitHub"),
					resource.TestCheckResourceAttr("agentops_integration_connection.test", "status", "connected"),
					resource.TestCheckResourceAttr("agentops_integration_connection.test", "metadata", `{"org":"komodorio"}`),
				),
			},
			{
				ResourceName:            "agentops_integration_connection.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"credentials"},
			},
			{
				// display_name is ForceNew -> replacement.
				Config: testAccConnectionConfig(mock.URL, "GitHub Org"),
				Check:  resource.TestCheckResourceAttr("agentops_integration_connection.test", "display_name", "GitHub Org"),
			},
		},
	})
}

func testAccConnectionConfig(endpoint, displayName string) string {
	return mockProviderConfig(endpoint) + fmt.Sprintf(`
resource "agentops_integration_connection" "test" {
  provider_key = "github"
  display_name = %q
  credentials  = { token = "t0ken" }
  metadata     = jsonencode({ org = "komodorio" })
}
`, displayName)
}
