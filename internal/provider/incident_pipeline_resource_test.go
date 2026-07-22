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

// TestAccIncidentPipelineResource_withEndpoint covers creating a standalone
// webhook endpoint (an agentops_trigger with no target) and linking it into the
// pipeline via trigger_id so the pipeline can be activated in a single apply —
// the create endpoint does not accept a trigger, so the provider links it via
// update before activating.
func TestAccIncidentPipelineResource_withEndpoint(t *testing.T) {
	mock := newMockServer(t)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: mockProviderConfig(mock.URL) + `
resource "agentops_trigger" "endpoint" {
  name = "incident-alerts"
}

resource "agentops_incident_pipeline" "with_endpoint" {
  name       = "prod-incidents-ep"
  status     = "active"
  trigger_id = agentops_trigger.endpoint.id

  alert_source = {
    provider     = "generic"
    monitor_mode = "create_catchall"
  }

  routing_rule = {
    route_all = true
  }

  orchestrator_binding = {
    agent_id = "agent_orch"
  }
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					// Standalone endpoint: created with no target.
					resource.TestCheckResourceAttrSet("agentops_trigger.endpoint", "id"),
					resource.TestCheckResourceAttrSet("agentops_trigger.endpoint", "token"),
					// The pipeline links the endpoint and activates.
					resource.TestCheckResourceAttrPair("agentops_incident_pipeline.with_endpoint", "trigger_id", "agentops_trigger.endpoint", "id"),
					resource.TestCheckResourceAttr("agentops_incident_pipeline.with_endpoint", "status", "active"),
				),
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
