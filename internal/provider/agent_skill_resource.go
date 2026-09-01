// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"strings"

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
	_ resource.Resource                = &agentSkillResource{}
	_ resource.ResourceWithConfigure   = &agentSkillResource{}
	_ resource.ResourceWithImportState = &agentSkillResource{}
)

// NewAgentSkillResource is the constructor registered with the provider.
func NewAgentSkillResource() resource.Resource {
	return &agentSkillResource{}
}

type agentSkillResource struct {
	client *client.Client
}

// agentSkillResourceModel maps the agentops_agent_skill schema to Go. It attaches
// an authored skill to an agent. The pair (agent_id, skill_id) identifies the
// binding and is create-only; only the pinned version can be changed in place
// (a "repin").
type agentSkillResourceModel struct {
	ID              types.String `tfsdk:"id"`
	AgentID         types.String `tfsdk:"agent_id"`
	SkillID         types.String `tfsdk:"skill_id"`
	PinVersion      types.Int64  `tfsdk:"pin_version"`
	PinnedVersionID types.String `tfsdk:"pinned_version_id"`
	Origin          types.String `tfsdk:"origin"`
	CreatedAt       types.String `tfsdk:"created_at"`
}

func (r *agentSkillResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_agent_skill"
}

func (r *agentSkillResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	// Changing either endpoint of the binding replaces it; only the pin can be
	// updated in place.
	forceNewString := []planmodifier.String{stringplanmodifier.RequiresReplace()}

	resp.Schema = schema.Schema{
		MarkdownDescription: "Attaches an authored skill to an agent. Changing the agent or skill forces a " +
			"new binding; the pinned version can be changed in place. Requires the account-level " +
			"`skills_registry` feature.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Binding identifier.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"agent_id": schema.StringAttribute{
				MarkdownDescription: "ID of the agent to attach the skill to. Changing this forces a new binding.",
				Required:            true,
				PlanModifiers:       forceNewString,
			},
			"skill_id": schema.StringAttribute{
				MarkdownDescription: "ID of the skill to attach. Changing this forces a new binding.",
				Required:            true,
				PlanModifiers:       forceNewString,
			},
			"pin_version": schema.Int64Attribute{
				MarkdownDescription: "Published version number to pin the binding to (e.g. `3`). Omit to float on " +
					"the latest published version. Changing it repins the binding in place.",
				Optional:      true,
				Computed:      true,
				PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},
			"pinned_version_id": schema.StringAttribute{
				MarkdownDescription: "Version ID the pin resolved to, or null when the binding floats on the latest version.",
				Computed:            true,
			},
			"origin": schema.StringAttribute{
				MarkdownDescription: "How the binding was created (e.g. `manual`).",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "Creation timestamp.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
		},
	}
}

func (r *agentSkillResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromProviderData(req.ProviderData, &resp.Diagnostics)
}

func (r *agentSkillResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan agentSkillResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := gen.AttachSkillRequest{
		SkillId:    plan.SkillID.ValueString(),
		PinVersion: int64ToIntPtr(plan.PinVersion),
	}

	apiResp, err := r.client.Gen.SkillsAttachSkillRouteWithResponse(ctx, plan.AgentID.ValueString(), body)
	if err != nil {
		resp.Diagnostics.AddError("Error attaching skill to agent", err.Error())
		return
	}
	if err := client.Check(apiResp.HTTPResponse, apiResp.Body); err != nil {
		resp.Diagnostics.AddError("Error attaching skill to agent", err.Error())
		return
	}
	if apiResp.JSON201 == nil {
		resp.Diagnostics.AddError("Error attaching skill to agent", "API returned an empty body")
		return
	}

	agentSkillApplyBinding(&plan, apiResp.JSON201)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *agentSkillResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state agentSkillResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// There is no GET-by-binding endpoint; the skill detail carries the list of
	// agents it is bound to, so read that and match on agent_id.
	apiResp, err := r.client.Gen.SkillsGetSkillRouteWithResponse(ctx, state.SkillID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading agent skill binding", err.Error())
		return
	}
	if err := client.Check(apiResp.HTTPResponse, apiResp.Body); err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading agent skill binding", err.Error())
		return
	}
	if apiResp.JSON200 == nil || apiResp.JSON200.UsedBy == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	var found *gen.SkillBindingUsage
	for i := range *apiResp.JSON200.UsedBy {
		if (*apiResp.JSON200.UsedBy)[i].AgentId == state.AgentID.ValueString() {
			found = &(*apiResp.JSON200.UsedBy)[i]
			break
		}
	}
	if found == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	state.ID = types.StringValue(found.BindingId)
	state.PinVersion = intPtrToInt64(found.Version)
	state.PinnedVersionID = ptrToString(found.PinnedVersionId)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *agentSkillResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan agentSkillResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := gen.RepinSkillRequest{
		PinVersion: int64ToIntPtr(plan.PinVersion),
	}

	apiResp, err := r.client.Gen.SkillsRepinSkillRouteWithResponse(ctx, plan.AgentID.ValueString(), plan.SkillID.ValueString(), body)
	if err != nil {
		resp.Diagnostics.AddError("Error repinning agent skill binding", err.Error())
		return
	}
	if err := client.Check(apiResp.HTTPResponse, apiResp.Body); err != nil {
		resp.Diagnostics.AddError("Error repinning agent skill binding", err.Error())
		return
	}
	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError("Error repinning agent skill binding", "API returned an empty body")
		return
	}

	agentSkillApplyBinding(&plan, apiResp.JSON200)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *agentSkillResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state agentSkillResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.Gen.SkillsDetachSkillRouteWithResponse(ctx, state.AgentID.ValueString(), state.SkillID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error detaching skill from agent", err.Error())
		return
	}
	if err := client.Check(apiResp.HTTPResponse, apiResp.Body); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Error detaching skill from agent", err.Error())
	}
}

// ImportState accepts a composite "agent_id/skill_id" identifier. The binding id
// (id), origin and pinned version are refreshed by the subsequent Read.
func (r *agentSkillResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.Split(req.ID, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError("Invalid import ID", "expected agent_id/skill_id")
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("agent_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("skill_id"), parts[1])...)
}

// agentSkillApplyBinding writes an AgentSkillBindingResponse into the model. The
// response reports the resolved pinned_version_id but not the version number, so
// pin_version is left to the caller (kept from the plan on write, read from the
// skill's used_by list on refresh).
func agentSkillApplyBinding(m *agentSkillResourceModel, b *gen.AgentSkillBindingResponse) {
	m.ID = types.StringValue(b.Id)
	m.AgentID = types.StringValue(b.AgentId)
	m.SkillID = types.StringValue(b.SkillId)
	m.PinnedVersionID = ptrToString(b.PinnedVersionId)
	m.Origin = types.StringValue(b.Origin)
	m.CreatedAt = types.StringValue(b.CreatedAt)

	// A binding created or repinned with no pin floats on the latest version:
	// normalise the unknown planned value to null so it can be written to state.
	if m.PinVersion.IsUnknown() {
		m.PinVersion = types.Int64Null()
	}
}
