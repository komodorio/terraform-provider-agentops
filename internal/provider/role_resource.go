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
	_ resource.Resource                = &roleResource{}
	_ resource.ResourceWithConfigure   = &roleResource{}
	_ resource.ResourceWithImportState = &roleResource{}
)

// NewRoleResource is the constructor registered with the provider.
func NewRoleResource() resource.Resource {
	return &roleResource{}
}

type roleResource struct {
	client *client.Client
}

// roleResourceModel maps the agentops_role schema to Go. builtin and holders are
// server-derived and only appear on the record.
type roleResourceModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	PolicyIds   types.List   `tfsdk:"policy_ids"`
	Builtin     types.Bool   `tfsdk:"builtin"`
	Holders     types.Int64  `tfsdk:"holders"`
}

func (r *roleResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_role"
}

func (r *roleResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "An authorization role: a named grouping of policies that can be granted to holders.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Role identifier.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Human-readable role name.",
				Required:            true,
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Free-form role description.",
				Optional:            true,
			},
			"policy_ids": schema.ListAttribute{
				MarkdownDescription: "Identifiers of the policies attached to this role.",
				ElementType:         types.StringType,
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.List{listplanmodifier.UseStateForUnknown()},
			},
			"builtin": schema.BoolAttribute{
				MarkdownDescription: "Whether the role is built in and managed by the system.",
				Computed:            true,
			},
			"holders": schema.Int64Attribute{
				MarkdownDescription: "Number of holders currently granted this role.",
				Computed:            true,
			},
		},
	}
}

func (r *roleResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromProviderData(req.ProviderData, &resp.Diagnostics)
}

func (r *roleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan roleResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := gen.RoleCreate{
		Name:        plan.Name.ValueString(),
		Description: stringToPtr(plan.Description),
	}
	resp.Diagnostics.Append(listToStringSlice(ctx, plan.PolicyIds, &body.PolicyIds)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.Gen.AuthzCreateRoleWithResponse(ctx, body)
	if err != nil {
		resp.Diagnostics.AddError("Error creating role", err.Error())
		return
	}
	if err := client.Check(apiResp.HTTPResponse, apiResp.Body); err != nil {
		resp.Diagnostics.AddError("Error creating role", err.Error())
		return
	}
	if apiResp.JSON201 == nil {
		resp.Diagnostics.AddError("Error creating role", "API returned an empty body")
		return
	}

	resp.Diagnostics.Append(roleRecordToState(ctx, apiResp.JSON201, &plan)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *roleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state roleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// There is no GET-by-id endpoint; list and match on id.
	apiResp, err := r.client.Gen.AuthzListRolesWithResponse(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error reading role", err.Error())
		return
	}
	if err := client.Check(apiResp.HTTPResponse, apiResp.Body); err != nil {
		resp.Diagnostics.AddError("Error reading role", err.Error())
		return
	}
	if apiResp.JSON200 == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	var found *gen.AuthzRole
	for i := range *apiResp.JSON200 {
		if (*apiResp.JSON200)[i].Id == state.ID.ValueString() {
			found = &(*apiResp.JSON200)[i]
			break
		}
	}
	if found == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	resp.Diagnostics.Append(roleRecordToState(ctx, found, &state)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *roleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan roleResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := gen.RoleUpdate{
		Name:        stringToPtr(plan.Name),
		Description: stringToPtr(plan.Description),
	}
	resp.Diagnostics.Append(listToStringSlice(ctx, plan.PolicyIds, &body.PolicyIds)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.Gen.AuthzUpdateRoleWithResponse(ctx, plan.ID.ValueString(), body)
	if err != nil {
		resp.Diagnostics.AddError("Error updating role", err.Error())
		return
	}
	if err := client.Check(apiResp.HTTPResponse, apiResp.Body); err != nil {
		resp.Diagnostics.AddError("Error updating role", err.Error())
		return
	}
	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError("Error updating role", "API returned an empty body")
		return
	}

	resp.Diagnostics.Append(roleRecordToState(ctx, apiResp.JSON200, &plan)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *roleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state roleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.Gen.AuthzDeleteRoleWithResponse(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error deleting role", err.Error())
		return
	}
	if err := client.Check(apiResp.HTTPResponse, apiResp.Body); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Error deleting role", err.Error())
	}
}

func (r *roleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// roleRecordToState maps an AuthzRole onto the resource model.
func roleRecordToState(ctx context.Context, record *gen.AuthzRole, m *roleResourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.StringValue(record.Id)
	m.Name = types.StringValue(record.Name)
	m.Description = ptrToString(record.Description)
	m.Builtin = types.BoolValue(record.Builtin)
	m.Holders = types.Int64Value(int64(record.Holders))

	if record.PolicyIds == nil {
		m.PolicyIds = types.ListNull(types.StringType)
	} else {
		ids, d := types.ListValueFrom(ctx, types.StringType, *record.PolicyIds)
		diags.Append(d...)
		m.PolicyIds = ids
	}

	return diags
}
