package provider

import (
	"context"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/meilisearch/meilisearch-go"
)

// Ensure provider defined types fully satisfy framework interfaces.
var (
	_ datasource.DataSource              = &versionDataSource{}
	_ datasource.DataSourceWithConfigure = &versionDataSource{}
)

func NewVersionDataSource() datasource.DataSource {
	return &versionDataSource{}
}

// versionDataSource defines the data source implementation.
type versionDataSource struct {
	client           meilisearch.ServiceManager
	operationTimeout time.Duration
}

type versionDataSourceModel struct {
	CommitSha  types.String `tfsdk:"commit_sha"`
	CommitDate types.String `tfsdk:"commit_date"`
	PkgVersion types.String `tfsdk:"pkg_version"`
	ID         types.String `tfsdk:"id"`
}

func (d *versionDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_version"
}

func (d *versionDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Retrieves the meilisearch version.",
		Attributes: map[string]schema.Attribute{
			"commit_sha": schema.StringAttribute{
				Description: "Commit identifier that tagged the pkgVersion release",
				Computed:    true,
			},
			"commit_date": schema.StringAttribute{
				Description: "Commit date reported by the server build, or unknown when unavailable.",
				Computed:    true,
			},
			"pkg_version": schema.StringAttribute{
				Description: "Meilisearch version",
				Computed:    true,
			},
			"id": schema.StringAttribute{
				Description: "Meilisearch package version.",
				Computed:    true,
			},
		},
	}
}

func (d *versionDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state versionDataSourceModel

	ctx, cancel := operationContext(ctx, d.operationTimeout)
	defer cancel()

	version, err := d.client.VersionWithContext(ctx)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Read Meilisearch Version",
			apiError(err),
		)
		return
	}

	// Map response body to model
	versionState := versionDataSourceModel{
		CommitSha:  types.StringValue(version.CommitSha),
		CommitDate: types.StringValue(version.CommitDate),
		PkgVersion: types.StringValue(version.PkgVersion),
	}

	state = versionState

	state.ID = types.StringValue(version.PkgVersion)

	// Set state
	diags := resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Configure adds the provider configured client to the data source.
func (d *versionDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	data, ok := req.ProviderData.(*providerData)
	if !ok || data == nil || data.client == nil {
		resp.Diagnostics.AddError("Invalid provider configuration", "Expected a configured Meilisearch client. Report this provider implementation error.")
		return
	}

	d.client = data.client
	d.operationTimeout = data.operationTimeout
}
