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
	_ datasource.DataSource              = &resourceTypesDataSource{}
	_ datasource.DataSourceWithConfigure = &resourceTypesDataSource{}
)

// NewResourceTypesDataSource is the constructor registered with the provider.
func NewResourceTypesDataSource() datasource.DataSource {
	return &resourceTypesDataSource{}
}

type resourceTypesDataSource struct {
	client *client.Client
}

type resourceTypesDataSourceModel struct {
	ResourceTypes []resourceTypeModel `tfsdk:"resource_types"`
}

type resourceTypeModel struct {
	Key   types.String `tfsdk:"key"`
	Notes types.String `tfsdk:"notes"`
	Scope types.String `tfsdk:"scope"`
}

func (d *resourceTypesDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_resource_types"
}

func (d *resourceTypesDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "The authorization resource types available to this account.",
		Attributes: map[string]schema.Attribute{
			"resource_types": schema.ListNestedAttribute{
				MarkdownDescription: "Available authorization resource types.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"key":   schema.StringAttribute{Computed: true, MarkdownDescription: "The resource type key."},
						"notes": schema.StringAttribute{Computed: true, MarkdownDescription: "Notes describing the resource type."},
						"scope": schema.StringAttribute{Computed: true, MarkdownDescription: "The scope of the resource type."},
					},
				},
			},
		},
	}
}

func (d *resourceTypesDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromProviderData(req.ProviderData, &resp.Diagnostics)
}

func (d *resourceTypesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	apiResp, err := d.client.Gen.AuthzListResourceTypesWithResponse(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error reading resource types", err.Error())
		return
	}
	if err := client.Check(apiResp.HTTPResponse, apiResp.Body); err != nil {
		resp.Diagnostics.AddError("Error reading resource types", err.Error())
		return
	}
	if apiResp.JSON200 == nil {
		return
	}

	var state resourceTypesDataSourceModel
	for _, rt := range *apiResp.JSON200 {
		state.ResourceTypes = append(state.ResourceTypes, resourceTypeModel{
			Key:   types.StringValue(rt.Key),
			Notes: types.StringValue(rt.Notes),
			Scope: types.StringValue(rt.Scope),
		})
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
