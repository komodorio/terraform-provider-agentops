// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/komodorio/terraform-provider-agentops/internal/client"
)

var (
	_ datasource.DataSource              = &skillsDataSource{}
	_ datasource.DataSourceWithConfigure = &skillsDataSource{}
)

// NewSkillsDataSource is the constructor registered with the provider.
func NewSkillsDataSource() datasource.DataSource {
	return &skillsDataSource{}
}

type skillsDataSource struct {
	client *client.Client
}

type skillsDataSourceModel struct {
	Skills []skillsSummaryModel `tfsdk:"skills"`
}

type skillsSummaryModel struct {
	SkillID     types.String `tfsdk:"skill_id"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	Md5         types.String `tfsdk:"md5"`
	UpdatedAt   types.String `tfsdk:"updated_at"`
	Path        types.String `tfsdk:"path"`
	Version     types.String `tfsdk:"version"`
	Tags        types.List   `tfsdk:"tags"`
}

func (d *skillsDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_skills"
}

func (d *skillsDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "The skills available to this account.",
		Attributes: map[string]schema.Attribute{
			"skills": schema.ListNestedAttribute{
				MarkdownDescription: "Available skills.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"skill_id":    schema.StringAttribute{Computed: true, MarkdownDescription: "Skill identifier."},
						"name":        schema.StringAttribute{Computed: true, MarkdownDescription: "Display name."},
						"description": schema.StringAttribute{Computed: true, MarkdownDescription: "Description."},
						"md5":         schema.StringAttribute{Computed: true, MarkdownDescription: "MD5 checksum of the skill."},
						"updated_at":  schema.StringAttribute{Computed: true, MarkdownDescription: "Timestamp of the last update."},
						"path":        schema.StringAttribute{Computed: true, MarkdownDescription: "Path of the skill."},
						"version":     schema.StringAttribute{Computed: true, MarkdownDescription: "Version of the skill."},
						"tags": schema.ListAttribute{
							Computed:            true,
							ElementType:         types.StringType,
							MarkdownDescription: "Tags associated with the skill.",
						},
					},
				},
			},
		},
	}
}

func (d *skillsDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromProviderData(req.ProviderData, &resp.Diagnostics)
}

func (d *skillsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	apiResp, err := d.client.Gen.SkillsListSkillsRouteWithResponse(ctx, nil)
	if err != nil {
		resp.Diagnostics.AddError("Error reading skills", err.Error())
		return
	}
	if err := client.Check(apiResp.HTTPResponse, apiResp.Body); err != nil {
		resp.Diagnostics.AddError("Error reading skills", err.Error())
		return
	}
	if apiResp.JSON200 == nil {
		return
	}

	var state skillsDataSourceModel
	for _, e := range *apiResp.JSON200 {
		tags, diags := types.ListValueFrom(ctx, types.StringType, e.Tags)
		resp.Diagnostics.Append(diags...)
		state.Skills = append(state.Skills, skillsSummaryModel{
			SkillID:     types.StringValue(e.SkillId),
			Name:        types.StringValue(e.Name),
			Description: types.StringValue(e.Description),
			Md5:         types.StringValue(e.Md5),
			UpdatedAt:   types.StringValue(e.UpdatedAt),
			Path:        ptrToString(e.Path),
			Version:     ptrToString(e.Version),
			Tags:        tags,
		})
	}
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
