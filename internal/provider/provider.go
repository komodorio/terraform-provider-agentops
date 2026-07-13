// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/komodorio/terraform-provider-agentops/internal/client"
)

// Ensure KomodorAgentOpsProvider satisfies the provider interface.
var _ provider.Provider = &KomodorAgentOpsProvider{}

// Environment variables used as fallbacks for provider configuration.
const (
	envEndpoint = "AGENTOPS_ENDPOINT"
	envAPIKey   = "AGENTOPS_API_KEY"
)

// KomodorAgentOpsProvider defines the provider implementation.
type KomodorAgentOpsProvider struct {
	// version is set to the provider version on release, "dev" when the
	// provider is built and ran locally, and "test" when running acceptance
	// testing.
	version string
}

// KomodorAgentOpsProviderModel describes the provider configuration.
type KomodorAgentOpsProviderModel struct {
	Endpoint types.String `tfsdk:"endpoint"`
	APIKey   types.String `tfsdk:"api_key"`
}

func (p *KomodorAgentOpsProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "agentops"
	resp.Version = p.version
}

func (p *KomodorAgentOpsProvider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manage Komodor AgentOps config-plane resources as code.",
		Attributes: map[string]schema.Attribute{
			"endpoint": schema.StringAttribute{
				MarkdownDescription: "AgentOps control-plane base URL. Defaults to `" + client.DefaultEndpoint +
					"`. May also be set via the `" + envEndpoint + "` environment variable. Use " +
					"`https://staging.agentops.komodor.com` for staging or your own URL for self-hosted.",
				Optional: true,
			},
			"api_key": schema.StringAttribute{
				MarkdownDescription: "AgentOps API key used as a Bearer token. May also be set via the `" +
					envAPIKey + "` environment variable.",
				Optional:  true,
				Sensitive: true,
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

	// Precedence: explicit config value, then environment variable, then default.
	endpoint := os.Getenv(envEndpoint)
	if !data.Endpoint.IsNull() {
		endpoint = data.Endpoint.ValueString()
	}
	if endpoint == "" {
		endpoint = client.DefaultEndpoint
	}

	apiKey := os.Getenv(envAPIKey)
	if !data.APIKey.IsNull() {
		apiKey = data.APIKey.ValueString()
	}
	if apiKey == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("api_key"),
			"Missing AgentOps API key",
			"An API key is required. Set the provider `api_key` attribute or the "+envAPIKey+" environment variable.",
		)
		return
	}

	c, err := client.New(endpoint, apiKey, p.version)
	if err != nil {
		resp.Diagnostics.AddError("Unable to create AgentOps client", err.Error())
		return
	}

	resp.DataSourceData = c
	resp.ResourceData = c
}

func (p *KomodorAgentOpsProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewTriggerResource,
		NewAPIKeyResource,
		NewServiceAccountResource,
		NewScheduleResource,
		NewCredentialResource,
		NewWorkflowResource,
		NewIntegrationConnectionResource,
		NewKnowledgeBaseResource,
		NewMCPGatewayServerResource,
		NewMCPGatewayGroupResource,
		NewMCPGatewayPolicyResource,
	}
}

func (p *KomodorAgentOpsProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewIntegrationCatalogDataSource,
	}
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &KomodorAgentOpsProvider{
			version: version,
		}
	}
}
