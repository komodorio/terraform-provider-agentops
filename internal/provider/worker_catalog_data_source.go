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
	_ datasource.DataSource              = &workerCatalogDataSource{}
	_ datasource.DataSourceWithConfigure = &workerCatalogDataSource{}
)

// NewWorkerCatalogDataSource is the constructor registered with the provider.
func NewWorkerCatalogDataSource() datasource.DataSource {
	return &workerCatalogDataSource{}
}

type workerCatalogDataSource struct {
	client *client.Client
}

type workerCatalogDataSourceModel struct {
	Entries []workerCatalogEntryModel `tfsdk:"entries"`
}

type workerCatalogEntryModel struct {
	ID                 types.String `tfsdk:"id"`
	Name               types.String `tfsdk:"name"`
	Description        types.String `tfsdk:"description"`
	Category           types.String `tfsdk:"category"`
	Status             types.String `tfsdk:"status"`
	DocsURL            types.String `tfsdk:"docs_url"`
	Ready              types.Bool   `tfsdk:"ready"`
	ConfigurableModel  types.Bool   `tfsdk:"configurable_model"`
	SupportsChat       types.Bool   `tfsdk:"supports_chat"`
	SupportsMCP        types.Bool   `tfsdk:"supports_mcp"`
	SupportsTriggers   types.Bool   `tfsdk:"supports_triggers"`
	MissingCredentials types.List   `tfsdk:"missing_credentials"`
}

func (d *workerCatalogDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_worker_catalog"
}

func (d *workerCatalogDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "The catalog of hosted worker agents available to this account.",
		Attributes: map[string]schema.Attribute{
			"entries": schema.ListNestedAttribute{
				MarkdownDescription: "Available worker catalog entries.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id":                 schema.StringAttribute{Computed: true, MarkdownDescription: "Catalog entry identifier."},
						"name":               schema.StringAttribute{Computed: true, MarkdownDescription: "Display name."},
						"description":        schema.StringAttribute{Computed: true, MarkdownDescription: "Description."},
						"category":           schema.StringAttribute{Computed: true, MarkdownDescription: "Category."},
						"status":             schema.StringAttribute{Computed: true, MarkdownDescription: "Status."},
						"docs_url":           schema.StringAttribute{Computed: true, MarkdownDescription: "Documentation URL."},
						"ready":              schema.BoolAttribute{Computed: true, MarkdownDescription: "Whether the catalog worker is ready to deploy."},
						"configurable_model": schema.BoolAttribute{Computed: true, MarkdownDescription: "Whether the worker's model is configurable."},
						"supports_chat":      schema.BoolAttribute{Computed: true, MarkdownDescription: "Whether the worker supports chat."},
						"supports_mcp":       schema.BoolAttribute{Computed: true, MarkdownDescription: "Whether the worker supports MCP."},
						"supports_triggers":  schema.BoolAttribute{Computed: true, MarkdownDescription: "Whether the worker supports triggers."},
						"missing_credentials": schema.ListAttribute{
							Computed:            true,
							ElementType:         types.StringType,
							MarkdownDescription: "Credentials that must be supplied before the worker can be deployed.",
						},
					},
				},
			},
		},
	}
}

func (d *workerCatalogDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromProviderData(req.ProviderData, &resp.Diagnostics)
}

func (d *workerCatalogDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	apiResp, err := d.client.Gen.WorkerCatalogListWorkerCatalogWithResponse(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error reading worker catalog", err.Error())
		return
	}
	if err := client.Check(apiResp.HTTPResponse, apiResp.Body); err != nil {
		resp.Diagnostics.AddError("Error reading worker catalog", err.Error())
		return
	}
	if apiResp.JSON200 == nil {
		return
	}

	var state workerCatalogDataSourceModel
	for _, e := range *apiResp.JSON200 {
		missingCredentials := types.ListNull(types.StringType)
		if e.MissingCredentials != nil {
			list, diags := types.ListValueFrom(ctx, types.StringType, *e.MissingCredentials)
			resp.Diagnostics.Append(diags...)
			missingCredentials = list
		}
		state.Entries = append(state.Entries, workerCatalogEntryModel{
			ID:                 types.StringValue(e.Id),
			Name:               types.StringValue(e.Name),
			Description:        types.StringValue(e.Description),
			Category:           types.StringValue(e.Category),
			Status:             types.StringValue(e.Status),
			DocsURL:            ptrToString(e.DocsUrl),
			Ready:              boolPtrToBool(e.Ready),
			ConfigurableModel:  boolPtrToBool(e.ConfigurableModel),
			SupportsChat:       boolPtrToBool(e.SupportsChat),
			SupportsMCP:        boolPtrToBool(e.SupportsMcp),
			SupportsTriggers:   boolPtrToBool(e.SupportsTriggers),
			MissingCredentials: missingCredentials,
		})
	}
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
