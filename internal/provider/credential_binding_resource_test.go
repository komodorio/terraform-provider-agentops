// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAccCredentialBindingResource(t *testing.T) {
	mock := newMockServer(t)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccCredentialBindingConfig(mock.URL),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("agentops_credential_binding.test", "id"),
					resource.TestCheckResourceAttrSet("agentops_credential_binding.test", "credential_id"),
					resource.TestCheckResourceAttr("agentops_credential_binding.test", "agent_id", "agent_1"),
					resource.TestCheckResourceAttr("agentops_credential_binding.test", "on_demand", "true"),
				),
			},
			{
				ResourceName:      "agentops_credential_binding.test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					rs := s.RootModule().Resources["agentops_credential_binding.test"]
					return rs.Primary.Attributes["credential_id"] + "/" + rs.Primary.Attributes["agent_id"], nil
				},
			},
		},
	})
}

func testAccCredentialBindingConfig(endpoint string) string {
	return mockProviderConfig(endpoint) + `
resource "agentops_credential" "test" {
  name  = "bind-cred"
  value = "secret"
}

resource "agentops_credential_binding" "test" {
  credential_id = agentops_credential.test.id
  agent_id      = "agent_1"
  on_demand     = true
}
`
}
