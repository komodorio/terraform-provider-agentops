// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAccMCPGatewayServerResource(t *testing.T) {
	mock := newMockServer(t)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccServerConfig(mock.URL, "https://mcp.example.com"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("agentops_mcp_gateway_server.test", "id"),
					resource.TestCheckResourceAttr("agentops_mcp_gateway_server.test", "name", "docs-mcp"),
					resource.TestCheckResourceAttr("agentops_mcp_gateway_server.test", "url", "https://mcp.example.com"),
				),
			},
			{
				ResourceName:      "agentops_mcp_gateway_server.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: testAccServerConfig(mock.URL, "https://mcp2.example.com"),
				Check:  resource.TestCheckResourceAttr("agentops_mcp_gateway_server.test", "url", "https://mcp2.example.com"),
			},
		},
	})
}

func testAccServerConfig(endpoint, url string) string {
	return mockProviderConfig(endpoint) + fmt.Sprintf(`
resource "agentops_mcp_gateway_server" "test" {
  name = "docs-mcp"
  url  = %q
}
`, url)
}

// TestAccMCPGatewayServerResource_outpostBinding walks the whole binding
// lifecycle on an existing server: first bind, rebind, unbind. The update
// response can only echo the old binding or null — UpdateServerRequest has no
// outpost field — so every step here fails if the plan value is not what drives
// the decision.
func TestAccMCPGatewayServerResource_outpostBinding(t *testing.T) {
	mock := newMockServer(t)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccServerOutpostConfig(mock.URL, ""),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckNoResourceAttr("agentops_mcp_gateway_server.test", "outpost_id"),
					checkServerBoundTo(mock, ""),
				),
			},
			{
				Config: testAccServerOutpostConfig(mock.URL, "agentops_outpost.a.id"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair("agentops_mcp_gateway_server.test", "outpost_id", "agentops_outpost.a", "id"),
					checkServerBoundTo(mock, "agentops_outpost.a"),
				),
			},
			{
				Config: testAccServerOutpostConfig(mock.URL, "agentops_outpost.b.id"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair("agentops_mcp_gateway_server.test", "outpost_id", "agentops_outpost.b", "id"),
					checkServerBoundTo(mock, "agentops_outpost.b"),
				),
			},
			{
				Config: testAccServerOutpostConfig(mock.URL, ""),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckNoResourceAttr("agentops_mcp_gateway_server.test", "outpost_id"),
					checkServerBoundTo(mock, ""),
				),
			},
		},
	})
}

// TestAccMCPGatewayServerResource_outpostSurvivesUnrelatedUpdate is the
// destructive case: the update response carries no outpost at all, so reading the
// binding off it instead of off the plan deletes a binding the config still asks
// for.
func TestAccMCPGatewayServerResource_outpostSurvivesUnrelatedUpdate(t *testing.T) {
	mock := newMockServer(t)
	mock.tune(func(m *mockServer) { m.serverUpdateDropsOutpost = true })

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccServerOutpostConfigURL(mock.URL, "agentops_outpost.a.id", "https://mcp.example.com"),
				Check:  checkServerBoundTo(mock, "agentops_outpost.a"),
			},
			{
				Config: testAccServerOutpostConfigURL(mock.URL, "agentops_outpost.a.id", "https://mcp-moved.example.com"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("agentops_mcp_gateway_server.test", "url", "https://mcp-moved.example.com"),
					resource.TestCheckResourceAttrPair("agentops_mcp_gateway_server.test", "outpost_id", "agentops_outpost.a", "id"),
					checkServerBoundTo(mock, "agentops_outpost.a"),
				),
			},
		},
	})
}

// checkServerBoundTo asserts the binding the control plane holds for the server
// under test, not the one Terraform recorded. Pass an empty outpostRes to assert
// there is none.
func checkServerBoundTo(mock *mockServer, outpostRes string) resource.TestCheckFunc {
	const serverRes = "agentops_mcp_gateway_server.test"
	return func(s *terraform.State) error {
		server, ok := s.RootModule().Resources[serverRes]
		if !ok {
			return fmt.Errorf("%s not found in state", serverRes)
		}
		want := ""
		if outpostRes != "" {
			outpost, ok := s.RootModule().Resources[outpostRes]
			if !ok {
				return fmt.Errorf("%s not found in state", outpostRes)
			}
			want = outpost.Primary.ID
		}
		if got := mock.serverOutpostBinding(server.Primary.ID); got != want {
			return fmt.Errorf("server %s bound to %q on the control plane, want %q", server.Primary.ID, got, want)
		}
		return nil
	}
}

func testAccServerOutpostConfig(endpoint, outpostRef string) string {
	return testAccServerOutpostConfigURL(endpoint, outpostRef, "https://mcp.example.com")
}

func testAccServerOutpostConfigURL(endpoint, outpostRef, url string) string {
	binding := ""
	if outpostRef != "" {
		binding = "  outpost_id = " + outpostRef + "\n"
	}
	return mockProviderConfig(endpoint) + fmt.Sprintf(`
resource "agentops_outpost" "a" {
  name = "relay-a"
}

resource "agentops_outpost" "b" {
  name = "relay-b"
}

resource "agentops_mcp_gateway_server" "test" {
  name = "docs-mcp"
  url  = %q
%s}
`, url, binding)
}

// TestAccMCPGatewayServerResource_bindFailureKeepsServer covers the create whose
// follow-up bind fails. The POST already registered the server, so the apply has
// to leave it in state — tainted — or the record is orphaned and the next apply
// registers a second one alongside it.
func TestAccMCPGatewayServerResource_bindFailureKeepsServer(t *testing.T) {
	mock := newMockServer(t)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: mockProviderConfig(mock.URL) + `
resource "agentops_outpost" "a" {
  name = "relay-a"
}

resource "agentops_mcp_gateway_server" "test" {
  name       = "docs-mcp"
  url        = "https://mcp.example.com"
  outpost_id = "op_does_not_exist"
}
`,
				ExpectError: regexp.MustCompile("Error binding MCP gateway server to outpost"),
			},
			{
				Config: testAccServerOutpostConfig(mock.URL, "agentops_outpost.a.id"),
				Check: resource.ComposeAggregateTestCheckFunc(
					checkServerBoundTo(mock, "agentops_outpost.a"),
					checkMCPServerCount(mock, 1),
				),
			},
		},
	})
}

// checkMCPServerCount asserts how many gateway servers the control plane holds —
// the only way to see a record Terraform lost track of.
func checkMCPServerCount(mock *mockServer, want int) resource.TestCheckFunc {
	return func(*terraform.State) error {
		if got := mock.mcpServerCount(); got != want {
			return fmt.Errorf("control plane holds %d gateway server(s), want %d", got, want)
		}
		return nil
	}
}
