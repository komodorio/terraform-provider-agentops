// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccMemberResource(t *testing.T) {
	mock := newMockServer(t)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccMemberConfig(mock.URL, "Dev One"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("agentops_member.test", "id"),
					resource.TestCheckResourceAttr("agentops_member.test", "email", "dev@example.com"),
					resource.TestCheckResourceAttr("agentops_member.test", "full_name", "Dev One"),
					resource.TestCheckResourceAttr("agentops_member.test", "status", "active"),
				),
			},
			{
				ResourceName:      "agentops_member.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				// full_name is ForceNew -> replacement.
				Config: testAccMemberConfig(mock.URL, "Dev Two"),
				Check:  resource.TestCheckResourceAttr("agentops_member.test", "full_name", "Dev Two"),
			},
		},
	})
}

func testAccMemberConfig(endpoint, fullName string) string {
	return mockProviderConfig(endpoint) + fmt.Sprintf(`
resource "agentops_member" "test" {
  email     = "dev@example.com"
  full_name = %q
}
`, fullName)
}
