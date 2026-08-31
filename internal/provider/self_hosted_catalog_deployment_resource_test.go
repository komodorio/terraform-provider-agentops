// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
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
