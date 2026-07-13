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
	_ datasource.DataSource              = &integrationCatalogDataSource{}
	_ datasource.DataSourceWithConfigure = &integrationCatalogDataSource{}
)

// NewIntegrationCatalogDataSource is the constructor registered with the provider.
func NewIntegrationCatalogDataSource() datasource.DataSource {
	return &integrationCatalogDataSource{}
}

type integrationCatalogDataSource struct {
	client *client.Client
}

type integrationCatalogDataSourceModel struct {
	Entries []integrationCatalogEntryModel `tfsdk:"entries"`
}

type integrationCatalogEntryModel struct {
	Provider      types.String `tfsdk:"provider"`
	Name          types.String `tfsdk:"name"`
	Description   types.String `tfsdk:"description"`
	Category      types.String `tfsdk:"category"`
	AuthMode      types.String `tfsdk:"auth_mode"`
	AuthConfigKey types.String `tfsdk:"auth_config_key"`
	Available     types.Bool   `tfsdk:"available"`
	Capabilities  types.List   `tfsdk:"capabilities"`
	DocsURL       types.String `tfsdk:"docs_url"`
	LogoURL       types.String `tfsdk:"logo_url"`
}

func (d *integrationCatalogDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_integration_catalog"
}

func (d *integrationCatalogDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "The catalog of integration providers available to this account.",
		Attributes: map[string]schema.Attribute{
			"entries": schema.ListNestedAttribute{
				MarkdownDescription: "Available integration catalog entries.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"provider":        schema.StringAttribute{Computed: true, MarkdownDescription: "Integration provider key."},
						"name":            schema.StringAttribute{Computed: true, MarkdownDescription: "Display name."},
						"description":     schema.StringAttribute{Computed: true, MarkdownDescription: "Description."},
						"category":        schema.StringAttribute{Computed: true, MarkdownDescription: "Category."},
						"auth_mode":       schema.StringAttribute{Computed: true, MarkdownDescription: "Authentication mode."},
						"auth_config_key": schema.StringAttribute{Computed: true, MarkdownDescription: "Auth configuration key."},
						"available":       schema.BoolAttribute{Computed: true, MarkdownDescription: "Whether the integration is available."},
						"capabilities": schema.ListAttribute{
							Computed:            true,
							ElementType:         types.StringType,
							MarkdownDescription: "Capabilities the integration provides.",
						},
						"docs_url": schema.StringAttribute{Computed: true, MarkdownDescription: "Documentation URL."},
						"logo_url": schema.StringAttribute{Computed: true, MarkdownDescription: "Logo URL."},
					},
				},
			},
		},
	}
}

func (d *integrationCatalogDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromProviderData(req.ProviderData, &resp.Diagnostics)
}

func (d *integrationCatalogDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	apiResp, err := d.client.Gen.IntegrationsListCatalogWithResponse(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error reading integration catalog", err.Error())
		return
	}
	if err := client.Check(apiResp.HTTPResponse, apiResp.Body); err != nil {
		resp.Diagnostics.AddError("Error reading integration catalog", err.Error())
		return
	}
	if apiResp.JSON200 == nil {
		return
	}

	var state integrationCatalogDataSourceModel
	for _, e := range *apiResp.JSON200 {
		capabilities := types.ListNull(types.StringType)
		if e.Capabilities != nil {
			list, diags := types.ListValueFrom(ctx, types.StringType, *e.Capabilities)
			resp.Diagnostics.Append(diags...)
			capabilities = list
		}
		state.Entries = append(state.Entries, integrationCatalogEntryModel{
			Provider:      types.StringValue(e.Provider),
			Name:          types.StringValue(e.Name),
			Description:   types.StringValue(e.Description),
			Category:      types.StringValue(e.Category),
			AuthMode:      types.StringValue(string(e.AuthMode)),
			AuthConfigKey: types.StringValue(e.AuthConfigKey),
			Available:     boolPtrToBool(e.Available),
			Capabilities:  capabilities,
			DocsURL:       ptrToString(e.DocsUrl),
			LogoURL:       ptrToString(e.LogoUrl),
		})
	}
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
