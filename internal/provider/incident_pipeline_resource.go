// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/komodorio/terraform-provider-agentops/internal/client"
	"github.com/komodorio/terraform-provider-agentops/internal/client/gen"
)

// Ensure the resource satisfies the framework interfaces.
var (
	_ resource.Resource                = &incidentPipelineResource{}
	_ resource.ResourceWithConfigure   = &incidentPipelineResource{}
	_ resource.ResourceWithImportState = &incidentPipelineResource{}
)

// NewIncidentPipelineResource is the constructor registered with the provider.
func NewIncidentPipelineResource() resource.Resource {
	return &incidentPipelineResource{}
}

type incidentPipelineResource struct {
	client *client.Client
}

// incidentPipelineResourceModel maps the agentops_incident_pipeline schema to Go.
type incidentPipelineResourceModel struct {
	ID                  types.String                      `tfsdk:"id"`
	Name                types.String                      `tfsdk:"name"`
	Status              types.String                      `tfsdk:"status"`
	AlertSource         *incidentPipelineAlertSourceModel `tfsdk:"alert_source"`
	RoutingRule         *incidentPipelineRoutingRuleModel `tfsdk:"routing_rule"`
	OrchestratorBinding *incidentPipelineBindingModel     `tfsdk:"orchestrator_binding"`
	SpecialistBindings  []incidentPipelineSpecialistModel `tfsdk:"specialist_bindings"`
	DeliveryConfig      *incidentPipelineDeliveryModel    `tfsdk:"delivery_config"`
	TriggerID           types.String                      `tfsdk:"trigger_id"`
	SourceProvider      types.String                      `tfsdk:"source_provider"`
	OrchestratorAgentID types.String                      `tfsdk:"orchestrator_agent_id"`
	SpecialistCount     types.Int64                       `tfsdk:"specialist_count"`
	WebhookURL          types.String                      `tfsdk:"webhook_url"`
	WebhookToken        types.String                      `tfsdk:"webhook_token"`
	LastIncidentAt      types.String                      `tfsdk:"last_incident_at"`
	CreatedAt           types.String                      `tfsdk:"created_at"`
}

type incidentPipelineAlertSourceModel struct {
	Provider          types.String `tfsdk:"provider"`
	MonitorMode       types.String `tfsdk:"monitor_mode"`
	ExternalMonitorID types.String `tfsdk:"external_monitor_id"`
}

type incidentPipelineRoutingRuleModel struct {
	RouteAll            types.Bool   `tfsdk:"route_all"`
	MissingFieldDefault types.Bool   `tfsdk:"missing_field_default"`
	Environment         types.String `tfsdk:"environment"`
	Service             types.String `tfsdk:"service"`
	Severity            types.String `tfsdk:"severity"`
	Tags                types.Map    `tfsdk:"tags"`
}

type incidentPipelineBindingModel struct {
	AgentID types.String `tfsdk:"agent_id"`
}

type incidentPipelineSpecialistModel struct {
	AgentID types.String `tfsdk:"agent_id"`
	Role    types.String `tfsdk:"role"`
	Enabled types.Bool   `tfsdk:"enabled"`
}

type incidentPipelineDeliveryModel struct {
	Slack *incidentPipelineSlackModel `tfsdk:"slack"`
}

type incidentPipelineSlackModel struct {
	ChannelID   types.String `tfsdk:"channel_id"`
	ChannelName types.String `tfsdk:"channel_name"`
	Enabled     types.Bool   `tfsdk:"enabled"`
}

func (r *incidentPipelineResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_incident_pipeline"
}

func (r *incidentPipelineResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "An incident pipeline (incident workflow): routes inbound alerts from a " +
			"provider to an orchestrator agent and its specialist agents, optionally delivering " +
			"summaries to Slack.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Pipeline identifier.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Human-readable pipeline name. Server-assigned when omitted.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"status": schema.StringAttribute{
				MarkdownDescription: "Lifecycle status. Newly created pipelines start as `draft`; set to " +
					"`active` or `paused` to publish or suspend the pipeline. One of `draft`, `active`, `paused`.",
				Optional:      true,
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"alert_source": schema.SingleNestedAttribute{
				MarkdownDescription: "Where inbound alerts originate.",
				Required:            true,
				Attributes: map[string]schema.Attribute{
					"provider": schema.StringAttribute{
						MarkdownDescription: "Alert provider. One of `datadog`, `generic`.",
						Required:            true,
					},
					"monitor_mode": schema.StringAttribute{
						MarkdownDescription: "How the pipeline binds to the provider's monitor. One of " +
							"`create_catchall`, `link_existing`.",
						Required: true,
					},
					"external_monitor_id": schema.StringAttribute{
						MarkdownDescription: "External monitor identifier to link when `monitor_mode = link_existing`.",
						Optional:            true,
					},
				},
			},
			"routing_rule": schema.SingleNestedAttribute{
				MarkdownDescription: "Which incidents this pipeline handles. Defaults to routing everything.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.Object{objectplanmodifier.UseStateForUnknown()},
				Attributes: map[string]schema.Attribute{
					"route_all": schema.BoolAttribute{
						MarkdownDescription: "Route every incident, ignoring the filters below.",
						Optional:            true,
						Computed:            true,
						PlanModifiers:       []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
					},
					"missing_field_default": schema.BoolAttribute{
						MarkdownDescription: "Whether an incident missing a filtered field is routed.",
						Optional:            true,
						Computed:            true,
						PlanModifiers:       []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
					},
					"environment": schema.StringAttribute{
						MarkdownDescription: "Only route incidents from this environment.",
						Optional:            true,
					},
					"service": schema.StringAttribute{
						MarkdownDescription: "Only route incidents for this service.",
						Optional:            true,
					},
					"severity": schema.StringAttribute{
						MarkdownDescription: "Only route incidents at this severity.",
						Optional:            true,
					},
					"tags": schema.MapAttribute{
						MarkdownDescription: "Only route incidents matching these tag key/value pairs.",
						ElementType:         types.StringType,
						Optional:            true,
					},
				},
			},
			"orchestrator_binding": schema.SingleNestedAttribute{
				MarkdownDescription: "The orchestrator agent that triages routed incidents. Server-provisioned when omitted.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.Object{objectplanmodifier.UseStateForUnknown()},
				Attributes: map[string]schema.Attribute{
					"agent_id": schema.StringAttribute{
						MarkdownDescription: "ID of the orchestrator agent.",
						Required:            true,
					},
				},
			},
			"specialist_bindings": schema.ListNestedAttribute{
				MarkdownDescription: "Specialist agents the orchestrator can delegate to.",
				Optional:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"agent_id": schema.StringAttribute{
							MarkdownDescription: "ID of the specialist agent.",
							Required:            true,
						},
						"role": schema.StringAttribute{
							MarkdownDescription: "Role the specialist plays in the pipeline.",
							Required:            true,
						},
						"enabled": schema.BoolAttribute{
							MarkdownDescription: "Whether the specialist is enabled.",
							Required:            true,
						},
					},
				},
			},
			"delivery_config": schema.SingleNestedAttribute{
				MarkdownDescription: "Where incident summaries are delivered.",
				Optional:            true,
				Attributes: map[string]schema.Attribute{
					"slack": schema.SingleNestedAttribute{
						MarkdownDescription: "Slack delivery target.",
						Optional:            true,
						Attributes: map[string]schema.Attribute{
							"channel_id": schema.StringAttribute{
								MarkdownDescription: "Slack channel ID to post to.",
								Required:            true,
							},
							"channel_name": schema.StringAttribute{
								MarkdownDescription: "Slack channel name. Resolved by the server when omitted.",
								Optional:            true,
								Computed:            true,
								PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
							},
							"enabled": schema.BoolAttribute{
								MarkdownDescription: "Whether Slack delivery is enabled.",
								Optional:            true,
								Computed:            true,
								PlanModifiers:       []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
							},
						},
					},
				},
			},
			"trigger_id": schema.StringAttribute{
				MarkdownDescription: "ID of the webhook trigger backing this pipeline.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"source_provider": schema.StringAttribute{
				MarkdownDescription: "Resolved alert provider for the pipeline.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"orchestrator_agent_id": schema.StringAttribute{
				MarkdownDescription: "Resolved orchestrator agent ID.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"specialist_count": schema.Int64Attribute{
				MarkdownDescription: "Number of specialist agents bound to the pipeline.",
				Computed:            true,
			},
			"webhook_url": schema.StringAttribute{
				MarkdownDescription: "Webhook URL alert providers post to.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"webhook_token": schema.StringAttribute{
				MarkdownDescription: "Webhook authentication token. Sensitive.",
				Computed:            true,
				Sensitive:           true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"last_incident_at": schema.StringAttribute{
				MarkdownDescription: "Timestamp of the most recent incident routed by this pipeline.",
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

func (r *incidentPipelineResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromProviderData(req.ProviderData, &resp.Diagnostics)
}

func (r *incidentPipelineResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan incidentPipelineResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := gen.CreateIncidentPipelineRequest{
		Name:                stringToPtr(plan.Name),
		AlertSource:         incidentPipelineAlertSourceToAPI(plan.AlertSource),
		RoutingRule:         incidentPipelineRoutingRuleToAPI(ctx, plan.RoutingRule, &resp.Diagnostics),
		OrchestratorBinding: incidentPipelineBindingToAPI(plan.OrchestratorBinding),
		SpecialistBindings:  incidentPipelineSpecialistsToAPI(plan.SpecialistBindings),
		DeliveryConfig:      incidentPipelineDeliveryToAPI(plan.DeliveryConfig),
	}
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.Gen.IncidentPipelinesCreateIncidentPipelineEndpointWithResponse(ctx, body)
	if err != nil {
		resp.Diagnostics.AddError("Error creating incident pipeline", err.Error())
		return
	}
	if err := client.Check(apiResp.HTTPResponse, apiResp.Body); err != nil {
		resp.Diagnostics.AddError("Error creating incident pipeline", err.Error())
		return
	}
	if apiResp.JSON201 == nil {
		resp.Diagnostics.AddError("Error creating incident pipeline", "API returned an empty body")
		return
	}

	desired := statusTarget(plan.Status)
	detail := apiResp.JSON201

	// The create endpoint does not accept a trigger (endpoint); link it via update so
	// the pipeline can be activated — activation requires a linked endpoint.
	if v := plan.TriggerID; !v.IsNull() && !v.IsUnknown() && v.ValueString() != "" {
		linked, err := r.client.Gen.IncidentPipelinesUpdateIncidentPipelineEndpointWithResponse(ctx, detail.Id,
			gen.UpdateIncidentPipelineRequest{TriggerId: stringToPtr(v)})
		if err != nil {
			resp.Diagnostics.AddError("Error linking endpoint to incident pipeline", err.Error())
			return
		}
		if err := client.Check(linked.HTTPResponse, linked.Body); err != nil {
			resp.Diagnostics.AddError("Error linking endpoint to incident pipeline", err.Error())
			return
		}
		if linked.JSON200 != nil {
			detail = linked.JSON200
		}
	}

	detail = r.reconcileStatus(ctx, detail.Id, desired, detail, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(incidentPipelineApply(ctx, &plan, detail)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *incidentPipelineResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state incidentPipelineResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.Gen.IncidentPipelinesGetIncidentPipelineEndpointWithResponse(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading incident pipeline", err.Error())
		return
	}
	if err := client.Check(apiResp.HTTPResponse, apiResp.Body); err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading incident pipeline", err.Error())
		return
	}
	if apiResp.JSON200 == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	resp.Diagnostics.Append(incidentPipelineApply(ctx, &state, apiResp.JSON200)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *incidentPipelineResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan incidentPipelineResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	alertSource := incidentPipelineAlertSourceToAPI(plan.AlertSource)
	body := gen.UpdateIncidentPipelineRequest{
		Name:                stringToPtr(plan.Name),
		AlertSource:         &alertSource,
		RoutingRule:         incidentPipelineRoutingRuleToAPI(ctx, plan.RoutingRule, &resp.Diagnostics),
		OrchestratorBinding: incidentPipelineBindingToAPI(plan.OrchestratorBinding),
		SpecialistBindings:  incidentPipelineSpecialistsToAPI(plan.SpecialistBindings),
		DeliveryConfig:      incidentPipelineDeliveryToAPI(plan.DeliveryConfig),
		TriggerId:           stringToPtr(plan.TriggerID),
	}
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.Gen.IncidentPipelinesUpdateIncidentPipelineEndpointWithResponse(ctx, plan.ID.ValueString(), body)
	if err != nil {
		resp.Diagnostics.AddError("Error updating incident pipeline", err.Error())
		return
	}
	if err := client.Check(apiResp.HTTPResponse, apiResp.Body); err != nil {
		resp.Diagnostics.AddError("Error updating incident pipeline", err.Error())
		return
	}
	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError("Error updating incident pipeline", "API returned an empty body")
		return
	}

	desired := statusTarget(plan.Status)
	detail := r.reconcileStatus(ctx, plan.ID.ValueString(), desired, apiResp.JSON200, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(incidentPipelineApply(ctx, &plan, detail)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *incidentPipelineResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state incidentPipelineResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.Gen.IncidentPipelinesDeleteIncidentPipelineEndpointWithResponse(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error deleting incident pipeline", err.Error())
		return
	}
	if err := client.Check(apiResp.HTTPResponse, apiResp.Body); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Error deleting incident pipeline", err.Error())
	}
}

func (r *incidentPipelineResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// reconcileStatus drives the pipeline to the desired lifecycle status via the
// activate/pause endpoints. It returns the latest detail (the input detail
// unchanged when no transition is needed).
func (r *incidentPipelineResource) reconcileStatus(ctx context.Context, id, desired string, detail *gen.IncidentPipelineDetail, diags *diag.Diagnostics) *gen.IncidentPipelineDetail {
	if desired == "" || desired == string(detail.Status) {
		return detail
	}

	switch desired {
	case "active":
		apiResp, err := r.client.Gen.IncidentPipelinesActivateIncidentPipelineEndpointWithResponse(ctx, id)
		if err != nil {
			diags.AddError("Error activating incident pipeline", err.Error())
			return detail
		}
		if err := client.Check(apiResp.HTTPResponse, apiResp.Body); err != nil {
			diags.AddError("Error activating incident pipeline", err.Error())
			return detail
		}
		if apiResp.JSON200 != nil {
			return apiResp.JSON200
		}
	case "paused":
		apiResp, err := r.client.Gen.IncidentPipelinesPauseIncidentPipelineEndpointWithResponse(ctx, id)
		if err != nil {
			diags.AddError("Error pausing incident pipeline", err.Error())
			return detail
		}
		if err := client.Check(apiResp.HTTPResponse, apiResp.Body); err != nil {
			diags.AddError("Error pausing incident pipeline", err.Error())
			return detail
		}
		if apiResp.JSON200 != nil {
			return apiResp.JSON200
		}
	case "draft":
		diags.AddAttributeError(path.Root("status"), "Cannot return an incident pipeline to draft",
			"A published pipeline cannot be reverted to `draft`; use `paused` to suspend it.")
	}
	return detail
}

// statusTarget returns the desired status string, or "" when the value is
// unset (null/unknown) so the pipeline keeps whatever the server reports.
func statusTarget(v types.String) string {
	if v.IsNull() || v.IsUnknown() {
		return ""
	}
	return v.ValueString()
}

func incidentPipelineAlertSourceToAPI(m *incidentPipelineAlertSourceModel) gen.IncidentPipelineAlertSource {
	if m == nil {
		return gen.IncidentPipelineAlertSource{}
	}
	return gen.IncidentPipelineAlertSource{
		Provider:          gen.AlertProvider(m.Provider.ValueString()),
		MonitorMode:       gen.MonitorMode(m.MonitorMode.ValueString()),
		ExternalMonitorId: stringToPtr(m.ExternalMonitorID),
	}
}

func incidentPipelineRoutingRuleToAPI(ctx context.Context, m *incidentPipelineRoutingRuleModel, diags *diag.Diagnostics) *gen.IncidentPipelineRoutingRule {
	if m == nil {
		return nil
	}
	rule := &gen.IncidentPipelineRoutingRule{
		RouteAll:            boolToPtr(m.RouteAll),
		MissingFieldDefault: boolToPtr(m.MissingFieldDefault),
		Environment:         stringToPtr(m.Environment),
		Service:             stringToPtr(m.Service),
		Severity:            stringToPtr(m.Severity),
	}
	if !m.Tags.IsNull() && !m.Tags.IsUnknown() {
		tags := map[string]string{}
		diags.Append(m.Tags.ElementsAs(ctx, &tags, false)...)
		rule.Tags = &tags
	}
	return rule
}

func incidentPipelineBindingToAPI(m *incidentPipelineBindingModel) *gen.IncidentPipelineOrchestratorBinding {
	if m == nil {
		return nil
	}
	return &gen.IncidentPipelineOrchestratorBinding{AgentId: m.AgentID.ValueString()}
}

func incidentPipelineSpecialistsToAPI(items []incidentPipelineSpecialistModel) *[]gen.IncidentPipelineSpecialistBinding {
	if items == nil {
		return nil
	}
	out := make([]gen.IncidentPipelineSpecialistBinding, 0, len(items))
	for _, s := range items {
		out = append(out, gen.IncidentPipelineSpecialistBinding{
			AgentId: s.AgentID.ValueString(),
			Role:    s.Role.ValueString(),
			Enabled: s.Enabled.ValueBool(),
		})
	}
	return &out
}

func incidentPipelineDeliveryToAPI(m *incidentPipelineDeliveryModel) *gen.DeliveryConfig {
	if m == nil {
		return nil
	}
	cfg := &gen.DeliveryConfig{}
	if m.Slack != nil {
		cfg.Slack = &gen.SlackDeliveryConfig{
			ChannelId:   m.Slack.ChannelID.ValueString(),
			ChannelName: stringToPtr(m.Slack.ChannelName),
			Enabled:     boolToPtr(m.Slack.Enabled),
		}
	}
	return cfg
}

// incidentPipelineApply writes an IncidentPipelineDetail response into the model.
func incidentPipelineApply(ctx context.Context, m *incidentPipelineResourceModel, d *gen.IncidentPipelineDetail) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.StringValue(d.Id)
	m.Name = types.StringValue(d.Name)
	m.Status = types.StringValue(string(d.Status))
	m.SourceProvider = types.StringValue(string(d.SourceProvider))
	m.OrchestratorAgentID = types.StringValue(d.OrchestratorAgentId)
	m.SpecialistCount = types.Int64Value(int64(d.SpecialistCount))
	m.TriggerID = ptrToString(d.TriggerId)
	m.WebhookURL = ptrToString(d.WebhookUrl)
	m.WebhookToken = ptrToString(d.WebhookToken)
	m.LastIncidentAt = ptrToString(d.LastIncidentAt)
	m.CreatedAt = types.StringValue(d.CreatedAt)

	m.AlertSource = &incidentPipelineAlertSourceModel{
		Provider:          types.StringValue(string(d.AlertSource.Provider)),
		MonitorMode:       types.StringValue(string(d.AlertSource.MonitorMode)),
		ExternalMonitorID: ptrToString(d.AlertSource.ExternalMonitorId),
	}

	rule := &incidentPipelineRoutingRuleModel{
		RouteAll:            boolPtrToBool(d.RoutingRule.RouteAll),
		MissingFieldDefault: boolPtrToBool(d.RoutingRule.MissingFieldDefault),
		Environment:         ptrToString(d.RoutingRule.Environment),
		Service:             ptrToString(d.RoutingRule.Service),
		Severity:            ptrToString(d.RoutingRule.Severity),
		Tags:                types.MapNull(types.StringType),
	}
	if d.RoutingRule.Tags != nil {
		tags, dg := types.MapValueFrom(ctx, types.StringType, *d.RoutingRule.Tags)
		diags.Append(dg...)
		rule.Tags = tags
	}
	m.RoutingRule = rule

	m.OrchestratorBinding = &incidentPipelineBindingModel{AgentID: types.StringValue(d.OrchestratorBinding.AgentId)}

	if d.SpecialistBindings != nil && len(*d.SpecialistBindings) > 0 {
		specialists := make([]incidentPipelineSpecialistModel, 0, len(*d.SpecialistBindings))
		for _, s := range *d.SpecialistBindings {
			specialists = append(specialists, incidentPipelineSpecialistModel{
				AgentID: types.StringValue(s.AgentId),
				Role:    types.StringValue(s.Role),
				Enabled: types.BoolValue(s.Enabled),
			})
		}
		m.SpecialistBindings = specialists
	} else {
		m.SpecialistBindings = nil
	}

	if d.DeliveryConfig != nil && d.DeliveryConfig.Slack != nil {
		m.DeliveryConfig = &incidentPipelineDeliveryModel{
			Slack: &incidentPipelineSlackModel{
				ChannelID:   types.StringValue(d.DeliveryConfig.Slack.ChannelId),
				ChannelName: ptrToString(d.DeliveryConfig.Slack.ChannelName),
				Enabled:     boolPtrToBool(d.DeliveryConfig.Slack.Enabled),
			},
		}
	} else {
		m.DeliveryConfig = nil
	}

	return diags
}
