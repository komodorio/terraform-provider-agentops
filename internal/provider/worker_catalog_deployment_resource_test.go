// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccWorkerCatalogDeploymentResource covers deploy (create), read-back and
// delete. The server derives the customer and returns a hosted agent, so the
// computed identity fields are asserted; the resource does not support import.
func TestAccWorkerCatalogDeploymentResource(t *testing.T) {
	mock := newMockServer(t)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccWorkerCatalogDeploymentConfig(mock.URL),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("agentops_worker_catalog_deployment.test", "id"),
					resource.TestCheckResourceAttr("agentops_worker_catalog_deployment.test", "catalog_id", "datadog-investigator"),
					resource.TestCheckResourceAttr("agentops_worker_catalog_deployment.test", "agent_id", "prod-ddog"),
					// customer is server-derived, not configured.
					resource.TestCheckResourceAttr("agentops_worker_catalog_deployment.test", "customer", "acme"),
					resource.TestCheckResourceAttr("agentops_worker_catalog_deployment.test", "status", "online"),
					resource.TestCheckResourceAttrSet("agentops_worker_catalog_deployment.test", "runtime_agent_id"),
					resource.TestCheckResourceAttr("agentops_worker_catalog_deployment.test", "repo_owner", "komodorio"),
				),
			},
		},
	})
}

// TestAccWorkerCatalogDeploymentResource_serverAssignedAgentID verifies the
// agent_id is computed from the server when the client omits it.
func TestAccWorkerCatalogDeploymentResource_serverAssignedAgentID(t *testing.T) {
	mock := newMockServer(t)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: mockProviderConfig(mock.URL) + `
resource "agentops_worker_catalog_deployment" "auto" {
  catalog_id = "datadog-investigator"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("agentops_worker_catalog_deployment.auto", "agent_id", "deployed-datadog-investigator"),
					resource.TestCheckResourceAttr("agentops_worker_catalog_deployment.auto", "customer", "acme"),
				),
			},
		},
	})
}

func testAccWorkerCatalogDeploymentConfig(endpoint string) string {
	return mockProviderConfig(endpoint) + `
resource "agentops_worker_catalog_deployment" "test" {
  catalog_id     = "datadog-investigator"
  agent_id       = "prod-ddog"
  display_name   = "Prod Datadog Investigator"
  credential_ref = "anthropic-api-key"

  integration_connections = {
    datadog = "conn_1"
  }
}
`
}
