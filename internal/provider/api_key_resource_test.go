// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccAPIKeyResource covers the create/read/import/destroy cycle and the
// ForceNew behaviour (api keys have no update endpoint) against the mock server.
func TestAccAPIKeyResource(t *testing.T) {
	mock := newMockServer(t)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{ // Create
				Config: testAccAPIKeyConfig(mock.URL, "acctest key"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("agentops_api_key.test", "id"),
					resource.TestCheckResourceAttrSet("agentops_api_key.test", "key"),
					resource.TestCheckResourceAttrSet("agentops_api_key.test", "principal_id"),
					resource.TestCheckResourceAttr("agentops_api_key.test", "name", "acctest key"),
					resource.TestCheckResourceAttr("agentops_api_key.test", "status", "active"),
				),
			},
			{ // Import: the secret is not retrievable, so ignore it on verify.
				ResourceName:            "agentops_api_key.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"key", "role_ids", "service_account_id"},
			},
			{ // Renaming forces replacement (new id).
				Config: testAccAPIKeyConfig(mock.URL, "acctest key renamed"),
				Check:  resource.TestCheckResourceAttr("agentops_api_key.test", "name", "acctest key renamed"),
			},
		},
	})
}

func testAccAPIKeyConfig(endpoint, name string) string {
	return mockProviderConfig(endpoint) + fmt.Sprintf(`
resource "agentops_api_key" "test" {
  name = %q
}
`, name)
}
