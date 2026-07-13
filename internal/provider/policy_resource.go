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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/komodorio/terraform-provider-agentops/internal/client"
	"github.com/komodorio/terraform-provider-agentops/internal/client/gen"
)

// Ensure the resource satisfies the framework interfaces.
var (
	_ resource.Resource                = &policyResource{}
	_ resource.ResourceWithConfigure   = &policyResource{}
	_ resource.ResourceWithImportState = &policyResource{}
)

// NewPolicyResource is the constructor registered with the provider.
func NewPolicyResource() resource.Resource {
	return &policyResource{}
}

type policyResource struct {
	client *client.Client
}

// policyResourceModel maps the agentops_policy schema to Go. grants is modeled
// as a JSON string of the grant inputs; builtin and created_at are server-derived.
type policyResourceModel struct {
	ID          types.String         `tfsdk:"id"`
	Name        types.String         `tfsdk:"name"`
	Description types.String         `tfsdk:"description"`
	Grants      jsontypes.Normalized `tfsdk:"grants"`
	Builtin     types.Bool           `tfsdk:"builtin"`
	CreatedAt   types.String         `tfsdk:"created_at"`
}

func (r *policyResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_policy"
}

func (r *policyResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "An authorization policy: a named collection of capability grants.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Policy identifier.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Human-readable policy name.",
				Required:            true,
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Free-form policy description.",
				Optional:            true,
			},
			"grants": schema.StringAttribute{
				MarkdownDescription: "Capability grants attached to the policy, encoded as JSON.",
				CustomType:          jsontypes.NormalizedType{},
				Optional:            true,
				Computed:            true,
			},
			"builtin": schema.BoolAttribute{
				MarkdownDescription: "Whether the policy is a built-in policy managed by the system.",
				Computed:            true,
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "Timestamp at which the policy was created.",
				Computed:            true,
			},
		},
	}
}

func (r *policyResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromProviderData(req.ProviderData, &resp.Diagnostics)
}

func (r *policyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan policyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := gen.PolicyCreate{
		Name:        plan.Name.ValueString(),
		Description: stringToPtr(plan.Description),
	}
	grants, diags := policyGrantsToRequest(plan.Grants)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	body.Grants = grants

	apiResp, err := r.client.Gen.AuthzCreatePolicyWithResponse(ctx, body)
	if err != nil {
		resp.Diagnostics.AddError("Error creating policy", err.Error())
		return
	}
	if err := client.Check(apiResp.HTTPResponse, apiResp.Body); err != nil {
		resp.Diagnostics.AddError("Error creating policy", err.Error())
		return
	}
	if apiResp.JSON201 == nil {
		resp.Diagnostics.AddError("Error creating policy", "API returned an empty body")
		return
	}

	resp.Diagnostics.Append(policyToState(apiResp.JSON201, &plan)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *policyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state policyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// There is no GET-by-id endpoint; list and match on id.
	apiResp, err := r.client.Gen.AuthzListPoliciesWithResponse(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error reading policy", err.Error())
		return
	}
	if err := client.Check(apiResp.HTTPResponse, apiResp.Body); err != nil {
		resp.Diagnostics.AddError("Error reading policy", err.Error())
		return
	}
	if apiResp.JSON200 == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	var found *gen.AuthzPolicy
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

	resp.Diagnostics.Append(policyToState(found, &state)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *policyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan policyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	name := plan.Name.ValueString()
	body := gen.PolicyUpdate{
		Name:        &name,
		Description: stringToPtr(plan.Description),
	}
	grants, diags := policyGrantsToRequest(plan.Grants)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	body.Grants = grants

	apiResp, err := r.client.Gen.AuthzUpdatePolicyWithResponse(ctx, plan.ID.ValueString(), body)
	if err != nil {
		resp.Diagnostics.AddError("Error updating policy", err.Error())
		return
	}
	if err := client.Check(apiResp.HTTPResponse, apiResp.Body); err != nil {
		resp.Diagnostics.AddError("Error updating policy", err.Error())
		return
	}
	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError("Error updating policy", "API returned an empty body")
		return
	}

	resp.Diagnostics.Append(policyToState(apiResp.JSON200, &plan)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *policyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state policyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.Gen.AuthzDeletePolicyWithResponse(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error deleting policy", err.Error())
		return
	}
	if err := client.Check(apiResp.HTTPResponse, apiResp.Body); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Error deleting policy", err.Error())
	}
}

func (r *policyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// policyGrantsToRequest decodes the JSON grants string into the generated grant
// input slice used by both the create and patch request bodies. A null/unknown
// value yields a nil slice so the field is omitted.
func policyGrantsToRequest(v jsontypes.Normalized) (*[]gen.PolicyGrantInput, diag.Diagnostics) {
	var diags diag.Diagnostics
	if v.IsNull() || v.IsUnknown() {
		return nil, diags
	}
	var grants []gen.PolicyGrantInput
	if err := json.Unmarshal([]byte(v.ValueString()), &grants); err != nil {
		diags.AddError("Invalid policy grants", err.Error())
		return nil, diags
	}
	return &grants, diags
}

// policyToState maps an AuthzPolicy onto the resource model.
func policyToState(record *gen.AuthzPolicy, m *policyResourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.StringValue(record.Id)
	m.Name = types.StringValue(record.Name)
	m.Description = ptrToString(record.Description)
	m.Builtin = types.BoolValue(record.Builtin)
	m.CreatedAt = types.StringValue(record.CreatedAt)

	if record.Grants == nil {
		m.Grants = jsontypes.NewNormalizedNull()
	} else {
		b, err := json.Marshal(*record.Grants)
		if err != nil {
			diags.AddError("Error encoding policy grants", err.Error())
		} else {
			m.Grants = jsontypes.NewNormalizedValue(string(b))
		}
	}

	return diags
}
