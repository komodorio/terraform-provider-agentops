// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	"strings"
	"time"

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
	_ resource.Resource                = &agentResource{}
	_ resource.ResourceWithConfigure   = &agentResource{}
	_ resource.ResourceWithImportState = &agentResource{}
)

func NewAgentResource() resource.Resource {
	return &agentResource{}
}

type agentResource struct {
	client *client.Client
}

type agentResourceModel struct {
	ID            types.String `tfsdk:"id"`
	AgentID       types.String `tfsdk:"agent_id"`
	Instructions  types.String `tfsdk:"instructions"`
	DisplayName   types.String `tfsdk:"display_name"`
	CredentialRef types.String `tfsdk:"credential_ref"`
	Model         types.String `tfsdk:"model"`
	McpGroupID    types.String `tfsdk:"mcp_group_id"`

	InstallValues   types.String `tfsdk:"install_values"`
	InstallCommand  types.String `tfsdk:"install_command"`
	WorkerTokenHint types.String `tfsdk:"worker_token_hint"`

	Status     types.String `tfsdk:"status"`
	Name       types.String `tfsdk:"name"`
	IsArchived types.Bool   `tfsdk:"is_archived"`
	CreatedAt  types.String `tfsdk:"created_at"`
}

func (r *agentResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_agent"
}

func (r *agentResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A self-hosted KaOps agent. Creating this resource registers the agent with the " +
			"control plane and returns a complete Helm values YAML and install command for deploying it " +
			"in your own cluster. The `install_values` output is a ready-to-use values file for the " +
			"`agentops-agent-base` Helm chart. Destroying the agent archives it first, then deletes it.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Internal agent identifier assigned by the control plane.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"agent_id": schema.StringAttribute{
				MarkdownDescription: "Stable agent identifier (slug), unique within the account. Changing this forces a new agent.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"instructions": schema.StringAttribute{
				MarkdownDescription: "System prompt / instructions for the agent.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"display_name": schema.StringAttribute{
				MarkdownDescription: "Human-readable display name shown in the UI.",
				Optional:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"credential_ref": schema.StringAttribute{
				MarkdownDescription: "Name of the LLM credential configured on the account (not the secret value).",
				Optional:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"model": schema.StringAttribute{
				MarkdownDescription: "LLM model the agent uses (e.g. `claude-opus-4-8`).",
				Optional:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"mcp_group_id": schema.StringAttribute{
				MarkdownDescription: "MCP gateway group to bind to this agent. Can be changed without recreating " +
					"the agent. " + mcpGroupDriftNote,
				Optional: true,
			},

			"install_values": schema.StringAttribute{
				MarkdownDescription: "Complete Helm values YAML returned by the API at creation time. Contains image " +
					"coordinates, agent config, control-plane URL, and the worker token. Use as " +
					"`-f values.yaml` with the `agentops-agent-base` chart, passing the worker token " +
					"separately via `--set-string` if desired.",
				Computed:      true,
				Sensitive:     true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"install_command": schema.StringAttribute{
				MarkdownDescription: "Helm install command returned by the API at creation time.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"worker_token_hint": schema.StringAttribute{
				MarkdownDescription: "Masked hint of the worker token (e.g. `wt_abc1...`).",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},

			"status": schema.StringAttribute{
				MarkdownDescription: "Agent status (`online`, `offline`, `archived`, etc.).",
				Computed:            true,
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

func (r *agentResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromProviderData(req.ProviderData, &resp.Diagnostics)
}

func (r *agentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan agentResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := gen.CreateAgentRequest{
		AgentId:       plan.AgentID.ValueString(),
		Instructions:  plan.Instructions.ValueString(),
		DisplayName:   stringToPtr(plan.DisplayName),
		CredentialRef: stringToPtr(plan.CredentialRef),
		Model:         stringToPtr(plan.Model),
	}

	apiResp, err := r.client.Gen.AgentsCreateAgentWithResponse(ctx, body)
	if err != nil {
		resp.Diagnostics.AddError("Error creating agent", err.Error())
		return
	}
	if err := client.Check(apiResp.HTTPResponse, apiResp.Body); err != nil {
		resp.Diagnostics.AddError("Error creating agent", err.Error())
		return
	}

	created := apiResp.JSON200
	if created == nil {
		created = apiResp.JSON207
	}
	if created == nil {
		resp.Diagnostics.AddError("Error creating agent", "API returned an empty body")
		return
	}

	plan.ID = types.StringValue(created.AgentId)
	plan.InstallValues = types.StringValue(created.Values)
	plan.InstallCommand = types.StringValue(created.Command)
	plan.WorkerTokenHint = types.StringValue(created.WorkerTokenHint)
	// The agent is registered from here on. Every remaining computed attribute has
	// to be known before state can be written on an error path below.
	agentResolveComputed(&plan, nil)

	if !plan.McpGroupID.IsNull() && !plan.McpGroupID.IsUnknown() {
		mcpResp, err := r.client.Gen.AgentsSetMcpGroupWithResponse(ctx, plan.AgentID.ValueString(),
			gen.SetMcpGroupRequest{McpGroupId: stringToPtr(plan.McpGroupID)})
		if err != nil {
			resp.Diagnostics.AddError("Error binding MCP group to agent", err.Error())
			resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
			return
		}
		if err := client.Check(mcpResp.HTTPResponse, mcpResp.Body); err != nil {
			resp.Diagnostics.AddError("Error binding MCP group to agent", err.Error())
			resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
			return
		}
	}

	r.refreshFromGetAPI(ctx, &plan, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *agentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state agentResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.Gen.AgentsGetAgentWithResponse(ctx, state.AgentID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading agent", err.Error())
		return
	}
	if err := client.Check(apiResp.HTTPResponse, apiResp.Body); err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading agent", err.Error())
		return
	}
	if apiResp.JSON200 == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	agentApplyInstance(&state, apiResp.JSON200, phaseRefresh)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *agentResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state agentResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// install_values/command/hint are preserved from the original create
	plan.InstallValues = state.InstallValues
	plan.InstallCommand = state.InstallCommand
	plan.WorkerTokenHint = state.WorkerTokenHint

	oldMcp := state.McpGroupID.ValueString()
	newMcp := plan.McpGroupID.ValueString()
	if oldMcp != newMcp {
		var mcpPtr *string
		if !plan.McpGroupID.IsNull() && !plan.McpGroupID.IsUnknown() {
			mcpPtr = stringToPtr(plan.McpGroupID)
		}
		mcpResp, err := r.client.Gen.AgentsSetMcpGroupWithResponse(ctx, plan.AgentID.ValueString(),
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

	agentResolveComputed(&plan, &state)
	r.refreshFromGetAPI(ctx, &plan, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *agentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state agentResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	archiveAndDeleteAgent(ctx, r.client, state.AgentID.ValueString(), &resp.Diagnostics)
}

// archiveAndDeleteAgent archives an agent, waits for the archive to settle, then
// deletes it. Shared by every resource that owns a self-hosted agent record.
func archiveAndDeleteAgent(ctx context.Context, cl *client.Client, agentID string, diags *diag.Diagnostics) {
	archiveResp, err := cl.Gen.AgentsArchiveAgentWithResponse(ctx, agentID)
	if err != nil {
		diags.AddError("Error archiving agent before delete", err.Error())
		return
	}
	if archiveErr := client.Check(archiveResp.HTTPResponse, archiveResp.Body); archiveErr != nil {
		if !client.IsNotFound(archiveErr) && !client.IsConflict(archiveErr) {
			diags.AddError("Error archiving agent before delete", archiveErr.Error())
			return
		}
	}

	// Wait briefly for the archive to settle before deleting — the API may
	// reject a delete while workers are still heartbeating.
	if err := awaitAgentDeletable(ctx, cl, agentID); err != nil {
		diags.AddWarning("Agent may not be fully archived", err.Error())
	}

	delResp, err := cl.Gen.AgentsDeleteAgentWithResponse(ctx, agentID)
	if err != nil {
		diags.AddError("Error deleting agent", err.Error())
		return
	}
	if delErr := client.Check(delResp.HTTPResponse, delResp.Body); delErr != nil {
		if client.IsNotFound(delErr) {
			if cause := agentDeleteRefusedCause(ctx, cl, agentID, string(delResp.Body)); cause != "" {
				diags.AddError(agentDeleteGatedSummary, agentDeleteGatedDetail(agentID, cause, string(delResp.Body)))
				return
			}
			return
		}
		if client.IsConflict(delErr) && strings.Contains(delErr.Error(), "online workers") {
			diags.AddError("Error deleting agent",
				fmt.Sprintf("Agent %s still has online workers. Kill the worker pods and wait ~60s for the heartbeat to expire, then run destroy again.", agentID))
			return
		}
		diags.AddError("Error deleting agent", delErr.Error())
	}
}

const agentDeleteGatedSummary = "Agent was not deleted — DELETE returned 404 but the agent still exists"

// lifecycleGateHints match the control-plane 404 raised when DELETE /agents/{id}
// is refused because the self_hosted_agent_lifecycle flag is off for the account.
// Matched as lowercased substrings rather than on the exact sentence: the detail
// text is not part of the API contract, so an exact match would silently degrade
// into the orphaning behaviour the moment the wording is reworded.
var lifecycleGateHints = []string{"lifecycle is not enabled", "self-hosted agent lifecycle", "lifecycle is disabled"}

// agentDeleteRefusedCause reports why a 404 from DELETE /agents/{id} is a refusal
// rather than "already gone", or "" when the agent really is gone. The delete route
// is feature-gated and answers 404 when the gate is shut, so taking every 404 as
// success drops a live agent — still registered, still holding a worker token — out
// of Terraform state. The read path is never gated, so a follow-up GET is the
// authoritative discriminator; the body match is the fallback for when that GET
// cannot be completed.
func agentDeleteRefusedCause(ctx context.Context, cl *client.Client, agentID, body string) string {
	lower := strings.ToLower(body)
	for _, hint := range lifecycleGateHints {
		if strings.Contains(lower, hint) {
			return "The control plane reported that the self-hosted agent lifecycle feature is not enabled for this account."
		}
	}
	if agentExists(ctx, cl, agentID) {
		return "The delete returned 404, but a follow-up read shows the agent is still registered."
	}
	return ""
}

// agentExists reports whether GET /agents/{id} still serves the record. Only a
// clean 2xx counts: an unreachable or refused read is not evidence either way, and
// must not be turned into a destroy failure.
func agentExists(ctx context.Context, cl *client.Client, agentID string) bool {
	apiResp, err := cl.Gen.AgentsGetAgentWithResponse(ctx, agentID)
	if err != nil {
		return false
	}
	return client.Check(apiResp.HTTPResponse, apiResp.Body) == nil
}

func agentDeleteGatedDetail(agentID, cause, body string) string {
	return fmt.Sprintf("%s\n\n"+
		"Agent %[2]s was archived but NOT deleted: it is still registered in AgentOps and its worker token is still live.\n\n"+
		"Terraform has kept the resource in state, so nothing is orphaned yet — but do not remove it from state and re-apply "+
		"with the same agent id: that adopts the surviving agent and rotates its worker token, invalidating the Kubernetes "+
		"secret already deployed in your cluster.\n\n"+
		"To recover:\n"+
		"  1. Have the self_hosted_agent_lifecycle feature enabled for your AgentOps account (Komodor support or your account "+
		"admin), then re-run 'terraform destroy'.\n"+
		"  2. Or delete agent %[2]s by hand in AgentOps, then re-run 'terraform destroy' — the destroy then succeeds because "+
		"the agent really is gone.\n\n"+
		"API response: %[3]s", cause, agentID, body)
}

func (r *agentResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("agent_id"), req, resp)
}

// agentResolveComputed makes every attribute the post-mutation read would fill
// known before it runs — from prior state on update, null on create. The read is
// deliberately soft, and an unknown left in state is rejected as an inconsistent
// apply result, which would turn a transient 502 into a failed apply.
func agentResolveComputed(m *agentResourceModel, prior *agentResourceModel) {
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

func (r *agentResource) refreshFromGetAPI(ctx context.Context, m *agentResourceModel, diags *diag.Diagnostics) {
	apiResp, err := r.client.Gen.AgentsGetAgentWithResponse(ctx, m.AgentID.ValueString())
	if err != nil {
		diags.AddWarning("Could not read agent after mutation", err.Error())
		return
	}
	if checkErr := client.Check(apiResp.HTTPResponse, apiResp.Body); checkErr != nil {
		diags.AddWarning("Could not read agent after mutation", checkErr.Error())
		return
	}
	if apiResp.JSON200 != nil {
		agentApplyInstance(m, apiResp.JSON200, phaseApply)
	}
}

func agentApplyInstance(m *agentResourceModel, inst *gen.AgentInstanceResponse, phase readPhase) {
	m.ID = types.StringValue(inst.AgentId)
	if inst.IdSlug != nil && *inst.IdSlug != "" {
		m.AgentID = types.StringValue(*inst.IdSlug)
	}
	m.Status = types.StringValue(string(inst.Status))
	m.Name = ptrToString(inst.Name)
	m.IsArchived = boolPtrToBool(inst.IsArchived)
	m.CreatedAt = ptrToString(inst.CreatedAt)

	m.McpGroupID = reconcileManagedOptional(phase, m.McpGroupID, strOrNull(enumPtrToString(inst.McpGroupId)))
}

const agentDeleteWaitTimeout = 90 * time.Second

func awaitAgentDeletable(ctx context.Context, cl *client.Client, agentID string) error {
	deadline := time.Now().Add(agentDeleteWaitTimeout)
	for {
		if agentReadyToDelete(cl, ctx, agentID) {
			return nil
		}
		if !time.Now().Before(deadline) {
			return fmt.Errorf("timed out waiting %s for agent %s to become deletable", agentDeleteWaitTimeout, agentID)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(5 * time.Second):
		}
	}
}

// agentReadyToDelete returns true when the agent is safe to delete (archived
// with no running instances), already gone, or unreadable (proceed with delete).
func agentReadyToDelete(cl *client.Client, ctx context.Context, agentID string) bool {
	apiResp, err := cl.Gen.AgentsGetAgentWithResponse(ctx, agentID)
	if err != nil {
		return true
	}
	if checkErr := client.Check(apiResp.HTTPResponse, apiResp.Body); checkErr != nil {
		return true
	}
	if apiResp.JSON200 == nil {
		return true
	}
	inst := apiResp.JSON200
	if inst.IsArchived != nil && *inst.IsArchived {
		total := 0
		if inst.InstancesTotal != nil {
			total = *inst.InstancesTotal
		}
		return total == 0
	}
	return false
}
