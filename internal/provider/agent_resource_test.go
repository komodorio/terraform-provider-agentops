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

// TestAccAgentResource_deleteGatedByLifecycleFlagFailsLoudly pins the destroy
// contract when DELETE /agents/{id} is refused. The route is gated on the
// self_hosted_agent_lifecycle feature flag, which is off by default, and answers
// 404 while it is shut. Reading that 404 as "already gone" drops a live agent —
// still registered, still holding a worker token the cluster is using — out of
// state, and a later apply reusing the id adopts it and rotates that token.
func TestAccAgentResource_deleteGatedByLifecycleFlagFailsLoudly(t *testing.T) {
	mock := newMockServer(t)
	mock.tune(func(m *mockServer) { m.deleteAgentAnswer = deleteAgentLifecycleOff })

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccAgentConfig(mock.URL, ""),
				Check:  resource.TestCheckResourceAttr("agentops_agent.test", "agent_id", "incident-responder"),
			},
			{
				Config:      testAccAgentConfig(mock.URL, ""),
				Destroy:     true,
				ExpectError: regexp.MustCompile("self_hosted_agent_lifecycle"),
			},
			{
				Config: testAccAgentConfig(mock.URL, ""),
				Check: resource.ComposeAggregateTestCheckFunc(
					// The refused destroy left the agent alive; the resource is still
					// in state, so the same config plans clean rather than recreating.
					func(*terraform.State) error {
						if got := mock.runtimeAgentCount(); got != 1 {
							return fmt.Errorf("control plane holds %d agent(s) after the refused destroy, want 1", got)
						}
						// Lift the gate so the framework's own teardown destroy can finish.
						mock.tune(func(m *mockServer) { m.deleteAgentAnswer = deleteAgentNormal })
						return nil
					},
				),
			},
		},
	})
}

// TestAccAgentResource_deleteOpaque404WithSurvivingAgentFails is the same contract
// without the tell-tale detail string: a bare 404 is only "already gone" if the
// agent is actually gone, so the destroy is confirmed with a read (the read path
// is never feature-gated) rather than trusted from the status code alone.
func TestAccAgentResource_deleteOpaque404WithSurvivingAgentFails(t *testing.T) {
	mock := newMockServer(t)
	mock.tune(func(m *mockServer) { m.deleteAgentAnswer = deleteAgentOpaque404 })

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: testAccAgentConfig(mock.URL, "")},
			{
				Config:      testAccAgentConfig(mock.URL, ""),
				Destroy:     true,
				ExpectError: regexp.MustCompile("still registered"),
			},
			{
				Config: testAccAgentConfig(mock.URL, ""),
				Check: func(*terraform.State) error {
					if got := mock.runtimeAgentCount(); got != 1 {
						return fmt.Errorf("control plane holds %d agent(s) after the refused destroy, want 1", got)
					}
					mock.tune(func(m *mockServer) { m.deleteAgentAnswer = deleteAgentNormal })
					return nil
				},
			},
		},
	})
}

// TestAccAgentResource_deleteAlreadyGoneSucceeds guards the other half: when the
// agent really has been removed out of band, the 404 stays a silent success and
// the destroy must not start failing.
func TestAccAgentResource_deleteAlreadyGoneSucceeds(t *testing.T) {
	mock := newMockServer(t)
	mock.tune(func(m *mockServer) { m.deleteAgentAnswer = deleteAgentAlreadyGone })

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: testAccAgentConfig(mock.URL, "")},
			{Config: testAccAgentConfig(mock.URL, ""), Destroy: true},
		},
	})
}

// TestAccAgentResource_mcpGroupOutOfBandUnbind is the refresh half of the same
// contract the self-hosted deployment owes. The binding is made by a separate
// PATCH, so nothing about the agent's own record changes when someone unbinds the
// group outside Terraform — and if the read only ever adopts a non-empty group,
// the stale value survives and the plan reports no changes for good, while the
// agent runs with no MCP group.
func TestAccAgentResource_mcpGroupOutOfBandUnbind(t *testing.T) {
	mock := newMockServer(t)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccAgentConfig(mock.URL, "grp_1"),
				Check:  resource.TestCheckResourceAttr("agentops_agent.test", "mcp_group_id", "grp_1"),
			},
			{
				PreConfig:          func() { mock.unbindAgentMcpGroup("incident-responder") },
				Config:             testAccAgentConfig(mock.URL, "grp_1"),
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
			{
				Config: testAccAgentConfig(mock.URL, "grp_1"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("agentops_agent.test", "mcp_group_id", "grp_1"),
					func(*terraform.State) error {
						if got := mock.agentMcpGroup("incident-responder"); got != "grp_1" {
							return fmt.Errorf("control plane bound group %q after the drift apply, want grp_1", got)
						}
						return nil
					},
				),
			},
		},
	})
}

// TestAccAgentResource_mcpGroupAbsentIgnoresForeignBinding covers the
// configuration that never mentions mcp_group_id: a group attached elsewhere is
// not adopted into state, so the next plan neither shows a diff nor tears the
// binding out.
func TestAccAgentResource_mcpGroupAbsentIgnoresForeignBinding(t *testing.T) {
	mock := newMockServer(t)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccAgentConfig(mock.URL, ""),
				Check:  resource.TestCheckNoResourceAttr("agentops_agent.test", "mcp_group_id"),
			},
			{
				PreConfig: func() { mock.bindAgentMcpGroup("incident-responder", "grp_foreign") },
				Config:    testAccAgentConfig(mock.URL, ""),
				PlanOnly:  true,
				Check:     resource.TestCheckNoResourceAttr("agentops_agent.test", "mcp_group_id"),
			},
		},
	})
}

// TestAccAgentResource_mcpGroupBindNotEchoed pins the apply half: the bind is a
// separate call the agent read is not obliged to echo back in the same instant,
// and mcp_group_id is Optional and not Computed, so the post-create read must not
// be allowed to overwrite the plan with what it happened to see.
func TestAccAgentResource_mcpGroupBindNotEchoed(t *testing.T) {
	mock := newMockServer(t)
	mock.tune(func(m *mockServer) { m.mcpGroupBindSilent = true })

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:             testAccAgentConfig(mock.URL, "grp_1"),
				Check:              resource.TestCheckResourceAttr("agentops_agent.test", "mcp_group_id", "grp_1"),
				ExpectNonEmptyPlan: true,
			},
		},
	})
}
