// Copyright (c) HashiCorp, Inc.

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
	"github.com/hoophq/terraform-provider-hoop/internal/hoop"
)

var providerDescription = `
The Hoop provider allows managing resources from a Hoop Gateway instance API.
`

// Ensure the implementation satisfies the expected interfaces.
var (
	_ provider.Provider = &hoopProvider{}
)

// New is a helper function to simplify provider server and testing implementation.
func New(version string, httpClient hoop.HttpClient) func() provider.Provider {
	return func() provider.Provider {
		return &hoopProvider{
			version:    version,
			httpClient: httpClient,
		}
	}
}

type hoopProvider struct {
	// version is set to the provider version on release, "dev" when the
	// provider is built and ran locally, and "test" when running acceptance
	// testing.
	version    string
	httpClient hoop.HttpClient
}

// hashicupsProviderModel maps provider schema data to a Go type.
type hoopProviderModel struct {
	ApiURL types.String `tfsdk:"api_url"`
	ApiKey types.String `tfsdk:"api_key"`
}

// Metadata returns the provider type name.
func (p *hoopProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "hoop"
	resp.Version = p.version
}

// Schema defines the provider-level schema for configuration data.
func (p *hoopProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: providerDescription,
		Attributes: map[string]schema.Attribute{
			"api_url": schema.StringAttribute{
				Description: "The API URL of the Hoop Gateway instance. It may also be provided via `HOOP_APIURL` environment variable.",
				Required:    true,
			},
			"api_key": schema.StringAttribute{
				Description: "The API Key to authenticate in the Hoop Gateway. May also be provided via `HOOP_APIKEY` environment variable.",
				Required:    true,
				Sensitive:   true,
			},
		},
	}
}

// Configure prepares a HashiCups API client for data sources and resources.
func (p *hoopProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	// Retrieve provider data from configuration
	var config hoopProviderModel
	diags := req.Config.Get(ctx, &config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// If practitioner provided a configuration value for any of the
	// attributes, it must be a known value.

	if config.ApiURL.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("api_url"),
			"Unknown Hoop API URL",
			"The provider cannot create the Hoop Gateway client as there is an unknown configuration value for the Hoop API URL. "+
				"Either target apply the source of the value first, set the value statically in the configuration, or use the HOOP_APIURL environment variable.",
		)
	}

	if config.ApiKey.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("api_key"),
			"Unknown Hoop API Key",
			"The provider cannot create the Hoop Gateway client as there is an unknown configuration value for the Hoop API Key. "+
				"Either target apply the source of the value first, set the value statically in the configuration, or use the HOOP_APIKEY environment variable.",
		)
	}

	if resp.Diagnostics.HasError() {
		return
	}

	// Default values to environment variables, but override
	// with Terraform configuration value if set.
	apiURL := os.Getenv("HOOP_APIURL")
	apiKey := os.Getenv("HOOP_APIKEY")

	if !config.ApiURL.IsNull() {
		apiURL = config.ApiURL.ValueString()
	}

	if !config.ApiKey.IsNull() {
		apiKey = config.ApiKey.ValueString()
	}

	// If any of the expected configurations are missing, return
	// errors with provider-specific guidance.

	if apiURL == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("api_url"),
			"Missing Hoop API URL",
			"The provider cannot create the Hoop Gateway client as there is an unknown configuration value for the Hoop API URL. "+
				"Either target apply the source of the value first, set the value statically in the configuration, or use the HOOP_APIURL environment variable."+
				"If either is already set, ensure the value is not empty.",
		)
	}

	if apiKey == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("api_key"),
			"Missing Hoop API Key",
			"The provider cannot create the Hoop Gateway client as there is an unknown configuration value for the Hoop API Key. "+
				"Either target apply the source of the value first, set the value statically in the configuration, or use the HOOP_APIKEY environment variable."+
				"If either is already set, ensure the value is not empty.",
		)
	}

	if resp.Diagnostics.HasError() {
		return
	}

	client := hoop.NewClient(apiURL, apiKey, p.httpClient)

	// Make the HashiCups client available during DataSource and Resource
	// type Configure methods.
	resp.DataSourceData = client
	resp.ResourceData = client

}

// DataSources defines the data sources implemented in the provider.
func (p *hoopProvider) DataSources(_ context.Context) []func() datasource.DataSource { return nil }

// Resources defines the resources implemented in the provider.
func (p *hoopProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewConnectionResource,
		NewPluginConnectionResource,
		NewPluginConfigResource,
		NewDatamaskingRulesResource,
		NewUserResource,
	}
}
