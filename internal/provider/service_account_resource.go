// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"

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

var (
	_ resource.Resource                = &serviceAccountResource{}
	_ resource.ResourceWithConfigure   = &serviceAccountResource{}
	_ resource.ResourceWithImportState = &serviceAccountResource{}
)

// NewServiceAccountResource is the constructor registered with the provider.
func NewServiceAccountResource() resource.Resource {
	return &serviceAccountResource{}
}

type serviceAccountResource struct {
	client *client.Client
}

// serviceAccountResourceModel maps the agentops_service_account schema to Go.
// The service account collection has no update endpoint, so every input is
// create-only (ForceNew).
type serviceAccountResourceModel struct {
	ID          types.String `tfsdk:"id"`
	DisplayName types.String `tfsdk:"display_name"`
	RoleIDs     types.List   `tfsdk:"role_ids"`
	Source      types.String `tfsdk:"source"`
	Status      types.String `tfsdk:"status"`
	CreatedAt   types.String `tfsdk:"created_at"`
}

func (r *serviceAccountResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_service_account"
}

func (r *serviceAccountResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	// The service account collection is create/delete only: no PATCH/PUT exists,
	// so any change to an input attribute must replace the service account.
	forceNewString := []planmodifier.String{stringplanmodifier.RequiresReplace()}
	forceNewList := []planmodifier.List{listplanmodifier.RequiresReplace()}

	resp.Schema = schema.Schema{
		MarkdownDescription: "An AgentOps service account. Service accounts cannot be updated in place; " +
			"changing any argument forces a new service account.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Service account identifier (principal ID).",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"display_name": schema.StringAttribute{
				MarkdownDescription: "Human-readable service account name.",
				Required:            true,
				PlanModifiers:       forceNewString,
			},
			"role_ids": schema.ListAttribute{
				MarkdownDescription: "Role IDs to grant the service account. Not returned by the API, so it is not refreshed after creation.",
				ElementType:         types.StringType,
				Optional:            true,
				PlanModifiers:       forceNewList,
			},
			"source": schema.StringAttribute{
				MarkdownDescription: "Origin of the service account.",
				Computed:            true,
			},
			"status": schema.StringAttribute{
				MarkdownDescription: "Service account status.",
				Computed:            true,
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "Creation timestamp.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
		},
	}
}

func (r *serviceAccountResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromProviderData(req.ProviderData, &resp.Diagnostics)
}

func (r *serviceAccountResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan serviceAccountResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := gen.CreateServiceAccountRequest{
		DisplayName: plan.DisplayName.ValueString(),
	}
	resp.Diagnostics.Append(listToStringSlice(ctx, plan.RoleIDs, &body.RoleIds)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.Gen.ServiceAccountsCreateServiceAccountEndpointWithResponse(ctx, body)
	if err != nil {
		resp.Diagnostics.AddError("Error creating service account", err.Error())
		return
	}
	if err := client.Check(apiResp.HTTPResponse, apiResp.Body); err != nil {
		resp.Diagnostics.AddError("Error creating service account", err.Error())
		return
	}
	if apiResp.JSON201 == nil {
		resp.Diagnostics.AddError("Error creating service account", "API returned an empty body")
		return
	}

	sa := apiResp.JSON201
	plan.ID = types.StringValue(sa.PrincipalId)
	plan.DisplayName = types.StringValue(sa.DisplayName)
	plan.Source = types.StringValue(string(sa.Source))
	plan.Status = types.StringValue(sa.Status)
	plan.CreatedAt = types.StringValue(sa.CreatedAt)

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *serviceAccountResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state serviceAccountResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// There is no GET-by-id endpoint; list and match. role_ids is not returned,
	// so it is preserved from state.
	apiResp, err := r.client.Gen.ServiceAccountsListServiceAccountsEndpointWithResponse(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error reading service account", err.Error())
		return
	}
	if err := client.Check(apiResp.HTTPResponse, apiResp.Body); err != nil {
		resp.Diagnostics.AddError("Error reading service account", err.Error())
		return
	}
	if apiResp.JSON200 == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	var found *gen.ServiceAccountView
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

	state.DisplayName = types.StringValue(found.DisplayName)
	state.Source = types.StringValue(string(found.Source))
	state.Status = types.StringValue(found.Status)
	state.CreatedAt = types.StringValue(found.CreatedAt)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update should be unreachable: every input attribute is RequiresReplace, so
// Terraform replaces the service account rather than updating it. Implemented to
// satisfy the resource.Resource interface.
func (r *serviceAccountResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError(
		"Service accounts cannot be updated",
		"This is a bug in the provider: an update was attempted on an immutable resource.",
	)
}

func (r *serviceAccountResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state serviceAccountResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.Gen.ServiceAccountsDisableServiceAccountEndpointWithResponse(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error disabling service account", err.Error())
		return
	}
	if err := client.Check(apiResp.HTTPResponse, apiResp.Body); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Error disabling service account", err.Error())
	}
}

func (r *serviceAccountResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
