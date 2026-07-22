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

// Object attribute types for the computed nested lists, which must be modeled
// as types.List so they can carry unknown values during planning.
var (
	reviewWorkflowRepoStatusAttrTypes = map[string]attr.Type{
		"repo_owner":     types.StringType,
		"repo_name":      types.StringType,
		"webhook_status": types.StringType,
		"github_hook_id": types.Int64Type,
		"webhook_error":  types.StringType,
	}
	reviewWorkflowManualSetupAttrTypes = map[string]attr.Type{
		"repo_owner":   types.StringType,
		"repo_name":    types.StringType,
		"webhook_url":  types.StringType,
		"secret":       types.StringType,
		"content_type": types.StringType,
		"events":       types.ListType{ElemType: types.StringType},
	}
)

// Ensure the resource satisfies the framework interfaces.
var (
	_ resource.Resource                = &reviewWorkflowResource{}
	_ resource.ResourceWithConfigure   = &reviewWorkflowResource{}
	_ resource.ResourceWithImportState = &reviewWorkflowResource{}
)

// NewReviewWorkflowResource is the constructor registered with the provider.
func NewReviewWorkflowResource() resource.Resource {
	return &reviewWorkflowResource{}
}

type reviewWorkflowResource struct {
	client *client.Client
}

// reviewWorkflowResourceModel maps the agentops_review_workflow schema to Go.
type reviewWorkflowResourceModel struct {
	ID               types.String              `tfsdk:"id"`
	Name             types.String              `tfsdk:"name"`
	Status           types.String              `tfsdk:"status"`
	BaseBranchFilter types.String              `tfsdk:"base_branch_filter"`
	ReviewerAgentIDs types.List                `tfsdk:"reviewer_agent_ids"`
	Repos            []reviewWorkflowRepoModel `tfsdk:"repos"`
	RepoStatus       types.List                `tfsdk:"repo_status"`
	ManualSetup      types.List                `tfsdk:"manual_setup"`
	RepoCount        types.Int64               `tfsdk:"repo_count"`
	WebhookURL       types.String              `tfsdk:"webhook_url"`
	CreatedAt        types.String              `tfsdk:"created_at"`
	UpdatedAt        types.String              `tfsdk:"updated_at"`
}

type reviewWorkflowRepoModel struct {
	RepoOwner types.String `tfsdk:"repo_owner"`
	RepoName  types.String `tfsdk:"repo_name"`
}

type reviewWorkflowRepoStatusModel struct {
	RepoOwner     types.String `tfsdk:"repo_owner"`
	RepoName      types.String `tfsdk:"repo_name"`
	WebhookStatus types.String `tfsdk:"webhook_status"`
	GithubHookID  types.Int64  `tfsdk:"github_hook_id"`
	WebhookError  types.String `tfsdk:"webhook_error"`
}

type reviewWorkflowManualSetupModel struct {
	RepoOwner   types.String `tfsdk:"repo_owner"`
	RepoName    types.String `tfsdk:"repo_name"`
	WebhookURL  types.String `tfsdk:"webhook_url"`
	Secret      types.String `tfsdk:"secret"`
	ContentType types.String `tfsdk:"content_type"`
	Events      types.List   `tfsdk:"events"`
}

func (r *reviewWorkflowResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_review_workflow"
}

func (r *reviewWorkflowResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A code-review workflow that runs reviewer agents against pull requests in " +
			"one or more GitHub repositories.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Workflow identifier.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Human-readable workflow name. Server-assigned when omitted.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"status": schema.StringAttribute{
				MarkdownDescription: "Lifecycle status. Newly created workflows start as `draft`; set to " +
					"`active` or `paused` to publish or suspend the workflow. One of `draft`, `active`, `paused`.",
				Optional:      true,
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"base_branch_filter": schema.StringAttribute{
				MarkdownDescription: "Only review pull requests targeting this base branch. Reviews all branches when omitted.",
				Optional:            true,
			},
			"reviewer_agent_ids": schema.ListAttribute{
				MarkdownDescription: "IDs of the reviewer agents that run against matching pull requests.",
				ElementType:         types.StringType,
				Required:            true,
			},
			"repos": schema.ListNestedAttribute{
				MarkdownDescription: "GitHub repositories this workflow reviews.",
				Optional:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"repo_owner": schema.StringAttribute{
							MarkdownDescription: "GitHub repository owner (user or organization).",
							Required:            true,
						},
						"repo_name": schema.StringAttribute{
							MarkdownDescription: "GitHub repository name.",
							Required:            true,
						},
					},
				},
			},
			"repo_status": schema.ListNestedAttribute{
				MarkdownDescription: "Observed webhook status for each configured repository.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"repo_owner":     schema.StringAttribute{Computed: true, MarkdownDescription: "GitHub repository owner."},
						"repo_name":      schema.StringAttribute{Computed: true, MarkdownDescription: "GitHub repository name."},
						"webhook_status": schema.StringAttribute{Computed: true, MarkdownDescription: "Webhook provisioning status."},
						"github_hook_id": schema.Int64Attribute{Computed: true, MarkdownDescription: "GitHub webhook ID, when provisioned."},
						"webhook_error":  schema.StringAttribute{Computed: true, MarkdownDescription: "Webhook provisioning error, when failed."},
					},
				},
			},
			"manual_setup": schema.ListNestedAttribute{
				MarkdownDescription: "Manual webhook setup instructions for repositories the server could not " +
					"configure automatically.",
				Computed: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"repo_owner":   schema.StringAttribute{Computed: true, MarkdownDescription: "GitHub repository owner."},
						"repo_name":    schema.StringAttribute{Computed: true, MarkdownDescription: "GitHub repository name."},
						"webhook_url":  schema.StringAttribute{Computed: true, MarkdownDescription: "Webhook URL to register in GitHub."},
						"secret":       schema.StringAttribute{Computed: true, Sensitive: true, MarkdownDescription: "Webhook signing secret. Sensitive."},
						"content_type": schema.StringAttribute{Computed: true, MarkdownDescription: "Webhook content type to configure."},
						"events": schema.ListAttribute{
							Computed:            true,
							ElementType:         types.StringType,
							MarkdownDescription: "GitHub events the webhook should send.",
						},
					},
				},
			},
			"repo_count": schema.Int64Attribute{
				MarkdownDescription: "Number of repositories configured on the workflow.",
				Computed:            true,
			},
			"webhook_url": schema.StringAttribute{
				MarkdownDescription: "Webhook URL GitHub posts pull-request events to.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
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

func (r *reviewWorkflowResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromProviderData(req.ProviderData, &resp.Diagnostics)
}

func (r *reviewWorkflowResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan reviewWorkflowResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := gen.CreateReviewWorkflowRequest{
		Name:             stringToPtr(plan.Name),
		BaseBranchFilter: stringToPtr(plan.BaseBranchFilter),
		Repos:            reviewWorkflowReposToAPI(plan.Repos),
	}
	resp.Diagnostics.Append(listToStringSlice(ctx, plan.ReviewerAgentIDs, &body.ReviewerAgentIds)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.Gen.ReviewWorkflowsCreateReviewWorkflowEndpointWithResponse(ctx, body)
	if err != nil {
		resp.Diagnostics.AddError("Error creating review workflow", err.Error())
		return
	}
	if err := client.Check(apiResp.HTTPResponse, apiResp.Body); err != nil {
		resp.Diagnostics.AddError("Error creating review workflow", err.Error())
		return
	}
	if apiResp.JSON201 == nil {
		resp.Diagnostics.AddError("Error creating review workflow", "API returned an empty body")
		return
	}

	desired := statusTarget(plan.Status)

	// The review workflow now exists server-side. Persist state before reconciling status
	// so a failed activate/pause still leaves the workflow tracked and destroyable rather
	// than orphaned on the server.
	if diags := reviewWorkflowApply(ctx, &plan, apiResp.JSON201); diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	detail := r.reconcileStatus(ctx, apiResp.JSON201.Id, desired, apiResp.JSON201, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(reviewWorkflowApply(ctx, &plan, detail)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *reviewWorkflowResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state reviewWorkflowResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.Gen.ReviewWorkflowsGetReviewWorkflowEndpointWithResponse(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading review workflow", err.Error())
		return
	}
	if err := client.Check(apiResp.HTTPResponse, apiResp.Body); err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading review workflow", err.Error())
		return
	}
	if apiResp.JSON200 == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	resp.Diagnostics.Append(reviewWorkflowApply(ctx, &state, apiResp.JSON200)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *reviewWorkflowResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan reviewWorkflowResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := gen.UpdateReviewWorkflowRequest{
		Name:             stringToPtr(plan.Name),
		BaseBranchFilter: stringToPtr(plan.BaseBranchFilter),
		Repos:            reviewWorkflowReposToAPI(plan.Repos),
	}
	resp.Diagnostics.Append(listToStringSlice(ctx, plan.ReviewerAgentIDs, &body.ReviewerAgentIds)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.Gen.ReviewWorkflowsUpdateReviewWorkflowEndpointWithResponse(ctx, plan.ID.ValueString(), body)
	if err != nil {
		resp.Diagnostics.AddError("Error updating review workflow", err.Error())
		return
	}
	if err := client.Check(apiResp.HTTPResponse, apiResp.Body); err != nil {
		resp.Diagnostics.AddError("Error updating review workflow", err.Error())
		return
	}
	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError("Error updating review workflow", "API returned an empty body")
		return
	}

	desired := statusTarget(plan.Status)
	detail := r.reconcileStatus(ctx, plan.ID.ValueString(), desired, apiResp.JSON200, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(reviewWorkflowApply(ctx, &plan, detail)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *reviewWorkflowResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state reviewWorkflowResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.Gen.ReviewWorkflowsDeleteReviewWorkflowEndpointWithResponse(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error deleting review workflow", err.Error())
		return
	}
	if err := client.Check(apiResp.HTTPResponse, apiResp.Body); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Error deleting review workflow", err.Error())
	}
}

func (r *reviewWorkflowResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// reconcileStatus drives the workflow to the desired lifecycle status via the
// activate/pause endpoints. It returns the latest detail (unchanged when no
// transition is needed).
func (r *reviewWorkflowResource) reconcileStatus(ctx context.Context, id, desired string, detail *gen.ReviewWorkflowDetail, diags *diag.Diagnostics) *gen.ReviewWorkflowDetail {
	if desired == "" || desired == string(detail.Status) {
		return detail
	}

	switch desired {
	case "active":
		apiResp, err := r.client.Gen.ReviewWorkflowsActivateReviewWorkflowEndpointWithResponse(ctx, id)
		if err != nil {
			diags.AddError("Error activating review workflow", err.Error())
			return detail
		}
		if err := client.Check(apiResp.HTTPResponse, apiResp.Body); err != nil {
			diags.AddError("Error activating review workflow", err.Error())
			return detail
		}
		if apiResp.JSON200 != nil {
			return apiResp.JSON200
		}
	case "paused":
		apiResp, err := r.client.Gen.ReviewWorkflowsPauseReviewWorkflowEndpointWithResponse(ctx, id)
		if err != nil {
			diags.AddError("Error pausing review workflow", err.Error())
			return detail
		}
		if err := client.Check(apiResp.HTTPResponse, apiResp.Body); err != nil {
			diags.AddError("Error pausing review workflow", err.Error())
			return detail
		}
		if apiResp.JSON200 != nil {
			return apiResp.JSON200
		}
	case "draft":
		diags.AddAttributeError(path.Root("status"), "Cannot return a review workflow to draft",
			"A published workflow cannot be reverted to `draft`; use `paused` to suspend it.")
	}
	return detail
}

func reviewWorkflowReposToAPI(items []reviewWorkflowRepoModel) *[]gen.ReviewWorkflowRepoInput {
	if items == nil {
		return nil
	}
	out := make([]gen.ReviewWorkflowRepoInput, 0, len(items))
	for _, repo := range items {
		out = append(out, gen.ReviewWorkflowRepoInput{
			RepoOwner: repo.RepoOwner.ValueString(),
			RepoName:  repo.RepoName.ValueString(),
		})
	}
	return &out
}

// reviewWorkflowApply writes a ReviewWorkflowDetail response into the model.
func reviewWorkflowApply(ctx context.Context, m *reviewWorkflowResourceModel, d *gen.ReviewWorkflowDetail) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.StringValue(d.Id)
	m.Name = types.StringValue(d.Name)
	m.Status = types.StringValue(string(d.Status))
	m.BaseBranchFilter = ptrToString(d.BaseBranchFilter)
	m.RepoCount = types.Int64Value(int64(d.RepoCount))
	m.WebhookURL = ptrToString(d.WebhookUrl)
	m.CreatedAt = types.StringValue(d.CreatedAt)
	m.UpdatedAt = types.StringValue(d.UpdatedAt)

	reviewers, dg := types.ListValueFrom(ctx, types.StringType, d.ReviewerAgentIds)
	diags.Append(dg...)
	m.ReviewerAgentIDs = reviewers

	// The desired repo set (owner/name) is reconstructed from the response so it
	// stays in sync with the server; observability lives in repo_status.
	repoStatusType := types.ObjectType{AttrTypes: reviewWorkflowRepoStatusAttrTypes}
	if d.Repos != nil && len(*d.Repos) > 0 {
		repos := make([]reviewWorkflowRepoModel, 0, len(*d.Repos))
		status := make([]reviewWorkflowRepoStatusModel, 0, len(*d.Repos))
		for _, repo := range *d.Repos {
			repos = append(repos, reviewWorkflowRepoModel{
				RepoOwner: types.StringValue(repo.RepoOwner),
				RepoName:  types.StringValue(repo.RepoName),
			})
			status = append(status, reviewWorkflowRepoStatusModel{
				RepoOwner:     types.StringValue(repo.RepoOwner),
				RepoName:      types.StringValue(repo.RepoName),
				WebhookStatus: types.StringValue(string(repo.WebhookStatus)),
				GithubHookID:  intPtrToInt64(repo.GithubHookId),
				WebhookError:  ptrToString(repo.WebhookError),
			})
		}
		m.Repos = repos
		statusList, dg := types.ListValueFrom(ctx, repoStatusType, status)
		diags.Append(dg...)
		m.RepoStatus = statusList
	} else {
		m.Repos = nil
		m.RepoStatus = types.ListValueMust(repoStatusType, []attr.Value{})
	}

	manualSetupType := types.ObjectType{AttrTypes: reviewWorkflowManualSetupAttrTypes}
	if d.ManualSetup != nil && len(*d.ManualSetup) > 0 {
		setup := make([]reviewWorkflowManualSetupModel, 0, len(*d.ManualSetup))
		for _, ms := range *d.ManualSetup {
			events, dg := types.ListValueFrom(ctx, types.StringType, ptrOrEmptySlice(ms.Events))
			diags.Append(dg...)
			setup = append(setup, reviewWorkflowManualSetupModel{
				RepoOwner:   types.StringValue(ms.RepoOwner),
				RepoName:    types.StringValue(ms.RepoName),
				WebhookURL:  types.StringValue(ms.WebhookUrl),
				Secret:      types.StringValue(ms.Secret),
				ContentType: strOrNull(enumPtrToString(ms.ContentType)),
				Events:      events,
			})
		}
		setupList, dg := types.ListValueFrom(ctx, manualSetupType, setup)
		diags.Append(dg...)
		m.ManualSetup = setupList
	} else {
		m.ManualSetup = types.ListValueMust(manualSetupType, []attr.Value{})
	}

	return diags
}

// intPtrToInt64 converts an optional API int into a Terraform int64, mapping nil
// to null.
func intPtrToInt64(p *int) types.Int64 {
	if p == nil {
		return types.Int64Null()
	}
	return types.Int64Value(int64(*p))
}

// ptrOrEmptySlice dereferences an optional string slice, returning an empty
// slice when nil.
func ptrOrEmptySlice(p *[]string) []string {
	if p == nil {
		return []string{}
	}
	return *p
}
