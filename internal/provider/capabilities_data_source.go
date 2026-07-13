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
	_ datasource.DataSource              = &capabilitiesDataSource{}
	_ datasource.DataSourceWithConfigure = &capabilitiesDataSource{}
)

// NewCapabilitiesDataSource is the constructor registered with the provider.
func NewCapabilitiesDataSource() datasource.DataSource {
	return &capabilitiesDataSource{}
}

type capabilitiesDataSource struct {
	client *client.Client
}

type capabilitiesDataSourceModel struct {
	Capabilities []capabilityModel `tfsdk:"capabilities"`
}

type capabilityModel struct {
	Allows      types.String `tfsdk:"allows"`
	Domain      types.String `tfsdk:"domain"`
	Key         types.String `tfsdk:"key"`
	Sensitivity types.String `tfsdk:"sensitivity"`
}

func (d *capabilitiesDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_capabilities"
}

func (d *capabilitiesDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "The authorization capabilities available to this account.",
		Attributes: map[string]schema.Attribute{
			"capabilities": schema.ListNestedAttribute{
				MarkdownDescription: "Available authorization capabilities.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"allows":      schema.StringAttribute{Computed: true, MarkdownDescription: "The action this capability allows."},
						"domain":      schema.StringAttribute{Computed: true, MarkdownDescription: "The domain the capability belongs to."},
						"key":         schema.StringAttribute{Computed: true, MarkdownDescription: "The capability key."},
						"sensitivity": schema.StringAttribute{Computed: true, MarkdownDescription: "The sensitivity level of the capability."},
					},
				},
			},
		},
	}
}

func (d *capabilitiesDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromProviderData(req.ProviderData, &resp.Diagnostics)
}

func (d *capabilitiesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	apiResp, err := d.client.Gen.AuthzListCapabilitiesWithResponse(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error reading capabilities", err.Error())
		return
	}
	if err := client.Check(apiResp.HTTPResponse, apiResp.Body); err != nil {
		resp.Diagnostics.AddError("Error reading capabilities", err.Error())
		return
	}
	if apiResp.JSON200 == nil {
		return
	}

	var state capabilitiesDataSourceModel
	for _, c := range *apiResp.JSON200 {
		state.Capabilities = append(state.Capabilities, capabilityModel{
			Allows:      types.StringValue(c.Allows),
			Domain:      types.StringValue(c.Domain),
			Key:         types.StringValue(c.Key),
			Sensitivity: types.StringValue(c.Sensitivity),
		})
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
