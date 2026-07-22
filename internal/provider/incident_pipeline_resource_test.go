// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccIncidentPipelineResource covers full CRUD + import, the nested
// alert_source/routing_rule/specialist_bindings/delivery_config blocks, and the
// draft/active/paused status lifecycle driven via the activate/pause endpoints.
func TestAccIncidentPipelineResource(t *testing.T) {
	mock := newMockServer(t)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccIncidentPipelineConfig(mock.URL, "active"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("agentops_incident_pipeline.test", "id"),
					resource.TestCheckResourceAttr("agentops_incident_pipeline.test", "name", "prod-incidents"),
					resource.TestCheckResourceAttr("agentops_incident_pipeline.test", "status", "active"),
					resource.TestCheckResourceAttr("agentops_incident_pipeline.test", "alert_source.provider", "datadog"),
					resource.TestCheckResourceAttr("agentops_incident_pipeline.test", "alert_source.monitor_mode", "create_catchall"),
					resource.TestCheckResourceAttr("agentops_incident_pipeline.test", "routing_rule.environment", "production"),
					resource.TestCheckResourceAttr("agentops_incident_pipeline.test", "orchestrator_binding.agent_id", "agent_orch"),
					resource.TestCheckResourceAttr("agentops_incident_pipeline.test", "specialist_bindings.0.agent_id", "agent_spec"),
					resource.TestCheckResourceAttr("agentops_incident_pipeline.test", "specialist_count", "1"),
					resource.TestCheckResourceAttr("agentops_incident_pipeline.test", "delivery_config.slack.channel_id", "C123"),
					resource.TestCheckResourceAttr("agentops_incident_pipeline.test", "source_provider", "datadog"),
					resource.TestCheckResourceAttrSet("agentops_incident_pipeline.test", "webhook_url"),
				),
			},
			{
				ResourceName:            "agentops_incident_pipeline.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"webhook_token"},
			},
			{
				Config: testAccIncidentPipelineConfig(mock.URL, "paused"),
				Check:  resource.TestCheckResourceAttr("agentops_incident_pipeline.test", "status", "paused"),
			},
		},
	})
}

func testAccIncidentPipelineConfig(endpoint, status string) string {
	return mockProviderConfig(endpoint) + fmt.Sprintf(`
resource "agentops_incident_pipeline" "test" {
  name   = "prod-incidents"
  status = %q

  alert_source = {
    provider     = "datadog"
    monitor_mode = "create_catchall"
  }

  routing_rule = {
    environment = "production"
  }

  orchestrator_binding = {
    agent_id = "agent_orch"
  }

  specialist_bindings = [
    {
      agent_id = "agent_spec"
      role     = "database"
      enabled  = true
    },
  ]

  delivery_config = {
    slack = {
      channel_id = "C123"
    }
  }
}
`, status)
}
