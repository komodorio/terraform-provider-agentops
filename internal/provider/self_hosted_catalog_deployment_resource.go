// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
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
	_ resource.Resource              = &selfHostedCatalogDeploymentResource{}
	_ resource.ResourceWithConfigure = &selfHostedCatalogDeploymentResource{}
)

// NewSelfHostedCatalogDeploymentResource is the constructor registered with the provider.
func NewSelfHostedCatalogDeploymentResource() resource.Resource {
	return &selfHostedCatalogDeploymentResource{}
}

type selfHostedCatalogDeploymentResource struct {
	client *client.Client
}

// selfHostedCatalogDeploymentResourceModel maps the
// agentops_self_hosted_catalog_deployment schema to Go. The deploy registers a
// self-hosted agent and mints its worker token; nothing runs until the operator
// installs the chart, so the deploy-time inputs are write-only (the API never
// echoes them) and only the agent's own fields are refreshed on read.
type selfHostedCatalogDeploymentResourceModel struct {
	ID                     types.String `tfsdk:"id"`
	CatalogID              types.String `tfsdk:"catalog_id"`
	AgentID                types.String `tfsdk:"agent_id"`
	CredentialRef          types.String `tfsdk:"credential_ref"`
	DisplayName            types.String `tfsdk:"display_name"`
	McpGroupID             types.String `tfsdk:"mcp_group_id"`
	IntegrationConnections types.Map    `tfsdk:"integration_connections"`

	Token           types.String `tfsdk:"token"`
	WorkerTokenHint types.String `tfsdk:"worker_token_hint"`
	Status          types.String `tfsdk:"status"`
	Name            types.String `tfsdk:"name"`
	IsArchived      types.Bool   `tfsdk:"is_archived"`
	CreatedAt       types.String `tfsdk:"created_at"`
}

func (r *selfHostedCatalogDeploymentResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_self_hosted_catalog_deployment"
}

func (r *selfHostedCatalogDeploymentResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Deploys a curated worker from the worker catalog into **your own** cluster. The " +
			"control plane validates the catalog entry's credentials and integrations, registers a self-hosted " +
			"agent and mints its worker token — it does not run anything. Feed `token` into a " +
			"`kubernetes_secret` and install the catalog entry's image with the `agentops-agent-base` chart to " +
			"bring the worker online.\n\n" +
			"Use `agentops_worker_catalog_deployment` instead when the control plane should host the worker for " +
			"you; that variant generates a git repository and exposes `repo_*` attributes, while this one " +
			"exposes the token you need to run the worker yourself.\n\n" +
			"Unlike `agentops_agent`, this endpoint returns no rendered Helm values: the chart coordinates come " +
			"from the catalog entry (see the `agentops_worker_catalog` data source) and the token is supplied " +
			"separately.\n\n" +
			"**Authentication:** the self-hosted deploy endpoint requires a user-bound API key (a PAT). A " +
			"service-account token is rejected with `403`, so configure the provider with a personal API key " +
			"for this resource.\n\n" +
			"Deploy-time inputs are write-only (the API does not return them), so out-of-band changes to them " +
			"are not detected, and changing any of them forces a new deployment — which mints a **new** worker " +
			"token and invalidates the secret already applied to the cluster. This resource does not support " +
			"import: the originating `catalog_id` and the once-only token cannot be recovered from the API.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Opaque identifier of the registered self-hosted agent, assigned by the " +
					"control plane. This — not `agent_id` — is what the deploy response returns.",
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"catalog_id": schema.StringAttribute{
				MarkdownDescription: "ID of the worker catalog entry to deploy. Changing this forces a new deployment.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"agent_id": schema.StringAttribute{
				MarkdownDescription: "Friendly identifier (slug) for the deployed instance, unique within the " +
					"account. Defaults to the catalog entry's own id when omitted. Changing this forces a new " +
					"deployment.",
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
				MarkdownDescription: "ID of an MCP gateway group to attach. Can be changed without redeploying — " +
					"rebinding the group does not rotate the worker token. " + mcpGroupDriftNote,
				Optional: true,
			},
			"integration_connections": schema.MapAttribute{
				MarkdownDescription: "Integration connections to bind at deploy time, keyed by the provider the " +
					"catalog entry requires. Write-only. Changing this forces a new deployment.",
				ElementType:   types.StringType,
				Optional:      true,
				PlanModifiers: []planmodifier.Map{mapplanmodifier.RequiresReplace()},
			},

			"token": schema.StringAttribute{
				MarkdownDescription: "Plaintext worker token, returned once at deploy time. Pass it to the worker " +
					"as `AGENTOPS_WORKER_TOKEN` (e.g. through a `kubernetes_secret` consumed by the " +
					"`agentops-agent-base` chart). Stored in Terraform state — treat the state as a secret.",
				Computed:      true,
				Sensitive:     true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"worker_token_hint": schema.StringAttribute{
				MarkdownDescription: "Masked hint of the worker token (e.g. `wt_abc1...`), safe to log.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"status": schema.StringAttribute{
				MarkdownDescription: "Agent status (`online`, `offline`, `draft`, etc.). Stays `draft`/`offline` " +
					"until the worker is installed in your cluster and heartbeats.",
				Computed: true,
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Display name as reported by the API.",
				Computed:            true,
			},
			"is_archived": schema.BoolAttribute{
				MarkdownDescription: "Whether the agent has been archived.",
				Computed:            true,
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "Creation timestamp.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
		},
	}
}

func (r *selfHostedCatalogDeploymentResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromProviderData(req.ProviderData, &resp.Diagnostics)
}

func (r *selfHostedCatalogDeploymentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan selfHostedCatalogDeploymentResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := gen.WorkerCatalogSelfHostedDeployRequest{
		AgentId:       stringToPtr(plan.AgentID),
		CredentialRef: stringToPtr(plan.CredentialRef),
		DisplayName:   stringToPtr(plan.DisplayName),
		McpGroupId:    stringToPtr(plan.McpGroupID),
	}
	resp.Diagnostics.Append(stringMapToPtr(ctx, plan.IntegrationConnections, &body.IntegrationConnections)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.Gen.WorkerCatalogSelfHostedDeployWorkerCatalogWithResponse(ctx, plan.CatalogID.ValueString(), body)
	if err != nil {
		resp.Diagnostics.AddError("Error self-hosted deploying worker catalog entry", err.Error())
		return
	}
	if err := client.Check(apiResp.HTTPResponse, apiResp.Body); err != nil {
		resp.Diagnostics.AddError("Error self-hosted deploying worker catalog entry", err.Error())
		return
	}

	// 207 is a partial success: the agent and its token exist even though a
	// requested trigger or settings field did not apply. The server never rolls the
	// agent back for one, so neither does this — keep it and surface the failures.
	deployed := apiResp.JSON201
	if deployed == nil {
		deployed = apiResp.JSON207
	}
	if deployed == nil {
		resp.Diagnostics.AddError("Error self-hosted deploying worker catalog entry", "API returned an empty body")
		return
	}
	if apiResp.HTTPResponse.StatusCode == http.StatusMultiStatus {
		resp.Diagnostics.AddWarning("Worker deployed, but part of the request did not apply",
			selfHostedDeployPartialDetail(deployed))
	}

	// The response carries the opaque agent id, never the friendly slug, so the
	// slug of a server-defaulted agent_id has to come back from the agent itself.
	plan.ID = types.StringValue(deployed.AgentId)
	plan.Token = types.StringValue(deployed.Token)
	plan.WorkerTokenHint = types.StringValue(deployed.WorkerTokenHint)
	// token is returned once and never again: from here on every path has to end
	// at State.Set, or a failed read makes an existing worker's token
	// unrecoverable and the agent has to be deleted and redeployed.
	selfHostedResolveComputed(&plan, nil)

	r.refreshAgent(ctx, &plan, &resp.Diagnostics)
	if plan.AgentID.IsNull() || plan.AgentID.IsUnknown() {
		plan.AgentID = types.StringValue(deployed.AgentId)
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *selfHostedCatalogDeploymentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state selfHostedCatalogDeploymentResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.Gen.AgentsGetAgentWithResponse(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading self-hosted deployed worker", err.Error())
		return
	}
	if err := client.Check(apiResp.HTTPResponse, apiResp.Body); err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading self-hosted deployed worker", err.Error())
		return
	}
	if apiResp.JSON200 == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	// Only the agent's own fields are refreshed; the token and the write-only
	// deploy inputs are preserved from state because the API never returns them.
	selfHostedCatalogDeploymentApplyInstance(&state, apiResp.JSON200, phaseRefresh)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update handles the one in-place input, mcp_group_id; every other deploy-time
// input forces replacement, so there is no catalog re-deploy to perform here.
func (r *selfHostedCatalogDeploymentResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state selfHostedCatalogDeploymentResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// The token is minted once at deploy time and never re-returned.
	plan.Token = state.Token
	plan.WorkerTokenHint = state.WorkerTokenHint

	if state.McpGroupID.ValueString() != plan.McpGroupID.ValueString() {
		var mcpPtr *string
		if !plan.McpGroupID.IsNull() && !plan.McpGroupID.IsUnknown() {
			mcpPtr = stringToPtr(plan.McpGroupID)
		}
		mcpResp, err := r.client.Gen.AgentsSetMcpGroupWithResponse(ctx, plan.ID.ValueString(),
			gen.SetMcpGroupRequest{McpGroupId: mcpPtr})
		if err != nil {
			resp.Diagnostics.AddError("Error updating MCP group binding", err.Error())
			return
		}
		if err := client.Check(mcpResp.HTTPResponse, mcpResp.Body); err != nil {
			resp.Diagnostics.AddError("Error updating MCP group binding", err.Error())
			return
		}
	}

	selfHostedResolveComputed(&plan, &state)
	r.refreshAgent(ctx, &plan, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *selfHostedCatalogDeploymentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state selfHostedCatalogDeploymentResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	archiveAndDeleteAgent(ctx, r.client, state.ID.ValueString(), &resp.Diagnostics)
}

// selfHostedResolveComputed makes every attribute the post-mutation read would
// fill known before it runs — from prior state on update, null on create. The
// read is deliberately soft, and an unknown left in state is rejected as an
// inconsistent apply result, which on create would lose the once-only token.
func selfHostedResolveComputed(m *selfHostedCatalogDeploymentResourceModel, prior *selfHostedCatalogDeploymentResourceModel) {
	priorStatus, priorName, priorCreatedAt := types.StringNull(), types.StringNull(), types.StringNull()
	priorArchived := types.BoolNull()
	if prior != nil {
		priorStatus, priorName, priorCreatedAt, priorArchived = prior.Status, prior.Name, prior.CreatedAt, prior.IsArchived
	}
	if m.Status.IsUnknown() {
		m.Status = priorStatus
	}
	if m.Name.IsUnknown() {
		m.Name = priorName
	}
	if m.CreatedAt.IsUnknown() {
		m.CreatedAt = priorCreatedAt
	}
	if m.IsArchived.IsUnknown() {
		m.IsArchived = priorArchived
	}
}

func (r *selfHostedCatalogDeploymentResource) refreshAgent(ctx context.Context, m *selfHostedCatalogDeploymentResourceModel, diags *diag.Diagnostics) {
	apiResp, err := r.client.Gen.AgentsGetAgentWithResponse(ctx, m.ID.ValueString())
	if err != nil {
		diags.AddWarning("Could not read self-hosted deployed worker after mutation", err.Error())
		return
	}
	if checkErr := client.Check(apiResp.HTTPResponse, apiResp.Body); checkErr != nil {
		diags.AddWarning("Could not read self-hosted deployed worker after mutation", checkErr.Error())
		return
	}
	if apiResp.JSON200 != nil {
		selfHostedCatalogDeploymentApplyInstance(m, apiResp.JSON200, phaseApply)
	}
}

// selfHostedCatalogDeploymentApplyInstance writes the agent's own fields into the
// model, leaving the token and the write-only deploy inputs untouched.
func selfHostedCatalogDeploymentApplyInstance(m *selfHostedCatalogDeploymentResourceModel, inst *gen.AgentInstanceResponse, phase readPhase) {
	// Never overwrite a configured slug: the account, not the API, is its source
	// of truth, and a normalized echo would read as an inconsistent apply result.
	if (m.AgentID.IsNull() || m.AgentID.IsUnknown()) && inst.IdSlug != nil && *inst.IdSlug != "" {
		m.AgentID = types.StringValue(*inst.IdSlug)
	}
	m.McpGroupID = reconcileManagedOptional(phase, m.McpGroupID, strOrNull(enumPtrToString(inst.McpGroupId)))
	m.Status = types.StringValue(string(inst.Status))
	m.Name = ptrToString(inst.Name)
	m.IsArchived = boolPtrToBool(inst.IsArchived)
	m.CreatedAt = ptrToString(inst.CreatedAt)
}

// selfHostedDeployPartialDetail renders the failed entries of a 207 response as
// warning detail, including the per-item retry endpoint the API hands back.
func selfHostedDeployPartialDetail(d *gen.WorkerCatalogSelfHostedDeployResponse) string {
	const generic = "The control plane reported a partial deploy. The agent and its worker token were created."

	var sections []string
	if d.Settings != nil && d.Settings.Sections != nil {
		for _, s := range *d.Settings.Sections {
			if !strings.EqualFold(string(s.Status), "failed") {
				continue
			}
			sections = append(sections, retryLine(fmt.Sprintf("- setting %q: %s", s.Section, ptrOrDash(s.Error)), s.Retry))
		}
	}

	var triggers []string
	if d.Triggers != nil {
		for _, t := range *d.Triggers {
			if !strings.EqualFold(string(t.Status), "failed") {
				continue
			}
			triggers = append(triggers, retryLine(
				fmt.Sprintf("- %s trigger %q: %s", t.Type, ptrOrDash(t.Name), ptrOrDash(t.Error)), t.Retry))
		}
	}

	var out []string
	if len(sections) > 0 {
		sort.Strings(sections)
		out = append(out, "The agent and its worker token were created, but these settings were not applied. "+
			"mcp_group_id is the one this resource tracks: the next plan reads the agent back, sees it missing "+
			"and re-binds it in place, without rotating the worker token.\n"+strings.Join(sections, "\n"))
	}
	if len(triggers) > 0 {
		sort.Strings(triggers)
		out = append(out, "The agent and its worker token were created, but these triggers were not. Re-running "+
			"apply will not fix them — create them with the agentops_trigger resource or the retry endpoint "+
			"below, because re-deploying rotates the worker token.\n"+strings.Join(triggers, "\n"))
	}
	if len(out) == 0 {
		return generic
	}
	return strings.Join(out, "\n\n")
}

func retryLine(line string, retry *string) string {
	if retry == nil || *retry == "" {
		return line
	}
	return line + fmt.Sprintf(" (retry: %s)", *retry)
}

func ptrOrDash(p *string) string {
	if p == nil || *p == "" {
		return "-"
	}
	return *p
}
