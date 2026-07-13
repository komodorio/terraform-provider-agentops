// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/komodorio/terraform-provider-agentops/internal/client"
	"github.com/komodorio/terraform-provider-agentops/internal/client/gen"
)

// Ensure the resource satisfies the framework interfaces.
var (
	_ resource.Resource                = &mcpGatewayGroupResource{}
	_ resource.ResourceWithConfigure   = &mcpGatewayGroupResource{}
	_ resource.ResourceWithImportState = &mcpGatewayGroupResource{}
)

// NewMCPGatewayGroupResource is the constructor registered with the provider.
func NewMCPGatewayGroupResource() resource.Resource {
	return &mcpGatewayGroupResource{}
}

type mcpGatewayGroupResource struct {
	client *client.Client
}

// mcpGatewayGroupResourceModel maps the agentops_mcp_gateway_group schema to Go.
type mcpGatewayGroupResourceModel struct {
	ID              types.String `tfsdk:"id"`
	Name            types.String `tfsdk:"name"`
	Allow           types.List   `tfsdk:"allow"`
	Deny            types.List   `tfsdk:"deny"`
	MemberServerIDs types.List   `tfsdk:"member_server_ids"`
}

func (r *mcpGatewayGroupResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_mcp_gateway_group"
}

func (r *mcpGatewayGroupResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "An MCP gateway group that bundles member servers under a shared allow/deny policy.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Group identifier.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Unique group name; used in the `/mcp/g/{group}` route and as the policy key.",
				Required:            true,
			},
			"allow": schema.ListAttribute{
				MarkdownDescription: "Group-level allow globs, applied on top of each member server's rules (empty = inherit server). Can only narrow: a tool the owning server denies stays denied even if a group allow matches it.",
				ElementType:         types.StringType,
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.List{listplanmodifier.UseStateForUnknown()},
			},
			"deny": schema.ListAttribute{
				MarkdownDescription: "Group-level deny globs, layered on top of server rules; deny wins.",
				ElementType:         types.StringType,
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.List{listplanmodifier.UseStateForUnknown()},
			},
			"member_server_ids": schema.ListAttribute{
				MarkdownDescription: "IDs of the MCP servers that belong to this group.",
				ElementType:         types.StringType,
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.List{listplanmodifier.UseStateForUnknown()},
			},
		},
	}
}

func (r *mcpGatewayGroupResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromProviderData(req.ProviderData, &resp.Diagnostics)
}

func (r *mcpGatewayGroupResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan mcpGatewayGroupResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := gen.GroupCreate{
		Name: plan.Name.ValueString(),
	}
	resp.Diagnostics.Append(listToStringSlice(ctx, plan.Allow, &body.Allow)...)
	resp.Diagnostics.Append(listToStringSlice(ctx, plan.Deny, &body.Deny)...)
	resp.Diagnostics.Append(listToStringSlice(ctx, plan.MemberServerIDs, &body.MemberServerIds)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.Gen.GatewayAdminCreateGroupWithResponse(ctx, body)
	if err != nil {
		resp.Diagnostics.AddError("Error creating MCP gateway group", err.Error())
		return
	}
	if err := client.Check(apiResp.HTTPResponse, apiResp.Body); err != nil {
		resp.Diagnostics.AddError("Error creating MCP gateway group", err.Error())
		return
	}
	if apiResp.JSON201 == nil {
		resp.Diagnostics.AddError("Error creating MCP gateway group", "API returned an empty body")
		return
	}

	resp.Diagnostics.Append(mcpGroupRecordToState(ctx, apiResp.JSON201, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *mcpGatewayGroupResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state mcpGatewayGroupResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.Gen.GatewayAdminGetGroupWithResponse(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading MCP gateway group", err.Error())
		return
	}
	if err := client.Check(apiResp.HTTPResponse, apiResp.Body); err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading MCP gateway group", err.Error())
		return
	}
	if apiResp.JSON200 == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	resp.Diagnostics.Append(mcpGroupRecordToState(ctx, apiResp.JSON200, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *mcpGatewayGroupResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan mcpGatewayGroupResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := gen.GroupPatch{
		Name: stringToPtr(plan.Name),
	}
	resp.Diagnostics.Append(listToStringSlice(ctx, plan.Allow, &body.Allow)...)
	resp.Diagnostics.Append(listToStringSlice(ctx, plan.Deny, &body.Deny)...)
	resp.Diagnostics.Append(listToStringSlice(ctx, plan.MemberServerIDs, &body.MemberServerIds)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.Gen.GatewayAdminUpdateGroupWithResponse(ctx, plan.ID.ValueString(), body)
	if err != nil {
		resp.Diagnostics.AddError("Error updating MCP gateway group", err.Error())
		return
	}
	if err := client.Check(apiResp.HTTPResponse, apiResp.Body); err != nil {
		resp.Diagnostics.AddError("Error updating MCP gateway group", err.Error())
		return
	}
	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError("Error updating MCP gateway group", "API returned an empty body")
		return
	}

	resp.Diagnostics.Append(mcpGroupRecordToState(ctx, apiResp.JSON200, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *mcpGatewayGroupResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state mcpGatewayGroupResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.Gen.GatewayAdminDeleteGroupWithResponse(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error deleting MCP gateway group", err.Error())
		return
	}
	if err := client.Check(apiResp.HTTPResponse, apiResp.Body); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Error deleting MCP gateway group", err.Error())
	}
}

func (r *mcpGatewayGroupResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// mcpGroupRecordToState maps a GroupRecord API response onto the model.
func mcpGroupRecordToState(ctx context.Context, g *gen.GroupRecord, m *mcpGatewayGroupResourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.StringValue(g.Id)
	m.Name = types.StringValue(g.Name)

	m.Allow, diags = mcpGroupStringList(ctx, g.Allow)
	if diags.HasError() {
		return diags
	}
	m.Deny, diags = mcpGroupStringList(ctx, g.Deny)
	if diags.HasError() {
		return diags
	}
	m.MemberServerIDs, diags = mcpGroupStringList(ctx, g.MemberServerIds)
	return diags
}

// mcpGroupStringList converts an optional API string slice into a Terraform
// list, mapping nil to a typed null list.
func mcpGroupStringList(ctx context.Context, s *[]string) (types.List, diag.Diagnostics) {
	if s == nil {
		return types.ListNull(types.StringType), nil
	}
	return types.ListValueFrom(ctx, types.StringType, *s)
}
