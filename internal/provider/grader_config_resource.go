// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/komodorio/terraform-provider-agentops/internal/client"
	"github.com/komodorio/terraform-provider-agentops/internal/client/gen"
)

// Ensure the resource satisfies the framework interfaces.
var (
	_ resource.Resource                = &graderConfigResource{}
	_ resource.ResourceWithConfigure   = &graderConfigResource{}
	_ resource.ResourceWithImportState = &graderConfigResource{}
)

// NewGraderConfigResource is the constructor registered with the provider.
func NewGraderConfigResource() resource.Resource {
	return &graderConfigResource{}
}

type graderConfigResource struct {
	client *client.Client
}

// graderConfigResourceModel maps the agentops_grader_config schema to Go.
type graderConfigResourceModel struct {
	ID            types.String `tfsdk:"id"`
	TargetAgentID types.String `tfsdk:"target_agent_id"`
	GraderAgentID types.String `tfsdk:"grader_agent_id"`
	Guidelines    types.String `tfsdk:"guidelines"`
	SampleRate    types.Int64  `tfsdk:"sample_rate"`
	RunsSeen      types.Int64  `tfsdk:"runs_seen"`
	CreatedAt     types.String `tfsdk:"created_at"`
	UpdatedAt     types.String `tfsdk:"updated_at"`
}

func (r *graderConfigResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_grader_config"
}

func (r *graderConfigResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A grader configuration: has a grader agent automatically score a sample of " +
			"another agent's runs for quality.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Grader configuration identifier.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"target_agent_id": schema.StringAttribute{
				MarkdownDescription: "ID of the agent whose runs are graded. Changing this forces a new grader config.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"grader_agent_id": schema.StringAttribute{
				MarkdownDescription: "ID of the agent that performs the grading.",
				Required:            true,
			},
			"guidelines": schema.StringAttribute{
				MarkdownDescription: "Free-form grading guidelines passed to the grader agent.",
				Optional:            true,
			},
			"sample_rate": schema.Int64Attribute{
				MarkdownDescription: "Percentage of the target agent's runs to grade (0-100). Server-defaulted when omitted.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},
			"runs_seen": schema.Int64Attribute{
				MarkdownDescription: "Number of target-agent runs observed by this grader config.",
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

func (r *graderConfigResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromProviderData(req.ProviderData, &resp.Diagnostics)
}

func (r *graderConfigResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan graderConfigResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := gen.CreateGraderConfigRequest{
		TargetAgentId: plan.TargetAgentID.ValueString(),
		GraderAgentId: plan.GraderAgentID.ValueString(),
		Guidelines:    stringToPtr(plan.Guidelines),
		SampleRate:    int64ToIntPtr(plan.SampleRate),
	}

	apiResp, err := r.client.Gen.GraderConfigsCreateGraderConfigEndpointWithResponse(ctx, body)
	if err != nil {
		resp.Diagnostics.AddError("Error creating grader config", err.Error())
		return
	}
	if err := client.Check(apiResp.HTTPResponse, apiResp.Body); err != nil {
		resp.Diagnostics.AddError("Error creating grader config", err.Error())
		return
	}
	if apiResp.JSON201 == nil {
		resp.Diagnostics.AddError("Error creating grader config", "API returned an empty body")
		return
	}

	graderConfigApply(&plan, apiResp.JSON201)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *graderConfigResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state graderConfigResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// The API has no get-by-id endpoint; list the target agent's configs and
	// match on id. The target filter is omitted on import (empty state) so the
	// full list is scanned.
	params := &gen.GraderConfigsListGraderConfigsEndpointParams{}
	if target := state.TargetAgentID.ValueString(); target != "" {
		params.TargetAgentId = &target
	}
	apiResp, err := r.client.Gen.GraderConfigsListGraderConfigsEndpointWithResponse(ctx, params)
	if err != nil {
		resp.Diagnostics.AddError("Error reading grader config", err.Error())
		return
	}
	if err := client.Check(apiResp.HTTPResponse, apiResp.Body); err != nil {
		resp.Diagnostics.AddError("Error reading grader config", err.Error())
		return
	}
	if apiResp.JSON200 == nil || apiResp.JSON200.Configs == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	id := state.ID.ValueString()
	for i := range *apiResp.JSON200.Configs {
		cfg := &(*apiResp.JSON200.Configs)[i]
		if cfg.Id == id {
			graderConfigApply(&state, cfg)
			resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
			return
		}
	}
	resp.State.RemoveResource(ctx)
}

func (r *graderConfigResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan graderConfigResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := gen.UpdateGraderConfigRequest{
		GraderAgentId: stringToPtr(plan.GraderAgentID),
		Guidelines:    stringToPtr(plan.Guidelines),
		SampleRate:    int64ToIntPtr(plan.SampleRate),
	}

	apiResp, err := r.client.Gen.GraderConfigsUpdateGraderConfigEndpointWithResponse(ctx, plan.ID.ValueString(), body)
	if err != nil {
		resp.Diagnostics.AddError("Error updating grader config", err.Error())
		return
	}
	if err := client.Check(apiResp.HTTPResponse, apiResp.Body); err != nil {
		resp.Diagnostics.AddError("Error updating grader config", err.Error())
		return
	}
	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError("Error updating grader config", "API returned an empty body")
		return
	}

	graderConfigApply(&plan, apiResp.JSON200)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *graderConfigResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state graderConfigResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.Gen.GraderConfigsDeleteGraderConfigEndpointWithResponse(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error deleting grader config", err.Error())
		return
	}
	if err := client.Check(apiResp.HTTPResponse, apiResp.Body); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Error deleting grader config", err.Error())
	}
}

func (r *graderConfigResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// graderConfigApply writes a GraderConfigResponse into the model.
func graderConfigApply(m *graderConfigResourceModel, c *gen.GraderConfigResponse) {
	m.ID = types.StringValue(c.Id)
	m.TargetAgentID = types.StringValue(c.TargetAgentId)
	m.GraderAgentID = types.StringValue(c.GraderAgentId)
	m.Guidelines = strOrNull(c.Guidelines)
	m.SampleRate = types.Int64Value(int64(c.SampleRate))
	m.RunsSeen = types.Int64Value(int64(c.RunsSeen))
	m.CreatedAt = types.StringValue(c.CreatedAt)
	m.UpdatedAt = types.StringValue(c.UpdatedAt)
}
