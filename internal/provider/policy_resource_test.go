// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccPolicyResource(t *testing.T) {
	mock := newMockServer(t)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccPolicyConfig(mock.URL, "read-only"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("agentops_policy.test", "id"),
					resource.TestCheckResourceAttr("agentops_policy.test", "name", "read-only"),
					resource.TestCheckResourceAttr("agentops_policy.test", "builtin", "false"),
				),
			},
			{
				ResourceName:      "agentops_policy.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: testAccPolicyConfig(mock.URL, "read-only-v2"),
				Check:  resource.TestCheckResourceAttr("agentops_policy.test", "name", "read-only-v2"),
			},
		},
	})
}

func testAccPolicyConfig(endpoint, name string) string {
	return mockProviderConfig(endpoint) + fmt.Sprintf(`
resource "agentops_policy" "test" {
  name        = %q
  description = "managed by tf"
}
`, name)
}
