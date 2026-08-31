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

func TestAccAgentResource(t *testing.T) {
	mock := newMockServer(t)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccAgentConfig(mock.URL, "grp_1"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("agentops_agent.test", "id"),
					resource.TestCheckResourceAttr("agentops_agent.test", "agent_id", "incident-responder"),
					resource.TestCheckResourceAttr("agentops_agent.test", "mcp_group_id", "grp_1"),
					resource.TestCheckResourceAttr("agentops_agent.test", "status", "draft"),
					resource.TestCheckResourceAttr("agentops_agent.test", "created_at", mockTS),
					resource.TestCheckResourceAttrSet("agentops_agent.test", "install_values"),
					resource.TestCheckResourceAttr("agentops_agent.test", "worker_token_hint", "wt_inci..."),
				),
			},
			{
				Config: testAccAgentConfig(mock.URL, "grp_2"),
				Check:  resource.TestCheckResourceAttr("agentops_agent.test", "mcp_group_id", "grp_2"),
			},
		},
	})
}

// TestAccAgentResource_readFailureKeepsInstallValues covers the soft post-create
// read: the Helm values and the worker token hint come from the create response
// and are never returned again, so a transient failure reading the agent back
// must leave a usable state rather than an unknown-after-apply error.
func TestAccAgentResource_readFailureKeepsInstallValues(t *testing.T) {
	mock := newMockServer(t)
	mock.tune(func(m *mockServer) { m.agentReadFails = 1 })

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccAgentConfig(mock.URL, ""),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("agentops_agent.test", "install_values"),
					resource.TestCheckResourceAttrSet("agentops_agent.test", "install_command"),
					resource.TestCheckNoResourceAttr("agentops_agent.test", "status"),
				),
			},
		},
	})
}

func testAccAgentConfig(endpoint, mcpGroup string) string {
	binding := ""
	if mcpGroup != "" {
		binding = fmt.Sprintf("  mcp_group_id = %q\n", mcpGroup)
	}
	return mockProviderConfig(endpoint) + fmt.Sprintf(`
resource "agentops_agent" "test" {
  agent_id     = "incident-responder"
  instructions = "Investigate and remediate."
%s}
`, binding)
}

// TestAccAgentResource_mcpBindFailureKeepsAgent covers the create whose follow-up
// MCP-group bind fails. The agent is already registered by then, so it has to
// reach state — tainted — or the next apply registers a second agent beside it
// and collides on agent_id.
func TestAccAgentResource_mcpBindFailureKeepsAgent(t *testing.T) {
	mock := newMockServer(t)
	mock.tune(func(m *mockServer) { m.mcpGroupBindFails = true })

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccAgentConfig(mock.URL, "grp_1"),
				ExpectError: regexp.MustCompile("Error binding MCP group to agent"),
			},
			{
				PreConfig: func() { mock.tune(func(m *mockServer) { m.mcpGroupBindFails = false }) },
				Config:    testAccAgentConfig(mock.URL, "grp_1"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("agentops_agent.test", "mcp_group_id", "grp_1"),
					func(*terraform.State) error {
						if got := mock.runtimeAgentCount(); got != 1 {
							return fmt.Errorf("control plane holds %d agent(s), want 1", got)
						}
						return nil
					},
				),
			},
		},
	})
}
