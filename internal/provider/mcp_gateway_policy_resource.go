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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/komodorio/terraform-provider-agentops/internal/client"
	"github.com/komodorio/terraform-provider-agentops/internal/client/gen"
)

// Ensure the resource satisfies the framework interfaces.
var (
	_ resource.Resource                = &mcpGatewayPolicyResource{}
	_ resource.ResourceWithConfigure   = &mcpGatewayPolicyResource{}
	_ resource.ResourceWithImportState = &mcpGatewayPolicyResource{}
)

// NewMCPGatewayPolicyResource is the constructor registered with the provider.
func NewMCPGatewayPolicyResource() resource.Resource {
	return &mcpGatewayPolicyResource{}
}

type mcpGatewayPolicyResource struct {
	client *client.Client
}

// mcpGatewayPolicyResourceModel maps the agentops_mcp_gateway_policy schema to
// Go. name and description are server-derived and only appear on the record.
type mcpGatewayPolicyResourceModel struct {
	ID          types.String         `tfsdk:"id"`
	Document    jsontypes.Normalized `tfsdk:"document"`
	Enabled     types.Bool           `tfsdk:"enabled"`
	TargetNames types.List           `tfsdk:"target_names"`
	TargetType  types.String         `tfsdk:"target_type"`
	Name        types.String         `tfsdk:"name"`
	Description types.String         `tfsdk:"description"`
}

func (r *mcpGatewayPolicyResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_mcp_gateway_policy"
}

func (r *mcpGatewayPolicyResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "An MCP gateway policy: a declarative policy document applied to one or more gateway targets.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Policy identifier.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"document": schema.StringAttribute{
				MarkdownDescription: "Top-level declarative policy document, encoded as JSON.",
				CustomType:          jsontypes.NormalizedType{},
				Required:            true,
			},
			"enabled": schema.BoolAttribute{
				MarkdownDescription: "Whether the policy is active.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"target_names": schema.ListAttribute{
				MarkdownDescription: "Names of the gateway targets this policy applies to.",
				ElementType:         types.StringType,
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.List{listplanmodifier.UseStateForUnknown()},
			},
			"target_type": schema.StringAttribute{
				MarkdownDescription: "Kind of target the policy applies to.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Human-readable policy name (server-derived).",
				Computed:            true,
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Free-form policy description (server-derived).",
				Computed:            true,
			},
		},
	}
}

func (r *mcpGatewayPolicyResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromProviderData(req.ProviderData, &resp.Diagnostics)
}

func (r *mcpGatewayPolicyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan mcpGatewayPolicyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	doc, diags := mcpPolicyDocumentToRequest(plan.Document)
	resp.Diagnostics.Append(diags...)

	body := gen.McpPolicyCreate{
		Document: doc,
		Enabled:  boolToPtr(plan.Enabled),
	}
	resp.Diagnostics.Append(listToStringSlice(ctx, plan.TargetNames, &body.TargetNames)...)
	if !plan.TargetType.IsNull() && !plan.TargetType.IsUnknown() {
		tt := gen.McpPolicyCreateTargetType(plan.TargetType.ValueString())
		body.TargetType = &tt
	}
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.Gen.GatewayAdminCreatePolicyWithResponse(ctx, body)
	if err != nil {
		resp.Diagnostics.AddError("Error creating MCP gateway policy", err.Error())
		return
	}
	if err := client.Check(apiResp.HTTPResponse, apiResp.Body); err != nil {
		resp.Diagnostics.AddError("Error creating MCP gateway policy", err.Error())
		return
	}
	if apiResp.JSON201 == nil {
		resp.Diagnostics.AddError("Error creating MCP gateway policy", "API returned an empty body")
		return
	}

	resp.Diagnostics.Append(mcpPolicyRecordToState(ctx, apiResp.JSON201, &plan)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *mcpGatewayPolicyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state mcpGatewayPolicyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// There is no GET-by-id endpoint; list and match on id.
	apiResp, err := r.client.Gen.GatewayAdminListPoliciesWithResponse(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error reading MCP gateway policy", err.Error())
		return
	}
	if err := client.Check(apiResp.HTTPResponse, apiResp.Body); err != nil {
		resp.Diagnostics.AddError("Error reading MCP gateway policy", err.Error())
		return
	}
	if apiResp.JSON200 == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	var found *gen.McpPolicyRecord
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

	resp.Diagnostics.Append(mcpPolicyRecordToState(ctx, found, &state)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *mcpGatewayPolicyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan mcpGatewayPolicyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	doc, diags := mcpPolicyDocumentToRequest(plan.Document)
	resp.Diagnostics.Append(diags...)

	body := gen.McpPolicyPatch{
		Document: &doc,
		Enabled:  boolToPtr(plan.Enabled),
	}
	resp.Diagnostics.Append(listToStringSlice(ctx, plan.TargetNames, &body.TargetNames)...)
	if !plan.TargetType.IsNull() && !plan.TargetType.IsUnknown() {
		tt := gen.McpPolicyPatchTargetType(plan.TargetType.ValueString())
		body.TargetType = &tt
	}
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.Gen.GatewayAdminUpdatePolicyWithResponse(ctx, plan.ID.ValueString(), body)
	if err != nil {
		resp.Diagnostics.AddError("Error updating MCP gateway policy", err.Error())
		return
	}
	if err := client.Check(apiResp.HTTPResponse, apiResp.Body); err != nil {
		resp.Diagnostics.AddError("Error updating MCP gateway policy", err.Error())
		return
	}
	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError("Error updating MCP gateway policy", "API returned an empty body")
		return
	}

	resp.Diagnostics.Append(mcpPolicyRecordToState(ctx, apiResp.JSON200, &plan)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *mcpGatewayPolicyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state mcpGatewayPolicyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.Gen.GatewayAdminDeletePolicyWithResponse(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error deleting MCP gateway policy", err.Error())
		return
	}
	if err := client.Check(apiResp.HTTPResponse, apiResp.Body); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Error deleting MCP gateway policy", err.Error())
	}
}

func (r *mcpGatewayPolicyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// mcpPolicyDocumentToRequest decodes the JSON document string into the generated
// PolicyDocument struct used by both the create and patch request bodies.
func mcpPolicyDocumentToRequest(v jsontypes.Normalized) (gen.PolicyDocument, diag.Diagnostics) {
	var diags diag.Diagnostics
	var doc gen.PolicyDocument
	if err := json.Unmarshal([]byte(v.ValueString()), &doc); err != nil {
		diags.AddError("Invalid policy document", err.Error())
	}
	return doc, diags
}

// mcpPolicyRecordToState maps an McpPolicyRecord onto the resource model.
func mcpPolicyRecordToState(ctx context.Context, record *gen.McpPolicyRecord, m *mcpGatewayPolicyResourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.StringValue(record.Id)
	m.Name = strOrNull(record.Name)
	m.Description = strOrNull(record.Description)
	m.Enabled = boolPtrToBool(record.Enabled)
	m.TargetType = strOrNull(enumPtrToString(record.TargetType))

	b, err := json.Marshal(record.Document)
	if err != nil {
		diags.AddError("Error encoding policy document", err.Error())
	} else {
		m.Document = jsontypes.NewNormalizedValue(string(b))
	}

	if record.TargetNames == nil {
		m.TargetNames = types.ListNull(types.StringType)
	} else {
		names, d := types.ListValueFrom(ctx, types.StringType, *record.TargetNames)
		diags.Append(d...)
		m.TargetNames = names
	}

	return diags
}
