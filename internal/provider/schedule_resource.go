// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"encoding/json"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/komodorio/terraform-provider-agentops/internal/client"
	"github.com/komodorio/terraform-provider-agentops/internal/client/gen"
)

// Ensure the resource satisfies the framework interfaces.
var (
	_ resource.Resource                = &scheduleResource{}
	_ resource.ResourceWithConfigure   = &scheduleResource{}
	_ resource.ResourceWithImportState = &scheduleResource{}
)

// NewScheduleResource is the constructor registered with the provider.
func NewScheduleResource() resource.Resource {
	return &scheduleResource{}
}

type scheduleResource struct {
	client *client.Client
}

// scheduleResourceModel maps the agentops_schedule schema to Go.
type scheduleResourceModel struct {
	ID          types.String         `tfsdk:"id"`
	AgentID     types.String         `tfsdk:"agent_id"`
	CronExpr    types.String         `tfsdk:"cron_expr"`
	Input       jsontypes.Normalized `tfsdk:"input"`
	IsEnabled   types.Bool           `tfsdk:"is_enabled"`
	Timezone    types.String         `tfsdk:"timezone"`
	Name        types.String         `tfsdk:"name"`
	Description types.String         `tfsdk:"description"`
	Source      types.String         `tfsdk:"source"`
	SourceID    types.String         `tfsdk:"source_id"`
	TriggerType types.String         `tfsdk:"trigger_type"`
	LastFiredAt types.String         `tfsdk:"last_fired_at"`
	Editable    types.Bool           `tfsdk:"editable"`
	CreatedAt   types.String         `tfsdk:"created_at"`
	UpdatedAt   types.String         `tfsdk:"updated_at"`
}

func (r *scheduleResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_schedule"
}

func (r *scheduleResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A cron schedule that periodically invokes an agent.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Schedule identifier.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"agent_id": schema.StringAttribute{
				MarkdownDescription: "ID of the agent this schedule invokes. Changing this forces a new schedule.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"cron_expr": schema.StringAttribute{
				MarkdownDescription: "Cron expression describing when the schedule fires.",
				Required:            true,
			},
			"input": schema.StringAttribute{
				CustomType:          jsontypes.NormalizedType{},
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Input payload passed to the agent, as a JSON object.",
			},
			"is_enabled": schema.BoolAttribute{
				MarkdownDescription: "Whether the schedule is active. Defaults to `true`.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
			},
			"timezone": schema.StringAttribute{
				MarkdownDescription: "IANA timezone in which the cron expression is evaluated. Server-assigned when omitted.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Human-readable schedule name.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Free-form description.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"source": schema.StringAttribute{
				MarkdownDescription: "Origin of the schedule.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"source_id": schema.StringAttribute{
				MarkdownDescription: "Identifier of the schedule's source.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"trigger_type": schema.StringAttribute{
				MarkdownDescription: "Type of trigger backing this schedule.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"last_fired_at": schema.StringAttribute{
				MarkdownDescription: "Timestamp of the last time the schedule fired.",
				Computed:            true,
			},
			"editable": schema.BoolAttribute{
				MarkdownDescription: "Whether the schedule can be edited.",
				Computed:            true,
				PlanModifiers:       []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
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

func (r *scheduleResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromProviderData(req.ProviderData, &resp.Diagnostics)
}

func (r *scheduleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan scheduleResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := gen.CreateScheduleRequest{
		AgentId:   plan.AgentID.ValueString(),
		CronExpr:  plan.CronExpr.ValueString(),
		IsEnabled: boolToPtr(plan.IsEnabled),
		Timezone:  stringToPtr(plan.Timezone),
	}
	resp.Diagnostics.Append(scheduleInputToRequest(plan.Input, &body.Input)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.Gen.SchedulesCreateScheduleEndpointWithResponse(ctx, body)
	if err != nil {
		resp.Diagnostics.AddError("Error creating schedule", err.Error())
		return
	}
	if err := client.Check(apiResp.HTTPResponse, apiResp.Body); err != nil {
		resp.Diagnostics.AddError("Error creating schedule", err.Error())
		return
	}
	if apiResp.JSON201 == nil {
		resp.Diagnostics.AddError("Error creating schedule", "API returned an empty body")
		return
	}

	resp.Diagnostics.Append(applyScheduleResponse(&plan, apiResp.JSON201)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *scheduleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state scheduleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.Gen.SchedulesGetScheduleEndpointWithResponse(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading schedule", err.Error())
		return
	}
	if err := client.Check(apiResp.HTTPResponse, apiResp.Body); err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading schedule", err.Error())
		return
	}
	if apiResp.JSON200 == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	resp.Diagnostics.Append(applyScheduleResponse(&state, apiResp.JSON200)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *scheduleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan scheduleResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := gen.UpdateScheduleRequest{
		CronExpr:  stringToPtr(plan.CronExpr),
		IsEnabled: boolToPtr(plan.IsEnabled),
		Timezone:  stringToPtr(plan.Timezone),
	}
	resp.Diagnostics.Append(scheduleInputToRequest(plan.Input, &body.Input)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.Gen.SchedulesUpdateScheduleEndpointWithResponse(ctx, plan.ID.ValueString(), body)
	if err != nil {
		resp.Diagnostics.AddError("Error updating schedule", err.Error())
		return
	}
	if err := client.Check(apiResp.HTTPResponse, apiResp.Body); err != nil {
		resp.Diagnostics.AddError("Error updating schedule", err.Error())
		return
	}
	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError("Error updating schedule", "API returned an empty body")
		return
	}

	resp.Diagnostics.Append(applyScheduleResponse(&plan, apiResp.JSON200)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *scheduleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state scheduleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.Gen.SchedulesDeleteScheduleEndpointWithResponse(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error deleting schedule", err.Error())
		return
	}
	if err := client.Check(apiResp.HTTPResponse, apiResp.Body); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Error deleting schedule", err.Error())
	}
}

func (r *scheduleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// applyScheduleResponse writes every field of a ScheduleResponse into the model.
func applyScheduleResponse(m *scheduleResourceModel, s *gen.ScheduleResponse) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.StringValue(s.ScheduleId)
	m.AgentID = types.StringValue(s.AgentId)
	m.CronExpr = types.StringValue(s.CronExpr)
	m.IsEnabled = types.BoolValue(s.IsEnabled)
	m.Timezone = types.StringValue(s.Timezone)
	m.Name = ptrToString(s.Name)
	m.Description = ptrToString(s.Description)
	m.Source = strOrNull(enumPtrToString(s.Source))
	m.SourceID = ptrToString(s.SourceId)
	m.TriggerType = strOrNull(enumPtrToString(s.TriggerType))
	m.LastFiredAt = ptrToString(s.LastFiredAt)
	m.Editable = boolPtrToBool(s.Editable)
	m.CreatedAt = types.StringValue(s.CreatedAt)
	m.UpdatedAt = types.StringValue(s.UpdatedAt)

	m.Input = scheduleInputToState(s.Input, &diags)
	return diags
}

// scheduleInputToRequest unmarshals the JSON-string input attribute into the
// free-form map expected by the request body, leaving it nil (omitted) when the
// attribute is null or unknown.
func scheduleInputToRequest(input jsontypes.Normalized, target **map[string]interface{}) diag.Diagnostics {
	var diags diag.Diagnostics
	if input.IsNull() || input.IsUnknown() {
		return diags
	}
	var v map[string]interface{}
	if err := json.Unmarshal([]byte(input.ValueString()), &v); err != nil {
		diags.AddError("Invalid input", "Failed to parse `input` as a JSON object: "+err.Error())
		return diags
	}
	*target = &v
	return diags
}

// scheduleInputToState renders the free-form input map from a response back into
// the normalized JSON-string attribute, mapping a nil/empty map to null.
func scheduleInputToState(input map[string]interface{}, diags *diag.Diagnostics) jsontypes.Normalized {
	if len(input) == 0 {
		return jsontypes.NewNormalizedNull()
	}
	b, err := json.Marshal(input)
	if err != nil {
		diags.AddError("Invalid input", "Failed to serialize `input` from the API response: "+err.Error())
		return jsontypes.NewNormalizedNull()
	}
	return jsontypes.NewNormalizedValue(string(b))
}
