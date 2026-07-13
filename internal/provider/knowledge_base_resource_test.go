// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccKnowledgeBaseResource(t *testing.T) {
	mock := newMockServer(t)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccKBConfig(mock.URL, "runbooks"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("agentops_knowledge_base.test", "id"),
					resource.TestCheckResourceAttr("agentops_knowledge_base.test", "name", "runbooks"),
				),
			},
			{
				ResourceName:      "agentops_knowledge_base.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: testAccKBConfig(mock.URL, "runbooks-v2"),
				Check:  resource.TestCheckResourceAttr("agentops_knowledge_base.test", "name", "runbooks-v2"),
			},
		},
	})
}

func testAccKBConfig(endpoint, name string) string {
	return mockProviderConfig(endpoint) + fmt.Sprintf(`
resource "agentops_knowledge_base" "test" {
  name = %q
}
`, name)
}
