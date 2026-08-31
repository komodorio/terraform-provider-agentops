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
	_ datasource.DataSource              = &outpostInstallDataSource{}
	_ datasource.DataSourceWithConfigure = &outpostInstallDataSource{}
)

func NewOutpostInstallDataSource() datasource.DataSource {
	return &outpostInstallDataSource{}
}

type outpostInstallDataSource struct {
	client *client.Client
}

type outpostInstallDataSourceModel struct {
	OutpostID            types.String `tfsdk:"outpost_id"`
	Chart                types.String `tfsdk:"chart"`
	Command              types.String `tfsdk:"command"`
	UpgradeCommand       types.String `tfsdk:"upgrade_command"`
	Values               types.String `tfsdk:"values"`
	UpgradeValues        types.String `tfsdk:"upgrade_values"`
	Namespace            types.String `tfsdk:"namespace"`
	ReleaseName          types.String `tfsdk:"release_name"`
	CredentialSecretName types.String `tfsdk:"credential_secret_name"`
	CredentialSecretKey  types.String `tfsdk:"credential_secret_key"`
}

func (d *outpostInstallDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_outpost_install"
}

func (d *outpostInstallDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Install information for an outpost — Helm chart coordinates, values YAML, and install/upgrade commands.",
		Attributes: map[string]schema.Attribute{
			"outpost_id":             schema.StringAttribute{Required: true, MarkdownDescription: "The outpost to get install info for."},
			"chart":                  schema.StringAttribute{Computed: true, MarkdownDescription: "Helm chart reference."},
			"command":                schema.StringAttribute{Computed: true, MarkdownDescription: "Helm install command."},
			"upgrade_command":        schema.StringAttribute{Computed: true, MarkdownDescription: "Helm upgrade command."},
			"values":                 schema.StringAttribute{Computed: true, MarkdownDescription: "Helm values YAML for initial install."},
			"upgrade_values":         schema.StringAttribute{Computed: true, MarkdownDescription: "Helm values YAML for upgrade."},
			"namespace":              schema.StringAttribute{Computed: true, MarkdownDescription: "Kubernetes namespace for the release."},
			"release_name":           schema.StringAttribute{Computed: true, MarkdownDescription: "Helm release name."},
			"credential_secret_name": schema.StringAttribute{Computed: true, MarkdownDescription: "Name of the Kubernetes secret holding credentials."},
			"credential_secret_key":  schema.StringAttribute{Computed: true, MarkdownDescription: "Key within the credential secret."},
		},
	}
}

func (d *outpostInstallDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromProviderData(req.ProviderData, &resp.Diagnostics)
}

func (d *outpostInstallDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config outpostInstallDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	outpostID := config.OutpostID.ValueString()
	apiResp, err := d.client.Gen.OutpostsInstallValuesEndpointWithResponse(ctx, outpostID)
	if err != nil {
		resp.Diagnostics.AddError("Error reading outpost install info", err.Error())
		return
	}
	if err := client.Check(apiResp.HTTPResponse, apiResp.Body); err != nil {
		resp.Diagnostics.AddError("Error reading outpost install info", err.Error())
		return
	}
	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError("Error reading outpost install info", "API returned an empty body")
		return
	}

	state := outpostInstallDataSourceModel{
		OutpostID:            config.OutpostID,
		Chart:                types.StringValue(apiResp.JSON200.Chart),
		Command:              types.StringValue(apiResp.JSON200.Command),
		UpgradeCommand:       types.StringValue(apiResp.JSON200.UpgradeCommand),
		Values:               types.StringValue(apiResp.JSON200.Values),
		UpgradeValues:        types.StringValue(apiResp.JSON200.UpgradeValues),
		Namespace:            types.StringValue(apiResp.JSON200.Namespace),
		ReleaseName:          types.StringValue(apiResp.JSON200.ReleaseName),
		CredentialSecretName: types.StringValue(apiResp.JSON200.CredentialSecretName),
		CredentialSecretKey:  types.StringValue(apiResp.JSON200.CredentialSecretKey),
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
