// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/komodorio/terraform-provider-agentops/internal/client"
	"github.com/komodorio/terraform-provider-agentops/internal/client/gen"
)

// hostedAgentPollInterval is how often the provider re-reads a hosted agent while
// waiting for it to become online. A var, not a const, so tests covering the poll
// loop do not have to spend a real interval per iteration.
var hostedAgentPollInterval = 5 * time.Second

const (
	// hostedAgentDefaultWaitTimeout is the default wait_timeout when the agent
	// is waited on but no explicit timeout is configured.
	hostedAgentDefaultWaitTimeout = 10 * time.Minute
	// runtimeDeployStatusFailed is the runtime agent record's deploy_status for a
	// provision that failed. The field is a free-form string in the spec, so it is
	// matched by value rather than by a generated enum.
	runtimeDeployStatusFailed = "failed"
	// runtimeDeployStatusDone is deploy_status for a deploy that finished.
	runtimeDeployStatusDone = "done"
)

// hostedAgentArchiveTimeout bounds the wait for an archive deploy to finish during
// a delete. A var so tests do not have to outlast it.
var hostedAgentArchiveTimeout = 10 * time.Minute

// Ensure the resource satisfies the framework interfaces.
var (
	_ resource.Resource                = &hostedAgentResource{}
	_ resource.ResourceWithConfigure   = &hostedAgentResource{}
	_ resource.ResourceWithImportState = &hostedAgentResource{}
)

// NewHostedAgentResource is the constructor registered with the provider.
func NewHostedAgentResource() resource.Resource {
	return &hostedAgentResource{}
}

type hostedAgentResource struct {
	client *client.Client
}

// hostedAgentResourceModel maps the agentops_hosted_agent schema to Go. The
// deployment-spec fields (instructions, model, skills, ...) are write-only: the
// API does not echo them, so they are preserved from configuration and only the
// identity/computed fields are refreshed on read.
type hostedAgentResourceModel struct {
	ID            types.String            `tfsdk:"id"`
	Customer      types.String            `tfsdk:"customer"`
	AgentID       types.String            `tfsdk:"agent_id"`
	Instructions  types.String            `tfsdk:"instructions"`
	CredentialRef types.String            `tfsdk:"credential_ref"`
	Model         types.String            `tfsdk:"model"`
	DisplayName   types.String            `tfsdk:"display_name"`
	CommitMessage types.String            `tfsdk:"commit_message"`
	McpGroupID    types.String            `tfsdk:"mcp_group_id"`
	Capabilities  types.Map               `tfsdk:"capabilities"`
	Skills        []hostedAgentSkillModel `tfsdk:"skills"`
	Agents        []hostedAgentAgentModel `tfsdk:"agents"`
	McpServers    []hostedAgentMcpModel   `tfsdk:"mcp_servers"`
	Image         *hostedAgentImageModel  `tfsdk:"image"`
	WaitForOnline types.Bool              `tfsdk:"wait_for_online"`
	WaitTimeout   types.String            `tfsdk:"wait_timeout"`

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

type hostedAgentSkillModel struct {
	ID      types.String `tfsdk:"id"`
	Content types.String `tfsdk:"content"`
}

type hostedAgentAgentModel struct {
	ID      types.String `tfsdk:"id"`
	Content types.String `tfsdk:"content"`
}

type hostedAgentMcpModel struct {
	Name types.String `tfsdk:"name"`
}

type hostedAgentImageModel struct {
	Registry   types.String `tfsdk:"registry"`
	Repository types.String `tfsdk:"repository"`
	Tag        types.String `tfsdk:"tag"`
	PullPolicy types.String `tfsdk:"pull_policy"`
}

func (r *hostedAgentResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_hosted_agent"
}

func (r *hostedAgentResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A Komodor-hosted agent: a managed agent deployment built from instructions, " +
			"skills and an optional container image. Deployment-spec fields are write-only — the API does " +
			"not return them, so out-of-band changes to them are not detected. Destroying an agent that " +
			"deployed successfully archives it first where the account requires that, and waits for the " +
			"archive to finish before deleting.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Hosted agent record identifier.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"customer": schema.StringAttribute{
				MarkdownDescription: "Customer/tenant the agent is hosted for. Optional: the server derives it from " +
					"your account when omitted. Changing this forces a new hosted agent.",
				Optional:      true,
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace(), stringplanmodifier.UseStateForUnknown()},
			},
			"agent_id": schema.StringAttribute{
				MarkdownDescription: "Stable agent identifier within the customer. Changing this forces a new hosted agent.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"instructions": schema.StringAttribute{
				MarkdownDescription: "System instructions for the agent. Write-only.",
				Required:            true,
			},
			"credential_ref": schema.StringAttribute{
				MarkdownDescription: "Reference to the credential the agent runs with. Write-only.",
				Required:            true,
			},
			"model": schema.StringAttribute{
				MarkdownDescription: "Model the agent uses. Write-only.",
				Optional:            true,
			},
			"display_name": schema.StringAttribute{
				MarkdownDescription: "Human-readable name. Write-only.",
				Optional:            true,
			},
			"commit_message": schema.StringAttribute{
				MarkdownDescription: "Commit message recorded when the agent's generated repo is updated. Write-only.",
				Optional:            true,
			},
			"mcp_group_id": schema.StringAttribute{
				MarkdownDescription: "ID of an MCP gateway group to attach. Write-only.",
				Optional:            true,
			},
			"capabilities": schema.MapAttribute{
				MarkdownDescription: "Capability toggles for the agent. Write-only.",
				ElementType:         types.BoolType,
				Optional:            true,
			},
			"skills": schema.ListNestedAttribute{
				MarkdownDescription: "Skills bundled into the agent. Write-only.",
				Optional:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id":      schema.StringAttribute{Required: true, MarkdownDescription: "Skill identifier."},
						"content": schema.StringAttribute{Required: true, MarkdownDescription: "Skill content."},
					},
				},
			},
			"agents": schema.ListNestedAttribute{
				MarkdownDescription: "Subagents bundled into the agent. Write-only.",
				Optional:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id":      schema.StringAttribute{Required: true, MarkdownDescription: "Subagent identifier."},
						"content": schema.StringAttribute{Required: true, MarkdownDescription: "Subagent content."},
					},
				},
			},
			"mcp_servers": schema.ListNestedAttribute{
				MarkdownDescription: "Named MCP servers the agent can reach. Write-only.",
				Optional:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"name": schema.StringAttribute{Required: true, MarkdownDescription: "MCP server name."},
					},
				},
			},
			"image": schema.SingleNestedAttribute{
				MarkdownDescription: "Container image overrides for the agent runtime. Write-only.",
				Optional:            true,
				Attributes: map[string]schema.Attribute{
					"registry":    schema.StringAttribute{Optional: true, MarkdownDescription: "Image registry host."},
					"repository":  schema.StringAttribute{Optional: true, MarkdownDescription: "Image repository."},
					"tag":         schema.StringAttribute{Optional: true, MarkdownDescription: "Image tag."},
					"pull_policy": schema.StringAttribute{Optional: true, MarkdownDescription: "Image pull policy."},
				},
			},
			"wait_for_online": schema.BoolAttribute{
				MarkdownDescription: "Whether create/update should block until the hosted agent reports `online` " +
					"(its first heartbeat). Defaults to `true`. Set to `false` to return as soon as the deployment " +
					"is accepted — useful for agents that are intentionally left in `draft`/scaled to zero and will " +
					"never heartbeat. While the agent is not yet online, a failed cluster-side provision ends the " +
					"wait as soon as it is observed, reporting the control plane's own reason rather than running " +
					"out the timeout. Updating an agent that is already online returns as soon as it reports " +
					"`online`, which the revision already running does, so the new revision's rollout is not waited on.",
				Optional: true,
			},
			"wait_timeout": schema.StringAttribute{
				MarkdownDescription: "Maximum time to wait for the agent to become `online`, as a Go duration " +
					"(e.g. `10m`, `90s`). Defaults to `10m`. Only used when `wait_for_online` is `true`.",
				Optional: true,
			},

			"identity": schema.StringAttribute{
				MarkdownDescription: "Resolved runtime identity of the hosted agent.",
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

func (r *hostedAgentResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromProviderData(req.ProviderData, &resp.Diagnostics)
}

func (r *hostedAgentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan hostedAgentResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := gen.HostedAgentCreateRequest{
		Customer:      stringToPtr(plan.Customer),
		AgentId:       plan.AgentID.ValueString(),
		Instructions:  plan.Instructions.ValueString(),
		CredentialRef: plan.CredentialRef.ValueString(),
		Model:         stringToPtr(plan.Model),
		DisplayName:   stringToPtr(plan.DisplayName),
		CommitMessage: stringToPtr(plan.CommitMessage),
		McpGroupId:    stringToPtr(plan.McpGroupID),
		Skills:        hostedAgentSkillsToAPI(plan.Skills),
		Agents:        hostedAgentAgentsToAPI(plan.Agents),
		McpServers:    hostedAgentMcpToAPI(plan.McpServers),
		Image:         hostedAgentImageToAPI(plan.Image),
	}
	resp.Diagnostics.Append(boolMapToPtr(ctx, plan.Capabilities, &body.Capabilities)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.Gen.HostedAgentsCreateHostedAgentWithResponse(ctx, body)
	if err != nil {
		resp.Diagnostics.AddError("Error creating hosted agent", err.Error())
		return
	}
	if err := client.Check(apiResp.HTTPResponse, apiResp.Body); err != nil {
		resp.Diagnostics.AddError("Error creating hosted agent", err.Error())
		return
	}
	if apiResp.JSON201 == nil {
		resp.Diagnostics.AddError("Error creating hosted agent", "API returned an empty body")
		return
	}

	hostedAgentApplyComputed(&plan, apiResp.JSON201)
	r.waitForOnlineIfRequested(ctx, &plan, &resp.State, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *hostedAgentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state hostedAgentResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.Gen.HostedAgentsGetHostedAgentWithResponse(ctx, state.Customer.ValueString(), state.AgentID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading hosted agent", err.Error())
		return
	}
	if err := client.Check(apiResp.HTTPResponse, apiResp.Body); err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading hosted agent", err.Error())
		return
	}
	if apiResp.JSON200 == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	// Only the identity/computed fields are refreshed; write-only spec fields are
	// preserved from state because the API never returns them.
	hostedAgentApplyComputed(&state, apiResp.JSON200)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *hostedAgentResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan hostedAgentResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := gen.HostedAgentUpdateRequest{
		Instructions:  stringToPtr(plan.Instructions),
		CredentialRef: stringToPtr(plan.CredentialRef),
		Model:         stringToPtr(plan.Model),
		DisplayName:   stringToPtr(plan.DisplayName),
		CommitMessage: stringToPtr(plan.CommitMessage),
		Skills:        hostedAgentSkillsToAPI(plan.Skills),
		Agents:        hostedAgentAgentsToAPI(plan.Agents),
		McpServers:    hostedAgentMcpToAPI(plan.McpServers),
		Image:         hostedAgentImageToAPI(plan.Image),
	}
	resp.Diagnostics.Append(boolMapToPtr(ctx, plan.Capabilities, &body.Capabilities)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.Gen.HostedAgentsUpdateHostedAgentWithResponse(ctx, plan.Customer.ValueString(), plan.AgentID.ValueString(), body)
	if err != nil {
		resp.Diagnostics.AddError("Error updating hosted agent", err.Error())
		return
	}
	if err := client.Check(apiResp.HTTPResponse, apiResp.Body); err != nil {
		resp.Diagnostics.AddError("Error updating hosted agent", err.Error())
		return
	}
	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError("Error updating hosted agent", "API returned an empty body")
		return
	}

	hostedAgentApplyComputed(&plan, apiResp.JSON200)
	r.waitForOnlineIfRequested(ctx, &plan, &resp.State, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *hostedAgentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state hostedAgentResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := deleteHostedAgent(ctx, r.client, state.Customer.ValueString(), state.AgentID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Error deleting hosted agent", err.Error())
	}
}

// deleteHostedAgent removes the hosted agent behind either resource that produces
// one. On an account with hosted lifecycle enabled, deleting an agent that
// deployed successfully is refused with 409 until it has been archived, so this
// archives and retries — but only after the archive has finished.
//
// Waiting is the whole point. Archiving starts its own deploy, and the delete
// routes on deploy status before it looks at the archive: deleting while that
// deploy is in flight takes the abandon-an-incomplete-deploy path, which drops
// the record without revoking the agent's worker token. The wait keeps the
// delete on the decommission path that does.
//
// The archive route is absent from the vendored OpenAPI spec, so it goes through
// client.Post rather than the generated client.
func deleteHostedAgent(ctx context.Context, cl *client.Client, customer, agentID string) error {
	del := func() error {
		apiResp, err := cl.Gen.HostedAgentsDeleteHostedAgentWithResponse(ctx, customer, agentID)
		if err != nil {
			return err
		}
		if err := client.Check(apiResp.HTTPResponse, apiResp.Body); err != nil && !client.IsNotFound(err) {
			return err
		}
		return nil
	}

	err := del()
	// Only the archive conflict is answered by archiving. Every other 409 is left
	// alone: archiving is not a free retry, it takes the agent offline.
	if !client.IsConflict(err) || !strings.Contains(err.Error(), "archive the agent") {
		return err
	}

	archive := fmt.Sprintf("/api/v1/hosted-agents/%s/%s/archive", url.PathEscape(customer), url.PathEscape(agentID))
	// A 409 here means the agent is already archived or already deploying, which is
	// the state the wait below is for. Anything else leaves the delete unfinished.
	if archiveErr := cl.Post(ctx, archive); archiveErr != nil && !client.IsConflict(archiveErr) {
		return fmt.Errorf("%w (archiving it first failed: %w)", err, archiveErr)
	}
	if waitErr := awaitArchiveSettled(ctx, cl, customer, agentID); waitErr != nil {
		return waitErr
	}
	return del()
}

// awaitArchiveSettled blocks until the archive deploy leaves in_progress. It
// reports the deploy status through the runtime agent record, the only one that
// carries it. A read it cannot make is not treated as settled: deleting on a
// guess is what leaks the worker token.
func awaitArchiveSettled(ctx context.Context, cl *client.Client, customer, agentID string) error {
	apiResp, err := cl.Gen.HostedAgentsGetHostedAgentWithResponse(ctx, customer, agentID)
	if err != nil {
		return fmt.Errorf("archived %s, but could not read it back to see the archive finish: %w", agentID, err)
	}
	if checkErr := client.Check(apiResp.HTTPResponse, apiResp.Body); checkErr != nil {
		if client.IsNotFound(checkErr) {
			return nil // already gone; nothing left to delete
		}
		return fmt.Errorf("archived %s, but could not read it back to see the archive finish: %w", agentID, checkErr)
	}
	runtimeAgentID := apiResp.JSON200.RuntimeAgentId

	deadline := time.Now().Add(hostedAgentArchiveTimeout)
	last := ""
	for {
		status, detail, readErr := runtimeDeployStatus(ctx, cl, runtimeAgentID)
		switch {
		case readErr != nil:
			last = "unreadable: " + readErr.Error()
		case status != "":
			last = status
		}
		if status == runtimeDeployStatusDone {
			return nil
		}
		if status == runtimeDeployStatusFailed {
			return fmt.Errorf("archiving %s failed (%s), so it cannot be deleted yet: unarchive it or resolve the deploy, then destroy again", agentID, detail)
		}

		if !time.Now().Before(deadline) {
			return fmt.Errorf(
				"archived %s but its scale-to-zero deploy was still %s after %s. Deleting now would drop the record without revoking the agent's worker token, so nothing was deleted: let the deploy finish, then destroy again",
				agentID, last, hostedAgentArchiveTimeout)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(hostedAgentPollInterval):
		}
	}
}

// ImportState accepts "<customer>/<agent_id>". Write-only spec fields cannot be
// recovered on import and must be supplied in configuration.
func (r *hostedAgentResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(req.ID, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError("Invalid import ID",
			fmt.Sprintf("Expected import ID in the form \"customer/agent_id\", got %q.", req.ID))
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("customer"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("agent_id"), parts[1])...)
}

func hostedAgentSkillsToAPI(items []hostedAgentSkillModel) *[]gen.HostedAgentSkill {
	if items == nil {
		return nil
	}
	out := make([]gen.HostedAgentSkill, 0, len(items))
	for _, s := range items {
		out = append(out, gen.HostedAgentSkill{Id: s.ID.ValueString(), Content: s.Content.ValueString()})
	}
	return &out
}

func hostedAgentAgentsToAPI(items []hostedAgentAgentModel) *[]gen.HostedAgentAgent {
	if items == nil {
		return nil
	}
	out := make([]gen.HostedAgentAgent, 0, len(items))
	for _, a := range items {
		out = append(out, gen.HostedAgentAgent{Id: a.ID.ValueString(), Content: a.Content.ValueString()})
	}
	return &out
}

func hostedAgentMcpToAPI(items []hostedAgentMcpModel) *[]gen.HostedAgentMcpServer {
	if items == nil {
		return nil
	}
	out := make([]gen.HostedAgentMcpServer, 0, len(items))
	for _, s := range items {
		out = append(out, gen.HostedAgentMcpServer{Name: s.Name.ValueString()})
	}
	return &out
}

func hostedAgentImageToAPI(m *hostedAgentImageModel) *gen.HostedAgentImage {
	if m == nil {
		return nil
	}
	return &gen.HostedAgentImage{
		Registry:   stringToPtr(m.Registry),
		Repository: stringToPtr(m.Repository),
		Tag:        stringToPtr(m.Tag),
		PullPolicy: stringToPtr(m.PullPolicy),
	}
}

// hostedAgentApplyComputed writes the identity/computed fields of a
// HostedAgentResponse into the model, leaving write-only spec fields untouched.
func hostedAgentApplyComputed(m *hostedAgentResourceModel, h *gen.HostedAgentResponse) {
	m.ID = types.StringValue(h.Id)
	m.Customer = types.StringValue(h.Customer)
	m.AgentID = types.StringValue(h.AgentId)
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

// hostedAgentWaitConfig resolves the wait_for_online / wait_timeout settings,
// defaulting to enabled with a 10m timeout when they are not set in config.
func hostedAgentWaitConfig(enabled types.Bool, timeout types.String) (bool, time.Duration, diag.Diagnostics) {
	var diags diag.Diagnostics
	wait := enabled.IsNull() || enabled.IsUnknown() || enabled.ValueBool()
	d := hostedAgentDefaultWaitTimeout
	if !timeout.IsNull() && !timeout.IsUnknown() && timeout.ValueString() != "" {
		parsed, err := time.ParseDuration(timeout.ValueString())
		if err != nil {
			diags.AddAttributeError(path.Root("wait_timeout"), "Invalid wait_timeout",
				fmt.Sprintf("Could not parse %q as a Go duration (e.g. \"10m\", \"90s\"): %s", timeout.ValueString(), err))
			return wait, d, diags
		}
		d = parsed
	}
	return wait, d, diags
}

// waitForOnlineIfRequested blocks until the hosted agent reports online when
// wait_for_online is enabled, refreshing plan with the final computed fields. On
// a failed deploy or a timeout it persists the current computed state (so the agent
// is tracked rather than orphaned) and records an error.
func (r *hostedAgentResource) waitForOnlineIfRequested(ctx context.Context, plan *hostedAgentResourceModel, state *tfsdk.State, diags *diag.Diagnostics) {
	wait, timeout, cfgDiags := hostedAgentWaitConfig(plan.WaitForOnline, plan.WaitTimeout)
	diags.Append(cfgDiags...)
	if diags.HasError() || !wait {
		return
	}

	final, err := waitForHostedAgentOnline(ctx, r.client, plan.Customer.ValueString(), plan.AgentID.ValueString(), timeout)
	if err != nil {
		diags.Append(state.Set(ctx, plan)...)
		diags.AddError("Hosted agent did not come online", err.Error())
		return
	}
	hostedAgentApplyComputed(plan, final)
}

// waitForHostedAgentOnline polls the hosted agent until its status is online,
// returning an error if the deploy failed or the timeout elapses. It is a package
// function so every resource that produces a hosted agent (a direct create or a
// worker-catalog deploy) can share the same wait behaviour.
func waitForHostedAgentOnline(ctx context.Context, cl *client.Client, customer, agentID string, timeout time.Duration) (*gen.HostedAgentResponse, error) {
	deadline := time.Now().Add(timeout)
	last, lastDeploy := "", ""
	var lastDeployErr error
	for {
		apiResp, err := cl.Gen.HostedAgentsGetHostedAgentWithResponse(ctx, customer, agentID)
		if err != nil {
			return nil, err
		}
		if checkErr := client.Check(apiResp.HTTPResponse, apiResp.Body); checkErr != nil {
			// The record can be briefly unqueryable right after create; keep
			// waiting on 404 and surface any other error immediately.
			if !client.IsNotFound(checkErr) {
				return nil, checkErr
			}
		} else if apiResp.JSON200 != nil {
			last = enumPtrToString(apiResp.JSON200.Status)
			switch last {
			case string(gen.HostedAgentResponseStatusOnline):
				return apiResp.JSON200, nil
			case string(gen.HostedAgentResponseStatusDeployFailed):
				return apiResp.JSON200, fmt.Errorf("hosted agent deployment failed (status %q)", last)
			}

			// The hosted record's status is derived from worker heartbeats alone, so a
			// provision that never produced a worker reads as "draft" here for as long
			// as the caller is willing to wait. Only the runtime agent record carries
			// the deploy outcome, so a failure is invisible without this second read.
			status, detail, readErr := runtimeDeployStatus(ctx, cl, apiResp.JSON200.RuntimeAgentId)
			lastDeployErr = readErr
			if status != "" {
				lastDeploy = status
			}
			if status == runtimeDeployStatusFailed {
				if detail == "" {
					detail = "the control plane reported no reason"
				}
				return apiResp.JSON200, fmt.Errorf("hosted agent deployment failed: %s (runtime agent %s)", detail, apiResp.JSON200.RuntimeAgentId)
			}
		}

		if !time.Now().Before(deadline) {
			return nil, fmt.Errorf("timed out after %s waiting for hosted agent to become online (%s)", timeout, waitSummary(last, lastDeploy, lastDeployErr))
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(hostedAgentPollInterval):
		}
	}
}

// runtimeDeployStatus reads deploy_status and deploy_error from the runtime agent
// record the hosted agent registers as. It never fails the wait by itself: a 404 is
// the record lagging the hosted one right after create, and any other failure is
// returned so the caller can say the deploy status was unreadable rather than
// silently reporting a plain timeout.
//
// That distinction matters because this endpoint needs a different capability than
// the hosted-agent read (AGENT_READ vs HOSTED_AGENT_READ), so a narrowly scoped key
// gets 403 here on every poll while the rest of the wait works.
func runtimeDeployStatus(ctx context.Context, cl *client.Client, runtimeAgentID string) (status, detail string, err error) {
	if runtimeAgentID == "" {
		return "", "", nil
	}
	apiResp, err := cl.Gen.AgentsGetAgentWithResponse(ctx, runtimeAgentID)
	if err != nil {
		return "", "", err
	}
	if checkErr := client.Check(apiResp.HTTPResponse, apiResp.Body); checkErr != nil {
		if client.IsNotFound(checkErr) {
			return "", "", nil
		}
		return "", "", checkErr
	}
	if apiResp.JSON200 == nil {
		return "", "", nil
	}
	return enumPtrToString(apiResp.JSON200.DeployStatus), enumPtrToString(apiResp.JSON200.DeployError), nil
}

// waitSummary renders what the wait last saw, so a timeout says whether the deploy
// status was still pending, or could not be read at all.
func waitSummary(hostedStatus, deployStatus string, readErr error) string {
	summary := fmt.Sprintf("last status %q", hostedStatus)
	switch {
	case readErr != nil:
		summary += fmt.Sprintf(", deploy status unreadable: %s", readErr)
	case deployStatus != "":
		summary += fmt.Sprintf(", deploy status %q", deployStatus)
	}
	return summary
}
