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

// TestAccMCPGatewayServerResource_credentialBinding walks the whole binding
// lifecycle: bind on create, rebind to a second credential, rebind across the
// source discriminator, then unbind by removing the attribute. Nothing the server
// read returns carries the binding, so every step fails if the plan value is not
// what drives the decision.
func TestAccMCPGatewayServerResource_credentialBinding(t *testing.T) {
	mock := newMockServer(t)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccServerCredentialConfig(mock.URL, "  credential_source_id = agentops_credential.a.id\n"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair("agentops_mcp_gateway_server.test", "credential_source_id", "agentops_credential.a", "id"),
					resource.TestCheckNoResourceAttr("agentops_mcp_gateway_server.test", "credential_source"),
					checkServerCredential(mock, "credential", "agentops_credential.a"),
				),
			},
			{
				Config: testAccServerCredentialConfig(mock.URL, "  credential_source_id = agentops_credential.b.id\n"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair("agentops_mcp_gateway_server.test", "credential_source_id", "agentops_credential.b", "id"),
					checkServerCredential(mock, "credential", "agentops_credential.b"),
				),
			},
			{
				Config: testAccServerCredentialConfig(mock.URL,
					"  credential_source    = \"integration\"\n  credential_source_id = agentops_integration_connection.c.id\n"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("agentops_mcp_gateway_server.test", "credential_source", "integration"),
					resource.TestCheckResourceAttrPair("agentops_mcp_gateway_server.test", "credential_source_id", "agentops_integration_connection.c", "id"),
					checkServerCredential(mock, "integration", "agentops_integration_connection.c"),
				),
			},
			{
				Config: testAccServerCredentialConfig(mock.URL, ""),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckNoResourceAttr("agentops_mcp_gateway_server.test", "credential_source_id"),
					resource.TestCheckNoResourceAttr("agentops_mcp_gateway_server.test", "credential_source"),
					checkServerCredential(mock, "", ""),
				),
			},
		},
	})
}

// TestAccMCPGatewayServerResource_credentialSurvivesUnrelatedUpdate is the
// destructive case: the update response carries no credential at all, so reading
// the binding off it instead of off the plan unbinds one the config still asks
// for. The plain server read is just as blind, so the refresh in between must not
// drop it either.
func TestAccMCPGatewayServerResource_credentialSurvivesUnrelatedUpdate(t *testing.T) {
	mock := newMockServer(t)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccServerCredentialConfigURL(mock.URL, "  credential_source_id = agentops_credential.a.id\n", "https://mcp.example.com"),
				Check:  checkServerCredential(mock, "credential", "agentops_credential.a"),
			},
			{
				Config: testAccServerCredentialConfigURL(mock.URL, "  credential_source_id = agentops_credential.a.id\n", "https://mcp-moved.example.com"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("agentops_mcp_gateway_server.test", "url", "https://mcp-moved.example.com"),
					resource.TestCheckResourceAttrPair("agentops_mcp_gateway_server.test", "credential_source_id", "agentops_credential.a", "id"),
					checkServerCredential(mock, "credential", "agentops_credential.a"),
				),
			},
		},
	})
}

// TestAccMCPGatewayServerResource_credentialUnsetIsStable is the perpetual-diff
// guard: a server that never declared a credential must plan clean on refresh,
// even with someone else's binding sitting in the account-wide listing.
func TestAccMCPGatewayServerResource_credentialUnsetIsStable(t *testing.T) {
	mock := newMockServer(t)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccServerCredentialConfig(mock.URL, ""),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckNoResourceAttr("agentops_mcp_gateway_server.test", "credential_source_id"),
					bindServerCredentialOutOfBand(mock, "agentops_credential.a"),
				),
			},
			{
				Config:   testAccServerCredentialConfig(mock.URL, ""),
				PlanOnly: true,
			},
		},
	})
}

// TestAccMCPGatewayServerResource_credentialDriftPlansRebind is the other half of
// the reconcile contract: once the config owns a binding, one removed out of band
// has to come back rather than hide behind stale state.
func TestAccMCPGatewayServerResource_credentialDriftPlansRebind(t *testing.T) {
	mock := newMockServer(t)
	const config = "  credential_source_id = agentops_credential.a.id\n"
	var serverID string

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccServerCredentialConfig(mock.URL, config),
				Check: resource.ComposeAggregateTestCheckFunc(
					checkServerCredential(mock, "credential", "agentops_credential.a"),
					captureServerID(&serverID),
				),
			},
			{
				// Between the two applies, not inside step 1: a Check runs before the
				// step's own refresh plan, which would then see the drift and fail.
				PreConfig: func() { mock.unbindServerCredential(serverID) },
				Config:    testAccServerCredentialConfig(mock.URL, config),
				Check:     checkServerCredential(mock, "credential", "agentops_credential.a"),
			},
		},
	})
}

// TestAccMCPGatewayServerResource_credentialUnbindToleratesAbsentBinding covers
// the unbind whose binding is already gone. DELETE .../credential answers 404 for
// a server with no binding — the route raises on an UPDATE that matches no
// unbound-to-bound row — so an attribute removed from the configuration after
// someone detached the binding in the UI fails the apply unless that 404 reads as
// success. The tolerance is the whole test: nothing else exercises it.
func TestAccMCPGatewayServerResource_credentialUnbindToleratesAbsentBinding(t *testing.T) {
	mock := newMockServer(t)
	// Set up front rather than in a PreConfig: the mock reads it from the request
	// goroutine, and step 1 issues no DELETE for it to change the answer to.
	mock.credentialUnbindRace = true

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccServerCredentialConfig(mock.URL, "  credential_source_id = agentops_credential.a.id\n"),
				Check:  checkServerCredential(mock, "credential", "agentops_credential.a"),
			},
			{
				Config: testAccServerCredentialConfig(mock.URL, ""),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckNoResourceAttr("agentops_mcp_gateway_server.test", "credential_source_id"),
					checkServerCredential(mock, "", ""),
				),
			},
		},
	})
}

// captureServerID records the server's id for a later step's PreConfig, which runs
// with no access to state.
func captureServerID(out *string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		server, ok := s.RootModule().Resources["agentops_mcp_gateway_server.test"]
		if !ok {
			return fmt.Errorf("agentops_mcp_gateway_server.test not found in state")
		}
		*out = server.Primary.ID
		return nil
	}
}

// TestAccMCPGatewayServerResource_credentialBindFailureKeepsServer covers the
// create whose follow-up bind is refused. The POST already registered the server,
// so the apply has to leave it in state — tainted — or the record is orphaned and
// the next apply registers a second one alongside it.
func TestAccMCPGatewayServerResource_credentialBindFailureKeepsServer(t *testing.T) {
	mock := newMockServer(t)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccServerCredentialConfig(mock.URL, "  credential_source_id = \"cred_does_not_exist\"\n"),
				ExpectError: regexp.MustCompile("Error binding credential to MCP gateway server"),
			},
			{
				Config: testAccServerCredentialConfig(mock.URL, "  credential_source_id = agentops_credential.a.id\n"),
				Check: resource.ComposeAggregateTestCheckFunc(
					checkServerCredential(mock, "credential", "agentops_credential.a"),
					checkMCPServerCount(mock, 1),
				),
			},
		},
	})
}

// TestAccMCPGatewayServerResource_credentialSourceNeedsID rejects the one config
// the API has no call for, at plan time.
func TestAccMCPGatewayServerResource_credentialSourceNeedsID(t *testing.T) {
	mock := newMockServer(t)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccServerCredentialConfig(mock.URL, "  credential_source = \"integration\"\n"),
				ExpectError: regexp.MustCompile("Missing credential_source_id"),
			},
			{
				Config:      testAccServerCredentialConfig(mock.URL, "  credential_source_id = \"c\"\n  credential_source = \"vault\"\n"),
				ExpectError: regexp.MustCompile("Invalid credential_source"),
			},
		},
	})
}

// TestAccMCPGatewayServerResource_credentialRefusalIsReadable pins the shape of a
// refusal, not just its existence. This route answers an inactive integration
// connection with a 422 whose `detail` is a plain string, which is not the
// validation-error list the spec declares for 422 — read through the generated
// typed wrapper the whole response is discarded and the operator gets an opaque
// json unmarshal error with no status and no message. The API's own words have to
// reach the diagnostic.
func TestAccMCPGatewayServerResource_credentialRefusalIsReadable(t *testing.T) {
	mock := newMockServer(t)
	mock.integrationInactive = true

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccServerCredentialConfig(mock.URL,
					"  credential_source    = \"integration\"\n  credential_source_id = agentops_integration_connection.c.id\n"),
				ExpectError: regexp.MustCompile(`(?s)HTTP 422.*is not active`),
			},
		},
	})
}

// checkServerCredential asserts the binding the control plane holds for the server
// under test, not the one Terraform recorded. Pass empty strings to assert there
// is none.
func checkServerCredential(mock *mockServer, wantSource, sourceRes string) resource.TestCheckFunc {
	const serverRes = "agentops_mcp_gateway_server.test"
	return func(s *terraform.State) error {
		server, ok := s.RootModule().Resources[serverRes]
		if !ok {
			return fmt.Errorf("%s not found in state", serverRes)
		}
		wantID := ""
		if sourceRes != "" {
			src, ok := s.RootModule().Resources[sourceRes]
			if !ok {
				return fmt.Errorf("%s not found in state", sourceRes)
			}
			wantID = src.Primary.ID
		}
		gotSource, gotID := mock.serverCredentialBinding(server.Primary.ID)
		if gotSource != wantSource || gotID != wantID {
			return fmt.Errorf("server %s bound to (%q, %q) on the control plane, want (%q, %q)",
				server.Primary.ID, gotSource, gotID, wantSource, wantID)
		}
		return nil
	}
}

// bindServerCredentialOutOfBand attaches a credential the configuration never
// declared, standing in for one bound in the UI or by another owner.
func bindServerCredentialOutOfBand(mock *mockServer, sourceRes string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		server, ok := s.RootModule().Resources["agentops_mcp_gateway_server.test"]
		if !ok {
			return fmt.Errorf("agentops_mcp_gateway_server.test not found in state")
		}
		src, ok := s.RootModule().Resources[sourceRes]
		if !ok {
			return fmt.Errorf("%s not found in state", sourceRes)
		}
		mock.bindServerCredential(server.Primary.ID, "credential", src.Primary.ID)
		return nil
	}
}

func testAccServerCredentialConfig(endpoint, binding string) string {
	return testAccServerCredentialConfigURL(endpoint, binding, "https://mcp.example.com")
}

func testAccServerCredentialConfigURL(endpoint, binding, url string) string {
	return mockProviderConfig(endpoint) + fmt.Sprintf(`
resource "agentops_credential" "a" {
  name  = "grafana-a"
  value = "token-a"
}

resource "agentops_credential" "b" {
  name  = "grafana-b"
  value = "token-b"
}

resource "agentops_integration_connection" "c" {
  provider_key = "github"
  display_name = "GitHub"
  credentials  = { token = "t0ken" }
}

resource "agentops_mcp_gateway_server" "test" {
  name = "docs-mcp"
  url  = %q
%s}
`, url, binding)
}
