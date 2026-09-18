package provider

import (
	"context"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/meilisearch/meilisearch-go"
)

// Ensure provider defined types fully satisfy framework interfaces.
var (
	_ datasource.DataSource              = &keyDataSource{}
	_ datasource.DataSourceWithConfigure = &keyDataSource{}
)

func NewKeyDataSource() datasource.DataSource {
	return &keyDataSource{}
}

// keyDataSource defines the data source implementation.
type keyDataSource struct {
	client           meilisearch.ServiceManager
	operationTimeout time.Duration
}

type keyDataSourceModel struct {
	UID         types.String `tfsdk:"uid"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	Key         types.String `tfsdk:"key"`
	Actions     types.Set    `tfsdk:"actions"`
	Indexes     types.Set    `tfsdk:"indexes"`
	ExpiresAt   types.String `tfsdk:"expires_at"`
	CreatedAt   types.String `tfsdk:"created_at"`
	UpdatedAt   types.String `tfsdk:"updated_at"`
	ID          types.String `tfsdk:"id"`
}

func (d *keyDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_key"
}

func (d *keyDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Retrieves a Meilisearch API key. The credential is sensitive but is stored in Terraform state.",
		Attributes: map[string]schema.Attribute{
			"uid": schema.StringAttribute{
				Description: "UID (uuid v4) used by Meilisearch to identify the key.",
				Required:    true,
			},
			"name": schema.StringAttribute{
				Description: "Name of the key.",
				Computed:    true,
			},
			"description": schema.StringAttribute{
				Description: "Description of the key.",
				Computed:    true,
			},
			"key": schema.StringAttribute{
				Description: "Actual key value.",
				Computed:    true,
				Sensitive:   true,
			},
			"actions": schema.SetAttribute{
				Description: "Actions permitted for the key.",
				ElementType: types.StringType,
				Computed:    true,
			},
			"indexes": schema.SetAttribute{
				Description: "Indexes the key is authorized to act on (with the actions specified in the scope of the key).",
				ElementType: types.StringType,
				Computed:    true,
			},
			"expires_at": schema.StringAttribute{
				Description: "Date and time when the key will expire (RFC3339)",
				Computed:    true,
			},
			"created_at": schema.StringAttribute{
				Description: "Date and time when the key was created (RFC3339)",
				Computed:    true,
			},
			"updated_at": schema.StringAttribute{
				Description: "Date and time when the key was last updated (RFC3339)",
				Computed:    true,
			},
			"id": schema.StringAttribute{
				Description: "Unique identifier of the API key.",
				Computed:    true,
			},
		},
	}
}

func (d *keyDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state keyDataSourceModel

	var identifier types.String

	diags := req.Config.GetAttribute(ctx, path.Root("uid"), &identifier)

	resp.Diagnostics.Append(diags...)

	if resp.Diagnostics.HasError() {
		return
	}

	ctx, cancel := operationContext(ctx, d.operationTimeout)
	defer cancel()

	key, err := d.client.GetKeyWithContext(ctx, identifier.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Read Meilisearch API key",
			apiError(err),
		)
		return
	}

	// Map response body to model
	keyState := keyDataSourceModel{
		UID:         types.StringValue(key.UID),
		Name:        types.StringValue(key.Name),
		Description: types.StringValue(key.Description),
		Key:         types.StringValue(key.Key),
		ExpiresAt:   types.StringNull(),
		CreatedAt:   types.StringValue(key.CreatedAt.Format(time.RFC3339)),
		UpdatedAt:   types.StringValue(key.UpdatedAt.Format(time.RFC3339)),
	}

	if !key.ExpiresAt.IsZero() {
		keyState.ExpiresAt = types.StringValue(key.ExpiresAt.Format(time.RFC3339))
	}

	keyState.Actions, diags = types.SetValueFrom(ctx, types.StringType, key.Actions)
	resp.Diagnostics.Append(diags...)

	keyState.Indexes, diags = types.SetValueFrom(ctx, types.StringType, key.Indexes)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	state = keyState

	state.ID = types.StringValue(key.UID)

	// Set state
	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Configure adds the provider configured client to the data source.
func (d *keyDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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
