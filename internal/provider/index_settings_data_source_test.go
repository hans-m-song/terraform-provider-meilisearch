package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	testresource "github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/meilisearch/meilisearch-go"
)

func configuredIndexSettingsTestDataSource(t *testing.T, fixture *indexSettingsHTTPFixture, timeout time.Duration) *indexSettingsDataSource {
	t.Helper()

	configured, ok := NewIndexSettingsDataSource().(*indexSettingsDataSource)
	if !ok {
		t.Fatal("index settings data source has unexpected type")
	}

	var response datasource.ConfigureResponse
	configured.Configure(context.Background(), datasource.ConfigureRequest{ProviderData: &providerData{
		client:           meilisearch.New(fixture.server.URL, meilisearch.WithAPIKey("synthetic-test-key"), meilisearch.WithCustomClient(fixture.server.Client())),
		operationTimeout: timeout,
		host:             fixture.server.URL,
		apiKey:           "synthetic-test-key",
		httpClient:       fixture.server.Client(),
	}}, &response)
	if response.Diagnostics.HasError() {
		t.Fatalf("index settings data source configure diagnostics=%v", response.Diagnostics)
	}
	return configured
}

func indexSettingsDataSourceTestSchema(t *testing.T) datasource.SchemaResponse {
	t.Helper()

	var response datasource.SchemaResponse
	NewIndexSettingsDataSource().Schema(context.Background(), datasource.SchemaRequest{}, &response)
	if response.Diagnostics.HasError() {
		t.Fatalf("index settings data source schema diagnostics=%v", response.Diagnostics)
	}
	return response
}

func indexSettingsDataSourceTestConfig(t *testing.T, schema datasource.SchemaResponse, uid string) tfsdk.Config {
	t.Helper()

	plan := tfsdk.Plan{Schema: schema.Schema}
	model := indexSettingsDataSourceModel{
		IndexUID:             types.StringValue(uid),
		SearchableAttributes: types.ListNull(types.StringType),
		DisplayedAttributes:  types.ListNull(types.StringType),
		FilterableAttributes: types.SetNull(types.StringType),
		SortableAttributes:   types.SetNull(types.StringType),
		RankingRules:         types.ListNull(types.StringType),
		StopWords:            types.SetNull(types.StringType),
		Synonyms:             types.MapNull(types.ListType{ElemType: types.StringType}),
		DistinctAttribute:    types.StringNull(),
		ID:                   types.StringNull(),
	}
	if diagnostics := plan.Set(context.Background(), &model); diagnostics.HasError() {
		t.Fatalf("index settings data source config diagnostics=%v", diagnostics)
	}
	return tfsdk.Config{Raw: plan.Raw, Schema: schema.Schema}
}

func TestIndexSettingsDataSourceReadsCoreSettings(t *testing.T) {
	fixture := newIndexSettingsHTTPFixture(t, "data-source-index")
	schema := indexSettingsDataSourceTestSchema(t)
	dataSource := configuredIndexSettingsTestDataSource(t, fixture, time.Second)
	config := indexSettingsDataSourceTestConfig(t, schema, fixture.uid)
	response := datasource.ReadResponse{State: tfsdk.State{Schema: schema.Schema}}

	dataSource.Read(context.Background(), datasource.ReadRequest{Config: config}, &response)
	if response.Diagnostics.HasError() {
		t.Fatalf("data source diagnostics=%v", response.Diagnostics)
	}

	var state indexSettingsDataSourceModel
	if diagnostics := response.State.Get(context.Background(), &state); diagnostics.HasError() {
		t.Fatalf("data source state diagnostics=%v", diagnostics)
	}
	if !state.IndexUID.Equal(types.StringValue(fixture.uid)) || !state.ID.Equal(types.StringValue(fixture.uid)) {
		t.Fatalf("data source identity index_uid=%v id=%v", state.IndexUID, state.ID)
	}
	if !state.SearchableAttributes.Equal(indexSettingsTestList("title")) {
		t.Fatalf("searchable_attributes=%v", state.SearchableAttributes)
	}
	if !state.DisplayedAttributes.Equal(indexSettingsTestList("title", "summary")) {
		t.Fatalf("displayed_attributes=%v", state.DisplayedAttributes)
	}
	if !state.FilterableAttributes.Equal(indexSettingsTestSet("category")) {
		t.Fatalf("filterable_attributes=%v", state.FilterableAttributes)
	}
	if !state.SortableAttributes.Equal(indexSettingsTestSet("created_at")) {
		t.Fatalf("sortable_attributes=%v", state.SortableAttributes)
	}
	if !state.RankingRules.Equal(indexSettingsTestList("words")) {
		t.Fatalf("ranking_rules=%v", state.RankingRules)
	}
	if !state.StopWords.Equal(indexSettingsTestSet("the")) {
		t.Fatalf("stop_words=%v", state.StopWords)
	}
	if !state.Synonyms.Equal(indexSettingsTestSynonyms(map[string][]string{"phone": {"telephone"}})) {
		t.Fatalf("synonyms=%v", state.Synonyms)
	}
	if !state.DistinctAttribute.Equal(types.StringValue("group")) {
		t.Fatalf("distinct_attribute=%v", state.DistinctAttribute)
	}
}

func TestIndexSettingsDataSourceMissingParent(t *testing.T) {
	const uid = "missing-data-source-index"
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method == http.MethodGet && request.URL.Path == "/indexes/"+uid {
			writeIndexSettingsTestJSON(writer, http.StatusNotFound, `{"code":"index_not_found"}`)
			return
		}
		writeIndexSettingsTestJSON(writer, http.StatusNotFound, `{"code":"unexpected_route"}`)
	}))
	defer server.Close()

	fixture := &indexSettingsHTTPFixture{server: server, uid: uid}
	schema := indexSettingsDataSourceTestSchema(t)
	dataSource := configuredIndexSettingsTestDataSource(t, fixture, time.Second)
	response := datasource.ReadResponse{State: tfsdk.State{Schema: schema.Schema}}
	dataSource.Read(context.Background(), datasource.ReadRequest{Config: indexSettingsDataSourceTestConfig(t, schema, uid)}, &response)
	assertIndexSettingsDiagnostic(t, response.Diagnostics, "Index settings parent not found")
}

func TestAccIndexSettingsDataSource(t *testing.T) {
	config := providerConfig + `
data "meilisearch_index_settings" "test" {
  index_uid = "test_index"
}
`
	testresource.Test(t, testresource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []testresource.TestStep{
			{
				Config: config,
				Check: testresource.ComposeAggregateTestCheckFunc(
					testresource.TestCheckResourceAttr("data.meilisearch_index_settings.test", "index_uid", "test_index"),
					testresource.TestCheckResourceAttr("data.meilisearch_index_settings.test", "id", "test_index"),
					testresource.TestCheckResourceAttrSet("data.meilisearch_index_settings.test", "searchable_attributes.#"),
					testresource.TestCheckResourceAttrSet("data.meilisearch_index_settings.test", "displayed_attributes.#"),
					testresource.TestCheckResourceAttrSet("data.meilisearch_index_settings.test", "filterable_attributes.#"),
					testresource.TestCheckResourceAttrSet("data.meilisearch_index_settings.test", "sortable_attributes.#"),
					testresource.TestCheckResourceAttrSet("data.meilisearch_index_settings.test", "ranking_rules.#"),
					testresource.TestCheckResourceAttrSet("data.meilisearch_index_settings.test", "stop_words.#"),
					testresource.TestCheckResourceAttrSet("data.meilisearch_index_settings.test", "synonyms.%"),
				),
			},
			{
				Config: config,
				ConfigPlanChecks: testresource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectEmptyPlan(),
				}},
			},
		},
	})
}
