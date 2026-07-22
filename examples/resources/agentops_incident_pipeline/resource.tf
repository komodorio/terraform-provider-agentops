resource "agentops_incident_pipeline" "prod" {
  name   = "production-incidents"
  status = "active"

  alert_source = {
    provider     = "datadog"
    monitor_mode = "create_catchall"
  }

  routing_rule = {
    environment = "production"
    severity    = "critical"
  }

  orchestrator_binding = {
    agent_id = "agent_01hxyz"
  }

  specialist_bindings = [
    {
      agent_id = "agent_db_01"
      role     = "database"
      enabled  = true
    },
    {
      agent_id = "agent_net_01"
      role     = "networking"
      enabled  = true
    },
  ]

  delivery_config = {
    slack = {
      channel_id = "C0123456789"
    }
  }
}
