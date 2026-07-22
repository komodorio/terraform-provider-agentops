// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
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

const (
	// hostedAgentPollInterval is how often the provider re-reads a hosted agent
	// while waiting for it to become online.
	hostedAgentPollInterval = 5 * time.Second
	// hostedAgentDefaultWaitTimeout is the default wait_timeout when the agent
	// is waited on but no explicit timeout is configured.
	hostedAgentDefaultWaitTimeout = 10 * time.Minute
)

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
			"not return them, so out-of-band changes to them are not detected.",
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
					"never heartbeat. Note: a failed cluster-side provision is not reported back as `deploy_failed` " +
					"for API-created agents, so a failure surfaces as a `wait_timeout` rather than an immediate error.",
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

	apiResp, err := r.client.Gen.HostedAgentsDeleteHostedAgentWithResponse(ctx, state.Customer.ValueString(), state.AgentID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error deleting hosted agent", err.Error())
		return
	}
	if err := client.Check(apiResp.HTTPResponse, apiResp.Body); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Error deleting hosted agent", err.Error())
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
// timeout/failure it persists the current computed state (so the agent is tracked
// rather than orphaned) and records an error.
func (r *hostedAgentResource) waitForOnlineIfRequested(ctx context.Context, plan *hostedAgentResourceModel, state *tfsdk.State, diags *diag.Diagnostics) {
	wait, timeout, cfgDiags := hostedAgentWaitConfig(plan.WaitForOnline, plan.WaitTimeout)
	diags.Append(cfgDiags...)
	if diags.HasError() || !wait {
		return
	}

	final, err := waitForHostedAgentOnline(ctx, r.client, plan.Customer.ValueString(), plan.AgentID.ValueString(), timeout)
	if err != nil {
		diags.Append(state.Set(ctx, plan)...)
		diags.AddError("Timed out waiting for hosted agent to become online", err.Error())
		return
	}
	hostedAgentApplyComputed(plan, final)
}

// waitForHostedAgentOnline polls the hosted agent until its status is online,
// returning an error if it reports deploy_failed or the timeout elapses. It is a
// package function so every resource that produces a hosted agent (a direct
// create or a worker-catalog deploy) can share the same wait behaviour.
func waitForHostedAgentOnline(ctx context.Context, cl *client.Client, customer, agentID string, timeout time.Duration) (*gen.HostedAgentResponse, error) {
	deadline := time.Now().Add(timeout)
	last := ""
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
		}

		if !time.Now().Before(deadline) {
			return nil, fmt.Errorf("timed out after %s waiting for hosted agent to become online (last status %q)", timeout, last)
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(hostedAgentPollInterval):
		}
	}
}
