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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/komodorio/terraform-provider-agentops/internal/client"
	"github.com/komodorio/terraform-provider-agentops/internal/client/gen"
)

// Ensure the resource satisfies the framework interfaces.
var (
	_ resource.Resource                = &mcpGatewayServerResource{}
	_ resource.ResourceWithConfigure   = &mcpGatewayServerResource{}
	_ resource.ResourceWithImportState = &mcpGatewayServerResource{}
)

// NewMCPGatewayServerResource is the constructor registered with the provider.
func NewMCPGatewayServerResource() resource.Resource {
	return &mcpGatewayServerResource{}
}

type mcpGatewayServerResource struct {
	client *client.Client
}

// mcpGatewayServerResourceModel maps the agentops_mcp_gateway_server schema to Go.
type mcpGatewayServerResourceModel struct {
	ID                types.String  `tfsdk:"id"`
	Name              types.String  `tfsdk:"name"`
	Url               types.String  `tfsdk:"url"`
	Allow             types.List    `tfsdk:"allow"`
	Deny              types.List    `tfsdk:"deny"`
	OauthScopes       types.List    `tfsdk:"oauth_scopes"`
	Tags              types.List    `tfsdk:"tags"`
	Labels            types.Map     `tfsdk:"labels"`
	StaticHeaders     types.Map     `tfsdk:"static_headers"`
	Enabled           types.Bool    `tfsdk:"enabled"`
	OauthClientID     types.String  `tfsdk:"oauth_client_id"`
	OauthTokenURL     types.String  `tfsdk:"oauth_token_url"`
	OauthClientSecret types.String  `tfsdk:"oauth_client_secret"`
	TimeoutSeconds    types.Float64 `tfsdk:"timeout_seconds"`
	Auth              types.String  `tfsdk:"auth"`
	Transport         types.String  `tfsdk:"transport"`
}

func (r *mcpGatewayServerResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_mcp_gateway_server"
}

func (r *mcpGatewayServerResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A registered upstream MCP server proxied by the gateway.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Server identifier.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Unique name; also used as the mount namespace/prefix. Letters, digits and hyphens only.",
				Required:            true,
			},
			"url": schema.StringAttribute{
				MarkdownDescription: "Endpoint URL for the upstream MCP server.",
				Required:            true,
			},
			"allow": schema.ListAttribute{
				MarkdownDescription: "Glob patterns of tool names to expose (empty = all).",
				ElementType:         types.StringType,
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.List{listplanmodifier.UseStateForUnknown()},
			},
			"deny": schema.ListAttribute{
				MarkdownDescription: "Glob patterns of tool names to hide (wins over allow).",
				ElementType:         types.StringType,
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.List{listplanmodifier.UseStateForUnknown()},
			},
			"oauth_scopes": schema.ListAttribute{
				MarkdownDescription: "OAuth scopes to request; empty = server/registration default.",
				ElementType:         types.StringType,
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.List{listplanmodifier.UseStateForUnknown()},
			},
			"tags": schema.ListAttribute{
				MarkdownDescription: "Free-form tags applied to the server.",
				ElementType:         types.StringType,
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.List{listplanmodifier.UseStateForUnknown()},
			},
			"labels": schema.MapAttribute{
				MarkdownDescription: "Arbitrary key/value labels applied to the server.",
				ElementType:         types.StringType,
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.Map{mapplanmodifier.UseStateForUnknown()},
			},
			"static_headers": schema.MapAttribute{
				MarkdownDescription: "Headers always sent upstream. Values may embed ${env:VAR} / ${file:path} secret references resolved at connect time.",
				ElementType:         types.StringType,
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.Map{mapplanmodifier.UseStateForUnknown()},
			},
			"enabled": schema.BoolAttribute{
				MarkdownDescription: "Whether the server is enabled.",
				Optional:            true,
				Computed:            true,
			},
			"oauth_client_id": schema.StringAttribute{
				MarkdownDescription: "OAuth client id for the client_credentials grant (not secret).",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"oauth_token_url": schema.StringAttribute{
				MarkdownDescription: "Token endpoint for the client_credentials grant.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"oauth_client_secret": schema.StringAttribute{
				MarkdownDescription: "OAuth client secret as a ${env:VAR} or ${file:path} reference, resolved at connect time. Raw secrets are rejected.",
				Optional:            true,
				Sensitive:           true,
			},
			"timeout_seconds": schema.Float64Attribute{
				MarkdownDescription: "Upstream connection timeout in seconds.",
				Optional:            true,
				Computed:            true,
			},
			"auth": schema.StringAttribute{
				MarkdownDescription: "Upstream authentication scheme (`none`, `oauth`, `oauth_client_credentials`).",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"transport": schema.StringAttribute{
				MarkdownDescription: "Transport used to reach the upstream MCP server (`http`, `sse`).",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
		},
	}
}

func (r *mcpGatewayServerResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromProviderData(req.ProviderData, &resp.Diagnostics)
}

func (r *mcpGatewayServerResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan mcpGatewayServerResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := gen.CreateServerRequest{
		Name:              plan.Name.ValueString(),
		Url:               plan.Url.ValueString(),
		Enabled:           boolToPtr(plan.Enabled),
		OauthClientId:     stringToPtr(plan.OauthClientID),
		OauthTokenUrl:     stringToPtr(plan.OauthTokenURL),
		OauthClientSecret: stringToPtr(plan.OauthClientSecret),
	}
	resp.Diagnostics.Append(listToStringSlice(ctx, plan.Allow, &body.Allow)...)
	resp.Diagnostics.Append(listToStringSlice(ctx, plan.Deny, &body.Deny)...)
	resp.Diagnostics.Append(listToStringSlice(ctx, plan.OauthScopes, &body.OauthScopes)...)
	resp.Diagnostics.Append(listToStringSlice(ctx, plan.Tags, &body.Tags)...)
	resp.Diagnostics.Append(mcpServerMapToPtr(ctx, plan.Labels, &body.Labels)...)
	resp.Diagnostics.Append(mcpServerMapToPtr(ctx, plan.StaticHeaders, &body.StaticHeaders)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if ts := mcpServerFloatToPtr(plan.TimeoutSeconds); ts != nil {
		body.TimeoutSeconds = ts
	}
	if !plan.Auth.IsNull() && !plan.Auth.IsUnknown() {
		a := gen.ServerAuth(plan.Auth.ValueString())
		body.Auth = &a
	}
	if !plan.Transport.IsNull() && !plan.Transport.IsUnknown() {
		t := gen.Transport(plan.Transport.ValueString())
		body.Transport = &t
	}

	apiResp, err := r.client.Gen.GatewayAdminCreateServerWithResponse(ctx, body)
	if err != nil {
		resp.Diagnostics.AddError("Error creating MCP gateway server", err.Error())
		return
	}
	if err := client.Check(apiResp.HTTPResponse, apiResp.Body); err != nil {
		resp.Diagnostics.AddError("Error creating MCP gateway server", err.Error())
		return
	}
	if apiResp.JSON201 == nil {
		resp.Diagnostics.AddError("Error creating MCP gateway server", "API returned an empty body")
		return
	}

	resp.Diagnostics.Append(mcpServerApply(ctx, &plan, apiResp.JSON201)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *mcpGatewayServerResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state mcpGatewayServerResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.Gen.GatewayAdminGetServerWithResponse(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading MCP gateway server", err.Error())
		return
	}
	if err := client.Check(apiResp.HTTPResponse, apiResp.Body); err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading MCP gateway server", err.Error())
		return
	}
	if apiResp.JSON200 == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	resp.Diagnostics.Append(mcpServerApply(ctx, &state, apiResp.JSON200)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *mcpGatewayServerResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan mcpGatewayServerResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := gen.UpdateServerRequest{
		Name:              stringToPtr(plan.Name),
		Url:               stringToPtr(plan.Url),
		Enabled:           boolToPtr(plan.Enabled),
		OauthClientId:     stringToPtr(plan.OauthClientID),
		OauthTokenUrl:     stringToPtr(plan.OauthTokenURL),
		OauthClientSecret: stringToPtr(plan.OauthClientSecret),
	}
	resp.Diagnostics.Append(listToStringSlice(ctx, plan.Allow, &body.Allow)...)
	resp.Diagnostics.Append(listToStringSlice(ctx, plan.Deny, &body.Deny)...)
	resp.Diagnostics.Append(listToStringSlice(ctx, plan.OauthScopes, &body.OauthScopes)...)
	resp.Diagnostics.Append(listToStringSlice(ctx, plan.Tags, &body.Tags)...)
	resp.Diagnostics.Append(mcpServerMapToPtr(ctx, plan.Labels, &body.Labels)...)
	resp.Diagnostics.Append(mcpServerMapToPtr(ctx, plan.StaticHeaders, &body.StaticHeaders)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if ts := mcpServerFloatToPtr(plan.TimeoutSeconds); ts != nil {
		body.TimeoutSeconds = ts
	}
	if !plan.Auth.IsNull() && !plan.Auth.IsUnknown() {
		a := gen.ServerAuth(plan.Auth.ValueString())
		body.Auth = &a
	}
	if !plan.Transport.IsNull() && !plan.Transport.IsUnknown() {
		t := gen.Transport(plan.Transport.ValueString())
		body.Transport = &t
	}

	apiResp, err := r.client.Gen.GatewayAdminUpdateServerWithResponse(ctx, plan.ID.ValueString(), body)
	if err != nil {
		resp.Diagnostics.AddError("Error updating MCP gateway server", err.Error())
		return
	}
	if err := client.Check(apiResp.HTTPResponse, apiResp.Body); err != nil {
		resp.Diagnostics.AddError("Error updating MCP gateway server", err.Error())
		return
	}
	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError("Error updating MCP gateway server", "API returned an empty body")
		return
	}

	resp.Diagnostics.Append(mcpServerApply(ctx, &plan, apiResp.JSON200)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *mcpGatewayServerResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state mcpGatewayServerResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.Gen.GatewayAdminDeleteServerWithResponse(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error deleting MCP gateway server", err.Error())
		return
	}
	if err := client.Check(apiResp.HTTPResponse, apiResp.Body); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Error deleting MCP gateway server", err.Error())
	}
}

func (r *mcpGatewayServerResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// mcpServerApply writes a LabeledServerRecord response into the model.
func mcpServerApply(ctx context.Context, m *mcpGatewayServerResourceModel, rec *gen.LabeledServerRecord) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.StringValue(rec.Id)
	m.Name = types.StringValue(rec.Name)
	m.Url = types.StringValue(rec.Url)
	m.Enabled = boolPtrToBool(rec.Enabled)
	m.OauthClientID = ptrToString(rec.OauthClientId)
	m.OauthTokenURL = ptrToString(rec.OauthTokenUrl)
	m.OauthClientSecret = ptrToString(rec.OauthClientSecret)
	m.Auth = strOrNull(enumPtrToString(rec.Auth))
	m.Transport = strOrNull(enumPtrToString(rec.Transport))

	if rec.TimeoutSeconds != nil {
		m.TimeoutSeconds = types.Float64Value(float64(*rec.TimeoutSeconds))
	} else {
		m.TimeoutSeconds = types.Float64Null()
	}

	allow, d := mcpServerStringListValue(ctx, rec.Allow)
	diags.Append(d...)
	m.Allow = allow

	deny, d := mcpServerStringListValue(ctx, rec.Deny)
	diags.Append(d...)
	m.Deny = deny

	scopes, d := mcpServerStringListValue(ctx, rec.OauthScopes)
	diags.Append(d...)
	m.OauthScopes = scopes

	tags, d := mcpServerStringListValue(ctx, rec.Tags)
	diags.Append(d...)
	m.Tags = tags

	labels, d := mcpServerStringMapValue(ctx, rec.Labels)
	diags.Append(d...)
	m.Labels = labels

	headers, d := mcpServerStringMapValue(ctx, rec.StaticHeaders)
	diags.Append(d...)
	m.StaticHeaders = headers

	return diags
}

// mcpServerMapToPtr converts an optional Terraform map into a *map[string]string
// request field, leaving it nil (omitted) when the map is null or unknown.
func mcpServerMapToPtr(ctx context.Context, m types.Map, target **map[string]string) diag.Diagnostics {
	if m.IsNull() || m.IsUnknown() {
		return nil
	}
	out := map[string]string{}
	diags := m.ElementsAs(ctx, &out, false)
	if diags.HasError() {
		return diags
	}
	*target = &out
	return diags
}

// mcpServerFloatToPtr converts a Terraform float into a *float32 request field,
// mapping null/unknown to nil so the field is omitted.
func mcpServerFloatToPtr(v types.Float64) *float32 {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	f := float32(v.ValueFloat64())
	return &f
}

// mcpServerStringListValue maps an optional API string slice to a Terraform
// list, mapping nil to a null list.
func mcpServerStringListValue(ctx context.Context, p *[]string) (types.List, diag.Diagnostics) {
	if p == nil {
		return types.ListNull(types.StringType), nil
	}
	return types.ListValueFrom(ctx, types.StringType, *p)
}

// mcpServerStringMapValue maps an optional API string map to a Terraform map,
// mapping nil to a null map.
func mcpServerStringMapValue(ctx context.Context, p *map[string]string) (types.Map, diag.Diagnostics) {
	if p == nil {
		return types.MapNull(types.StringType), nil
	}
	return types.MapValueFrom(ctx, types.StringType, *p)
}
