# MCP gateway servers are imported by their id.
terraform import agentops_mcp_gateway_server.docs srv_01hxyz

# An imported server carries no credential binding: the server record does not echo
# one, and a credential attribute the configuration has not set is deliberately left
# unmanaged rather than adopted. So a server that already has a binding imports with
# both attributes null, and the first plan after the import shows a re-bind of the
# credential the configuration names — an idempotent upsert of the binding that is
# already there, not a change to the upstream.
