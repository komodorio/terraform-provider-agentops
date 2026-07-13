// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/komodorio/terraform-provider-agentops/internal/client"
	"github.com/komodorio/terraform-provider-agentops/internal/client/gen"
)

var (
	_ resource.Resource                = &memberResource{}
	_ resource.ResourceWithConfigure   = &memberResource{}
	_ resource.ResourceWithImportState = &memberResource{}
)

// NewMemberResource is the constructor registered with the provider.
func NewMemberResource() resource.Resource {
	return &memberResource{}
}

type memberResource struct {
	client *client.Client
}

// memberResourceModel maps the agentops_member schema to Go. The member
// collection has no update endpoint, so every input is create-only (ForceNew).
type memberResourceModel struct {
	ID          types.String `tfsdk:"id"`
	Email       types.String `tfsdk:"email"`
	FullName    types.String `tfsdk:"full_name"`
	DisplayName types.String `tfsdk:"display_name"`
	Status      types.String `tfsdk:"status"`
	UserID      types.String `tfsdk:"user_id"`
	CreatedAt   types.String `tfsdk:"created_at"`
	LastLoginAt types.String `tfsdk:"last_login_at"`
}

func (r *memberResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_member"
}

func (r *memberResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	// The member collection is invite/remove only: no PATCH/PUT exists, so any
	// change to an input attribute must replace the member.
	forceNewString := []planmodifier.String{stringplanmodifier.RequiresReplace()}

	resp.Schema = schema.Schema{
		MarkdownDescription: "An AgentOps member. Members cannot be updated in place; " +
			"changing any argument forces a new member.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Member identifier (principal ID).",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"email": schema.StringAttribute{
				MarkdownDescription: "Email address of the member to invite.",
				Required:            true,
				PlanModifiers:       forceNewString,
			},
			"full_name": schema.StringAttribute{
				MarkdownDescription: "Full name of the member.",
				Optional:            true,
				PlanModifiers:       forceNewString,
			},
			"display_name": schema.StringAttribute{
				MarkdownDescription: "Human-readable member name.",
				Computed:            true,
			},
			"status": schema.StringAttribute{
				MarkdownDescription: "Member status.",
				Computed:            true,
			},
			"user_id": schema.StringAttribute{
				MarkdownDescription: "User identifier associated with the member.",
				Computed:            true,
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "Creation timestamp.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"last_login_at": schema.StringAttribute{
				MarkdownDescription: "Timestamp of the member's last login.",
				Computed:            true,
			},
		},
	}
}

func (r *memberResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromProviderData(req.ProviderData, &resp.Diagnostics)
}

func (r *memberResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan memberResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := gen.InviteMemberRequest{
		Email:    openapi_types.Email(plan.Email.ValueString()),
		FullName: stringToPtr(plan.FullName),
	}

	apiResp, err := r.client.Gen.MembersInviteMemberEndpointWithResponse(ctx, body)
	if err != nil {
		resp.Diagnostics.AddError("Error inviting member", err.Error())
		return
	}
	if err := client.Check(apiResp.HTTPResponse, apiResp.Body); err != nil {
		resp.Diagnostics.AddError("Error inviting member", err.Error())
		return
	}
	if apiResp.JSON201 == nil {
		resp.Diagnostics.AddError("Error inviting member", "API returned an empty body")
		return
	}

	memberSetState(&plan, apiResp.JSON201)

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *memberResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state memberResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// There is no GET-by-id endpoint; list and match on the principal ID.
	apiResp, err := r.client.Gen.MembersListMembersEndpointWithResponse(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error reading member", err.Error())
		return
	}
	if err := client.Check(apiResp.HTTPResponse, apiResp.Body); err != nil {
		resp.Diagnostics.AddError("Error reading member", err.Error())
		return
	}
	if apiResp.JSON200 == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	var found *gen.MemberView
	for i := range *apiResp.JSON200 {
		if (*apiResp.JSON200)[i].PrincipalId == state.ID.ValueString() {
			found = &(*apiResp.JSON200)[i]
			break
		}
	}
	if found == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	memberSetState(&state, found)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update should be unreachable: every input attribute is RequiresReplace, so
// Terraform replaces the member rather than updating it. Implemented to satisfy
// the resource.Resource interface.
func (r *memberResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError(
		"Members cannot be updated",
		"This is a bug in the provider: an update was attempted on an immutable resource.",
	)
}

func (r *memberResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state memberResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.Gen.MembersRemoveMemberEndpointWithResponse(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error removing member", err.Error())
		return
	}
	if err := client.Check(apiResp.HTTPResponse, apiResp.Body); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Error removing member", err.Error())
	}
}

func (r *memberResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// memberSetState refreshes the computed and echoed attributes on the model from
// a MemberView. full_name is preserved from the model when the API omits it.
func memberSetState(m *memberResourceModel, v *gen.MemberView) {
	m.ID = types.StringValue(v.PrincipalId)
	m.Email = types.StringValue(v.Email)
	m.DisplayName = types.StringValue(v.DisplayName)
	m.Status = types.StringValue(v.Status)
	m.UserID = types.StringValue(v.UserId)
	m.CreatedAt = types.StringValue(v.CreatedAt)
	m.LastLoginAt = ptrToString(v.LastLoginAt)
	if v.FullName != nil {
		m.FullName = types.StringValue(*v.FullName)
	}
}
