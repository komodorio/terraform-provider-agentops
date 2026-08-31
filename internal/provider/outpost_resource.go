// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/komodorio/terraform-provider-agentops/internal/client"
	"github.com/komodorio/terraform-provider-agentops/internal/client/gen"
)

var (
	_ resource.Resource                = &outpostResource{}
	_ resource.ResourceWithConfigure   = &outpostResource{}
	_ resource.ResourceWithImportState = &outpostResource{}
)

func NewOutpostResource() resource.Resource {
	return &outpostResource{}
}

type outpostResource struct {
	client *client.Client
}

type outpostResourceModel struct {
	ID             types.String `tfsdk:"id"`
	Name           types.String `tfsdk:"name"`
	Description    types.String `tfsdk:"description"`
	Allowlist      types.List   `tfsdk:"allowlist"`
	Labels         types.Map    `tfsdk:"labels"`
	Credential     types.String `tfsdk:"credential"`
	CredentialHint types.String `tfsdk:"credential_hint"`
	Status         types.String `tfsdk:"status"`
	Connected      types.Bool   `tfsdk:"connected"`
	CreatedAt      types.String `tfsdk:"created_at"`
	UpdatedAt      types.String `tfsdk:"updated_at"`
}

type outpostAllowRuleModel struct {
	Scheme     types.String `tfsdk:"scheme"`
	Host       types.String `tfsdk:"host"`
	Port       types.Int64  `tfsdk:"port"`
	PathPrefix types.String `tfsdk:"path_prefix"`
}

var outpostAllowRuleObjType = types.ObjectType{
	AttrTypes: map[string]attr.Type{
		"scheme":      types.StringType,
		"host":        types.StringType,
		"port":        types.Int64Type,
		"path_prefix": types.StringType,
	},
}

func (r *outpostResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_outpost"
}

func (r *outpostResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "An outpost that proxies agent traffic from a remote network.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Outpost identifier.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Human-readable outpost name.",
				Required:            true,
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Free-form description. Removing it from config clears the stored description.",
				Optional:            true,
			},
			"allowlist": schema.ListNestedAttribute{
				MarkdownDescription: "Upstream endpoints the outpost is allowed to reach. Replaced wholesale on " +
					"every apply: removing the blocks (or setting `allowlist = []`) clears the allowlist, which " +
					"leaves the outpost able to reach nothing.",
				Optional: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"scheme": schema.StringAttribute{
							MarkdownDescription: "URL scheme (`http` or `https`).",
							Required:            true,
						},
						"host": schema.StringAttribute{
							MarkdownDescription: "Hostname or IP address.",
							Required:            true,
						},
						"port": schema.Int64Attribute{
							MarkdownDescription: "Port number.",
							Required:            true,
						},
						"path_prefix": schema.StringAttribute{
							MarkdownDescription: "Optional path prefix.",
							Optional:            true,
						},
					},
				},
			},
			"labels": schema.MapAttribute{
				MarkdownDescription: "Arbitrary key/value labels. Replaced wholesale on every apply: removing the " +
					"map (or setting `labels = {}`) clears them.",
				ElementType: types.StringType,
				Optional:    true,
			},
			"credential": schema.StringAttribute{
				MarkdownDescription: "Enrollment credential returned once at creation. Never returned on subsequent reads.",
				Computed:            true,
				Sensitive:           true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"credential_hint": schema.StringAttribute{
				MarkdownDescription: "Safe-to-display hint for the credential.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"status": schema.StringAttribute{
				MarkdownDescription: "Enrollment status (`PENDING`, `ENROLLED`).",
				Computed:            true,
			},
			"connected": schema.BoolAttribute{
				MarkdownDescription: "Whether the outpost is currently connected.",
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

func (r *outpostResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromProviderData(req.ProviderData, &resp.Diagnostics)
}

func (r *outpostResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan outpostResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	wantAllowlist, wantLabels, wantDescription := plan.Allowlist, plan.Labels, plan.Description

	body := gen.CreateOutpostRequest{
		Name:        plan.Name.ValueString(),
		Description: stringToPtr(plan.Description),
	}
	resp.Diagnostics.Append(outpostAllowlistToRequest(ctx, plan.Allowlist, &body.Allowlist)...)
	resp.Diagnostics.Append(outpostLabelsToRequest(ctx, plan.Labels, &body.Labels)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.Gen.OutpostsCreateOutpostEndpointWithResponse(ctx, body)
	if err != nil {
		resp.Diagnostics.AddError("Error creating outpost", err.Error())
		return
	}
	if err := client.Check(apiResp.HTTPResponse, apiResp.Body); err != nil {
		resp.Diagnostics.AddError("Error creating outpost", err.Error())
		return
	}
	if apiResp.JSON201 == nil {
		resp.Diagnostics.AddError("Error creating outpost", "API returned an empty body")
		return
	}

	plan.Credential = types.StringValue(apiResp.JSON201.Credential.Credential)
	plan.CredentialHint = types.StringValue(apiResp.JSON201.Credential.Hint)

	resp.Diagnostics.Append(outpostApplyDetail(ctx, &plan, &apiResp.JSON201.Outpost)...)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.Allowlist = outpostKeepConfigured(wantAllowlist, plan.Allowlist)
	plan.Labels = outpostKeepConfiguredMap(wantLabels, plan.Labels)
	plan.Description = outpostKeepConfiguredString(wantDescription, plan.Description)

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *outpostResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state outpostResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.Gen.OutpostsGetOutpostEndpointWithResponse(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading outpost", err.Error())
		return
	}
	if err := client.Check(apiResp.HTTPResponse, apiResp.Body); err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading outpost", err.Error())
		return
	}
	if apiResp.JSON200 == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	priorAllowlist, priorLabels, priorDescription := state.Allowlist, state.Labels, state.Description
	resp.Diagnostics.Append(outpostApplyDetail(ctx, &state, apiResp.JSON200)...)
	if resp.Diagnostics.HasError() {
		return
	}
	state.Allowlist = outpostKeepConfigured(priorAllowlist, state.Allowlist)
	state.Labels = outpostKeepConfiguredMap(priorLabels, state.Labels)
	state.Description = outpostKeepConfiguredString(priorDescription, state.Description)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *outpostResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan outpostResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	wantAllowlist, wantLabels, wantDescription := plan.Allowlist, plan.Labels, plan.Description

	// Every omitted field means "leave alone" to the API, so a config that drops
	// one has to send an explicit empty value or the removal is a silent no-op —
	// the wrong direction for a deny-by-default list.
	body := gen.UpdateOutpostRequest{
		Name:        stringToPtr(plan.Name),
		Description: outpostStringToRequest(plan.Description),
	}
	resp.Diagnostics.Append(outpostAllowlistToRequest(ctx, plan.Allowlist, &body.Allowlist)...)
	resp.Diagnostics.Append(outpostLabelsToRequest(ctx, plan.Labels, &body.Labels)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.Gen.OutpostsUpdateOutpostEndpointWithResponse(ctx, plan.ID.ValueString(), body)
	if err != nil {
		resp.Diagnostics.AddError("Error updating outpost", err.Error())
		return
	}
	if err := client.Check(apiResp.HTTPResponse, apiResp.Body); err != nil {
		resp.Diagnostics.AddError("Error updating outpost", err.Error())
		return
	}
	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError("Error updating outpost", "API returned an empty body")
		return
	}

	resp.Diagnostics.Append(outpostApplyDetail(ctx, &plan, apiResp.JSON200)...)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.Allowlist = outpostKeepConfigured(wantAllowlist, plan.Allowlist)
	plan.Labels = outpostKeepConfiguredMap(wantLabels, plan.Labels)
	plan.Description = outpostKeepConfiguredString(wantDescription, plan.Description)

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *outpostResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state outpostResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.Gen.OutpostsDeleteOutpostEndpointWithResponse(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error deleting outpost", err.Error())
		return
	}
	if err := client.Check(apiResp.HTTPResponse, apiResp.Body); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Error deleting outpost", err.Error())
	}
}

func (r *outpostResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// outpostAllowlistToRequest always sets the request field, sending [] for a null
// config: the API leaves an omitted allowlist alone, so omission cannot clear it.
func outpostAllowlistToRequest(ctx context.Context, list types.List, target **[]gen.OutpostAllowRule) diag.Diagnostics {
	if list.IsUnknown() {
		return nil
	}
	if list.IsNull() {
		empty := []gen.OutpostAllowRule{}
		*target = &empty
		return nil
	}
	var rules []outpostAllowRuleModel
	diags := list.ElementsAs(ctx, &rules, false)
	if diags.HasError() {
		return diags
	}
	out := make([]gen.OutpostAllowRule, 0, len(rules))
	for _, rule := range rules {
		r := gen.OutpostAllowRule{
			Scheme: gen.Scheme(rule.Scheme.ValueString()),
			Host:   rule.Host.ValueString(),
			Port:   int(rule.Port.ValueInt64()),
		}
		if !rule.PathPrefix.IsNull() && !rule.PathPrefix.IsUnknown() {
			s := rule.PathPrefix.ValueString()
			r.PathPrefix = &s
		}
		out = append(out, r)
	}
	*target = &out
	return diags
}

func outpostAllowlistValue(ctx context.Context, rules *[]gen.OutpostAllowRule) (types.List, diag.Diagnostics) {
	if rules == nil {
		return types.ListNull(outpostAllowRuleObjType), nil
	}
	models := make([]outpostAllowRuleModel, 0, len(*rules))
	for _, r := range *rules {
		models = append(models, outpostAllowRuleModel{
			Scheme:     types.StringValue(string(r.Scheme)),
			Host:       types.StringValue(r.Host),
			Port:       types.Int64Value(int64(r.Port)),
			PathPrefix: ptrToString(r.PathPrefix),
		})
	}
	return types.ListValueFrom(ctx, outpostAllowRuleObjType, models)
}

// outpostLabelsToRequest is the labels analogue of outpostAllowlistToRequest.
func outpostLabelsToRequest(ctx context.Context, m types.Map, target **map[string]string) diag.Diagnostics {
	if m.IsUnknown() {
		return nil
	}
	if m.IsNull() {
		empty := map[string]string{}
		*target = &empty
		return nil
	}
	return stringMapToPtr(ctx, m, target)
}

// outpostKeepConfigured keeps the configured value when the API answered with an
// equivalent empty collection, so a null and an explicit [] both round-trip on an
// Optional, non-Computed attribute instead of planning a diff forever.
func outpostKeepConfigured(configured, applied types.List) types.List {
	if configured.IsUnknown() {
		return applied
	}
	if len(applied.Elements()) == 0 && len(configured.Elements()) == 0 {
		return configured
	}
	return applied
}

// outpostStringToRequest sends "" for a null config so the field is cleared: an
// omitted one is left alone by the API.
func outpostStringToRequest(v types.String) *string {
	if v.IsUnknown() {
		return nil
	}
	s := v.ValueString()
	return &s
}

// outpostKeepConfiguredString is the scalar analogue of outpostKeepConfigured.
func outpostKeepConfiguredString(configured, applied types.String) types.String {
	if configured.IsUnknown() {
		return applied
	}
	if configured.IsNull() && applied.ValueString() == "" {
		return configured
	}
	return applied
}

// outpostKeepConfiguredMap is the labels analogue of outpostKeepConfigured.
func outpostKeepConfiguredMap(configured, applied types.Map) types.Map {
	if configured.IsUnknown() {
		return applied
	}
	if len(applied.Elements()) == 0 && len(configured.Elements()) == 0 {
		return configured
	}
	return applied
}

func outpostApplyDetail(ctx context.Context, m *outpostResourceModel, detail *gen.OutpostDetail) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.StringValue(detail.OutpostId)
	m.Name = types.StringValue(detail.Name)
	m.Description = ptrToString(detail.Description)
	m.Status = types.StringValue(string(detail.Status))
	m.Connected = boolPtrToBool(detail.Connected)
	m.CreatedAt = types.StringValue(detail.CreatedAt)
	m.UpdatedAt = types.StringValue(detail.UpdatedAt)

	allowlist, d := outpostAllowlistValue(ctx, detail.Allowlist)
	diags.Append(d...)
	m.Allowlist = allowlist

	labels, d := stringMapValue(ctx, detail.Labels)
	diags.Append(d...)
	m.Labels = labels

	return diags
}
