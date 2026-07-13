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
	_ resource.Resource                = &grantResource{}
	_ resource.ResourceWithConfigure   = &grantResource{}
	_ resource.ResourceWithImportState = &grantResource{}
)

// NewGrantResource is the constructor registered with the provider.
func NewGrantResource() resource.Resource {
	return &grantResource{}
}

type grantResource struct {
	client *client.Client
}

// grantResourceModel maps the agentops_grant schema to Go. subject and selector
// are opaque JSON documents; created_at is server-derived.
type grantResourceModel struct {
	ID           types.String         `tfsdk:"id"`
	GrantKind    types.String         `tfsdk:"grant_kind"`
	ResourceID   types.String         `tfsdk:"resource_id"`
	ResourceType types.String         `tfsdk:"resource_type"`
	Subject      jsontypes.Normalized `tfsdk:"subject"`
	Selector     jsontypes.Normalized `tfsdk:"selector"`
	Capability   types.String         `tfsdk:"capability"`
	Description  types.String         `tfsdk:"description"`
	ExpiresAt    types.String         `tfsdk:"expires_at"`
	PolicyID     types.String         `tfsdk:"policy_id"`
	RoleID       types.String         `tfsdk:"role_id"`
	CreatedAt    types.String         `tfsdk:"created_at"`
}

func (r *grantResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_grant"
}

func (r *grantResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "An authorization grant: binds a subject to a capability or role over a resource.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Grant identifier.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"grant_kind": schema.StringAttribute{
				MarkdownDescription: "Kind of grant, one of \"capability\" or \"role\".",
				Required:            true,
			},
			"resource_id": schema.StringAttribute{
				MarkdownDescription: "Identifier of the resource the grant applies to.",
				Required:            true,
			},
			"resource_type": schema.StringAttribute{
				MarkdownDescription: "Type of the resource the grant applies to.",
				Required:            true,
			},
			"subject": schema.StringAttribute{
				MarkdownDescription: "Subject the grant is bound to, encoded as JSON (id and kind).",
				CustomType:          jsontypes.NormalizedType{},
				Required:            true,
			},
			"selector": schema.StringAttribute{
				MarkdownDescription: "Optional attribute selector, encoded as JSON.",
				CustomType:          jsontypes.NormalizedType{},
				Optional:            true,
				Computed:            true,
			},
			"capability": schema.StringAttribute{
				MarkdownDescription: "Capability granted (when grant_kind is \"capability\").",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Free-form grant description.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"expires_at": schema.StringAttribute{
				MarkdownDescription: "Optional expiry timestamp for the grant.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"policy_id": schema.StringAttribute{
				MarkdownDescription: "Policy the grant references (when applicable).",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"role_id": schema.StringAttribute{
				MarkdownDescription: "Role granted (when grant_kind is \"role\").",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "Timestamp the grant was created (server-derived).",
				Computed:            true,
			},
		},
	}
}

func (r *grantResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromProviderData(req.ProviderData, &resp.Diagnostics)
}

func (r *grantResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan grantResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	subject, diags := grantSubjectToRequest(plan.Subject)
	resp.Diagnostics.Append(diags...)
	selector, diags := grantSelectorToRequest(plan.Selector)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := gen.GrantCreate{
		GrantKind:    gen.GrantCreateGrantKind(plan.GrantKind.ValueString()),
		ResourceId:   plan.ResourceID.ValueString(),
		ResourceType: plan.ResourceType.ValueString(),
		Subject:      subject,
		Selector:     selector,
		Capability:   stringToPtr(plan.Capability),
		Description:  stringToPtr(plan.Description),
		ExpiresAt:    stringToPtr(plan.ExpiresAt),
		PolicyId:     stringToPtr(plan.PolicyID),
		RoleId:       stringToPtr(plan.RoleID),
	}

	apiResp, err := r.client.Gen.AuthzCreateGrantWithResponse(ctx, body)
	if err != nil {
		resp.Diagnostics.AddError("Error creating grant", err.Error())
		return
	}
	if err := client.Check(apiResp.HTTPResponse, apiResp.Body); err != nil {
		resp.Diagnostics.AddError("Error creating grant", err.Error())
		return
	}
	if apiResp.JSON201 == nil {
		resp.Diagnostics.AddError("Error creating grant", "API returned an empty body")
		return
	}

	resp.Diagnostics.Append(grantRecordToState(apiResp.JSON201, &plan)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *grantResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state grantResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// There is no GET-by-id endpoint; list and match on id.
	apiResp, err := r.client.Gen.AuthzListGrantsWithResponse(ctx, nil)
	if err != nil {
		resp.Diagnostics.AddError("Error reading grant", err.Error())
		return
	}
	if err := client.Check(apiResp.HTTPResponse, apiResp.Body); err != nil {
		resp.Diagnostics.AddError("Error reading grant", err.Error())
		return
	}
	if apiResp.JSON200 == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	var found *gen.AuthzGrant
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

	resp.Diagnostics.Append(grantRecordToState(found, &state)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *grantResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan grantResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	subject, diags := grantSubjectToRequest(plan.Subject)
	resp.Diagnostics.Append(diags...)
	selector, diags := grantSelectorToRequest(plan.Selector)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	grantKind := gen.GrantUpdateGrantKind(plan.GrantKind.ValueString())
	body := gen.GrantUpdate{
		GrantKind:    &grantKind,
		ResourceId:   stringToPtr(plan.ResourceID),
		ResourceType: stringToPtr(plan.ResourceType),
		Subject:      &subject,
		Selector:     selector,
		Capability:   stringToPtr(plan.Capability),
		Description:  stringToPtr(plan.Description),
		ExpiresAt:    stringToPtr(plan.ExpiresAt),
		PolicyId:     stringToPtr(plan.PolicyID),
		RoleId:       stringToPtr(plan.RoleID),
	}

	apiResp, err := r.client.Gen.AuthzUpdateGrantWithResponse(ctx, plan.ID.ValueString(), body)
	if err != nil {
		resp.Diagnostics.AddError("Error updating grant", err.Error())
		return
	}
	if err := client.Check(apiResp.HTTPResponse, apiResp.Body); err != nil {
		resp.Diagnostics.AddError("Error updating grant", err.Error())
		return
	}
	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError("Error updating grant", "API returned an empty body")
		return
	}

	resp.Diagnostics.Append(grantRecordToState(apiResp.JSON200, &plan)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *grantResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state grantResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.Gen.AuthzDeleteGrantWithResponse(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error deleting grant", err.Error())
		return
	}
	if err := client.Check(apiResp.HTTPResponse, apiResp.Body); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Error deleting grant", err.Error())
	}
}

func (r *grantResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// grantSubjectToRequest decodes the JSON subject string into the generated
// SubjectModel used by both the create and update request bodies.
func grantSubjectToRequest(v jsontypes.Normalized) (gen.SubjectModel, diag.Diagnostics) {
	var diags diag.Diagnostics
	var subject gen.SubjectModel
	if err := json.Unmarshal([]byte(v.ValueString()), &subject); err != nil {
		diags.AddError("Invalid subject", err.Error())
	}
	return subject, diags
}

// grantSelectorToRequest decodes the optional JSON selector string into a
// *map[string]interface{}, leaving it nil (omitted) when null or unknown.
func grantSelectorToRequest(v jsontypes.Normalized) (*map[string]interface{}, diag.Diagnostics) {
	var diags diag.Diagnostics
	if v.IsNull() || v.IsUnknown() {
		return nil, diags
	}
	var selector map[string]interface{}
	if err := json.Unmarshal([]byte(v.ValueString()), &selector); err != nil {
		diags.AddError("Invalid selector", err.Error())
		return nil, diags
	}
	return &selector, diags
}

// grantRecordToState maps an AuthzGrant onto the resource model.
func grantRecordToState(record *gen.AuthzGrant, m *grantResourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.StringValue(record.Id)
	m.GrantKind = types.StringValue(record.GrantKind)
	m.ResourceID = types.StringValue(record.ResourceId)
	m.ResourceType = types.StringValue(record.ResourceType)
	m.Capability = ptrToString(record.Capability)
	m.Description = ptrToString(record.Description)
	m.ExpiresAt = ptrToString(record.ExpiresAt)
	m.PolicyID = ptrToString(record.PolicyId)
	m.RoleID = ptrToString(record.RoleId)
	m.CreatedAt = types.StringValue(record.CreatedAt)

	b, err := json.Marshal(record.Subject)
	if err != nil {
		diags.AddError("Error encoding subject", err.Error())
	} else {
		m.Subject = jsontypes.NewNormalizedValue(string(b))
	}

	if record.Selector == nil {
		m.Selector = jsontypes.NewNormalizedNull()
	} else {
		sb, err := json.Marshal(*record.Selector)
		if err != nil {
			diags.AddError("Error encoding selector", err.Error())
		} else {
			m.Selector = jsontypes.NewNormalizedValue(string(sb))
		}
	}

	return diags
}
