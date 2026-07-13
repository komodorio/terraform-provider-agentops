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
	_ resource.Resource                = &apiKeyResource{}
	_ resource.ResourceWithConfigure   = &apiKeyResource{}
	_ resource.ResourceWithImportState = &apiKeyResource{}
)

// NewAPIKeyResource is the constructor registered with the provider.
func NewAPIKeyResource() resource.Resource {
	return &apiKeyResource{}
}

type apiKeyResource struct {
	client *client.Client
}

// apiKeyResourceModel maps the agentops_api_key schema to Go. The API key
// collection has no update endpoint, so every input is create-only (ForceNew).
type apiKeyResourceModel struct {
	ID               types.String `tfsdk:"id"`
	Name             types.String `tfsdk:"name"`
	BoundTo          types.String `tfsdk:"bound_to"`
	ExpiresAt        types.String `tfsdk:"expires_at"`
	RoleIDs          types.List   `tfsdk:"role_ids"`
	Scopes           types.List   `tfsdk:"scopes"`
	ServiceAccountID types.String `tfsdk:"service_account_id"`
	Key              types.String `tfsdk:"key"`
	Status           types.String `tfsdk:"status"`
	PrincipalID      types.String `tfsdk:"principal_id"`
	CreatedAt        types.String `tfsdk:"created_at"`
}

func (r *apiKeyResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_api_key"
}

func (r *apiKeyResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	// The API key collection is create/delete only: no PATCH/PUT exists, so any
	// change to an input attribute must replace the key.
	forceNewString := []planmodifier.String{stringplanmodifier.RequiresReplace()}
	forceNewList := []planmodifier.List{listplanmodifier.RequiresReplace()}

	resp.Schema = schema.Schema{
		MarkdownDescription: "An AgentOps API key. Keys cannot be updated in place; changing any argument " +
			"forces a new key. The secret is returned only once, at creation.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "API key identifier.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Human-readable key name.",
				Required:            true,
				PlanModifiers:       forceNewString,
			},
			"bound_to": schema.StringAttribute{
				MarkdownDescription: "Principal kind the key acts as: `user` or `service_account`. " +
					"Derived from the bound principal when omitted.",
				Optional:      true,
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace(), stringplanmodifier.UseStateForUnknown()},
			},
			"expires_at": schema.StringAttribute{
				MarkdownDescription: "Expiry timestamp (RFC 3339). Omit for a non-expiring key.",
				Optional:            true,
				PlanModifiers:       forceNewString,
			},
			"role_ids": schema.ListAttribute{
				MarkdownDescription: "Role IDs to grant the key. Not returned by the API, so it is not refreshed after creation.",
				ElementType:         types.StringType,
				Optional:            true,
				PlanModifiers:       forceNewList,
			},
			"scopes": schema.ListAttribute{
				MarkdownDescription: "Scopes granted to the key.",
				ElementType:         types.StringType,
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.List{listplanmodifier.RequiresReplace(), listplanmodifier.UseStateForUnknown()},
			},
			"service_account_id": schema.StringAttribute{
				MarkdownDescription: "Service account to bind the key to. Not returned by the API, so it is not refreshed after creation.",
				Optional:            true,
				PlanModifiers:       forceNewString,
			},
			"key": schema.StringAttribute{
				MarkdownDescription: "The secret key value. Returned only at creation; store it securely.",
				Computed:            true,
				Sensitive:           true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"status": schema.StringAttribute{
				MarkdownDescription: "Key status (`active` or `revoked`).",
				Computed:            true,
			},
			"principal_id": schema.StringAttribute{
				MarkdownDescription: "ID of the principal the key authenticates as.",
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

func (r *apiKeyResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromProviderData(req.ProviderData, &resp.Diagnostics)
}

func (r *apiKeyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan apiKeyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := gen.CreateApiKeyRequest{
		Name:             plan.Name.ValueString(),
		ExpiresAt:        stringToPtr(plan.ExpiresAt),
		ServiceAccountId: stringToPtr(plan.ServiceAccountID),
	}
	if !plan.BoundTo.IsNull() && !plan.BoundTo.IsUnknown() {
		bt := gen.ApiKeyBoundTo(plan.BoundTo.ValueString())
		body.BoundTo = &bt
	}
	resp.Diagnostics.Append(listToStringSlice(ctx, plan.RoleIDs, &body.RoleIds)...)
	resp.Diagnostics.Append(listToStringSlice(ctx, plan.Scopes, &body.Scopes)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.Gen.ApiKeysCreateApiKeyEndpointWithResponse(ctx, body)
	if err != nil {
		resp.Diagnostics.AddError("Error creating API key", err.Error())
		return
	}
	if err := client.Check(apiResp.HTTPResponse, apiResp.Body); err != nil {
		resp.Diagnostics.AddError("Error creating API key", err.Error())
		return
	}
	if apiResp.JSON201 == nil {
		resp.Diagnostics.AddError("Error creating API key", "API returned an empty body")
		return
	}

	k := apiResp.JSON201
	plan.ID = types.StringValue(k.Id)
	plan.Key = types.StringValue(k.Key)
	plan.Status = types.StringValue(string(k.Status))
	plan.BoundTo = types.StringValue(string(k.BoundTo))
	plan.PrincipalID = types.StringValue(k.PrincipalId)
	plan.CreatedAt = types.StringValue(k.CreatedAt)
	plan.ExpiresAt = ptrToString(k.ExpiresAt)
	scopes, diags := types.ListValueFrom(ctx, types.StringType, k.Scopes)
	resp.Diagnostics.Append(diags...)
	plan.Scopes = scopes

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *apiKeyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state apiKeyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// There is no GET-by-id endpoint; list and match. Key, role_ids and
	// service_account_id are not returned, so they are preserved from state.
	apiResp, err := r.client.Gen.ApiKeysListApiKeysEndpointWithResponse(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error reading API key", err.Error())
		return
	}
	if err := client.Check(apiResp.HTTPResponse, apiResp.Body); err != nil {
		resp.Diagnostics.AddError("Error reading API key", err.Error())
		return
	}
	if apiResp.JSON200 == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	var found *gen.ApiKeyView
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

	state.Name = types.StringValue(found.Name)
	state.Status = types.StringValue(string(found.Status))
	state.BoundTo = types.StringValue(string(found.BoundTo))
	state.PrincipalID = types.StringValue(found.PrincipalId)
	state.CreatedAt = types.StringValue(found.CreatedAt)
	state.ExpiresAt = ptrToString(found.ExpiresAt)
	scopes, diags := types.ListValueFrom(ctx, types.StringType, found.Scopes)
	resp.Diagnostics.Append(diags...)
	state.Scopes = scopes

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update should be unreachable: every input attribute is RequiresReplace, so
// Terraform replaces the key rather than updating it. Implemented to satisfy the
// resource.Resource interface.
func (r *apiKeyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError(
		"API keys cannot be updated",
		"This is a bug in the provider: an update was attempted on an immutable resource.",
	)
}

func (r *apiKeyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state apiKeyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.Gen.ApiKeysRevokeApiKeyEndpointWithResponse(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error revoking API key", err.Error())
		return
	}
	if err := client.Check(apiResp.HTTPResponse, apiResp.Body); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Error revoking API key", err.Error())
	}
}

func (r *apiKeyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
