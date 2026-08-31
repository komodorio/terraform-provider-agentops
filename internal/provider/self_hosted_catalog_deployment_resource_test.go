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

// TestAccSelfHostedCatalogDeploymentResource covers deploy (create), read-back and
// delete. The deploy mints a worker token the operator feeds into their own chart,
// so the token and its hint are asserted; the resource does not support import.
func TestAccSelfHostedCatalogDeploymentResource(t *testing.T) {
	mock := newMockServer(t)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccSelfHostedCatalogDeploymentConfig(mock.URL),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("agentops_self_hosted_catalog_deployment.test", "id"),
					resource.TestCheckResourceAttr("agentops_self_hosted_catalog_deployment.test", "catalog_id", "datadog-investigator"),
					resource.TestCheckResourceAttr("agentops_self_hosted_catalog_deployment.test", "agent_id", "prod-ddog-self"),
					resource.TestCheckResourceAttr("agentops_self_hosted_catalog_deployment.test", "token", "wt_prod-ddog-self_secret"),
					resource.TestCheckResourceAttr("agentops_self_hosted_catalog_deployment.test", "worker_token_hint", "wt_prod..."),
					resource.TestCheckResourceAttr("agentops_self_hosted_catalog_deployment.test", "status", "draft"),
					resource.TestCheckResourceAttr("agentops_self_hosted_catalog_deployment.test", "created_at", mockTS),
					// A hosted deploy's repo_* attributes have no analogue here.
					resource.TestCheckNoResourceAttr("agentops_self_hosted_catalog_deployment.test", "repo_owner"),
				),
			},
		},
	})
}

// TestAccSelfHostedCatalogDeploymentResource_serverAssignedAgentID verifies the
// slug is recovered from the agent record when the client omits agent_id — the
// deploy response only ever carries the opaque id.
func TestAccSelfHostedCatalogDeploymentResource_serverAssignedAgentID(t *testing.T) {
	mock := newMockServer(t)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: mockProviderConfig(mock.URL) + `
resource "agentops_self_hosted_catalog_deployment" "auto" {
  catalog_id = "datadog-investigator"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("agentops_self_hosted_catalog_deployment.auto", "agent_id", "datadog-investigator"),
					resource.TestCheckResourceAttrSet("agentops_self_hosted_catalog_deployment.auto", "token"),
				),
			},
		},
	})
}

// TestAccSelfHostedCatalogDeploymentResource_partialSuccess pins the 207 contract:
// the agent and its token were created even though a trigger was not, so the
// deploy must be kept in state rather than reported as an error.
func TestAccSelfHostedCatalogDeploymentResource_partialSuccess(t *testing.T) {
	mock := newMockServer(t)
	mock.tune(func(m *mockServer) { m.selfHostedTriggerFails = true })

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccSelfHostedCatalogDeploymentConfig(mock.URL),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("agentops_self_hosted_catalog_deployment.test", "agent_id", "prod-ddog-self"),
					resource.TestCheckResourceAttr("agentops_self_hosted_catalog_deployment.test", "token", "wt_prod-ddog-self_secret"),
				),
			},
		},
	})
}

func testAccSelfHostedCatalogDeploymentConfig(endpoint string) string {
	return mockProviderConfig(endpoint) + `
resource "agentops_self_hosted_catalog_deployment" "test" {
  catalog_id     = "datadog-investigator"
  agent_id       = "prod-ddog-self"
  display_name   = "Prod Datadog Investigator (self-hosted)"
  credential_ref = "anthropic-api-key"

  integration_connections = {
    datadog = "conn_1"
  }
}
`
}

// TestAccSelfHostedCatalogDeploymentResource_readFailureKeepsToken pins the
// consequence that makes the post-deploy read soft: the token is minted once, so
// a transient failure reading the agent back must not cost the apply its state.
// Losing it means the worker can never be installed and the agent has to be
// deleted and redeployed.
func TestAccSelfHostedCatalogDeploymentResource_readFailureKeepsToken(t *testing.T) {
	mock := newMockServer(t)
	mock.tune(func(m *mockServer) { m.agentReadFails = 1 })

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccSelfHostedCatalogDeploymentConfig(mock.URL),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("agentops_self_hosted_catalog_deployment.test", "id"),
					resource.TestCheckResourceAttr("agentops_self_hosted_catalog_deployment.test", "token", "wt_prod-ddog-self_secret"),
					resource.TestCheckResourceAttr("agentops_self_hosted_catalog_deployment.test", "agent_id", "prod-ddog-self"),
					// The read never landed, so the agent's own fields stay unset
					// rather than unknown — an unknown here fails the whole apply.
					resource.TestCheckNoResourceAttr("agentops_self_hosted_catalog_deployment.test", "status"),
					resource.TestCheckNoResourceAttr("agentops_self_hosted_catalog_deployment.test", "name"),
				),
			},
		},
	})
}

// TestAccSelfHostedCatalogDeploymentResource_deleteGatedByLifecycleFlag covers the
// second caller of archiveAndDeleteAgent: a gated 404 from DELETE /agents/{id}
// must fail this resource's destroy too, not silently orphan the deployed agent.
func TestAccSelfHostedCatalogDeploymentResource_deleteGatedByLifecycleFlag(t *testing.T) {
	mock := newMockServer(t)
	mock.tune(func(m *mockServer) { m.deleteAgentAnswer = deleteAgentLifecycleOff })

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccSelfHostedCatalogDeploymentConfig(mock.URL),
				Check:  resource.TestCheckResourceAttr("agentops_self_hosted_catalog_deployment.test", "agent_id", "prod-ddog-self"),
			},
			{
				Config:      testAccSelfHostedCatalogDeploymentConfig(mock.URL),
				Destroy:     true,
				ExpectError: regexp.MustCompile("self_hosted_agent_lifecycle"),
			},
			{
				Config: testAccSelfHostedCatalogDeploymentConfig(mock.URL),
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

// testAccSelfHostedCatalogDeploymentMcpConfig is the deploy config with the one
// in-place input, mcp_group_id, set to group (omitted entirely when empty).
func testAccSelfHostedCatalogDeploymentMcpConfig(endpoint, group string) string {
	binding := ""
	if group != "" {
		binding = fmt.Sprintf("  mcp_group_id = %q\n", group)
	}
	return mockProviderConfig(endpoint) + fmt.Sprintf(`
resource "agentops_self_hosted_catalog_deployment" "test" {
  catalog_id = "datadog-investigator"
  agent_id   = "prod-ddog-self"
%s}
`, binding)
}

// TestAccSelfHostedCatalogDeploymentResource_mcpGroupLifecycle covers the whole
// life of the one attribute that can change in place. mcp_group_id is Optional and
// not Computed, so each apply owes Terraform a final state equal to the plan, and
// each refresh owes it the truth — the two obligations the post-mutation read used
// to conflate.
func TestAccSelfHostedCatalogDeploymentResource_mcpGroupLifecycle(t *testing.T) {
	mock := newMockServer(t)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccSelfHostedCatalogDeploymentMcpConfig(mock.URL, "grp_1"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("agentops_self_hosted_catalog_deployment.test", "mcp_group_id", "grp_1"),
					func(*terraform.State) error {
						if got := mock.agentMcpGroup("prod-ddog-self"); got != "grp_1" {
							return fmt.Errorf("control plane bound group %q, want grp_1", got)
						}
						return nil
					},
				),
			},
			// The framework's own post-apply plan already has to be empty; this
			// pins it as the thing under test rather than an implicit side effect.
			{
				Config:   testAccSelfHostedCatalogDeploymentMcpConfig(mock.URL, "grp_1"),
				PlanOnly: true,
			},
			{
				Config: testAccSelfHostedCatalogDeploymentMcpConfig(mock.URL, "grp_2"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("agentops_self_hosted_catalog_deployment.test", "mcp_group_id", "grp_2"),
					resource.TestCheckResourceAttr("agentops_self_hosted_catalog_deployment.test", "token", "wt_prod-ddog-self_secret"),
					func(*terraform.State) error {
						if got := mock.agentMcpGroup("prod-ddog-self"); got != "grp_2" {
							return fmt.Errorf("control plane bound group %q after the update, want grp_2", got)
						}
						return nil
					},
				),
			},
		},
	})
}

// TestAccSelfHostedCatalogDeploymentResource_mcpGroupOutOfBandUnbind is the
// refresh half. Someone unbinds the group outside Terraform; the next plan has to
// show a diff and the apply after it has to re-bind. Reporting no changes here
// leaves the worker running with no MCP group forever, because the configuration
// never changes again.
func TestAccSelfHostedCatalogDeploymentResource_mcpGroupOutOfBandUnbind(t *testing.T) {
	mock := newMockServer(t)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccSelfHostedCatalogDeploymentMcpConfig(mock.URL, "grp_1"),
				Check:  resource.TestCheckResourceAttr("agentops_self_hosted_catalog_deployment.test", "mcp_group_id", "grp_1"),
			},
			{
				PreConfig:          func() { mock.unbindAgentMcpGroup("prod-ddog-self") },
				Config:             testAccSelfHostedCatalogDeploymentMcpConfig(mock.URL, "grp_1"),
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
			{
				Config: testAccSelfHostedCatalogDeploymentMcpConfig(mock.URL, "grp_1"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("agentops_self_hosted_catalog_deployment.test", "mcp_group_id", "grp_1"),
					func(*terraform.State) error {
						if got := mock.agentMcpGroup("prod-ddog-self"); got != "grp_1" {
							return fmt.Errorf("control plane bound group %q after the drift apply, want grp_1", got)
						}
						return nil
					},
				),
			},
		},
	})
}

// TestAccSelfHostedCatalogDeploymentResource_mcpGroupNotApplied pins the failure
// the live endpoint actually produces. Settings are applied after the worker token
// is minted and report per-field failure rather than raising, so the deploy answers
// 207 and the post-deploy read honestly reports no group at all. The apply must
// still finish — the token is minted and unrecoverable — with a warning, and the
// next plan must converge by re-binding in place.
func TestAccSelfHostedCatalogDeploymentResource_mcpGroupNotApplied(t *testing.T) {
	mock := newMockServer(t)
	mock.tune(func(m *mockServer) { m.selfHostedMcpGroupFails = true })

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccSelfHostedCatalogDeploymentMcpConfig(mock.URL, "grp_1"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("agentops_self_hosted_catalog_deployment.test", "token", "wt_prod-ddog-self_secret"),
					resource.TestCheckResourceAttr("agentops_self_hosted_catalog_deployment.test", "mcp_group_id", "grp_1"),
					func(*terraform.State) error {
						if got := mock.agentMcpGroup("prod-ddog-self"); got != "" {
							return fmt.Errorf("control plane bound group %q, want none", got)
						}
						return nil
					},
				),
				// The apply kept the plan, so state and the control plane disagree —
				// which is the point: the refresh right after it is already a diff.
				ExpectNonEmptyPlan: true,
			},
			// The group never landed, so the refresh reports the truth and the plan
			// is a diff — the escape hatch that turns a partial deploy into a
			// convergent one without rotating the worker token.
			{
				PreConfig:          func() { mock.tune(func(m *mockServer) { m.selfHostedMcpGroupFails = false }) },
				Config:             testAccSelfHostedCatalogDeploymentMcpConfig(mock.URL, "grp_1"),
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
			{
				Config: testAccSelfHostedCatalogDeploymentMcpConfig(mock.URL, "grp_1"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("agentops_self_hosted_catalog_deployment.test", "mcp_group_id", "grp_1"),
					func(*terraform.State) error {
						if got := mock.agentMcpGroup("prod-ddog-self"); got != "grp_1" {
							return fmt.Errorf("control plane bound group %q after the retry apply, want grp_1", got)
						}
						return nil
					},
				),
			},
		},
	})
}

// TestAccSelfHostedCatalogDeploymentResource_mcpGroupAbsentIgnoresForeignBinding
// covers the configuration that never mentions mcp_group_id. A catalog entry may
// bind a group of its own, and adopting it would put a value in state the
// configuration does not have — planning a removal of a binding this resource
// never made.
func TestAccSelfHostedCatalogDeploymentResource_mcpGroupAbsentIgnoresForeignBinding(t *testing.T) {
	mock := newMockServer(t)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccSelfHostedCatalogDeploymentMcpConfig(mock.URL, ""),
				Check:  resource.TestCheckNoResourceAttr("agentops_self_hosted_catalog_deployment.test", "mcp_group_id"),
			},
			{
				PreConfig: func() { mock.bindAgentMcpGroup("prod-ddog-self", "grp_catalog") },
				Config:    testAccSelfHostedCatalogDeploymentMcpConfig(mock.URL, ""),
				PlanOnly:  true,
				Check:     resource.TestCheckNoResourceAttr("agentops_self_hosted_catalog_deployment.test", "mcp_group_id"),
			},
		},
	})
}
