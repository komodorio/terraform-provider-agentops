// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAccOutpostResource(t *testing.T) {
	mock := newMockServer(t)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccOutpostConfig(mock.URL, "edge-relay"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("agentops_outpost.test", "id"),
					resource.TestCheckResourceAttr("agentops_outpost.test", "name", "edge-relay"),
					resource.TestCheckResourceAttr("agentops_outpost.test", "status", "pending"),
					resource.TestCheckResourceAttr("agentops_outpost.test", "allowlist.#", "1"),
					resource.TestCheckResourceAttr("agentops_outpost.test", "allowlist.0.host", "internal.example.com"),
					resource.TestCheckResourceAttr("agentops_outpost.test", "labels.env", "prod"),
					resource.TestCheckResourceAttrSet("agentops_outpost.test", "credential"),
				),
			},
			{
				ResourceName:            "agentops_outpost.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"credential", "credential_hint"},
			},
			{
				Config: testAccOutpostConfig(mock.URL, "edge-relay-2"),
				Check:  resource.TestCheckResourceAttr("agentops_outpost.test", "name", "edge-relay-2"),
			},
		},
	})
}

// TestAccOutpostResource_clearsAllowlistAndLabels covers removing the blocks from
// config. The API leaves an omitted allowlist alone, so the update has to send an
// explicit empty value — otherwise the outpost keeps reaching hosts the operator
// believes they revoked, with no diff to say so.
func TestAccOutpostResource_clearsAllowlistAndLabels(t *testing.T) {
	mock := newMockServer(t)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccOutpostConfig(mock.URL, "edge-relay"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("agentops_outpost.test", "allowlist.#", "1"),
					resource.TestCheckResourceAttr("agentops_outpost.test", "labels.env", "prod"),
				),
			},
			{
				Config: mockProviderConfig(mock.URL) + `
resource "agentops_outpost" "test" {
  name        = "edge-relay"
  description = "relay for the private network"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckNoResourceAttr("agentops_outpost.test", "allowlist.#"),
					resource.TestCheckNoResourceAttr("agentops_outpost.test", "labels.%"),
					checkOutpostCleared(mock, "agentops_outpost.test"),
				),
			},
			{
				Config: mockProviderConfig(mock.URL) + `
resource "agentops_outpost" "test" {
  name = "edge-relay"
}
`,
				Check: resource.TestCheckNoResourceAttr("agentops_outpost.test", "description"),
			},
		},
	})
}

// checkOutpostCleared asserts the control plane, not just the state, no longer
// holds an allowlist or labels for this outpost.
func checkOutpostCleared(mock *mockServer, resName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resName]
		if !ok {
			return fmt.Errorf("%s not found in state", resName)
		}
		rec := mock.outpostRecord(rs.Primary.ID)
		if rules, _ := rec["allowlist"].([]any); len(rules) != 0 {
			return fmt.Errorf("outpost %s still has %d allowlist rules on the server", rs.Primary.ID, len(rules))
		}
		if labels, _ := rec["labels"].(map[string]any); len(labels) != 0 {
			return fmt.Errorf("outpost %s still has %d labels on the server", rs.Primary.ID, len(labels))
		}
		return nil
	}
}

func testAccOutpostConfig(endpoint, name string) string {
	return mockProviderConfig(endpoint) + fmt.Sprintf(`
resource "agentops_outpost" "test" {
  name        = %q
  description = "relay for the private network"

  allowlist = [{
    scheme = "https"
    host   = "internal.example.com"
    port   = 443
  }]

  labels = {
    env = "prod"
  }
}
`, name)
}
