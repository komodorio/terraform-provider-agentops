// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/komodorio/terraform-provider-agentops/internal/client"
	"github.com/komodorio/terraform-provider-agentops/internal/client/gen"
)

// Ensure the resource satisfies the framework interfaces.
var (
	_ resource.Resource                = &credentialBindingResource{}
	_ resource.ResourceWithConfigure   = &credentialBindingResource{}
	_ resource.ResourceWithImportState = &credentialBindingResource{}
)

// NewCredentialBindingResource is the constructor registered with the provider.
func NewCredentialBindingResource() resource.Resource {
	return &credentialBindingResource{}
}

type credentialBindingResource struct {
	client *client.Client
}

// credentialBindingResourceModel maps the agentops_credential_binding schema to
// Go. Binding a credential to an agent is an association with no update
// endpoint, so every input is create-only (ForceNew). The synthetic id is
// "<credential_id>/<agent_id>".
type credentialBindingResourceModel struct {
	ID           types.String `tfsdk:"id"`
	CredentialID types.String `tfsdk:"credential_id"`
	AgentID      types.String `tfsdk:"agent_id"`
	OnDemand     types.Bool   `tfsdk:"on_demand"`
	CreatedAt    types.String `tfsdk:"created_at"`
}

func (r *credentialBindingResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_credential_binding"
}

func (r *credentialBindingResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Binds a credential to an agent. This association has no update endpoint, so " +
			"changing any argument forces a new binding.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Synthetic identifier in the form `<credential_id>/<agent_id>`.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"credential_id": schema.StringAttribute{
				MarkdownDescription: "ID of the credential to bind. Changing this forces a new binding.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"agent_id": schema.StringAttribute{
				MarkdownDescription: "ID of the agent to bind the credential to. Changing this forces a new binding.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"on_demand": schema.BoolAttribute{
				MarkdownDescription: "Whether the credential is supplied on demand rather than pre-attached. " +
					"Changing this forces a new binding.",
				Optional:      true,
				Computed:      true,
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.RequiresReplace(), boolplanmodifier.UseStateForUnknown()},
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "Creation timestamp.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
		},
	}
}

func (r *credentialBindingResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromProviderData(req.ProviderData, &resp.Diagnostics)
}

func (r *credentialBindingResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan credentialBindingResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	credentialID := plan.CredentialID.ValueString()
	agentID := plan.AgentID.ValueString()

	body := gen.BindCredentialRequest{
		AgentId:  agentID,
		OnDemand: boolToPtr(plan.OnDemand),
	}

	apiResp, err := r.client.Gen.CredentialsBindCredentialWithResponse(ctx, credentialID, body)
	if err != nil {
		resp.Diagnostics.AddError("Error binding credential", err.Error())
		return
	}
	if err := client.Check(apiResp.HTTPResponse, apiResp.Body); err != nil {
		resp.Diagnostics.AddError("Error binding credential", err.Error())
		return
	}
	if apiResp.JSON201 == nil {
		resp.Diagnostics.AddError("Error binding credential", "API returned an empty body")
		return
	}

	b := apiResp.JSON201
	plan.ID = types.StringValue(credentialBindingID(credentialID, agentID))
	plan.CreatedAt = types.StringValue(b.CreatedAt)
	plan.OnDemand = boolPtrToBool(b.OnDemand)

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *credentialBindingResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state credentialBindingResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// There is no GET-by-id endpoint; list the credential's bindings and match
	// on agent_id.
	apiResp, err := r.client.Gen.CredentialsListCredentialBindingsWithResponse(ctx, state.CredentialID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading credential binding", err.Error())
		return
	}
	if err := client.Check(apiResp.HTTPResponse, apiResp.Body); err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading credential binding", err.Error())
		return
	}
	if apiResp.JSON200 == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	var found *gen.CredentialBinding
	for i := range *apiResp.JSON200 {
		if (*apiResp.JSON200)[i].AgentId == state.AgentID.ValueString() {
			found = &(*apiResp.JSON200)[i]
			break
		}
	}
	if found == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	state.CreatedAt = types.StringValue(found.CreatedAt)
	state.OnDemand = boolPtrToBool(found.OnDemand)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update should be unreachable: every input attribute is RequiresReplace, so
// Terraform replaces the binding rather than updating it. Implemented to
// satisfy the resource.Resource interface.
func (r *credentialBindingResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError(
		"Credential bindings cannot be updated",
		"This is a bug in the provider: an update was attempted on an immutable resource.",
	)
}

func (r *credentialBindingResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state credentialBindingResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.Gen.CredentialsUnbindCredentialWithResponse(ctx, state.CredentialID.ValueString(), state.AgentID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error unbinding credential", err.Error())
		return
	}
	if err := client.Check(apiResp.HTTPResponse, apiResp.Body); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Error unbinding credential", err.Error())
	}
}

func (r *credentialBindingResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// The binding has no standalone ID; import by the synthetic
	// "<credential_id>/<agent_id>" key and let Read refresh the rest.
	parts := strings.Split(req.ID, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError(
			"Invalid import ID",
			"expected credential_id/agent_id",
		)
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("credential_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("agent_id"), parts[1])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), credentialBindingID(parts[0], parts[1]))...)
}

// credentialBindingID builds the synthetic resource ID from its two components.
func credentialBindingID(credentialID, agentID string) string {
	return fmt.Sprintf("%s/%s", credentialID, agentID)
}
