// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/komodorio/terraform-provider-agentops/internal/client"
	"github.com/komodorio/terraform-provider-agentops/internal/client/gen"
)

// Ensure the resource satisfies the framework interfaces.
var (
	_ resource.Resource              = &workerCatalogDeploymentResource{}
	_ resource.ResourceWithConfigure = &workerCatalogDeploymentResource{}
)

// NewWorkerCatalogDeploymentResource is the constructor registered with the provider.
func NewWorkerCatalogDeploymentResource() resource.Resource {
	return &workerCatalogDeploymentResource{}
}

type workerCatalogDeploymentResource struct {
	client *client.Client
}

// workerCatalogDeploymentResourceModel maps the agentops_worker_catalog_deployment
// schema to Go. A catalog deploy produces a hosted agent: the server pins the image
// and derives the customer/credentials, so the deploy-time inputs are write-only (the
// API never echoes them) and only the identity/computed fields are refreshed on read.
type workerCatalogDeploymentResourceModel struct {
	ID                     types.String `tfsdk:"id"`
	CatalogID              types.String `tfsdk:"catalog_id"`
	AgentID                types.String `tfsdk:"agent_id"`
	CredentialRef          types.String `tfsdk:"credential_ref"`
	DisplayName            types.String `tfsdk:"display_name"`
	McpGroupID             types.String `tfsdk:"mcp_group_id"`
	IntegrationConnections types.Map    `tfsdk:"integration_connections"`
	WaitForOnline          types.Bool   `tfsdk:"wait_for_online"`
	WaitTimeout            types.String `tfsdk:"wait_timeout"`

	Customer       types.String         `tfsdk:"customer"`
	Identity       types.String         `tfsdk:"identity"`
	RuntimeAgentID types.String         `tfsdk:"runtime_agent_id"`
	RepoOwner      types.String         `tfsdk:"repo_owner"`
	RepoName       types.String         `tfsdk:"repo_name"`
	RepoBranch     types.String         `tfsdk:"repo_branch"`
	RepoPath       types.String         `tfsdk:"repo_path"`
	LastCommitSha  types.String         `tfsdk:"last_commit_sha"`
	Status         types.String         `tfsdk:"status"`
	Values         jsontypes.Normalized `tfsdk:"values"`
	CreatedAt      types.String         `tfsdk:"created_at"`
	UpdatedAt      types.String         `tfsdk:"updated_at"`
}

func (r *workerCatalogDeploymentResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_worker_catalog_deployment"
}

func (r *workerCatalogDeploymentResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Deploys a curated worker from the worker catalog. The server pins the image and " +
			"derives the customer and credentials from the catalog entry and your account, so you only name the " +
			"instance — the result is a managed hosted agent. Deploy-time inputs are write-only (the API does not " +
			"return them), so out-of-band changes to them are not detected, and changing any of them forces a new " +
			"deployment. This resource does not support import: the originating `catalog_id` and the write-only " +
			"inputs cannot be recovered from the API.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Hosted agent record identifier of the deployed worker.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"catalog_id": schema.StringAttribute{
				MarkdownDescription: "ID of the worker catalog entry to deploy. Changing this forces a new deployment.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"agent_id": schema.StringAttribute{
				MarkdownDescription: "Stable identifier for the deployed instance. Server-assigned when omitted. " +
					"Changing this forces a new deployment.",
				Optional:      true,
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace(), stringplanmodifier.UseStateForUnknown()},
			},
			"credential_ref": schema.StringAttribute{
				MarkdownDescription: "Name of the LLM credential the worker runs with. Must be one of the catalog " +
					"entry's allowed credentials; the server picks the default when omitted. Write-only. Changing " +
					"this forces a new deployment.",
				Optional:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"display_name": schema.StringAttribute{
				MarkdownDescription: "Human-readable name for the deployed worker. Write-only. Changing this forces a new deployment.",
				Optional:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"mcp_group_id": schema.StringAttribute{
				MarkdownDescription: "ID of an MCP gateway group to attach. Write-only. Changing this forces a new deployment.",
				Optional:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"integration_connections": schema.MapAttribute{
				MarkdownDescription: "Integration connections to bind at deploy time, keyed by the provider the " +
					"catalog entry requires. Write-only. Changing this forces a new deployment.",
				ElementType:   types.StringType,
				Optional:      true,
				PlanModifiers: []planmodifier.Map{mapplanmodifier.RequiresReplace()},
			},
			"wait_for_online": schema.BoolAttribute{
				MarkdownDescription: "Whether create should block until the deployed worker reports `online` (its " +
					"first heartbeat). Defaults to `true`. Set to `false` to return as soon as the deployment is " +
					"accepted. Note: a failed cluster-side provision is not reported back as `deploy_failed` for " +
					"API-created workers, so a failure surfaces as a `wait_timeout` rather than an immediate error.",
				Optional: true,
			},
			"wait_timeout": schema.StringAttribute{
				MarkdownDescription: "Maximum time to wait for the worker to become `online`, as a Go duration " +
					"(e.g. `10m`, `90s`). Defaults to `10m`. Only used when `wait_for_online` is `true`.",
				Optional: true,
			},

			"customer": schema.StringAttribute{
				MarkdownDescription: "Customer/tenant the worker was deployed under, derived by the server.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"identity": schema.StringAttribute{
				MarkdownDescription: "Resolved runtime identity of the deployed worker.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"runtime_agent_id": schema.StringAttribute{
				MarkdownDescription: "Runtime agent ID the deployment registers as.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"repo_owner": schema.StringAttribute{
				MarkdownDescription: "Owner of the generated agent repository.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"repo_name": schema.StringAttribute{
				MarkdownDescription: "Name of the generated agent repository.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"repo_branch": schema.StringAttribute{
				MarkdownDescription: "Branch of the generated agent repository.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"repo_path": schema.StringAttribute{
				MarkdownDescription: "Path within the generated agent repository.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"last_commit_sha": schema.StringAttribute{
				MarkdownDescription: "SHA of the most recent commit to the generated repository.",
				Computed:            true,
			},
			"status": schema.StringAttribute{
				MarkdownDescription: "Deployment status (`online`, `offline`, `draft`, `deploying`, `deploy_failed`).",
				Computed:            true,
			},
			"values": schema.StringAttribute{
				MarkdownDescription: "Resolved deployment values, as a JSON object.",
				CustomType:          jsontypes.NormalizedType{},
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

func (r *workerCatalogDeploymentResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromProviderData(req.ProviderData, &resp.Diagnostics)
}

func (r *workerCatalogDeploymentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan workerCatalogDeploymentResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := gen.WorkerCatalogDeployRequest{
		AgentId:       stringToPtr(plan.AgentID),
		CredentialRef: stringToPtr(plan.CredentialRef),
		DisplayName:   stringToPtr(plan.DisplayName),
		McpGroupId:    stringToPtr(plan.McpGroupID),
	}
	resp.Diagnostics.Append(stringMapToPtr(ctx, plan.IntegrationConnections, &body.IntegrationConnections)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.Gen.WorkerCatalogDeployWorkerCatalogWithResponse(ctx, plan.CatalogID.ValueString(), body)
	if err != nil {
		resp.Diagnostics.AddError("Error deploying worker catalog entry", err.Error())
		return
	}
	if err := client.Check(apiResp.HTTPResponse, apiResp.Body); err != nil {
		resp.Diagnostics.AddError("Error deploying worker catalog entry", err.Error())
		return
	}
	if apiResp.JSON201 == nil {
		resp.Diagnostics.AddError("Error deploying worker catalog entry", "API returned an empty body")
		return
	}

	workerCatalogDeploymentApplyComputed(&plan, apiResp.JSON201)
	r.waitForOnlineIfRequested(ctx, &plan, &resp.State, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *workerCatalogDeploymentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state workerCatalogDeploymentResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.Gen.HostedAgentsGetHostedAgentWithResponse(ctx, state.Customer.ValueString(), state.AgentID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading deployed worker", err.Error())
		return
	}
	if err := client.Check(apiResp.HTTPResponse, apiResp.Body); err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading deployed worker", err.Error())
		return
	}
	if apiResp.JSON200 == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	// Only the identity/computed fields are refreshed; write-only deploy inputs are
	// preserved from state because the API never returns them.
	workerCatalogDeploymentApplyComputed(&state, apiResp.JSON200)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update handles changes to the behavioural-only inputs (wait_for_online,
// wait_timeout); every deploy-time input forces replacement, so there is no
// catalog re-deploy to perform here — the current computed state is refreshed and
// the new wait settings persisted.
func (r *workerCatalogDeploymentResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan workerCatalogDeploymentResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.Gen.HostedAgentsGetHostedAgentWithResponse(ctx, plan.Customer.ValueString(), plan.AgentID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading deployed worker", err.Error())
		return
	}
	if err := client.Check(apiResp.HTTPResponse, apiResp.Body); err != nil {
		resp.Diagnostics.AddError("Error reading deployed worker", err.Error())
		return
	}
	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError("Error reading deployed worker", "API returned an empty body")
		return
	}

	workerCatalogDeploymentApplyComputed(&plan, apiResp.JSON200)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *workerCatalogDeploymentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state workerCatalogDeploymentResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.Gen.HostedAgentsDeleteHostedAgentWithResponse(ctx, state.Customer.ValueString(), state.AgentID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error deleting deployed worker", err.Error())
		return
	}
	if err := client.Check(apiResp.HTTPResponse, apiResp.Body); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Error deleting deployed worker", err.Error())
	}
}

// workerCatalogDeploymentApplyComputed writes the identity/computed fields of a
// HostedAgentResponse into the model, leaving write-only deploy inputs untouched.
func workerCatalogDeploymentApplyComputed(m *workerCatalogDeploymentResourceModel, h *gen.HostedAgentResponse) {
	m.ID = types.StringValue(h.Id)
	m.AgentID = types.StringValue(h.AgentId)
	m.Customer = types.StringValue(h.Customer)
	m.Identity = types.StringValue(h.Identity)
	m.RuntimeAgentID = types.StringValue(h.RuntimeAgentId)
	m.RepoOwner = types.StringValue(h.RepoOwner)
	m.RepoName = types.StringValue(h.RepoName)
	m.RepoBranch = types.StringValue(h.RepoBranch)
	m.RepoPath = types.StringValue(h.RepoPath)
	m.LastCommitSha = ptrToString(h.LastCommitSha)
	m.Status = strOrNull(enumPtrToString(h.Status))
	m.Values = mapPtrToJSON(h.Values)
	m.CreatedAt = types.StringValue(h.CreatedAt)
	m.UpdatedAt = types.StringValue(h.UpdatedAt)
}

// waitForOnlineIfRequested blocks until the deployed worker reports online when
// wait_for_online is enabled, refreshing plan with the final computed fields. On
// timeout/failure it persists the current computed state (so the worker is tracked
// rather than orphaned) and records an error.
func (r *workerCatalogDeploymentResource) waitForOnlineIfRequested(ctx context.Context, plan *workerCatalogDeploymentResourceModel, state *tfsdk.State, diags *diag.Diagnostics) {
	wait, timeout, cfgDiags := hostedAgentWaitConfig(plan.WaitForOnline, plan.WaitTimeout)
	diags.Append(cfgDiags...)
	if diags.HasError() || !wait {
		return
	}

	final, err := waitForHostedAgentOnline(ctx, r.client, plan.Customer.ValueString(), plan.AgentID.ValueString(), timeout)
	if err != nil {
		diags.Append(state.Set(ctx, plan)...)
		diags.AddError("Timed out waiting for deployed worker to become online", err.Error())
		return
	}
	workerCatalogDeploymentApplyComputed(plan, final)
}
