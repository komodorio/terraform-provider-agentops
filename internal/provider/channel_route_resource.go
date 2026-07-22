// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
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
	_ resource.Resource                = &channelRouteResource{}
	_ resource.ResourceWithConfigure   = &channelRouteResource{}
	_ resource.ResourceWithImportState = &channelRouteResource{}
)

// NewChannelRouteResource is the constructor registered with the provider.
func NewChannelRouteResource() resource.Resource {
	return &channelRouteResource{}
}

type channelRouteResource struct {
	client *client.Client
}

// channelRouteResourceModel maps the agentops_channel_route schema to Go.
type channelRouteResourceModel struct {
	ID         types.String         `tfsdk:"id"`
	ChannelID  types.String         `tfsdk:"channel_id"`
	RuleType   types.String         `tfsdk:"rule_type"`
	TargetType types.String         `tfsdk:"target_type"`
	TargetID   types.String         `tfsdk:"target_id"`
	Priority   types.Int64          `tfsdk:"priority"`
	IsDefault  types.Bool           `tfsdk:"is_default"`
	IsEnabled  types.Bool           `tfsdk:"is_enabled"`
	Match      jsontypes.Normalized `tfsdk:"match"`
	Input      jsontypes.Normalized `tfsdk:"input"`
	AccountID  types.String         `tfsdk:"account_id"`
	CreatedAt  types.String         `tfsdk:"created_at"`
	UpdatedAt  types.String         `tfsdk:"updated_at"`
}

func (r *channelRouteResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_channel_route"
}

func (r *channelRouteResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A route on an `agentops_channel` that forwards matching events to a target agent or workflow.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Route identifier.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"channel_id": schema.StringAttribute{
				MarkdownDescription: "ID of the channel this route belongs to. Changing this forces a new route.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"rule_type": schema.StringAttribute{
				MarkdownDescription: "Match rule type. One of `private`, `channel`, `keyword`, `mention`.",
				Required:            true,
			},
			"target_type": schema.StringAttribute{
				MarkdownDescription: "Kind of target the route forwards to (e.g. `agent`, `workflow`).",
				Required:            true,
			},
			"target_id": schema.StringAttribute{
				MarkdownDescription: "ID of the target agent or workflow.",
				Required:            true,
			},
			"priority": schema.Int64Attribute{
				MarkdownDescription: "Evaluation priority; lower values are evaluated first.",
				Required:            true,
			},
			"is_default": schema.BoolAttribute{
				MarkdownDescription: "Whether this is the channel's default route.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"is_enabled": schema.BoolAttribute{
				MarkdownDescription: "Whether the route is enabled.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"match": schema.StringAttribute{
				MarkdownDescription: "Match criteria for the route, as a JSON object.",
				CustomType:          jsontypes.NormalizedType{},
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"input": schema.StringAttribute{
				MarkdownDescription: "Input payload template forwarded to the target, as a JSON object.",
				CustomType:          jsontypes.NormalizedType{},
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"account_id": schema.StringAttribute{
				MarkdownDescription: "Account that owns the route.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "Creation timestamp.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"updated_at": schema.StringAttribute{
				MarkdownDescription: "Last-update timestamp.",
				Computed:            true,
			},
		},
	}
}

func (r *channelRouteResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromProviderData(req.ProviderData, &resp.Diagnostics)
}

func (r *channelRouteResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan channelRouteResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := gen.CreateRouteRequest{
		RuleType:   gen.RuleType(plan.RuleType.ValueString()),
		TargetType: plan.TargetType.ValueString(),
		TargetId:   plan.TargetID.ValueString(),
		Priority:   int(plan.Priority.ValueInt64()),
		IsDefault:  boolToPtr(plan.IsDefault),
		IsEnabled:  boolToPtr(plan.IsEnabled),
	}
	resp.Diagnostics.Append(jsonToMapPtr(plan.Match, &body.Match)...)
	resp.Diagnostics.Append(jsonToMapPtr(plan.Input, &body.Input)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.Gen.ChannelsAddRouteEndpointWithResponse(ctx, plan.ChannelID.ValueString(), body)
	if err != nil {
		resp.Diagnostics.AddError("Error creating channel route", err.Error())
		return
	}
	if err := client.Check(apiResp.HTTPResponse, apiResp.Body); err != nil {
		resp.Diagnostics.AddError("Error creating channel route", err.Error())
		return
	}
	if apiResp.JSON201 == nil {
		resp.Diagnostics.AddError("Error creating channel route", "API returned an empty body")
		return
	}

	channelRouteApply(&plan, apiResp.JSON201)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *channelRouteResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state channelRouteResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Routes have no get-by-id endpoint; list the channel's routes and match id.
	apiResp, err := r.client.Gen.ChannelsListRoutesEndpointWithResponse(ctx, state.ChannelID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading channel route", err.Error())
		return
	}
	if err := client.Check(apiResp.HTTPResponse, apiResp.Body); err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading channel route", err.Error())
		return
	}
	if apiResp.JSON200 == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	id := state.ID.ValueString()
	for i := range *apiResp.JSON200 {
		route := &(*apiResp.JSON200)[i]
		if route.Id == id {
			channelRouteApply(&state, route)
			resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
			return
		}
	}
	resp.State.RemoveResource(ctx)
}

func (r *channelRouteResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan channelRouteResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	ruleType := gen.RuleType(plan.RuleType.ValueString())
	priority := int(plan.Priority.ValueInt64())
	body := gen.UpdateRouteRequest{
		RuleType:   &ruleType,
		TargetType: stringToPtr(plan.TargetType),
		TargetId:   stringToPtr(plan.TargetID),
		Priority:   &priority,
		IsDefault:  boolToPtr(plan.IsDefault),
		IsEnabled:  boolToPtr(plan.IsEnabled),
	}
	resp.Diagnostics.Append(jsonToMapPtr(plan.Match, &body.Match)...)
	resp.Diagnostics.Append(jsonToMapPtr(plan.Input, &body.Input)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.Gen.ChannelsUpdateRouteEndpointWithResponse(ctx, plan.ChannelID.ValueString(), plan.ID.ValueString(), body)
	if err != nil {
		resp.Diagnostics.AddError("Error updating channel route", err.Error())
		return
	}
	if err := client.Check(apiResp.HTTPResponse, apiResp.Body); err != nil {
		resp.Diagnostics.AddError("Error updating channel route", err.Error())
		return
	}
	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError("Error updating channel route", "API returned an empty body")
		return
	}

	channelRouteApply(&plan, apiResp.JSON200)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *channelRouteResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state channelRouteResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.Gen.ChannelsDeleteRouteEndpointWithResponse(ctx, state.ChannelID.ValueString(), state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error deleting channel route", err.Error())
		return
	}
	if err := client.Check(apiResp.HTTPResponse, apiResp.Body); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Error deleting channel route", err.Error())
	}
}

// ImportState accepts "<channel_id>/<route_id>" since a route is scoped to its channel.
func (r *channelRouteResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(req.ID, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError("Invalid import ID",
			fmt.Sprintf("Expected import ID in the form \"channel_id/route_id\", got %q.", req.ID))
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("channel_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), parts[1])...)
}

// channelRouteApply writes a ChannelRouteResponse into the model.
func channelRouteApply(m *channelRouteResourceModel, route *gen.ChannelRouteResponse) {
	m.ID = types.StringValue(route.Id)
	m.ChannelID = types.StringValue(route.ChannelId)
	m.RuleType = types.StringValue(route.RuleType)
	m.TargetType = types.StringValue(route.TargetType)
	m.TargetID = types.StringValue(route.TargetId)
	m.Priority = types.Int64Value(int64(route.Priority))
	m.IsDefault = types.BoolValue(route.IsDefault)
	m.IsEnabled = types.BoolValue(route.IsEnabled)
	m.Match = mapPtrToJSON(route.MatchJson)
	m.Input = mapPtrToJSON(route.InputJson)
	m.AccountID = types.StringValue(route.AccountId)
	m.CreatedAt = types.StringValue(route.CreatedAt)
	m.UpdatedAt = types.StringValue(route.UpdatedAt)
}
