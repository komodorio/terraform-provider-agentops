// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"encoding/json"
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/komodorio/terraform-provider-agentops/internal/client/gen"
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
					checkSlackTriggerOnControlPlane(mock, "C0123ABCDEF", "mention", ""),
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

// TestAccIncidentPipelineSlackTriggerResource_replaceOnArgChange is the update
// path: the API has no update for a Slack trigger, so a changed argument has to
// tear the route down and create a new one. Each step changes exactly one
// argument, so a RequiresReplace missing from one of them cannot hide behind
// another. Asserting the count is what catches a replace that leaves the old
// route behind on the control plane.
func TestAccIncidentPipelineSlackTriggerResource_replaceOnArgChange(t *testing.T) {
	mock := newMockServer(t)
	var pipelineID, afterRule, afterMatch, afterKeyword string

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccSlackTriggerConfig(mock.URL,
					"  channel_id = \"C0123ABCDEF\"\n  rule_type  = \"mention\"\n"),
				Check: resource.ComposeAggregateTestCheckFunc(
					captureSlackTriggerPipelineID(&pipelineID),
					captureSlackTriggerID(&afterRule),
					checkSlackTriggerCount(mock, &pipelineID, 1),
					checkSlackTriggerOnControlPlane(mock, "C0123ABCDEF", "mention", ""),
				),
			},
			{
				// rule_type alone.
				Config: testAccSlackTriggerConfig(mock.URL,
					"  channel_id = \"C0123ABCDEF\"\n  rule_type  = \"channel\"\n"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(slackTriggerRes, "rule_type", "channel"),
					checkSlackTriggerReplaced(&afterRule),
					checkSlackTriggerCount(mock, &pipelineID, 1),
					checkSlackTriggerOnControlPlane(mock, "C0123ABCDEF", "channel", ""),
					captureSlackTriggerID(&afterMatch),
				),
			},
			{
				// match alone, on a rule that reads it.
				Config: testAccSlackTriggerConfig(mock.URL,
					"  channel_id = \"C0123ABCDEF\"\n  rule_type  = \"keyword\"\n  match      = jsonencode({ keyword = \"oom\" })\n"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(slackTriggerRes, "match", `{"keyword":"oom"}`),
					checkSlackTriggerReplaced(&afterMatch),
					checkSlackTriggerCount(mock, &pipelineID, 1),
					checkSlackTriggerOnControlPlane(mock, "C0123ABCDEF", "keyword", `{"keyword":"oom"}`),
					captureSlackTriggerID(&afterKeyword),
				),
			},
			{
				// A different keyword under the same rule_type.
				Config: testAccSlackTriggerConfig(mock.URL,
					"  channel_id = \"C0123ABCDEF\"\n  rule_type  = \"keyword\"\n  match      = jsonencode({ keyword = \"crashloop\" })\n"),
				Check: resource.ComposeAggregateTestCheckFunc(
					checkSlackTriggerReplaced(&afterKeyword),
					checkSlackTriggerCount(mock, &pipelineID, 1),
					checkSlackTriggerOnControlPlane(mock, "C0123ABCDEF", "keyword", `{"keyword":"crashloop"}`),
				),
			},
			{
				// channel_id alone.
				Config: testAccSlackTriggerConfig(mock.URL,
					"  channel_id = \"C9999ZZZZZZ\"\n  rule_type  = \"keyword\"\n  match      = jsonencode({ keyword = \"crashloop\" })\n"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(slackTriggerRes, "channel_id", "C9999ZZZZZZ"),
					checkSlackTriggerCount(mock, &pipelineID, 1),
					checkSlackTriggerOnControlPlane(mock, "C9999ZZZZZZ", "keyword", `{"keyword":"crashloop"}`),
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

// TestAccIncidentPipelineSlackTriggerResource_matchDropsInjectedChannels is the
// superset guard. The control plane stores match_json as the configured object
// plus a channels key holding the channel_id argument, and answers with that
// superset — so writing the response back verbatim puts a value in state that the
// configuration never asked for, which Terraform rejects as an inconsistent apply
// result. It is the documented keyword case, so the resource is unusable without
// this. The route on the control plane is asserted too: the key is dropped from
// state, not from the request.
func TestAccIncidentPipelineSlackTriggerResource_matchDropsInjectedChannels(t *testing.T) {
	mock := newMockServer(t)
	const config = "  channel_id = \"C0123ABCDEF\"\n  rule_type  = \"keyword\"\n  match      = jsonencode({ keyword = \"oom\" })\n"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccSlackTriggerConfig(mock.URL, config),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(slackTriggerRes, "match", `{"keyword":"oom"}`),
					checkSlackTriggerOnControlPlane(mock, "C0123ABCDEF", "keyword", `{"keyword":"oom"}`),
					checkSlackTriggerStoredChannels(mock, "C0123ABCDEF"),
				),
			},
			{
				// The refresh reads the same superset out of the listing, so a strip
				// applied only on create would plan a diff here, for good.
				Config:   testAccSlackTriggerConfig(mock.URL, config),
				PlanOnly: true,
			},
			{
				ResourceName:      slackTriggerRes,
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateIdFunc: slackTriggerImportID,
			},
		},
	})
}

// TestAccIncidentPipelineSlackTriggerResource_matchKeepsDeclaredChannels covers a
// configuration that names channels itself. The control plane overwrites it with
// [channel_id] either way, so only the key the control plane derived is dropped:
// a value the configuration declared stays in state, where a diff against it is
// the honest report.
func TestAccIncidentPipelineSlackTriggerResource_matchKeepsDeclaredChannels(t *testing.T) {
	mock := newMockServer(t)
	const config = "  channel_id = \"C0123ABCDEF\"\n  rule_type  = \"channel\"\n" +
		"  match      = jsonencode({ channels = [\"C0123ABCDEF\"] })\n"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccSlackTriggerConfig(mock.URL, config),
				Check: resource.TestCheckResourceAttr(slackTriggerRes,
					"match", `{"channels":["C0123ABCDEF"]}`),
			},
			{
				Config:   testAccSlackTriggerConfig(mock.URL, config),
				PlanOnly: true,
			},
		},
	})
}

// checkSlackTriggerStoredChannels asserts the control plane still holds the
// channels key the provider drops from state, so the test cannot pass by never
// having been sent one.
func checkSlackTriggerStoredChannels(mock *mockServer, wantChannel string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		trigger, err := slackTriggerState(s)
		if err != nil {
			return err
		}
		route := mock.slackTriggerRoutes(trigger.Primary.Attributes["pipeline_id"])[trigger.Primary.ID]
		stored, _ := route["match_json"].(map[string]any)
		channels, err := json.Marshal(stored["channels"])
		if err != nil {
			return err
		}
		if want := fmt.Sprintf("[%q]", wantChannel); string(channels) != want {
			return fmt.Errorf("control plane holds match_json.channels %s, want %s", channels, want)
		}
		return nil
	}
}

// TestAccIncidentPipelineSlackTriggerResource_requiresSlackConnector covers the
// account with no Slack connector, on both statuses the refusal comes back as: the
// 409 a current control plane answers, and the 422 an older one answers with a
// garbled serializer body in place of the precondition. Either way the diagnostic
// has to carry the prerequisite itself, so the regexes assert the hint and not
// just the control plane's own words.
func TestAccIncidentPipelineSlackTriggerResource_requiresSlackConnector(t *testing.T) {
	for name, tc := range map[string]struct {
		legacy bool
		expect *regexp.Regexp
	}{
		"conflict":   {expect: regexp.MustCompile(`(?s)HTTP 409.*Connect Slack`)},
		"legacy_422": {legacy: true, expect: regexp.MustCompile(`(?s)HTTP 422.*Connect Slack`)},
	} {
		t.Run(name, func(t *testing.T) {
			mock := newMockServer(t)
			mock.noSlackConnector = true
			mock.legacySlackRefusal = tc.legacy

			resource.Test(t, resource.TestCase{
				PreCheck:                 func() { testAccPreCheck(t) },
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				Steps: []resource.TestStep{
					{
						Config:      testAccSlackTriggerConfig(mock.URL, "  channel_id = \"C0123ABCDEF\"\n"),
						ExpectError: tc.expect,
					},
				},
			})
		})
	}
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
// the control plane actually stored, field for field. wantMatch is the match_json
// the configuration asked for, as compact JSON, or "" for a route that carries
// none: the stored object also holds the channels the control plane injects, which
// is asserted separately rather than written into every expectation.
func checkSlackTriggerOnControlPlane(mock *mockServer, wantChannel, wantRule, wantMatch string) resource.TestCheckFunc {
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
		stored, ok := route["match_json"].(map[string]any)
		if !ok {
			return fmt.Errorf("route %s has match_json %v, want an object", trigger.Primary.ID, route["match_json"])
		}
		channels, err := json.Marshal(stored["channels"])
		if err != nil {
			return err
		}
		if want := fmt.Sprintf("[%q]", wantChannel); string(channels) != want {
			return fmt.Errorf("route %s has match_json.channels %s, want %s", trigger.Primary.ID, channels, want)
		}
		declared := map[string]any{}
		for k, v := range stored {
			if k != "channels" {
				declared[k] = v
			}
		}
		match, err := json.Marshal(declared)
		if err != nil {
			return err
		}
		got := string(match)
		if got == "{}" {
			got = ""
		}
		if got != wantMatch {
			return fmt.Errorf("route %s has match_json %s, want %q besides channels", trigger.Primary.ID, match, wantMatch)
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

// TestSlackTriggerMatch covers every shape match_json can come back in. Two of them
// no acceptance step can reach through the API: the control plane derives channels
// from channel_id unconditionally, so it never answers a channels it did not derive,
// and never answers an empty object to a configuration that declared one.
func TestSlackTriggerMatch(t *testing.T) {
	const channel = "C0123ABCDEF"

	for name, tc := range map[string]struct {
		configured jsontypes.Normalized
		matchJSON  map[string]interface{}
		want       jsontypes.Normalized
	}{
		"match unset, only the injected key": {
			configured: jsontypes.NewNormalizedUnknown(),
			matchJSON:  map[string]interface{}{"channels": []interface{}{channel}},
			want:       jsontypes.NewNormalizedNull(),
		},
		"configured match plus the injected key": {
			configured: jsontypes.NewNormalizedValue(`{"keyword":"oom"}`),
			matchJSON:  map[string]interface{}{"keyword": "oom", "channels": []interface{}{channel}},
			want:       jsontypes.NewNormalizedValue(`{"keyword":"oom"}`),
		},
		"channels the configuration declared itself": {
			configured: jsontypes.NewNormalizedValue(`{"channels":["C0123ABCDEF"]}`),
			matchJSON:  map[string]interface{}{"channels": []interface{}{channel}},
			want:       jsontypes.NewNormalizedValue(`{"channels":["C0123ABCDEF"]}`),
		},
		"channels not derived from channel_id is reported, not dropped": {
			configured: jsontypes.NewNormalizedValue(`{"keyword":"oom"}`),
			matchJSON:  map[string]interface{}{"keyword": "oom", "channels": []interface{}{"C9999ZZZZZZ", channel}},
			want:       jsontypes.NewNormalizedValue(`{"channels":["C9999ZZZZZZ","C0123ABCDEF"],"keyword":"oom"}`),
		},
		"an empty object the configuration set explicitly survives": {
			configured: jsontypes.NewNormalizedValue(`{}`),
			matchJSON:  map[string]interface{}{"channels": []interface{}{channel}},
			want:       jsontypes.NewNormalizedValue(`{}`),
		},
		"no match_json at all": {
			configured: jsontypes.NewNormalizedNull(),
			want:       jsontypes.NewNormalizedNull(),
		},
	} {
		t.Run(name, func(t *testing.T) {
			route := &gen.SlackTriggerInfo{ChannelId: channel, RuleType: "keyword"}
			if tc.matchJSON != nil {
				route.MatchJson = &tc.matchJSON
			}
			if got := slackTriggerMatch(tc.configured, route); !got.Equal(tc.want) {
				t.Errorf("slackTriggerMatch() = %s, want %s", got, tc.want)
			}
		})
	}
}
