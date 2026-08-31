// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/komodorio/terraform-provider-agentops/internal/client"
	"github.com/komodorio/terraform-provider-agentops/internal/client/gen"
)

// Ensure the resource satisfies the framework interfaces.
var (
	_ resource.Resource                = &incidentPipelineSlackTriggerResource{}
	_ resource.ResourceWithConfigure   = &incidentPipelineSlackTriggerResource{}
	_ resource.ResourceWithImportState = &incidentPipelineSlackTriggerResource{}
)

// slackConnectorPrerequisite is the hint appended to a refused create. The
// control plane's own message for the missing connector is a garbled serializer
// error, so without this the operator has nothing actionable to go on.
const slackConnectorPrerequisite = "An incident pipeline can only be triggered from Slack on an account with an active Slack connector. " +
	"Connect Slack (Settings -> Integrations) before declaring this resource; a control plane without one refuses the create, " +
	"in some versions with an unhelpful serializer message rather than a description of the precondition."

// NewIncidentPipelineSlackTriggerResource is the constructor registered with the provider.
func NewIncidentPipelineSlackTriggerResource() resource.Resource {
	return &incidentPipelineSlackTriggerResource{}
}

type incidentPipelineSlackTriggerResource struct {
	client *client.Client
}

// incidentPipelineSlackTriggerResourceModel maps the
// agentops_incident_pipeline_slack_trigger schema to Go.
type incidentPipelineSlackTriggerResourceModel struct {
	ID         types.String         `tfsdk:"id"`
	PipelineID types.String         `tfsdk:"pipeline_id"`
	ChannelID  types.String         `tfsdk:"channel_id"`
	RuleType   types.String         `tfsdk:"rule_type"`
	Match      jsontypes.Normalized `tfsdk:"match"`
	IsEnabled  types.Bool           `tfsdk:"is_enabled"`
}

func (r *incidentPipelineSlackTriggerResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_incident_pipeline_slack_trigger"
}

func (r *incidentPipelineSlackTriggerResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A Slack channel that starts an `agentops_incident_pipeline`. Slack activity in the channel " +
			"matching `rule_type` opens an incident on the pipeline instead of waiting for an alert from its webhook.\n\n" +
			"~> **An active Slack connector is a prerequisite.** The route is created against the account's Slack " +
			"connector, so connect Slack before declaring this resource. On an account without one the create is " +
			"refused, in some control-plane versions with an unhelpful serializer message rather than a description " +
			"of the precondition.\n\n" +
			"The API has no update for a Slack trigger, so changing any argument replaces the route.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Route identifier (the API's `route_id`).",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"pipeline_id": schema.StringAttribute{
				MarkdownDescription: "ID of the incident pipeline this trigger starts. Changing this forces a new trigger.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"channel_id": schema.StringAttribute{
				MarkdownDescription: "Slack channel ID the trigger listens on (e.g. `C0123ABCDEF`), as it appears in the " +
					"account's Slack connector. Changing this forces a new trigger.",
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"rule_type": schema.StringAttribute{
				MarkdownDescription: "Match rule type. One of `private`, `channel`, `keyword`, `mention`. Defaults to " +
					"`mention`. Changing this forces a new trigger.",
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"match": schema.StringAttribute{
				MarkdownDescription: "Match criteria for the rule, as a JSON object — a `keyword` rule reads its keyword " +
					"from here. Changing this forces a new trigger.",
				CustomType: jsontypes.NormalizedType{},
				Optional:   true,
				Computed:   true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"is_enabled": schema.BoolAttribute{
				MarkdownDescription: "Whether the trigger is enabled. Set by the control plane; the create API takes no " +
					"enabled flag and there is no update.",
				Computed:      true,
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
		},
	}
}

func (r *incidentPipelineSlackTriggerResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromProviderData(req.ProviderData, &resp.Diagnostics)
}

func (r *incidentPipelineSlackTriggerResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan incidentPipelineSlackTriggerResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := gen.CreateSlackTriggerRequest{
		ChannelId: plan.ChannelID.ValueString(),
		RuleType:  stringToPtr(plan.RuleType),
	}
	resp.Diagnostics.Append(jsonToMapPtr(plan.Match, &body.MatchJson)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// client.Do, not the typed wrapper: the missing-connector refusal is a 422 with
	// a plain-string detail, which the wrapper drops on the floor. See client.Do.
	raw, err := client.Do(r.client.Gen.IncidentPipelinesCreateSlackTriggerEndpoint(ctx, plan.PipelineID.ValueString(), body))
	if err != nil {
		resp.Diagnostics.AddError("Error creating incident pipeline Slack trigger", slackTriggerCreateDetail(err))
		return
	}
	var info gen.SlackTriggerInfo
	if err := json.Unmarshal(raw, &info); err != nil {
		resp.Diagnostics.AddError("Error creating incident pipeline Slack trigger",
			"API returned a body that is not a Slack trigger: "+err.Error())
		return
	}

	slackTriggerApply(&plan, &info)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *incidentPipelineSlackTriggerResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state incidentPipelineSlackTriggerResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Slack triggers have no get-by-id endpoint; list the pipeline's and match id.
	// A deleted pipeline 404s the listing, which is this route gone too.
	raw, err := client.Do(r.client.Gen.IncidentPipelinesListSlackTriggersEndpoint(ctx, state.PipelineID.ValueString()))
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading incident pipeline Slack trigger", err.Error())
		return
	}
	var routes []gen.SlackTriggerInfo
	if err := json.Unmarshal(raw, &routes); err != nil {
		resp.Diagnostics.AddError("Error reading incident pipeline Slack trigger",
			"API returned a body that is not a Slack trigger list: "+err.Error())
		return
	}

	id := state.ID.ValueString()
	for i := range routes {
		if routes[i].RouteId == id {
			slackTriggerApply(&state, &routes[i])
			resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
			return
		}
	}
	resp.State.RemoveResource(ctx)
}

// Update exists only to satisfy the interface: every configurable argument is
// RequiresReplace because the API has no update for a Slack trigger.
func (r *incidentPipelineSlackTriggerResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan incidentPipelineSlackTriggerResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *incidentPipelineSlackTriggerResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state incidentPipelineSlackTriggerResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// A 404 is either route or pipeline already gone; both mean there is nothing
	// left to delete.
	_, err := client.Do(r.client.Gen.IncidentPipelinesDeleteSlackTriggerEndpoint(ctx,
		state.PipelineID.ValueString(), state.ID.ValueString()))
	if err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Error deleting incident pipeline Slack trigger", err.Error())
	}
}

// ImportState accepts "<pipeline_id>/<route_id>", the same shape
// agentops_channel_route uses, since a trigger is scoped to its pipeline.
func (r *incidentPipelineSlackTriggerResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(req.ID, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError("Invalid import ID",
			fmt.Sprintf("Expected import ID in the form \"pipeline_id/route_id\", got %q.", req.ID))
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("pipeline_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), parts[1])...)
}

// slackTriggerCreateDetail annotates a refused create with the connector
// prerequisite. 409 is the precondition's own status; a 422 is either the same
// precondition on a control plane that predates that fix, or a body the endpoint
// rejected — the hint is worded so it is safe to show for both.
func slackTriggerCreateDetail(err error) string {
	if client.IsConflict(err) || client.IsUnprocessable(err) {
		return err.Error() + "\n\n" + slackConnectorPrerequisite
	}
	return err.Error()
}

// slackTriggerApply writes a SlackTriggerInfo into the model.
func slackTriggerApply(m *incidentPipelineSlackTriggerResourceModel, route *gen.SlackTriggerInfo) {
	m.ID = types.StringValue(route.RouteId)
	m.ChannelID = types.StringValue(route.ChannelId)
	m.RuleType = types.StringValue(route.RuleType)
	m.Match = mapPtrToJSON(route.MatchJson)
	m.IsEnabled = types.BoolValue(route.IsEnabled)
}
