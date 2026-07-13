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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/komodorio/terraform-provider-agentops/internal/client"
	"github.com/komodorio/terraform-provider-agentops/internal/client/gen"
)

// Ensure the resource satisfies the framework interfaces.
var (
	_ resource.Resource                = &workflowResource{}
	_ resource.ResourceWithConfigure   = &workflowResource{}
	_ resource.ResourceWithImportState = &workflowResource{}
)

// NewWorkflowResource is the constructor registered with the provider.
func NewWorkflowResource() resource.Resource {
	return &workflowResource{}
}

type workflowResource struct {
	client *client.Client
}

// workflowResourceModel maps the agentops_workflow schema to Go.
type workflowResourceModel struct {
	ID          types.String         `tfsdk:"id"`
	Name        types.String         `tfsdk:"name"`
	Description types.String         `tfsdk:"description"`
	IsEnabled   types.Bool           `tfsdk:"is_enabled"`
	Labels      types.Map            `tfsdk:"labels"`
	Steps       jsontypes.Normalized `tfsdk:"steps"`
	Trigger     jsontypes.Normalized `tfsdk:"trigger"`
	CreatedAt   types.String         `tfsdk:"created_at"`
	UpdatedAt   types.String         `tfsdk:"updated_at"`
}

func (r *workflowResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_workflow"
}

func (r *workflowResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A workflow composed of ordered steps and an optional trigger.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Workflow identifier.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Human-readable workflow name.",
				Required:            true,
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Free-form description.",
				Optional:            true,
			},
			"is_enabled": schema.BoolAttribute{
				MarkdownDescription: "Whether the workflow is active. Defaults to `true`.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
			},
			"labels": schema.MapAttribute{
				MarkdownDescription: "Arbitrary key/value labels attached to the workflow.",
				ElementType:         types.StringType,
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.Map{mapplanmodifier.UseStateForUnknown()},
			},
			"steps": schema.StringAttribute{
				CustomType:          jsontypes.NormalizedType{},
				MarkdownDescription: "Ordered list of workflow steps, encoded as a JSON array (JSON).",
				Optional:            true,
				Computed:            true,
			},
			"trigger": schema.StringAttribute{
				CustomType:          jsontypes.NormalizedType{},
				MarkdownDescription: "Trigger configuration for the workflow, encoded as a JSON object (JSON).",
				Optional:            true,
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

func (r *workflowResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromProviderData(req.ProviderData, &resp.Diagnostics)
}

func (r *workflowResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan workflowResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := gen.CreateWorkflowRequest{
		Name:        plan.Name.ValueString(),
		Description: stringToPtr(plan.Description),
		IsEnabled:   boolToPtr(plan.IsEnabled),
	}
	if steps, ok := workflowDecodeSteps(plan.Steps, &resp.Diagnostics); ok {
		body.Steps = steps
	}
	if trigger, ok := workflowDecodeTrigger(plan.Trigger, &resp.Diagnostics); ok {
		body.Trigger = trigger
	}
	workflowLabelsToRequest(ctx, plan.Labels, &body.Labels, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.Gen.WorkflowsCreateWorkflowEndpointWithResponse(ctx, body)
	if err != nil {
		resp.Diagnostics.AddError("Error creating workflow", err.Error())
		return
	}
	if err := client.Check(apiResp.HTTPResponse, apiResp.Body); err != nil {
		resp.Diagnostics.AddError("Error creating workflow", err.Error())
		return
	}
	if apiResp.JSON201 == nil {
		resp.Diagnostics.AddError("Error creating workflow", "API returned an empty body")
		return
	}

	r.applyWorkflowResponse(ctx, &plan, apiResp.JSON201, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *workflowResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state workflowResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.Gen.WorkflowsGetWorkflowEndpointWithResponse(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading workflow", err.Error())
		return
	}
	if err := client.Check(apiResp.HTTPResponse, apiResp.Body); err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading workflow", err.Error())
		return
	}
	if apiResp.JSON200 == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	r.applyWorkflowResponse(ctx, &state, apiResp.JSON200, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *workflowResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan workflowResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := gen.UpdateWorkflowRequest{
		Name:        stringToPtr(plan.Name),
		Description: stringToPtr(plan.Description),
		IsEnabled:   boolToPtr(plan.IsEnabled),
	}
	if steps, ok := workflowDecodeSteps(plan.Steps, &resp.Diagnostics); ok {
		body.Steps = steps
	}
	if trigger, ok := workflowDecodeTrigger(plan.Trigger, &resp.Diagnostics); ok {
		body.Trigger = trigger
	}
	workflowLabelsToRequest(ctx, plan.Labels, &body.Labels, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.Gen.WorkflowsUpdateWorkflowEndpointWithResponse(ctx, plan.ID.ValueString(), body)
	if err != nil {
		resp.Diagnostics.AddError("Error updating workflow", err.Error())
		return
	}
	if err := client.Check(apiResp.HTTPResponse, apiResp.Body); err != nil {
		resp.Diagnostics.AddError("Error updating workflow", err.Error())
		return
	}
	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError("Error updating workflow", "API returned an empty body")
		return
	}

	r.applyWorkflowResponse(ctx, &plan, apiResp.JSON200, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *workflowResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state workflowResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.Gen.WorkflowsDeleteWorkflowEndpointWithResponse(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error deleting workflow", err.Error())
		return
	}
	if err := client.Check(apiResp.HTTPResponse, apiResp.Body); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Error deleting workflow", err.Error())
	}
}

func (r *workflowResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// applyWorkflowResponse writes a WorkflowResponse into the model, encoding the
// nested steps/trigger back into their JSON string representations.
func (r *workflowResource) applyWorkflowResponse(ctx context.Context, m *workflowResourceModel, w *gen.WorkflowResponse, diags *diag.Diagnostics) {
	m.ID = types.StringValue(w.WorkflowId)
	m.Name = types.StringValue(w.Name)
	m.Description = ptrToString(w.Description)
	m.IsEnabled = types.BoolValue(w.IsEnabled)
	m.CreatedAt = types.StringValue(w.CreatedAt)
	m.UpdatedAt = types.StringValue(w.UpdatedAt)

	if b, err := json.Marshal(w.Steps); err != nil {
		diags.AddError("Error encoding workflow steps", err.Error())
	} else {
		m.Steps = jsontypes.NewNormalizedValue(string(b))
	}

	if b, err := json.Marshal(w.Trigger); err != nil {
		diags.AddError("Error encoding workflow trigger", err.Error())
	} else {
		m.Trigger = jsontypes.NewNormalizedValue(string(b))
	}

	m.Labels = workflowLabelsToModel(ctx, w.Labels, diags)
}

// workflowDecodeSteps unmarshals the steps JSON string into a request slice. It
// returns ok=false when the attribute is null/unknown (so the field stays
// omitted) or when decoding fails (an error diagnostic is emitted).
func workflowDecodeSteps(v jsontypes.Normalized, diags *diag.Diagnostics) (*[]gen.WorkflowStep, bool) {
	if v.IsNull() || v.IsUnknown() {
		return nil, false
	}
	var steps []gen.WorkflowStep
	if err := json.Unmarshal([]byte(v.ValueString()), &steps); err != nil {
		diags.AddError("Invalid workflow steps", "steps must be a valid JSON array: "+err.Error())
		return nil, false
	}
	return &steps, true
}

// workflowDecodeTrigger unmarshals the trigger JSON string into a request value.
// It returns ok=false when the attribute is null/unknown or when decoding fails.
func workflowDecodeTrigger(v jsontypes.Normalized, diags *diag.Diagnostics) (*gen.WorkflowTrigger, bool) {
	if v.IsNull() || v.IsUnknown() {
		return nil, false
	}
	var trigger gen.WorkflowTrigger
	if err := json.Unmarshal([]byte(v.ValueString()), &trigger); err != nil {
		diags.AddError("Invalid workflow trigger", "trigger must be a valid JSON object: "+err.Error())
		return nil, false
	}
	return &trigger, true
}

// workflowLabelsToRequest copies the labels map into the request field, leaving
// it nil (omitted) when the map is null or unknown.
func workflowLabelsToRequest(ctx context.Context, labels types.Map, target **map[string]string, diags *diag.Diagnostics) {
	if labels.IsNull() || labels.IsUnknown() {
		return
	}
	m := map[string]string{}
	diags.Append(labels.ElementsAs(ctx, &m, false)...)
	if diags.HasError() {
		return
	}
	*target = &m
}

// workflowLabelsToModel converts an optional API labels map into a Terraform
// map, mapping nil to a null map.
func workflowLabelsToModel(ctx context.Context, labels *map[string]string, diags *diag.Diagnostics) types.Map {
	if labels == nil {
		return types.MapNull(types.StringType)
	}
	v, d := types.MapValueFrom(ctx, types.StringType, *labels)
	diags.Append(d...)
	return v
}
