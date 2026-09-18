package provider

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/meilisearch/meilisearch-go"
)

var (
	_ provider.Provider                   = &MeilisearchProvider{}
	_ provider.ProviderWithValidateConfig = &MeilisearchProvider{}
)

type MeilisearchProvider struct {
	version string
}

type MeilisearchProviderModel struct {
	Host             types.String `tfsdk:"host"`
	APIKey           types.String `tfsdk:"api_key"`
	OperationTimeout types.String `tfsdk:"operation_timeout"`
}

type providerData struct {
	client           meilisearch.ServiceManager
	operationTimeout time.Duration
	host             string
	apiKey           string
	httpClient       *http.Client
}

func (p *MeilisearchProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "meilisearch"
	resp.Version = p.version
}

func (p *MeilisearchProvider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages Meilisearch server APIs. Requires Terraform 1.14 or later.",
		Attributes: map[string]schema.Attribute{
			"operation_timeout": schema.StringAttribute{
				Description: "Maximum duration of an API operation, including asynchronous task completion, as a positive Go duration (for example, 5m or 30s). Defaults to 5m.",
				Optional:    true,
			},
			"host": schema.StringAttribute{
				Description: "HTTP(S) URL of the Meilisearch server, without credentials, query parameters, or a fragment. May also be provided via MEILISEARCH_HOST.",
				Optional:    true,
			},
			"api_key": schema.StringAttribute{
				Description: "Meilisearch API key with permissions for the managed operations. May also be provided via MEILISEARCH_API_KEY.",
				Optional:    true,
				Sensitive:   true,
			},
		},
	}
}

func (p *MeilisearchProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var config MeilisearchProviderModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)

	if resp.Diagnostics.HasError() {
		return
	}

	if config.Host.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("host"),
			"Unknown Meilisearch host",
			"The host must be known before configuring the provider.",
		)
	}

	if config.APIKey.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("api_key"),
			"Unknown Meilisearch API key",
			"The API key must be known before configuring the provider.",
		)
	}

	if config.OperationTimeout.IsUnknown() {
		resp.Diagnostics.AddAttributeError(path.Root("operation_timeout"), "Unknown operation timeout", "The operation timeout must be known before configuring the provider.")
	}

	if resp.Diagnostics.HasError() {
		return
	}

	host := config.Host.ValueString()
	if config.Host.IsNull() {
		host = os.Getenv("MEILISEARCH_HOST")
	}

	apiKey := config.APIKey.ValueString()
	if config.APIKey.IsNull() {
		apiKey = os.Getenv("MEILISEARCH_API_KEY")
	}

	operationTimeout, err := configuredOperationTimeout(config.OperationTimeout)
	if err != nil {
		resp.Diagnostics.AddAttributeError(path.Root("operation_timeout"), "Invalid operation timeout", "Provide a positive Go duration, such as 5m or 30s.")
	}

	if !validHost(host) {
		resp.Diagnostics.AddAttributeError(
			path.Root("host"),
			"Invalid or missing Meilisearch host",
			"Set host or MEILISEARCH_HOST to an absolute HTTP(S) URL without credentials, query parameters, or a fragment.",
		)
	}

	if strings.TrimSpace(apiKey) == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("api_key"),
			"Missing Meilisearch API key",
			"Set api_key or MEILISEARCH_API_KEY to a non-empty API key.",
		)
	}

	if resp.Diagnostics.HasError() {
		return
	}

	httpClient := &http.Client{Timeout: operationTimeout}
	data := &providerData{
		client: meilisearch.New(host,
			meilisearch.WithAPIKey(apiKey),
			meilisearch.WithCustomClient(httpClient),
		),
		operationTimeout: operationTimeout,
		host:             host,
		apiKey:           apiKey,
		httpClient:       httpClient,
	}

	resp.DataSourceData = data
	resp.ResourceData = data
}

func (p *MeilisearchProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewKeyResource,
		NewIndexResource,
	}
}

func (p *MeilisearchProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewKeyDataSource,
		NewIndexDataSource,
		NewVersionDataSource,
	}
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &MeilisearchProvider{
			version: version,
		}
	}
}

func (p *MeilisearchProvider) ValidateConfig(ctx context.Context, req provider.ValidateConfigRequest, resp *provider.ValidateConfigResponse) {
	var config MeilisearchProviderModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if !config.Host.IsNull() && !config.Host.IsUnknown() && !validHost(config.Host.ValueString()) {
		resp.Diagnostics.AddAttributeError(path.Root("host"), "Invalid Meilisearch host", "Use an absolute HTTP(S) URL with a hostname and without credentials, query parameters, or a fragment.")
	}

	if !config.APIKey.IsNull() && !config.APIKey.IsUnknown() && strings.TrimSpace(config.APIKey.ValueString()) == "" {
		resp.Diagnostics.AddAttributeError(path.Root("api_key"), "Empty Meilisearch API key", "Provide a non-empty API key or omit this attribute to use MEILISEARCH_API_KEY.")
	}

	if !config.OperationTimeout.IsUnknown() {
		if _, err := configuredOperationTimeout(config.OperationTimeout); err != nil {
			resp.Diagnostics.AddAttributeError(path.Root("operation_timeout"), "Invalid operation timeout", "Provide a positive Go duration, such as 5m or 30s.")
		}
	}
}

func validHost(host string) bool {
	parsed, err := url.Parse(host)
	if err != nil {
		return false
	}

	return (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Hostname() != "" && parsed.User == nil && parsed.RawQuery == "" && !parsed.ForceQuery && parsed.Fragment == ""
}

func configuredOperationTimeout(value types.String) (time.Duration, error) {
	if value.IsNull() {
		return defaultOperationTimeout, nil
	}

	timeout, err := time.ParseDuration(value.ValueString())
	if err != nil {
		return 0, err
	}

	if timeout <= 0 {
		return 0, errors.New("operation timeout must be positive")
	}

	return timeout, nil
}
