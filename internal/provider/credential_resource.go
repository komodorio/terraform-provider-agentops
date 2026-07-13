// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/komodorio/terraform-provider-agentops/internal/client"
	"github.com/komodorio/terraform-provider-agentops/internal/client/gen"
)

// Ensure the resource satisfies the framework interfaces.
var (
	_ resource.Resource                = &credentialResource{}
	_ resource.ResourceWithConfigure   = &credentialResource{}
	_ resource.ResourceWithImportState = &credentialResource{}
)

// NewCredentialResource is the constructor registered with the provider.
func NewCredentialResource() resource.Resource {
	return &credentialResource{}
}

type credentialResource struct {
	client *client.Client
}

// credentialResourceModel maps the agentops_credential schema to Go.
type credentialResourceModel struct {
	ID             types.String `tfsdk:"id"`
	Name           types.String `tfsdk:"name"`
	Value          types.String `tfsdk:"value"`
	Description    types.String `tfsdk:"description"`
	Owner          types.String `tfsdk:"owner"`
	Labels         types.Map    `tfsdk:"labels"`
	Status         types.String `tfsdk:"status"`
	ActiveVersion  types.Int64  `tfsdk:"active_version"`
	LastReplacedAt types.String `tfsdk:"last_replaced_at"`
	LastUsedAt     types.String `tfsdk:"last_used_at"`
	CreatedAt      types.String `tfsdk:"created_at"`
	UpdatedAt      types.String `tfsdk:"updated_at"`
}

func (r *credentialResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_credential"
}

func (r *credentialResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A stored credential whose secret value is managed out-of-band. The value is " +
			"write-only and never returned by the API; changing it replaces the credential's secret in place.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Credential identifier.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Human-readable credential name. Changing this forces a new credential.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"value": schema.StringAttribute{
				MarkdownDescription: "The secret value. Write-only; never returned by the API. Changing it " +
					"replaces the stored secret.",
				Required:  true,
				Sensitive: true,
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Free-form description.",
				Optional:            true,
			},
			"owner": schema.StringAttribute{
				MarkdownDescription: "Owner of the credential.",
				Optional:            true,
			},
			"labels": schema.MapAttribute{
				MarkdownDescription: "Arbitrary key/value labels.",
				ElementType:         types.StringType,
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.Map{mapplanmodifier.UseStateForUnknown()},
			},
			"status": schema.StringAttribute{
				MarkdownDescription: "Credential status. Server-assigned when omitted.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"active_version": schema.Int64Attribute{
				MarkdownDescription: "Currently active secret version.",
				Computed:            true,
			},
			"last_replaced_at": schema.StringAttribute{
				MarkdownDescription: "Timestamp of the last value replacement.",
				Computed:            true,
			},
			"last_used_at": schema.StringAttribute{
				MarkdownDescription: "Timestamp the credential was last used.",
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

func (r *credentialResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromProviderData(req.ProviderData, &resp.Diagnostics)
}

func (r *credentialResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan credentialResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := gen.CreateCredentialRequest{
		Name:        plan.Name.ValueString(),
		Value:       plan.Value.ValueString(),
		Description: stringToPtr(plan.Description),
		Owner:       stringToPtr(plan.Owner),
	}
	resp.Diagnostics.Append(credentialLabelsToRequest(ctx, plan.Labels, &body.Labels)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.Gen.CredentialsCreateCredentialWithResponse(ctx, body)
	if err != nil {
		resp.Diagnostics.AddError("Error creating credential", err.Error())
		return
	}
	if err := client.Check(apiResp.HTTPResponse, apiResp.Body); err != nil {
		resp.Diagnostics.AddError("Error creating credential", err.Error())
		return
	}
	if apiResp.JSON201 == nil {
		resp.Diagnostics.AddError("Error creating credential", "API returned an empty body")
		return
	}

	// value is write-only; it stays as supplied in the plan.
	resp.Diagnostics.Append(credentialApplyMetadata(ctx, &plan, apiResp.JSON201)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *credentialResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state credentialResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.Gen.CredentialsGetCredentialWithResponse(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading credential", err.Error())
		return
	}
	if err := client.Check(apiResp.HTTPResponse, apiResp.Body); err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading credential", err.Error())
		return
	}
	if apiResp.JSON200 == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	// value is never returned; preserve the existing state.
	resp.Diagnostics.Append(credentialApplyMetadata(ctx, &state, apiResp.JSON200)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *credentialResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state credentialResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := gen.UpdateCredentialMetadataRequest{
		Description: stringToPtr(plan.Description),
		Owner:       stringToPtr(plan.Owner),
	}
	resp.Diagnostics.Append(credentialLabelsToRequest(ctx, plan.Labels, &body.Labels)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !plan.Status.IsNull() && !plan.Status.IsUnknown() {
		st := gen.UpdateCredentialMetadataRequestStatus(plan.Status.ValueString())
		body.Status = &st
	}

	apiResp, err := r.client.Gen.CredentialsUpdateCredentialMetadataWithResponse(ctx, plan.ID.ValueString(), body)
	if err != nil {
		resp.Diagnostics.AddError("Error updating credential", err.Error())
		return
	}
	if err := client.Check(apiResp.HTTPResponse, apiResp.Body); err != nil {
		resp.Diagnostics.AddError("Error updating credential", err.Error())
		return
	}
	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError("Error updating credential", "API returned an empty body")
		return
	}
	meta := apiResp.JSON200

	// If the secret value changed, replace it via the dedicated PUT endpoint and
	// use its (fresher) metadata response instead.
	if !plan.Value.Equal(state.Value) {
		valResp, err := r.client.Gen.CredentialsReplaceCredentialValueWithResponse(ctx, plan.ID.ValueString(),
			gen.ReplaceCredentialValueRequest{Value: plan.Value.ValueString()})
		if err != nil {
			resp.Diagnostics.AddError("Error replacing credential value", err.Error())
			return
		}
		if err := client.Check(valResp.HTTPResponse, valResp.Body); err != nil {
			resp.Diagnostics.AddError("Error replacing credential value", err.Error())
			return
		}
		if valResp.JSON200 == nil {
			resp.Diagnostics.AddError("Error replacing credential value", "API returned an empty body")
			return
		}
		meta = valResp.JSON200
	}

	// value is write-only; keep the (possibly new) value from the plan.
	resp.Diagnostics.Append(credentialApplyMetadata(ctx, &plan, meta)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *credentialResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state credentialResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.Gen.CredentialsDeleteCredentialWithResponse(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error deleting credential", err.Error())
		return
	}
	if err := client.Check(apiResp.HTTPResponse, apiResp.Body); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Error deleting credential", err.Error())
	}
}

func (r *credentialResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// credentialLabelsToRequest converts the labels map into a *map[string]string
// request field, leaving it nil (omitted) when the map is null or unknown.
func credentialLabelsToRequest(ctx context.Context, labels types.Map, target **map[string]string) diag.Diagnostics {
	if labels.IsNull() || labels.IsUnknown() {
		return nil
	}
	m := map[string]string{}
	diags := labels.ElementsAs(ctx, &m, false)
	if diags.HasError() {
		return diags
	}
	*target = &m
	return diags
}

// credentialApplyMetadata writes a CredentialMetadata response into the model.
// The write-only value attribute is deliberately left untouched.
func credentialApplyMetadata(ctx context.Context, m *credentialResourceModel, meta *gen.CredentialMetadata) diag.Diagnostics {
	var diags diag.Diagnostics
	m.ID = types.StringValue(meta.Id)
	m.Name = types.StringValue(meta.Name)
	m.Description = ptrToString(meta.Description)
	m.Owner = ptrToString(meta.Owner)
	m.Status = types.StringValue(string(meta.Status))
	m.CreatedAt = types.StringValue(meta.CreatedAt)
	m.UpdatedAt = types.StringValue(meta.UpdatedAt)
	m.LastReplacedAt = ptrToString(meta.LastReplacedAt)
	m.LastUsedAt = ptrToString(meta.LastUsedAt)

	if meta.ActiveVersion != nil {
		m.ActiveVersion = types.Int64Value(int64(*meta.ActiveVersion))
	} else {
		m.ActiveVersion = types.Int64Null()
	}

	if meta.Labels != nil {
		labels, d := types.MapValueFrom(ctx, types.StringType, *meta.Labels)
		diags = append(diags, d...)
		m.Labels = labels
	} else {
		m.Labels = types.MapNull(types.StringType)
	}
	return diags
}
