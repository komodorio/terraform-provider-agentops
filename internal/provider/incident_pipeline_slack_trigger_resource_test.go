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

const slackTriggerRes = "agentops_incident_pipeline_slack_trigger.test"

// TestAccIncidentPipelineSlackTriggerResource covers create with the API's
// default rule, import as "pipeline_id/route_id", and destroy. The route is
// asserted on the control plane, not only in state, because the resource's only
// read is the pipeline-scoped listing.
func TestAccIncidentPipelineSlackTriggerResource(t *testing.T) {
	mock := newMockServer(t)
	var pipelineID string

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccSlackTriggerConfig(mock.URL, "  channel_id = \"C0123ABCDEF\"\n"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(slackTriggerRes, "id"),
					resource.TestCheckResourceAttr(slackTriggerRes, "channel_id", "C0123ABCDEF"),
					// The create request omits rule_type, so "mention" can only come
					// from the response being read back into state.
					resource.TestCheckResourceAttr(slackTriggerRes, "rule_type", "mention"),
					resource.TestCheckResourceAttr(slackTriggerRes, "is_enabled", "true"),
					resource.TestCheckNoResourceAttr(slackTriggerRes, "match"),
					captureSlackTriggerPipelineID(&pipelineID),
					checkSlackTriggerCount(mock, &pipelineID, 1),
					checkSlackTriggerOnControlPlane(mock, "C0123ABCDEF", "mention"),
				),
			},
			{
				ResourceName:      slackTriggerRes,
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateIdFunc: slackTriggerImportID,
			},
			{
				// The trigger dropped from the config while its pipeline stays. The
				// route has to be deleted on the control plane, not just forgotten.
				Config: testAccSlackTriggerPipelineOnlyConfig(mock.URL),
				Check:  checkSlackTriggerCount(mock, &pipelineID, 0),
			},
		},
	})
}

// TestAccIncidentPipelineSlackTriggerResource_replaceOnRuleChange is the
// update path: the API has no update for a Slack trigger, so a changed argument
// has to tear the route down and create a new one. Asserting the count is what
// catches a replace that leaves the old route behind on the control plane.
func TestAccIncidentPipelineSlackTriggerResource_replaceOnRuleChange(t *testing.T) {
	mock := newMockServer(t)
	var first, pipelineID string

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccSlackTriggerConfig(mock.URL,
					"  channel_id = \"C0123ABCDEF\"\n  rule_type  = \"mention\"\n"),
				Check: resource.ComposeAggregateTestCheckFunc(
					captureSlackTriggerID(&first),
					captureSlackTriggerPipelineID(&pipelineID),
					checkSlackTriggerCount(mock, &pipelineID, 1),
				),
			},
			{
				Config: testAccSlackTriggerConfig(mock.URL,
					"  channel_id = \"C0123ABCDEF\"\n  rule_type  = \"keyword\"\n  match      = jsonencode({ keyword = \"oom\" })\n"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(slackTriggerRes, "rule_type", "keyword"),
					resource.TestCheckResourceAttr(slackTriggerRes, "match", "{\"keyword\":\"oom\"}"),
					checkSlackTriggerReplaced(&first),
					checkSlackTriggerCount(mock, &pipelineID, 1),
					checkSlackTriggerOnControlPlane(mock, "C0123ABCDEF", "keyword"),
				),
			},
			{
				Config: testAccSlackTriggerConfig(mock.URL,
					"  channel_id = \"C9999ZZZZZZ\"\n  rule_type  = \"keyword\"\n  match      = jsonencode({ keyword = \"oom\" })\n"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(slackTriggerRes, "channel_id", "C9999ZZZZZZ"),
					checkSlackTriggerCount(mock, &pipelineID, 1),
					checkSlackTriggerOnControlPlane(mock, "C9999ZZZZZZ", "keyword"),
				),
			},
		},
	})
}

// TestAccIncidentPipelineSlackTriggerResource_unsetIsStable is the
// perpetual-diff guard: rule_type and match are Optional-and-Computed, so a
// config that declares neither must plan clean once the response has filled them
// in.
func TestAccIncidentPipelineSlackTriggerResource_unsetIsStable(t *testing.T) {
	mock := newMockServer(t)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccSlackTriggerConfig(mock.URL, "  channel_id = \"C0123ABCDEF\"\n"),
			},
			{
				Config:   testAccSlackTriggerConfig(mock.URL, "  channel_id = \"C0123ABCDEF\"\n"),
				PlanOnly: true,
			},
		},
	})
}

// TestAccIncidentPipelineSlackTriggerResource_recreatesAfterOutOfBandDelete
// covers a route removed behind Terraform's back. The listing simply stops
// carrying it, which the refresh has to read as "gone" rather than as an error or
// as no change at all.
func TestAccIncidentPipelineSlackTriggerResource_recreatesAfterOutOfBandDelete(t *testing.T) {
	mock := newMockServer(t)
	const config = "  channel_id = \"C0123ABCDEF\"\n"
	var pipelineID, routeID string

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccSlackTriggerConfig(mock.URL, config),
				Check: resource.ComposeAggregateTestCheckFunc(
					captureSlackTriggerPipelineID(&pipelineID),
					captureSlackTriggerID(&routeID),
				),
			},
			{
				// Between the two applies, not inside step 1: a Check runs before the
				// step's own refresh plan, which would then see the drift and fail.
				PreConfig: func() { mock.deleteSlackTriggerOutOfBand(pipelineID, routeID) },
				Config:    testAccSlackTriggerConfig(mock.URL, config),
				Check: resource.ComposeAggregateTestCheckFunc(
					checkSlackTriggerCount(mock, &pipelineID, 1),
					checkSlackTriggerReplaced(&routeID),
				),
			},
		},
	})
}

// TestAccIncidentPipelineSlackTriggerResource_requiresSlackConnector covers the
// account with no Slack connector. The control plane's own message for it is a
// garbled serializer error, so the diagnostic has to carry the prerequisite
// itself or the operator has nothing to act on.
func TestAccIncidentPipelineSlackTriggerResource_requiresSlackConnector(t *testing.T) {
	mock := newMockServer(t)
	mock.noSlackConnector = true

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccSlackTriggerConfig(mock.URL, "  channel_id = \"C0123ABCDEF\"\n"),
				ExpectError: regexp.MustCompile(`active Slack connector`),
			},
		},
	})
}

// TestAccIncidentPipelineSlackTriggerResource_badImportID rejects an import ID
// that does not name both halves, rather than silently importing a route with no
// pipeline.
func TestAccIncidentPipelineSlackTriggerResource_badImportID(t *testing.T) {
	mock := newMockServer(t)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccSlackTriggerConfig(mock.URL, "  channel_id = \"C0123ABCDEF\"\n"),
			},
			{
				ResourceName:  slackTriggerRes,
				ImportState:   true,
				ImportStateId: "slr_1",
				ExpectError:   regexp.MustCompile(`Expected import ID in the form "pipeline_id/route_id"`),
			},
		},
	})
}

// checkSlackTriggerCount asserts how many routes the control plane holds for the
// pipeline under test, which is the only way to see a replace that failed to
// clean up after itself.
// It takes the pipeline id by pointer rather than reading it out of state, so a
// step whose config no longer declares the trigger — a destroy — can still assert
// the route is gone upstream and not merely dropped from state.
func checkSlackTriggerCount(mock *mockServer, pipelineID *string, want int) resource.TestCheckFunc {
	return func(_ *terraform.State) error {
		got := mock.slackTriggerRoutes(*pipelineID)
		if len(got) != want {
			return fmt.Errorf("control plane holds %d Slack trigger routes for pipeline %s, want %d",
				len(got), *pipelineID, want)
		}
		return nil
	}
}

// checkSlackTriggerOnControlPlane asserts the route Terraform recorded is the one
// the control plane actually stored, field for field.
func checkSlackTriggerOnControlPlane(mock *mockServer, wantChannel, wantRule string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		trigger, err := slackTriggerState(s)
		if err != nil {
			return err
		}
		routes := mock.slackTriggerRoutes(trigger.Primary.Attributes["pipeline_id"])
		route, ok := routes[trigger.Primary.ID]
		if !ok {
			return fmt.Errorf("route %s is in state but not on the control plane", trigger.Primary.ID)
		}
		if got := toString(route["channel_id"]); got != wantChannel {
			return fmt.Errorf("route %s is on channel %q, want %q", trigger.Primary.ID, got, wantChannel)
		}
		if got := toString(route["rule_type"]); got != wantRule {
			return fmt.Errorf("route %s has rule_type %q, want %q", trigger.Primary.ID, got, wantRule)
		}
		return nil
	}
}

// checkSlackTriggerReplaced asserts the route in state is not the one previously
// captured, i.e. that the step created a new one rather than reusing it.
func checkSlackTriggerReplaced(previous *string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		trigger, err := slackTriggerState(s)
		if err != nil {
			return err
		}
		if trigger.Primary.ID == *previous {
			return fmt.Errorf("route %s was reused, want a new route", trigger.Primary.ID)
		}
		return nil
	}
}

func captureSlackTriggerID(out *string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		trigger, err := slackTriggerState(s)
		if err != nil {
			return err
		}
		*out = trigger.Primary.ID
		return nil
	}
}

func captureSlackTriggerPipelineID(out *string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		trigger, err := slackTriggerState(s)
		if err != nil {
			return err
		}
		*out = trigger.Primary.Attributes["pipeline_id"]
		return nil
	}
}

// slackTriggerImportID renders the composite "pipeline_id/route_id" the resource
// imports by.
func slackTriggerImportID(s *terraform.State) (string, error) {
	trigger, err := slackTriggerState(s)
	if err != nil {
		return "", err
	}
	return trigger.Primary.Attributes["pipeline_id"] + "/" + trigger.Primary.ID, nil
}

func slackTriggerState(s *terraform.State) (*terraform.ResourceState, error) {
	trigger, ok := s.RootModule().Resources[slackTriggerRes]
	if !ok {
		return nil, fmt.Errorf("%s not found in state", slackTriggerRes)
	}
	return trigger, nil
}

// testAccSlackTriggerConfig builds a draft pipeline plus one Slack trigger on it.
// The pipeline stays draft so the test never has to pause it before teardown.
func testAccSlackTriggerConfig(endpoint, args string) string {
	return testAccSlackTriggerPipelineOnlyConfig(endpoint) + fmt.Sprintf(`
resource "agentops_incident_pipeline_slack_trigger" "test" {
  pipeline_id = agentops_incident_pipeline.test.id
%s}
`, args)
}

// testAccSlackTriggerPipelineOnlyConfig is the same pipeline with no trigger, so a
// step can destroy the trigger alone.
func testAccSlackTriggerPipelineOnlyConfig(endpoint string) string {
	return mockProviderConfig(endpoint) + `
resource "agentops_incident_pipeline" "test" {
  name = "prod-incidents"

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
}
`
}
