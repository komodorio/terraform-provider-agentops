// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/komodorio/terraform-provider-agentops/internal/client"
	"github.com/komodorio/terraform-provider-agentops/internal/client/gen"
)

// Ensure the resource satisfies the framework interfaces.
var (
	_ resource.Resource                = &triggerResource{}
	_ resource.ResourceWithConfigure   = &triggerResource{}
	_ resource.ResourceWithImportState = &triggerResource{}
)

// NewTriggerResource is the constructor registered with the provider.
func NewTriggerResource() resource.Resource {
	return &triggerResource{}
}

type triggerResource struct {
	client *client.Client
}

// triggerResourceModel maps the agentops_trigger schema to Go.
type triggerResourceModel struct {
	ID                  types.String `tfsdk:"id"`
	Name                types.String `tfsdk:"name"`
	Description         types.String `tfsdk:"description"`
	TargetID            types.String `tfsdk:"target_id"`
	TargetType          types.String `tfsdk:"target_type"`
	WebhookType         types.String `tfsdk:"webhook_type"`
	Header              types.String `tfsdk:"header"`
	IsEnabled           types.Bool   `tfsdk:"is_enabled"`
	SigningCredentialID types.String `tfsdk:"signing_credential_id"`
	SigningSecret       types.String `tfsdk:"signing_secret"`
	Token               types.String `tfsdk:"token"`
	CreatedAt           types.String `tfsdk:"created_at"`
	UpdatedAt           types.String `tfsdk:"updated_at"`
}

func (r *triggerResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_trigger"
}

func (r *triggerResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A webhook trigger that fires a target agent or workflow when called.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Trigger identifier.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Human-readable trigger name.",
				Optional:            true,
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Free-form description.",
				Optional:            true,
			},
			"target_id": schema.StringAttribute{
				MarkdownDescription: "ID of the agent or workflow this trigger invokes. Changing this forces a new trigger.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"target_type": schema.StringAttribute{
				MarkdownDescription: "Kind of target (e.g. `agent`, `workflow`). Changing this forces a new trigger.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"webhook_type": schema.StringAttribute{
				MarkdownDescription: "Webhook payload type. Defaults to the server's choice when omitted.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"header": schema.StringAttribute{
				MarkdownDescription: "Name of the HTTP header carrying the signature. Server-assigned when omitted.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"is_enabled": schema.BoolAttribute{
				MarkdownDescription: "Whether the trigger is active. Defaults to `true`.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
			},
			"signing_credential_id": schema.StringAttribute{
				MarkdownDescription: "ID of a credential used to sign/verify webhook payloads.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"signing_secret": schema.StringAttribute{
				MarkdownDescription: "Inline signing secret. Write-only; never returned by the API.",
				Optional:            true,
				Sensitive:           true,
			},
			"token": schema.StringAttribute{
				MarkdownDescription: "Webhook invocation token. Returned only at create and on rotation.",
				Computed:            true,
				Sensitive:           true,
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

func (r *triggerResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromProviderData(req.ProviderData, &resp.Diagnostics)
}

func (r *triggerResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan triggerResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := gen.CreateWebhookTriggerRequest{
		Name:                stringToPtr(plan.Name),
		Description:         stringToPtr(plan.Description),
		Header:              stringToPtr(plan.Header),
		IsEnabled:           boolToPtr(plan.IsEnabled),
		SigningCredentialId: stringToPtr(plan.SigningCredentialID),
		SigningSecret:       stringToPtr(plan.SigningSecret),
		TargetId:            plan.TargetID.ValueString(),
		TargetType:          gen.TriggerTargetType(plan.TargetType.ValueString()),
	}
	if !plan.WebhookType.IsNull() && !plan.WebhookType.IsUnknown() {
		wt := gen.CreateWebhookTriggerRequestWebhookType(plan.WebhookType.ValueString())
		body.WebhookType = &wt
	}

	apiResp, err := r.client.Gen.TriggersCreateTriggerEndpointWithResponse(ctx, body)
	if err != nil {
		resp.Diagnostics.AddError("Error creating trigger", err.Error())
		return
	}
	if err := client.Check(apiResp.HTTPResponse, apiResp.Body); err != nil {
		resp.Diagnostics.AddError("Error creating trigger", err.Error())
		return
	}
	if apiResp.JSON201 == nil {
		resp.Diagnostics.AddError("Error creating trigger", "API returned an empty body")
		return
	}

	t := apiResp.JSON201
	plan.ID = types.StringValue(t.TriggerId)
	plan.Token = types.StringValue(t.Token)
	applyTriggerCommon(&plan, t.Name, t.Description, t.Header, t.IsEnabled, t.TargetId, string(t.TargetType),
		enumPtrToString(t.WebhookType), signingCredentialID(t.SigningCredential), t.CreatedAt, t.UpdatedAt)

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *triggerResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state triggerResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.Gen.TriggersGetTriggerEndpointWithResponse(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading trigger", err.Error())
		return
	}
	if err := client.Check(apiResp.HTTPResponse, apiResp.Body); err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading trigger", err.Error())
		return
	}
	if apiResp.JSON200 == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	t := apiResp.JSON200
	// Token and signing_secret are never returned; preserve the existing state.
	applyTriggerCommon(&state, t.Name, t.Description, t.Header, t.IsEnabled, t.TargetId, string(t.TargetType),
		enumPtrToString(t.WebhookType), signingCredentialID(t.SigningCredential), t.CreatedAt, t.UpdatedAt)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *triggerResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan triggerResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := gen.UpdateWebhookTriggerRequest{
		Name:                stringToPtr(plan.Name),
		Description:         stringToPtr(plan.Description),
		Header:              stringToPtr(plan.Header),
		IsEnabled:           boolToPtr(plan.IsEnabled),
		SigningCredentialId: stringToPtr(plan.SigningCredentialID),
		SigningSecret:       stringToPtr(plan.SigningSecret),
	}
	if !plan.WebhookType.IsNull() && !plan.WebhookType.IsUnknown() {
		wt := gen.UpdateWebhookTriggerRequestWebhookType(plan.WebhookType.ValueString())
		body.WebhookType = &wt
	}

	apiResp, err := r.client.Gen.TriggersUpdateTriggerEndpointWithResponse(ctx, plan.ID.ValueString(), body)
	if err != nil {
		resp.Diagnostics.AddError("Error updating trigger", err.Error())
		return
	}
	if err := client.Check(apiResp.HTTPResponse, apiResp.Body); err != nil {
		resp.Diagnostics.AddError("Error updating trigger", err.Error())
		return
	}
	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError("Error updating trigger", "API returned an empty body")
		return
	}

	t := apiResp.JSON200
	// Update does not return the token; keep the value already in the plan/state.
	applyTriggerCommon(&plan, t.Name, t.Description, t.Header, t.IsEnabled, t.TargetId, string(t.TargetType),
		enumPtrToString(t.WebhookType), signingCredentialID(t.SigningCredential), t.CreatedAt, t.UpdatedAt)

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *triggerResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state triggerResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.Gen.TriggersDeleteTriggerEndpointWithResponse(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error deleting trigger", err.Error())
		return
	}
	if err := client.Check(apiResp.HTTPResponse, apiResp.Body); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Error deleting trigger", err.Error())
	}
}

func (r *triggerResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// applyTriggerCommon writes the fields shared by every trigger response shape
// into the model. Token and signing_secret are deliberately not touched here
// because the API never returns them on read/update.
func applyTriggerCommon(m *triggerResourceModel, name, description *string, header string, isEnabled bool,
	targetID, targetType, webhookType, signingCredentialID string, createdAt, updatedAt string) {
	m.Name = ptrToString(name)
	m.Description = ptrToString(description)
	m.Header = types.StringValue(header)
	m.IsEnabled = types.BoolValue(isEnabled)
	m.TargetID = types.StringValue(targetID)
	m.TargetType = types.StringValue(targetType)
	m.WebhookType = strOrNull(webhookType)
	m.SigningCredentialID = strOrNull(signingCredentialID)
	m.CreatedAt = types.StringValue(createdAt)
	m.UpdatedAt = types.StringValue(updatedAt)
}

func signingCredentialID(ref *gen.SigningCredentialRef) string {
	if ref == nil {
		return ""
	}
	return ref.Id
}
