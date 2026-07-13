// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccCredentialResource(t *testing.T) {
	mock := newMockServer(t)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccCredentialConfig(mock.URL, "secret-1", "first"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("agentops_credential.test", "id"),
					resource.TestCheckResourceAttr("agentops_credential.test", "name", "openai"),
					resource.TestCheckResourceAttr("agentops_credential.test", "value", "secret-1"),
					resource.TestCheckResourceAttr("agentops_credential.test", "description", "first"),
					resource.TestCheckResourceAttr("agentops_credential.test", "status", "active"),
				),
			},
			{
				ResourceName:            "agentops_credential.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"value"},
			},
			{
				// Change both description (PATCH) and value (PUT /value).
				Config: testAccCredentialConfig(mock.URL, "secret-2", "second"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("agentops_credential.test", "value", "secret-2"),
					resource.TestCheckResourceAttr("agentops_credential.test", "description", "second"),
				),
			},
		},
	})
}

func testAccCredentialConfig(endpoint, value, desc string) string {
	return mockProviderConfig(endpoint) + fmt.Sprintf(`
resource "agentops_credential" "test" {
  name        = "openai"
  value       = %q
  description = %q
}
`, value, desc)
}
