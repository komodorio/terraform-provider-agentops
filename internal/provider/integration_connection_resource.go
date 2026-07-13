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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/komodorio/terraform-provider-agentops/internal/client"
	"github.com/komodorio/terraform-provider-agentops/internal/client/gen"
)

// Ensure the resource satisfies the framework interfaces.
var (
	_ resource.Resource                = &integrationConnectionResource{}
	_ resource.ResourceWithConfigure   = &integrationConnectionResource{}
	_ resource.ResourceWithImportState = &integrationConnectionResource{}
)

// NewIntegrationConnectionResource is the constructor registered with the provider.
func NewIntegrationConnectionResource() resource.Resource {
	return &integrationConnectionResource{}
}

type integrationConnectionResource struct {
	client *client.Client
}

// integrationConnectionResourceModel maps the agentops_integration_connection
// schema to Go. The connection collection has no update endpoint, so every input
// is create-only (ForceNew).
type integrationConnectionResourceModel struct {
	ID                   types.String         `tfsdk:"id"`
	Provider             types.String         `tfsdk:"provider_key"`
	DisplayName          types.String         `tfsdk:"display_name"`
	Credentials          types.Map            `tfsdk:"credentials"`
	Metadata             jsontypes.Normalized `tfsdk:"metadata"`
	AuthConfigKey        types.String         `tfsdk:"auth_config_key"`
	ExternalConnectionID types.String         `tfsdk:"external_connection_id"`
	Status               types.String         `tfsdk:"status"`
	CreatedAt            types.String         `tfsdk:"created_at"`
	UpdatedAt            types.String         `tfsdk:"updated_at"`
	CreatedBy            types.String         `tfsdk:"created_by"`
	LastUsedAt           types.String         `tfsdk:"last_used_at"`
	LastValidatedAt      types.String         `tfsdk:"last_validated_at"`
	CredentialsType      types.String         `tfsdk:"credentials_type"`
	Scopes               types.List           `tfsdk:"scopes"`
}

func (r *integrationConnectionResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_integration_connection"
}

func (r *integrationConnectionResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "An AgentOps integration connection. Connections cannot be updated in place: " +
			"changing any argument forces a new connection. Credentials are write-only and never returned by the API.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Integration connection identifier.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"provider_key": schema.StringAttribute{
				MarkdownDescription: "Integration provider key (e.g. the catalog provider identifier). Changing this forces a new connection.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"display_name": schema.StringAttribute{
				MarkdownDescription: "Human-readable connection name. Changing this forces a new connection.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"credentials": schema.MapAttribute{
				MarkdownDescription: "Provider credentials as string key/value pairs. Write-only: never returned by " +
					"the API, so it is not refreshed after creation. Changing this forces a new connection.",
				ElementType:   types.StringType,
				Optional:      true,
				Sensitive:     true,
				PlanModifiers: []planmodifier.Map{mapplanmodifier.RequiresReplace()},
			},
			"metadata": schema.StringAttribute{
				CustomType: jsontypes.NormalizedType{},
				MarkdownDescription: "Arbitrary connection metadata, encoded as a JSON object (JSON). " +
					"Changing this forces a new connection.",
				Optional:      true,
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"auth_config_key": schema.StringAttribute{
				MarkdownDescription: "Auth configuration key assigned to the connection.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"external_connection_id": schema.StringAttribute{
				MarkdownDescription: "Provider-side connection identifier.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"status": schema.StringAttribute{
				MarkdownDescription: "Connection status.",
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
			"created_by": schema.StringAttribute{
				MarkdownDescription: "Identifier of the principal that created the connection.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"last_used_at": schema.StringAttribute{
				MarkdownDescription: "Timestamp the connection was last used.",
				Computed:            true,
			},
			"last_validated_at": schema.StringAttribute{
				MarkdownDescription: "Timestamp the connection was last validated.",
				Computed:            true,
			},
			"credentials_type": schema.StringAttribute{
				MarkdownDescription: "Type of credentials backing the connection.",
				Computed:            true,
			},
			"scopes": schema.ListAttribute{
				MarkdownDescription: "Scopes granted to the connection.",
				ElementType:         types.StringType,
				Computed:            true,
				PlanModifiers:       []planmodifier.List{listplanmodifier.UseStateForUnknown()},
			},
		},
	}
}

func (r *integrationConnectionResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromProviderData(req.ProviderData, &resp.Diagnostics)
}

func (r *integrationConnectionResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan integrationConnectionResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := gen.CreateIntegrationConnectionRequest{
		Provider:    plan.Provider.ValueString(),
		DisplayName: plan.DisplayName.ValueString(),
	}
	integrationConnectionCredentialsToRequest(ctx, plan.Credentials, &body.Credentials, &resp.Diagnostics)
	integrationConnectionMetadataToRequest(plan.Metadata, &body.Metadata, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.Gen.IntegrationsCreateConnectionWithResponse(ctx, body)
	if err != nil {
		resp.Diagnostics.AddError("Error creating integration connection", err.Error())
		return
	}
	if err := client.Check(apiResp.HTTPResponse, apiResp.Body); err != nil {
		resp.Diagnostics.AddError("Error creating integration connection", err.Error())
		return
	}
	if apiResp.JSON201 == nil {
		resp.Diagnostics.AddError("Error creating integration connection", "API returned an empty body")
		return
	}

	// The create response is minimal (id, auth_config_key, status). Fetch the full
	// detail by the new id so every computed field is populated in state.
	detail := r.getConnection(ctx, apiResp.JSON201.ConnectionId, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	if detail == nil {
		resp.Diagnostics.AddError(
			"Error creating integration connection",
			"connection was created but could not be read back by id "+apiResp.JSON201.ConnectionId,
		)
		return
	}

	// Credentials are write-only; preserve the value from the plan.
	integrationConnectionApplyDetail(ctx, &plan, detail, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *integrationConnectionResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state integrationConnectionResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.Gen.IntegrationsGetConnectionWithResponse(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading integration connection", err.Error())
		return
	}
	if err := client.Check(apiResp.HTTPResponse, apiResp.Body); err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading integration connection", err.Error())
		return
	}
	if apiResp.JSON200 == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	// Credentials are never returned; preserve the existing state.
	integrationConnectionApplyDetail(ctx, &state, apiResp.JSON200, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update should be unreachable: every input attribute is RequiresReplace, so
// Terraform replaces the connection rather than updating it. Implemented to
// satisfy the resource.Resource interface.
func (r *integrationConnectionResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError(
		"Integration connections cannot be updated",
		"This is a bug in the provider: an update was attempted on an immutable resource.",
	)
}

func (r *integrationConnectionResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state integrationConnectionResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.Gen.IntegrationsDeleteConnectionWithResponse(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error deleting integration connection", err.Error())
		return
	}
	if err := client.Check(apiResp.HTTPResponse, apiResp.Body); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Error deleting integration connection", err.Error())
	}
}

func (r *integrationConnectionResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// getConnection fetches a connection detail by id, returning nil when the API
// reports it as not found. Any other error is recorded in diags.
func (r *integrationConnectionResource) getConnection(ctx context.Context, id string, diags *diag.Diagnostics) *gen.IntegrationConnectionDetail {
	apiResp, err := r.client.Gen.IntegrationsGetConnectionWithResponse(ctx, id)
	if err != nil {
		diags.AddError("Error reading integration connection", err.Error())
		return nil
	}
	if err := client.Check(apiResp.HTTPResponse, apiResp.Body); err != nil {
		if client.IsNotFound(err) {
			return nil
		}
		diags.AddError("Error reading integration connection", err.Error())
		return nil
	}
	return apiResp.JSON200
}

// integrationConnectionApplyDetail writes the fields returned by the get-by-id
// endpoint into the model. Credentials are deliberately not touched here because
// the API never returns them; callers preserve the value from plan/state.
func integrationConnectionApplyDetail(ctx context.Context, m *integrationConnectionResourceModel, d *gen.IntegrationConnectionDetail, diags *diag.Diagnostics) {
	m.ID = types.StringValue(d.Id)
	m.Provider = types.StringValue(d.Provider)
	m.DisplayName = types.StringValue(d.DisplayName)
	m.AuthConfigKey = types.StringValue(d.AuthConfigKey)
	m.ExternalConnectionID = types.StringValue(d.ExternalConnectionId)
	m.Status = types.StringValue(string(d.Status))
	m.CreatedAt = types.StringValue(d.CreatedAt)
	m.UpdatedAt = types.StringValue(d.UpdatedAt)
	m.CreatedBy = ptrToString(d.CreatedBy)
	m.LastUsedAt = ptrToString(d.LastUsedAt)
	m.LastValidatedAt = ptrToString(d.LastValidatedAt)
	m.CredentialsType = ptrToString(d.CredentialsType)
	m.Scopes = integrationConnectionScopesToModel(ctx, d.Scopes, diags)
	m.Metadata = integrationConnectionMetadataToModel(d.Metadata, diags)
}

// integrationConnectionCredentialsToRequest copies the credentials map into the
// request field, leaving it nil (omitted) when the map is null or unknown.
func integrationConnectionCredentialsToRequest(ctx context.Context, creds types.Map, target **map[string]string, diags *diag.Diagnostics) {
	if creds.IsNull() || creds.IsUnknown() {
		return
	}
	m := map[string]string{}
	diags.Append(creds.ElementsAs(ctx, &m, false)...)
	if diags.HasError() {
		return
	}
	*target = &m
}

// integrationConnectionMetadataToRequest decodes the metadata JSON string into a
// generic map for the request body, leaving the field nil (omitted) when null or
// unknown.
func integrationConnectionMetadataToRequest(v jsontypes.Normalized, target **map[string]interface{}, diags *diag.Diagnostics) {
	if v.IsNull() || v.IsUnknown() {
		return
	}
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(v.ValueString()), &m); err != nil {
		diags.AddError("Invalid metadata JSON", err.Error())
		return
	}
	*target = &m
}

// integrationConnectionMetadataToModel renders an optional API metadata map as a
// normalized JSON string, mapping nil to a null value.
func integrationConnectionMetadataToModel(m *map[string]interface{}, diags *diag.Diagnostics) jsontypes.Normalized {
	if m == nil {
		return jsontypes.NewNormalizedNull()
	}
	b, err := json.Marshal(*m)
	if err != nil {
		diags.AddError("Error encoding metadata", err.Error())
		return jsontypes.NewNormalizedNull()
	}
	return jsontypes.NewNormalizedValue(string(b))
}

// integrationConnectionScopesToModel converts an optional API scopes slice into a
// Terraform list, mapping nil to a null list.
func integrationConnectionScopesToModel(ctx context.Context, scopes *[]string, diags *diag.Diagnostics) types.List {
	if scopes == nil {
		return types.ListNull(types.StringType)
	}
	list, d := types.ListValueFrom(ctx, types.StringType, *scopes)
	diags.Append(d...)
	return list
}
