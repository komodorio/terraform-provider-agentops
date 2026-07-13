// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/action"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/ephemeral"
	"github.com/hashicorp/terraform-plugin-framework/function"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure KomodorAgentOpsProvider satisfies various provider interfaces.
var _ provider.Provider = &KomodorAgentOpsProvider{}
var _ provider.ProviderWithFunctions = &KomodorAgentOpsProvider{}
var _ provider.ProviderWithEphemeralResources = &KomodorAgentOpsProvider{}
var _ provider.ProviderWithActions = &KomodorAgentOpsProvider{}

// KomodorAgentOpsProvider defines the provider implementation.
type KomodorAgentOpsProvider struct {
	// version is set to the provider version on release, "dev" when the
	// provider is built and ran locally, and "test" when running acceptance
	// testing.
	version string
}

// KomodorAgentOpsProviderModel describes the provider data model.
type KomodorAgentOpsProviderModel struct {
	Endpoint types.String `tfsdk:"endpoint"`
}

func (p *KomodorAgentOpsProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "agentops"
	resp.Version = p.version
}

func (p *KomodorAgentOpsProvider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"endpoint": schema.StringAttribute{
				MarkdownDescription: "Example provider attribute",
				Optional:            true,
			},
		},
	}
}

func (p *KomodorAgentOpsProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var data KomodorAgentOpsProviderModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Configuration values are now available.
	// if data.Endpoint.IsNull() { /* ... */ }

	// Example client configuration for data sources and resources
	client := http.DefaultClient
	resp.DataSourceData = client
	resp.ResourceData = client
}

func (p *KomodorAgentOpsProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewExampleResource,
	}
}

func (p *KomodorAgentOpsProvider) EphemeralResources(ctx context.Context) []func() ephemeral.EphemeralResource {
	return []func() ephemeral.EphemeralResource{
		NewExampleEphemeralResource,
	}
}

func (p *KomodorAgentOpsProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewExampleDataSource,
	}
}

func (p *KomodorAgentOpsProvider) Functions(ctx context.Context) []func() function.Function {
	return []func() function.Function{
		NewExampleFunction,
	}
}

func (p *KomodorAgentOpsProvider) Actions(ctx context.Context) []func() action.Action {
	return []func() action.Action{
		NewExampleAction,
	}
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &KomodorAgentOpsProvider{
			version: version,
		}
	}
}
