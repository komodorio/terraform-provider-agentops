// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"

	"github.com/komodorio/terraform-provider-agentops/internal/client"
)

var (
	_ resource.Resource                = &graderConfigResource{}
	_ resource.ResourceWithConfigure   = &graderConfigResource{}
	_ resource.ResourceWithImportState = &graderConfigResource{}
)

func NewGraderConfigResource() resource.Resource {
	return &graderConfigResource{}
}

type graderConfigResource struct {
	client *client.Client
}

func (r *graderConfigResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_grader_config"
}

func (r *graderConfigResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		DeprecationMessage: "The grader_config API has been removed. This resource type will be removed in a future release.",
		MarkdownDescription: "A grader configuration: has a grader agent automatically score a sample of " +
			"another agent's runs for quality.\n\n**Deprecated:** the underlying API has been removed.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"target_agent_id": schema.StringAttribute{
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"grader_agent_id": schema.StringAttribute{
				Required: true,
			},
			"guidelines": schema.StringAttribute{
				Optional: true,
			},
			"sample_rate": schema.Int64Attribute{
				Optional:      true,
				Computed:      true,
				PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},
			"runs_seen": schema.Int64Attribute{
				Computed: true,
			},
			"created_at": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"updated_at": schema.StringAttribute{
				Computed: true,
			},
		},
	}
}

func (r *graderConfigResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromProviderData(req.ProviderData, &resp.Diagnostics)
}

const graderRemovedMsg = "The grader_config API has been removed from the control plane. " +
	"Remove this resource from your configuration and state (`terraform state rm`)."

func (r *graderConfigResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	resp.Diagnostics.AddError("Grader config API removed", graderRemovedMsg)
}

func (r *graderConfigResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	resp.State.RemoveResource(ctx)
}

func (r *graderConfigResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError("Grader config API removed", graderRemovedMsg)
}

func (r *graderConfigResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
}

func (r *graderConfigResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
