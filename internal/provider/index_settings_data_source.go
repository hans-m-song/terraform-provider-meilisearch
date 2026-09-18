package provider

import (
	"context"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/meilisearch/meilisearch-go"
)

var (
	_ datasource.DataSource              = &indexSettingsDataSource{}
	_ datasource.DataSourceWithConfigure = &indexSettingsDataSource{}
)

func NewIndexSettingsDataSource() datasource.DataSource {
	return &indexSettingsDataSource{}
}

type indexSettingsDataSource struct {
	client           meilisearch.ServiceManager
	operationTimeout time.Duration
	provider         *providerData
}

type indexSettingsDataSourceModel struct {
	IndexUID             types.String `tfsdk:"index_uid"`
	SearchableAttributes types.List   `tfsdk:"searchable_attributes"`
	DisplayedAttributes  types.List   `tfsdk:"displayed_attributes"`
	FilterableAttributes types.Set    `tfsdk:"filterable_attributes"`
	SortableAttributes   types.Set    `tfsdk:"sortable_attributes"`
	RankingRules         types.List   `tfsdk:"ranking_rules"`
	StopWords            types.Set    `tfsdk:"stop_words"`
	Synonyms             types.Map    `tfsdk:"synonyms"`
	DistinctAttribute    types.String `tfsdk:"distinct_attribute"`
	ID                   types.String `tfsdk:"id"`
}

func (d *indexSettingsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_index_settings"
}

func (d *indexSettingsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = indexSettingsDataSourceSchema()
}

func indexSettingsDataSourceSchema() schema.Schema {
	return schema.Schema{
		Description: "Reads selected core settings from an existing Meilisearch index.",
		Attributes: map[string]schema.Attribute{
			"index_uid": schema.StringAttribute{
				Description: "UID of the existing Meilisearch index.",
				Required:    true,
			},
			"searchable_attributes": indexSettingsDataSourceListAttribute("Searchable attributes in priority order."),
			"displayed_attributes":  indexSettingsDataSourceListAttribute("Attributes returned in search results."),
			"filterable_attributes": indexSettingsDataSourceSetAttribute("Attributes available to filtering. Advanced filterable rules are unsupported."),
			"sortable_attributes":   indexSettingsDataSourceSetAttribute("Attributes available to sorting."),
			"ranking_rules":         indexSettingsDataSourceListAttribute("Ranking rules in evaluation order."),
			"stop_words":            indexSettingsDataSourceSetAttribute("Stop words ignored during search."),
			"synonyms": schema.MapAttribute{
				Description: "Synonym groups keyed by their source term.",
				ElementType: types.ListType{ElemType: types.StringType},
				Computed:    true,
			},
			"distinct_attribute": schema.StringAttribute{
				Description: "Attribute used to deduplicate search results.",
				Computed:    true,
			},
			"id": schema.StringAttribute{
				Description: "Remote index UID.",
				Computed:    true,
			},
		},
	}
}

func indexSettingsDataSourceListAttribute(description string) schema.ListAttribute {
	return schema.ListAttribute{
		Description: description,
		ElementType: types.StringType,
		Computed:    true,
	}
}

func indexSettingsDataSourceSetAttribute(description string) schema.SetAttribute {
	return schema.SetAttribute{
		Description: description,
		ElementType: types.StringType,
		Computed:    true,
	}
}

func (d *indexSettingsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	data, ok := req.ProviderData.(*providerData)
	if !ok || data == nil || data.client == nil {
		resp.Diagnostics.AddError(
			"Invalid provider data",
			"The Meilisearch index settings data source requires provider data created by the Meilisearch provider.",
		)
		return
	}

	d.client = data.client
	d.provider = data
	d.operationTimeout = data.operationTimeout
	if d.operationTimeout <= 0 {
		d.operationTimeout = defaultOperationTimeout
	}
}

func (d *indexSettingsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config indexSettingsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if d.client == nil {
		resp.Diagnostics.AddError("Index settings data source is not configured", "The Meilisearch provider did not supply a client.")
		return
	}

	operationCtx, cancel := operationContext(ctx, d.operationTimeout)
	defer cancel()

	resourceClient := &indexSettingsResource{client: d.client, operationTimeout: d.operationTimeout, provider: d.provider}
	if err := resourceClient.ensureIndex(operationCtx, config.IndexUID.ValueString()); err != nil {
		if isMissingIndexError(err) {
			resp.Diagnostics.AddError("Index settings parent not found", "The requested Meilisearch index does not exist.")
			return
		}
		resp.Diagnostics.AddError("Error checking index settings parent", indexSettingsErrorMessage(err))
		return
	}

	values, err := resourceClient.getSettings(operationCtx, config.IndexUID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading index settings", indexSettingsErrorMessage(err))
		return
	}

	state := indexSettingsDataSourceModel{
		IndexUID:             config.IndexUID,
		SearchableAttributes: values.SearchableAttributes,
		DisplayedAttributes:  values.DisplayedAttributes,
		FilterableAttributes: values.FilterableAttributes,
		SortableAttributes:   values.SortableAttributes,
		RankingRules:         values.RankingRules,
		StopWords:            values.StopWords,
		Synonyms:             values.Synonyms,
		DistinctAttribute:    values.DistinctAttribute,
		ID:                   types.StringValue(config.IndexUID.ValueString()),
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
	if values.FilterableAdvanced {
		resp.Diagnostics.AddError("Unsupported advanced filterable attributes", "The index contains advanced filterable attribute rules that this data source cannot represent.")
	}
}
