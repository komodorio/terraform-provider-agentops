// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/komodorio/terraform-provider-agentops/internal/client"
	"github.com/komodorio/terraform-provider-agentops/internal/client/gen"
)

// Ensure the resource satisfies the framework interfaces.
var (
	_ resource.Resource                = &channelResource{}
	_ resource.ResourceWithConfigure   = &channelResource{}
	_ resource.ResourceWithImportState = &channelResource{}
)

// NewChannelResource is the constructor registered with the provider.
func NewChannelResource() resource.Resource {
	return &channelResource{}
}

type channelResource struct {
	client *client.Client
}

// channelResourceModel maps the agentops_channel schema to Go.
type channelResourceModel struct {
	ID               types.String         `tfsdk:"id"`
	Provider         types.String         `tfsdk:"channel_provider"`
	Connector        types.String         `tfsdk:"connector"`
	DisplayName      types.String         `tfsdk:"display_name"`
	AppToken         types.String         `tfsdk:"app_token"`
	Config           jsontypes.Normalized `tfsdk:"config"`
	ExternalID       types.String         `tfsdk:"external_id"`
	IntegrationID    types.String         `tfsdk:"integration_id"`
	Labels           types.Map            `tfsdk:"labels"`
	Status           types.String         `tfsdk:"status"`
	AccountID        types.String         `tfsdk:"account_id"`
	Slug             types.String         `tfsdk:"slug"`
	CapabilitiesJSON jsontypes.Normalized `tfsdk:"capabilities_json"`
	CreatedBy        types.String         `tfsdk:"created_by"`
	LastEventAt      types.String         `tfsdk:"last_event_at"`
	CreatedAt        types.String         `tfsdk:"created_at"`
	UpdatedAt        types.String         `tfsdk:"updated_at"`
}

func (r *channelResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_channel"
}

func (r *channelResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A notification channel that delivers agent events to an external destination " +
			"(e.g. Slack). Routes are managed with `agentops_channel_route`.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Channel identifier.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"channel_provider": schema.StringAttribute{
				MarkdownDescription: "Channel provider (e.g. `slack`). Named `channel_provider` because `provider` " +
					"is a reserved Terraform argument. Changing this forces a new channel.",
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"connector": schema.StringAttribute{
				MarkdownDescription: "Connector kind within the provider. Changing this forces a new channel.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"display_name": schema.StringAttribute{
				MarkdownDescription: "Human-readable channel name.",
				Required:            true,
			},
			"app_token": schema.StringAttribute{
				MarkdownDescription: "Provider app/bot token. Write-only; never returned by the API.",
				Optional:            true,
				Sensitive:           true,
			},
			"config": schema.StringAttribute{
				MarkdownDescription: "Provider-specific configuration, as a JSON object.",
				CustomType:          jsontypes.NormalizedType{},
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"external_id": schema.StringAttribute{
				MarkdownDescription: "External identifier for the channel in the provider. Changing this forces a new channel.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace(), stringplanmodifier.UseStateForUnknown()},
			},
			"integration_id": schema.StringAttribute{
				MarkdownDescription: "ID of the integration connection backing this channel. Changing this forces a new channel.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace(), stringplanmodifier.UseStateForUnknown()},
			},
			"labels": schema.MapAttribute{
				MarkdownDescription: "Arbitrary key/value labels applied to the channel.",
				ElementType:         types.StringType,
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.Map{mapplanmodifier.UseStateForUnknown()},
			},
			"status": schema.StringAttribute{
				MarkdownDescription: "Delivery status. Set to `paused` to suspend delivery or `active` to resume it.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"account_id": schema.StringAttribute{
				MarkdownDescription: "Account that owns the channel.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"slug": schema.StringAttribute{
				MarkdownDescription: "URL-safe channel slug.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"capabilities_json": schema.StringAttribute{
				MarkdownDescription: "Provider-reported capabilities, as a JSON object.",
				CustomType:          jsontypes.NormalizedType{},
				Computed:            true,
			},
			"created_by": schema.StringAttribute{
				MarkdownDescription: "Principal that created the channel.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"last_event_at": schema.StringAttribute{
				MarkdownDescription: "Timestamp of the last event delivered through the channel.",
				Computed:            true,
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

func (r *channelResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromProviderData(req.ProviderData, &resp.Diagnostics)
}

func (r *channelResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan channelResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := gen.CreateChannelRequest{
		Provider:      plan.Provider.ValueString(),
		Connector:     plan.Connector.ValueString(),
		DisplayName:   plan.DisplayName.ValueString(),
		AppToken:      stringToPtr(plan.AppToken),
		ExternalId:    stringToPtr(plan.ExternalID),
		IntegrationId: stringToPtr(plan.IntegrationID),
	}
	resp.Diagnostics.Append(jsonToMapPtr(plan.Config, &body.Config)...)
	resp.Diagnostics.Append(stringMapToPtr(ctx, plan.Labels, &body.Labels)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.Gen.ChannelsCreateChannelEndpointWithResponse(ctx, body)
	if err != nil {
		resp.Diagnostics.AddError("Error creating channel", err.Error())
		return
	}
	if err := client.Check(apiResp.HTTPResponse, apiResp.Body); err != nil {
		resp.Diagnostics.AddError("Error creating channel", err.Error())
		return
	}
	if apiResp.JSON201 == nil {
		resp.Diagnostics.AddError("Error creating channel", "API returned an empty body")
		return
	}

	desired := statusTarget(plan.Status)

	// The channel now exists server-side. Persist state before reconciling status so a
	// failed pause/resume still leaves the channel tracked and destroyable rather than
	// orphaned on the server.
	if diags := channelApply(ctx, &plan, apiResp.JSON201); diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	detail := r.reconcileStatus(ctx, apiResp.JSON201.Id, desired, apiResp.JSON201, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(channelApply(ctx, &plan, detail)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *channelResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state channelResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.Gen.ChannelsGetChannelEndpointWithResponse(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading channel", err.Error())
		return
	}
	if err := client.Check(apiResp.HTTPResponse, apiResp.Body); err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading channel", err.Error())
		return
	}
	if apiResp.JSON200 == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	resp.Diagnostics.Append(channelApply(ctx, &state, apiResp.JSON200)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *channelResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan channelResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := gen.UpdateChannelRequest{
		DisplayName: stringToPtr(plan.DisplayName),
		AppToken:    stringToPtr(plan.AppToken),
	}
	resp.Diagnostics.Append(jsonToMapPtr(plan.Config, &body.Config)...)
	resp.Diagnostics.Append(stringMapToPtr(ctx, plan.Labels, &body.Labels)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.Gen.ChannelsUpdateChannelEndpointWithResponse(ctx, plan.ID.ValueString(), body)
	if err != nil {
		resp.Diagnostics.AddError("Error updating channel", err.Error())
		return
	}
	if err := client.Check(apiResp.HTTPResponse, apiResp.Body); err != nil {
		resp.Diagnostics.AddError("Error updating channel", err.Error())
		return
	}
	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError("Error updating channel", "API returned an empty body")
		return
	}

	desired := statusTarget(plan.Status)
	detail := r.reconcileStatus(ctx, plan.ID.ValueString(), desired, apiResp.JSON200, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(channelApply(ctx, &plan, detail)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *channelResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state channelResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.Gen.ChannelsDeleteChannelEndpointWithResponse(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error deleting channel", err.Error())
		return
	}
	if err := client.Check(apiResp.HTTPResponse, apiResp.Body); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Error deleting channel", err.Error())
	}
}

func (r *channelResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// reconcileStatus drives the channel to the desired delivery status via the
// pause/resume endpoints. It returns the latest response (unchanged when no
// transition is needed).
func (r *channelResource) reconcileStatus(ctx context.Context, id, desired string, detail *gen.ChannelResponse, diags *diag.Diagnostics) *gen.ChannelResponse {
	if desired == "" || desired == detail.Status {
		return detail
	}

	switch desired {
	case "paused":
		apiResp, err := r.client.Gen.ChannelsPauseChannelEndpointWithResponse(ctx, id)
		if err != nil {
			diags.AddError("Error pausing channel", err.Error())
			return detail
		}
		if err := client.Check(apiResp.HTTPResponse, apiResp.Body); err != nil {
			diags.AddError("Error pausing channel", err.Error())
			return detail
		}
		if apiResp.JSON200 != nil {
			return apiResp.JSON200
		}
	case "active":
		apiResp, err := r.client.Gen.ChannelsResumeChannelEndpointWithResponse(ctx, id)
		if err != nil {
			diags.AddError("Error resuming channel", err.Error())
			return detail
		}
		if err := client.Check(apiResp.HTTPResponse, apiResp.Body); err != nil {
			diags.AddError("Error resuming channel", err.Error())
			return detail
		}
		if apiResp.JSON200 != nil {
			return apiResp.JSON200
		}
	}
	return detail
}

// channelApply writes a ChannelResponse into the model.
func channelApply(ctx context.Context, m *channelResourceModel, c *gen.ChannelResponse) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.StringValue(c.Id)
	m.Provider = types.StringValue(c.Provider)
	m.Connector = types.StringValue(c.Connector)
	m.DisplayName = types.StringValue(c.DisplayName)
	m.ExternalID = ptrToString(c.ExternalId)
	m.IntegrationID = ptrToString(c.IntegrationId)
	m.Status = types.StringValue(c.Status)
	m.AccountID = types.StringValue(c.AccountId)
	m.Slug = types.StringValue(c.Slug)
	m.CreatedBy = ptrToString(c.CreatedBy)
	m.LastEventAt = ptrToString(c.LastEventAt)
	m.CreatedAt = types.StringValue(c.CreatedAt)
	m.UpdatedAt = types.StringValue(c.UpdatedAt)
	m.Config = mapPtrToJSON(c.ConfigJson)
	m.CapabilitiesJSON = mapPtrToJSON(c.CapabilitiesJson)

	labels, d := stringMapValue(ctx, c.Labels)
	diags.Append(d...)
	m.Labels = labels

	return diags
}
