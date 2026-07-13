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
	_ resource.Resource                = &knowledgeBaseResource{}
	_ resource.ResourceWithConfigure   = &knowledgeBaseResource{}
	_ resource.ResourceWithImportState = &knowledgeBaseResource{}
)

// NewKnowledgeBaseResource is the constructor registered with the provider.
func NewKnowledgeBaseResource() resource.Resource {
	return &knowledgeBaseResource{}
}

type knowledgeBaseResource struct {
	client *client.Client
}

// knowledgeBaseResourceModel maps the agentops_knowledge_base schema to Go.
type knowledgeBaseResourceModel struct {
	ID           types.String `tfsdk:"id"`
	Name         types.String `tfsdk:"name"`
	Description  types.String `tfsdk:"description"`
	Labels       types.Map    `tfsdk:"labels"`
	DocCount     types.Int64  `tfsdk:"doc_count"`
	IndexedCount types.Int64  `tfsdk:"indexed_count"`
	CreatedAt    types.String `tfsdk:"created_at"`
	UpdatedAt    types.String `tfsdk:"updated_at"`
}

func (r *knowledgeBaseResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_knowledge_base"
}

func (r *knowledgeBaseResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A knowledge base that stores documents made searchable by agents.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Knowledge base identifier.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Human-readable knowledge base name.",
				Required:            true,
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Free-form description.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"labels": schema.MapAttribute{
				MarkdownDescription: "Arbitrary key/value labels attached to the knowledge base.",
				ElementType:         types.StringType,
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.Map{mapplanmodifier.UseStateForUnknown()},
			},
			"doc_count": schema.Int64Attribute{
				MarkdownDescription: "Number of documents stored in the knowledge base.",
				Computed:            true,
			},
			"indexed_count": schema.Int64Attribute{
				MarkdownDescription: "Number of documents that have been indexed.",
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

func (r *knowledgeBaseResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromProviderData(req.ProviderData, &resp.Diagnostics)
}

func (r *knowledgeBaseResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan knowledgeBaseResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := gen.CreateKnowledgeBaseRequest{
		Name:        plan.Name.ValueString(),
		Description: stringToPtr(plan.Description),
	}
	resp.Diagnostics.Append(knowledgeBaseLabelsToPtr(ctx, plan.Labels, &body.Labels)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.Gen.KnowledgeCreateKnowledgeBaseWithResponse(ctx, body)
	if err != nil {
		resp.Diagnostics.AddError("Error creating knowledge base", err.Error())
		return
	}
	if err := client.Check(apiResp.HTTPResponse, apiResp.Body); err != nil {
		resp.Diagnostics.AddError("Error creating knowledge base", err.Error())
		return
	}
	if apiResp.JSON201 == nil {
		resp.Diagnostics.AddError("Error creating knowledge base", "API returned an empty body")
		return
	}

	resp.Diagnostics.Append(knowledgeBaseApplySummary(ctx, &plan, apiResp.JSON201)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *knowledgeBaseResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state knowledgeBaseResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.Gen.KnowledgeGetKnowledgeBaseWithResponse(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading knowledge base", err.Error())
		return
	}
	if err := client.Check(apiResp.HTTPResponse, apiResp.Body); err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading knowledge base", err.Error())
		return
	}
	if apiResp.JSON200 == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	resp.Diagnostics.Append(knowledgeBaseApplySummary(ctx, &state, apiResp.JSON200)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *knowledgeBaseResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan knowledgeBaseResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := gen.UpdateKnowledgeBaseRequest{
		Name:        stringToPtr(plan.Name),
		Description: stringToPtr(plan.Description),
	}
	resp.Diagnostics.Append(knowledgeBaseLabelsToPtr(ctx, plan.Labels, &body.Labels)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.Gen.KnowledgeUpdateKnowledgeBaseWithResponse(ctx, plan.ID.ValueString(), body)
	if err != nil {
		resp.Diagnostics.AddError("Error updating knowledge base", err.Error())
		return
	}
	if err := client.Check(apiResp.HTTPResponse, apiResp.Body); err != nil {
		resp.Diagnostics.AddError("Error updating knowledge base", err.Error())
		return
	}
	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError("Error updating knowledge base", "API returned an empty body")
		return
	}

	resp.Diagnostics.Append(knowledgeBaseApplySummary(ctx, &plan, apiResp.JSON200)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *knowledgeBaseResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state knowledgeBaseResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.Gen.KnowledgeDeleteKnowledgeBaseWithResponse(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error deleting knowledge base", err.Error())
		return
	}
	if err := client.Check(apiResp.HTTPResponse, apiResp.Body); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Error deleting knowledge base", err.Error())
	}
}

func (r *knowledgeBaseResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// knowledgeBaseApplySummary writes a KnowledgeBaseSummary response into the model.
func knowledgeBaseApplySummary(ctx context.Context, m *knowledgeBaseResourceModel, kb *gen.KnowledgeBaseSummary) diag.Diagnostics {
	m.ID = types.StringValue(kb.KbId)
	m.Name = types.StringValue(kb.Name)
	m.Description = ptrToString(kb.Description)
	m.DocCount = types.Int64Value(int64(kb.DocCount))
	m.IndexedCount = types.Int64Value(int64(kb.IndexedCount))
	m.CreatedAt = types.StringValue(kb.CreatedAt)
	m.UpdatedAt = types.StringValue(kb.UpdatedAt)

	labels, diags := knowledgeBaseLabelsFromPtr(ctx, kb.Labels)
	if diags.HasError() {
		return diags
	}
	m.Labels = labels
	return diags
}

// knowledgeBaseLabelsToPtr converts an optional Terraform map into a
// *map[string]string request field, leaving it nil (omitted) when the map is
// null or unknown.
func knowledgeBaseLabelsToPtr(ctx context.Context, m types.Map, target **map[string]string) diag.Diagnostics {
	if m.IsNull() || m.IsUnknown() {
		return nil
	}
	labels := map[string]string{}
	diags := m.ElementsAs(ctx, &labels, false)
	if diags.HasError() {
		return diags
	}
	*target = &labels
	return diags
}

// knowledgeBaseLabelsFromPtr converts an optional API label map into a Terraform
// map, mapping nil to a null map.
func knowledgeBaseLabelsFromPtr(ctx context.Context, p *map[string]string) (types.Map, diag.Diagnostics) {
	if p == nil {
		return types.MapNull(types.StringType), nil
	}
	return types.MapValueFrom(ctx, types.StringType, *p)
}
