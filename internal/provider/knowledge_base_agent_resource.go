// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/komodorio/terraform-provider-agentops/internal/client"
	"github.com/komodorio/terraform-provider-agentops/internal/client/gen"
)

// Ensure the resource satisfies the framework interfaces.
var (
	_ resource.Resource                = &knowledgeBaseAgentResource{}
	_ resource.ResourceWithConfigure   = &knowledgeBaseAgentResource{}
	_ resource.ResourceWithImportState = &knowledgeBaseAgentResource{}
)

// NewKnowledgeBaseAgentResource is the constructor registered with the provider.
func NewKnowledgeBaseAgentResource() resource.Resource {
	return &knowledgeBaseAgentResource{}
}

type knowledgeBaseAgentResource struct {
	client *client.Client
}

// knowledgeBaseAgentResourceModel maps the agentops_knowledge_base_agent schema
// to Go. This resource attaches an agent to a knowledge base; there is no update
// endpoint, so both inputs are create-only (ForceNew).
type knowledgeBaseAgentResourceModel struct {
	ID        types.String `tfsdk:"id"`
	KBID      types.String `tfsdk:"kb_id"`
	AgentID   types.String `tfsdk:"agent_id"`
	CreatedAt types.String `tfsdk:"created_at"`
}

func (r *knowledgeBaseAgentResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_knowledge_base_agent"
}

func (r *knowledgeBaseAgentResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	// Attaching an agent to a knowledge base has no update endpoint: changing
	// either input replaces the grant.
	forceNewString := []planmodifier.String{stringplanmodifier.RequiresReplace()}

	resp.Schema = schema.Schema{
		MarkdownDescription: "Attaches an agent to a knowledge base. This association cannot be updated in " +
			"place; changing either argument forces a new grant.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Grant identifier.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"kb_id": schema.StringAttribute{
				MarkdownDescription: "ID of the knowledge base to attach the agent to. Changing this forces a new grant.",
				Required:            true,
				PlanModifiers:       forceNewString,
			},
			"agent_id": schema.StringAttribute{
				MarkdownDescription: "ID of the agent to attach. Changing this forces a new grant.",
				Required:            true,
				PlanModifiers:       forceNewString,
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "Creation timestamp.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
		},
	}
}

func (r *knowledgeBaseAgentResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromProviderData(req.ProviderData, &resp.Diagnostics)
}

func (r *knowledgeBaseAgentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan knowledgeBaseAgentResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := gen.AttachAgentToKBRequest{
		AgentId: plan.AgentID.ValueString(),
	}

	apiResp, err := r.client.Gen.KnowledgeAttachAgentWithResponse(ctx, plan.KBID.ValueString(), body)
	if err != nil {
		resp.Diagnostics.AddError("Error attaching agent to knowledge base", err.Error())
		return
	}
	if err := client.Check(apiResp.HTTPResponse, apiResp.Body); err != nil {
		resp.Diagnostics.AddError("Error attaching agent to knowledge base", err.Error())
		return
	}
	if apiResp.JSON201 == nil {
		resp.Diagnostics.AddError("Error attaching agent to knowledge base", "API returned an empty body")
		return
	}

	g := apiResp.JSON201
	plan.ID = types.StringValue(g.GrantId)
	plan.CreatedAt = types.StringValue(g.CreatedAt)

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *knowledgeBaseAgentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state knowledgeBaseAgentResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// There is no GET-by-id endpoint; list the knowledge base's agent grants and
	// match on agent_id.
	apiResp, err := r.client.Gen.KnowledgeListAgentGrantsWithResponse(ctx, state.KBID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading knowledge base agent grant", err.Error())
		return
	}
	if err := client.Check(apiResp.HTTPResponse, apiResp.Body); err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading knowledge base agent grant", err.Error())
		return
	}
	if apiResp.JSON200 == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	var found *gen.KBAgentGrant
	for i := range *apiResp.JSON200 {
		if (*apiResp.JSON200)[i].AgentId == state.AgentID.ValueString() {
			found = &(*apiResp.JSON200)[i]
			break
		}
	}
	if found == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	state.ID = types.StringValue(found.GrantId)
	state.CreatedAt = types.StringValue(found.CreatedAt)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update should be unreachable: both input attributes are RequiresReplace, so
// Terraform replaces the grant rather than updating it. Implemented to satisfy
// the resource.Resource interface.
func (r *knowledgeBaseAgentResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError(
		"Knowledge base agent grants cannot be updated",
		"This is a bug in the provider: an update was attempted on an immutable resource.",
	)
}

func (r *knowledgeBaseAgentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state knowledgeBaseAgentResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.Gen.KnowledgeRevokeAgentWithResponse(ctx, state.KBID.ValueString(), state.AgentID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error revoking knowledge base agent grant", err.Error())
		return
	}
	if err := client.Check(apiResp.HTTPResponse, apiResp.Body); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Error revoking knowledge base agent grant", err.Error())
	}
}

// ImportState accepts a composite "kb_id/agent_id" identifier. The grant_id (id)
// and created_at are refreshed by the subsequent Read.
func (r *knowledgeBaseAgentResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.Split(req.ID, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError("Invalid import ID", "expected kb_id/agent_id")
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("kb_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("agent_id"), parts[1])...)
}
