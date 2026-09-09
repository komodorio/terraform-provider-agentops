// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/komodorio/terraform-provider-agentops/internal/client"
	"github.com/komodorio/terraform-provider-agentops/internal/client/gen"
)

// Ensure the resource satisfies the framework interfaces.
var (
	_ resource.Resource                = &skillResource{}
	_ resource.ResourceWithConfigure   = &skillResource{}
	_ resource.ResourceWithImportState = &skillResource{}
)

// NewSkillResource is the constructor registered with the provider.
func NewSkillResource() resource.Resource {
	return &skillResource{}
}

type skillResource struct {
	client *client.Client
}

// skillResourceModel maps the agentops_skill schema to Go. An authored skill is a
// versioned Markdown document: the metadata (name/description/tags/labels) is
// edited in place, while every change to content publishes a new version.
type skillResourceModel struct {
	ID             types.String             `tfsdk:"id"`
	Name           types.String             `tfsdk:"name"`
	Description    types.String             `tfsdk:"description"`
	Content        types.String             `tfsdk:"content"`
	Resources      []skillResourceFileModel `tfsdk:"resources"`
	Labels         types.Map                `tfsdk:"labels"`
	Tags           types.List               `tfsdk:"tags"`
	Kind           types.String             `tfsdk:"kind"`
	ContentVersion types.Int64              `tfsdk:"content_version"`
	UpdatedAt      types.String             `tfsdk:"updated_at"`
}

// skillResourceFileModel is one supporting file of the skill folder: `content` is
// SKILL.md, and each of these is a sibling it links to by relative path.
type skillResourceFileModel struct {
	Path       types.String `tfsdk:"path"`
	Content    types.String `tfsdk:"content"`
	Executable types.Bool   `tfsdk:"executable"`
}

func (r *skillResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_skill"
}

func (r *skillResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "An authored skill in the Skills Registry: a versioned Markdown document an " +
			"agent can be attached to. Requires the account-level `skills_registry` feature; mutations " +
			"return 404 while it is disabled.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Skill identifier.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Human-readable skill name. Must be unique within the account.",
				Required:            true,
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Free-form description. When `content` carries a front-matter " +
					"`description`, the control plane derives this field from it instead.",
				Optional:      true,
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"content": schema.StringAttribute{
				MarkdownDescription: "Markdown body of the skill. Setting it on create publishes version 1; " +
					"changing it afterwards publishes a new version.",
				Optional:      true,
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"resources": schema.ListNestedAttribute{
				MarkdownDescription: "The rest of the skill folder: the `references/`, `assets/` and " +
					"`scripts/` files `content` links to. Each `path` is relative to the skill folder and is " +
					"exactly the path written in the SKILL.md body, so an agent finds the file where its own " +
					"instructions say it is. Published together with `content` as one version, so a change to " +
					"either publishes a new one.",
				Optional: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"path": schema.StringAttribute{
							MarkdownDescription: "Path relative to the skill folder, e.g. `references/rollback.md` " +
								"or `scripts/deploy.sh`. Must not be absolute, contain `..`, or be `SKILL.md`.",
							Required: true,
						},
						"content": schema.StringAttribute{
							MarkdownDescription: "The file's text.",
							Optional:            true,
							Computed:            true,
							Default:             stringdefault.StaticString(""),
						},
						"executable": schema.BoolAttribute{
							MarkdownDescription: "Set for a script the skill tells the agent to run: the worker " +
								"restores the executable bit when it writes the folder.",
							Optional: true,
							Computed: true,
							Default:  booldefault.StaticBool(false),
						},
					},
				},
			},
			"labels": schema.MapAttribute{
				MarkdownDescription: "Arbitrary key/value labels used for ABAC scoping.",
				ElementType:         types.StringType,
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.Map{mapplanmodifier.UseStateForUnknown()},
			},
			"tags": schema.ListAttribute{
				MarkdownDescription: "Free-form tags for organising skills.",
				ElementType:         types.StringType,
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.List{listplanmodifier.UseStateForUnknown()},
			},
			"kind": schema.StringAttribute{
				MarkdownDescription: "Skill kind. Terraform-managed skills are always `authored`.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"content_version": schema.Int64Attribute{
				MarkdownDescription: "Latest published content version number, or null when no content has been published.",
				Computed:            true,
			},
			"updated_at": schema.StringAttribute{
				MarkdownDescription: "Last-update timestamp.",
				Computed:            true,
			},
		},
	}
}

func (r *skillResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromProviderData(req.ProviderData, &resp.Diagnostics)
}

func (r *skillResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan skillResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := gen.CreateSkillRequest{
		Name:        plan.Name.ValueString(),
		Description: stringToPtr(plan.Description),
		Content:     stringToPtr(plan.Content),
	}
	// The create route publishes version 1 from `content` alone — it carries no resources — so a
	// skill with a folder is created without content and gets one version published below instead.
	// Sending both would make the folder arrive as version 2, with a contentless version 1 behind it.
	if len(plan.Resources) > 0 {
		body.Content = nil
	}
	resp.Diagnostics.Append(stringMapToPtr(ctx, plan.Labels, &body.Labels)...)
	resp.Diagnostics.Append(listToStringSlice(ctx, plan.Tags, &body.Tags)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.Gen.SkillsCreateSkillRouteWithResponse(ctx, body)
	if err != nil {
		resp.Diagnostics.AddError("Error creating skill", err.Error())
		return
	}
	if err := client.Check(apiResp.HTTPResponse, apiResp.Body); err != nil {
		resp.Diagnostics.AddError("Error creating skill", err.Error())
		return
	}
	if apiResp.JSON201 == nil {
		resp.Diagnostics.AddError("Error creating skill", "API returned an empty body")
		return
	}

	detail := apiResp.JSON201
	if len(plan.Resources) > 0 {
		detail = r.publishVersion(ctx, apiResp.JSON201.SkillId, plan, &resp.Diagnostics)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	resp.Diagnostics.Append(skillApplyDetail(ctx, &plan, detail)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *skillResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state skillResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.Gen.SkillsGetSkillRouteWithResponse(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading skill", err.Error())
		return
	}
	if err := client.Check(apiResp.HTTPResponse, apiResp.Body); err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading skill", err.Error())
		return
	}
	if apiResp.JSON200 == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	resp.Diagnostics.Append(skillApplyDetail(ctx, &state, apiResp.JSON200)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *skillResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state skillResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Metadata (name/description/tags/labels) is edited in place. Content is not
	// touched by this route: it is versioned separately below.
	body := gen.UpdateSkillRequest{
		Name:        stringToPtr(plan.Name),
		Description: stringToPtr(plan.Description),
	}
	resp.Diagnostics.Append(stringMapToPtr(ctx, plan.Labels, &body.Labels)...)
	resp.Diagnostics.Append(listToStringSlice(ctx, plan.Tags, &body.Tags)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.Gen.SkillsUpdateSkillRouteWithResponse(ctx, plan.ID.ValueString(), body)
	if err != nil {
		resp.Diagnostics.AddError("Error updating skill", err.Error())
		return
	}
	if err := client.Check(apiResp.HTTPResponse, apiResp.Body); err != nil {
		resp.Diagnostics.AddError("Error updating skill", err.Error())
		return
	}
	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError("Error updating skill", "API returned an empty body")
		return
	}
	detail := apiResp.JSON200

	// A changed content body or a changed folder publishes a new version — they are one version, so
	// either one alone is a publish. Compared against prior state so an unchanged skill does not churn
	// the version counter.
	contentChanged := !plan.Content.IsNull() && !plan.Content.IsUnknown() && plan.Content.ValueString() != state.Content.ValueString()
	if contentChanged || !skillResourcesEqual(plan.Resources, state.Resources) {
		detail = r.publishVersion(ctx, plan.ID.ValueString(), plan, &resp.Diagnostics)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	resp.Diagnostics.Append(skillApplyDetail(ctx, &plan, detail)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *skillResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state skillResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.Gen.SkillsDeleteSkillRouteWithResponse(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error deleting skill", err.Error())
		return
	}
	if err := client.Check(apiResp.HTTPResponse, apiResp.Body); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Error deleting skill", err.Error())
	}
}

func (r *skillResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// publishVersion publishes `content` plus the whole folder as one new version and returns the skill
// as it reads back afterwards, so content/content_version/updated_at reflect what was published.
func (r *skillResource) publishVersion(ctx context.Context, id string, plan skillResourceModel, diags *diag.Diagnostics) *gen.SkillDetail {
	verResp, err := r.client.Gen.SkillsPublishSkillVersionRouteWithResponse(ctx, id, gen.PublishSkillVersionRequest{
		Content:   plan.Content.ValueString(),
		Resources: skillResourcesToAPI(plan.Resources),
	})
	if err != nil {
		diags.AddError("Error publishing skill version", err.Error())
		return nil
	}
	if err := client.Check(verResp.HTTPResponse, verResp.Body); err != nil {
		diags.AddError("Error publishing skill version", err.Error())
		return nil
	}

	getResp, err := r.client.Gen.SkillsGetSkillRouteWithResponse(ctx, id)
	if err != nil {
		diags.AddError("Error reading skill after publishing version", err.Error())
		return nil
	}
	if err := client.Check(getResp.HTTPResponse, getResp.Body); err != nil {
		diags.AddError("Error reading skill after publishing version", err.Error())
		return nil
	}
	if getResp.JSON200 == nil {
		diags.AddError("Error reading skill after publishing version", "API returned an empty body")
		return nil
	}
	return getResp.JSON200
}

func skillResourcesToAPI(items []skillResourceFileModel) *[]gen.SkillResource {
	if len(items) == 0 {
		return nil
	}
	out := make([]gen.SkillResource, 0, len(items))
	for _, item := range items {
		out = append(out, gen.SkillResource{
			Path:       item.Path.ValueString(),
			Content:    stringToPtr(item.Content),
			Executable: boolToPtr(item.Executable),
		})
	}
	return &out
}

func skillResourcesFromAPI(items *[]gen.SkillResource) []skillResourceFileModel {
	if items == nil || len(*items) == 0 {
		return nil
	}
	out := make([]skillResourceFileModel, 0, len(*items))
	for _, item := range *items {
		content := ""
		if item.Content != nil {
			content = *item.Content
		}
		executable := false
		if item.Executable != nil {
			executable = *item.Executable
		}
		out = append(out, skillResourceFileModel{
			Path:       types.StringValue(item.Path),
			Content:    types.StringValue(content),
			Executable: types.BoolValue(executable),
		})
	}
	return out
}

func skillResourcesEqual(a, b []skillResourceFileModel) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !a[i].Path.Equal(b[i].Path) || !a[i].Content.Equal(b[i].Content) || !a[i].Executable.Equal(b[i].Executable) {
			return false
		}
	}
	return true
}

// skillApplyDetail writes a SkillDetail response into the model.
func skillApplyDetail(ctx context.Context, m *skillResourceModel, s *gen.SkillDetail) diag.Diagnostics {
	m.ID = types.StringValue(s.SkillId)
	m.Name = types.StringValue(s.Name)
	m.Description = types.StringValue(s.Description)
	m.Content = types.StringValue(s.Content)
	m.Resources = skillResourcesFromAPI(s.Resources)
	m.Kind = types.StringValue(enumPtrToString(s.Kind))
	m.ContentVersion = intPtrToInt64(s.ContentVersion)
	m.UpdatedAt = types.StringValue(s.UpdatedAt)

	labels, diags := stringMapValue(ctx, s.Labels)
	if diags.HasError() {
		return diags
	}
	m.Labels = labels

	tags, tagDiags := types.ListValueFrom(ctx, types.StringType, s.Tags)
	diags.Append(tagDiags...)
	m.Tags = tags
	return diags
}
