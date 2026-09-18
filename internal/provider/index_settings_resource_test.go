package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	testresource "github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/meilisearch/meilisearch-go"
)

type indexSettingsHTTPFixture struct {
	server          *httptest.Server
	uid             string
	mu              sync.Mutex
	settings        map[string]json.RawMessage
	patches         []map[string]json.RawMessage
	indexGets       int
	deleteRequests  int
	advanced        bool
	acceptedTimeout bool
	taskTimeout     bool
	taskUID         int64
}

func newIndexSettingsHTTPFixture(t *testing.T, uid string) *indexSettingsHTTPFixture {
	t.Helper()

	fixture := &indexSettingsHTTPFixture{
		uid:     uid,
		taskUID: 100,
		settings: map[string]json.RawMessage{
			"searchableAttributes": json.RawMessage(`["title"]`),
			"displayedAttributes":  json.RawMessage(`["title","summary"]`),
			"filterableAttributes": json.RawMessage(`["category"]`),
			"sortableAttributes":   json.RawMessage(`["created_at"]`),
			"rankingRules":         json.RawMessage(`["words"]`),
			"stopWords":            json.RawMessage(`["the"]`),
			"synonyms":             json.RawMessage(`{"phone":["telephone"]}`),
			"distinctAttribute":    json.RawMessage(`"group"`),
		},
	}
	fixture.server = httptest.NewServer(http.HandlerFunc(fixture.handle))
	t.Cleanup(fixture.server.Close)
	return fixture
}

func (f *indexSettingsHTTPFixture) handle(writer http.ResponseWriter, request *http.Request) {
	if request.Method == http.MethodGet && request.URL.Path == "/indexes/"+f.uid {
		f.mu.Lock()
		f.indexGets++
		f.mu.Unlock()
		writeIndexSettingsTestJSON(writer, http.StatusOK, fmt.Sprintf(`{"uid":%q,"primaryKey":"id"}`, f.uid))
		return
	}

	if request.Method == http.MethodGet && request.URL.Path == "/indexes/"+f.uid+"/settings" {
		f.mu.Lock()
		defer f.mu.Unlock()
		body := make(map[string]json.RawMessage, len(f.settings))
		for key, value := range f.settings {
			body[key] = append(json.RawMessage(nil), value...)
		}
		if f.advanced {
			body["filterableAttributes"] = json.RawMessage(`[{"attributePatterns":["*"],"features":{"facetSearch":true,"filter":{"equality":true,"comparison":true}}}]`)
		}
		writeIndexSettingsTestValue(writer, http.StatusOK, body)
		return
	}

	if request.Method == http.MethodPatch && request.URL.Path == "/indexes/"+f.uid+"/settings" {
		if f.acceptedTimeout {
			writer.WriteHeader(http.StatusAccepted)
			_, _ = writer.Write([]byte(`{"taskUid":101}`))
			if flusher, ok := writer.(http.Flusher); ok {
				flusher.Flush()
			}
			<-request.Context().Done()
			return
		}

		var patch map[string]json.RawMessage
		if err := json.NewDecoder(request.Body).Decode(&patch); err != nil {
			writeIndexSettingsTestJSON(writer, http.StatusBadRequest, `{"code":"invalid_request"}`)
			return
		}
		f.mu.Lock()
		f.taskUID++
		copyPatch := make(map[string]json.RawMessage, len(patch))
		for key, value := range patch {
			copyPatch[key] = append(json.RawMessage(nil), value...)
			if strings.TrimSpace(string(value)) == "null" {
				f.settings[key] = indexSettingsTestDefault(key)
				continue
			}
			f.settings[key] = append(json.RawMessage(nil), value...)
		}
		f.patches = append(f.patches, copyPatch)
		taskUID := f.taskUID
		f.mu.Unlock()
		writeIndexSettingsTestJSON(writer, http.StatusAccepted, fmt.Sprintf(`{"taskUid":%d}`, taskUID))
		return
	}

	if request.Method == http.MethodGet && strings.HasPrefix(request.URL.Path, "/tasks/") {
		if f.taskTimeout {
			<-request.Context().Done()
			return
		}
		writeIndexSettingsTestJSON(writer, http.StatusOK, `{"uid":101,"status":"succeeded"}`)
		return
	}

	if request.Method == http.MethodDelete {
		f.mu.Lock()
		f.deleteRequests++
		f.mu.Unlock()
	}
	writeIndexSettingsTestJSON(writer, http.StatusNotFound, `{"code":"unexpected_route"}`)
}

func indexSettingsTestDefault(field string) json.RawMessage {
	switch field {
	case "distinctAttribute":
		return json.RawMessage(`null`)
	case "synonyms":
		return json.RawMessage(`{}`)
	default:
		return json.RawMessage(`[]`)
	}
}

func (f *indexSettingsHTTPFixture) patchSnapshot() []map[string]json.RawMessage {
	f.mu.Lock()
	defer f.mu.Unlock()
	patches := make([]map[string]json.RawMessage, len(f.patches))
	for index, patch := range f.patches {
		patches[index] = make(map[string]json.RawMessage, len(patch))
		for key, value := range patch {
			patches[index][key] = append(json.RawMessage(nil), value...)
		}
	}
	return patches
}

func (f *indexSettingsHTTPFixture) setSetting(field string, value json.RawMessage) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.settings[field] = append(json.RawMessage(nil), value...)
}

func (f *indexSettingsHTTPFixture) setting(field string) json.RawMessage {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append(json.RawMessage(nil), f.settings[field]...)
}

func writeIndexSettingsTestJSON(writer http.ResponseWriter, status int, body string) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_, _ = writer.Write([]byte(body))
}

func writeIndexSettingsTestValue(writer http.ResponseWriter, status int, value interface{}) {
	body, err := json.Marshal(value)
	if err != nil {
		writeIndexSettingsTestJSON(writer, http.StatusInternalServerError, `{"code":"marshal_failure"}`)
		return
	}
	writeIndexSettingsTestJSON(writer, status, string(body))
}

func configuredIndexSettingsTestResource(t *testing.T, fixture *indexSettingsHTTPFixture, timeout time.Duration) *indexSettingsResource {
	t.Helper()

	configured, ok := NewIndexSettingsResource().(*indexSettingsResource)
	if !ok {
		t.Fatal("index settings resource has unexpected type")
	}

	var response resource.ConfigureResponse
	configured.Configure(context.Background(), resource.ConfigureRequest{ProviderData: &providerData{
		client:           meilisearch.New(fixture.server.URL, meilisearch.WithAPIKey("synthetic-test-key"), meilisearch.WithCustomClient(fixture.server.Client())),
		operationTimeout: timeout,
		host:             fixture.server.URL,
		apiKey:           "synthetic-test-key",
		httpClient:       fixture.server.Client(),
	}}, &response)
	if response.Diagnostics.HasError() {
		t.Fatalf("index settings configure diagnostics=%v", response.Diagnostics)
	}
	return configured
}

func indexSettingsResourceTestSchema(t *testing.T) resource.SchemaResponse {
	t.Helper()

	var response resource.SchemaResponse
	NewIndexSettingsResource().Schema(context.Background(), resource.SchemaRequest{}, &response)
	if response.Diagnostics.HasError() {
		t.Fatalf("index settings schema diagnostics=%v", response.Diagnostics)
	}
	return response
}

func indexSettingsTestModel(uid string, managed []string) indexSettingsResourceModel {
	managedFields := types.SetNull(types.StringType)
	if managed != nil {
		managedFields = indexSettingsFieldSet(managed)
	}
	return indexSettingsResourceModel{
		IndexUID:             types.StringValue(uid),
		SearchableAttributes: types.ListNull(types.StringType),
		DisplayedAttributes:  types.ListNull(types.StringType),
		FilterableAttributes: types.SetNull(types.StringType),
		SortableAttributes:   types.SetNull(types.StringType),
		RankingRules:         types.ListNull(types.StringType),
		StopWords:            types.SetNull(types.StringType),
		Synonyms:             types.MapNull(types.ListType{ElemType: types.StringType}),
		DistinctAttribute:    types.StringNull(),
		ManagedFields:        managedFields,
		ID:                   types.StringValue(uid),
	}
}

func indexSettingsTestPlan(t *testing.T, schema resource.SchemaResponse, model indexSettingsResourceModel) tfsdk.Plan {
	t.Helper()

	plan := tfsdk.Plan{Schema: schema.Schema}
	if diagnostics := plan.Set(context.Background(), &model); diagnostics.HasError() {
		t.Fatalf("index settings plan diagnostics=%v", diagnostics)
	}
	return plan
}

func indexSettingsTestState(t *testing.T, schema resource.SchemaResponse, model indexSettingsResourceModel) tfsdk.State {
	t.Helper()

	state := tfsdk.State{Schema: schema.Schema}
	if diagnostics := state.Set(context.Background(), &model); diagnostics.HasError() {
		t.Fatalf("index settings state diagnostics=%v", diagnostics)
	}
	return state
}

func indexSettingsTestList(values ...string) types.List {
	elements := make([]attr.Value, 0, len(values))
	for _, value := range values {
		elements = append(elements, types.StringValue(value))
	}
	return types.ListValueMust(types.StringType, elements)
}

func indexSettingsTestSet(values ...string) types.Set {
	elements := make([]attr.Value, 0, len(values))
	for _, value := range values {
		elements = append(elements, types.StringValue(value))
	}
	return types.SetValueMust(types.StringType, elements)
}

func indexSettingsTestSynonyms(values map[string][]string) types.Map {
	converted := make(map[string]attrValueList, len(values))
	for key, synonyms := range values {
		converted[key] = attrValueList(synonyms)
	}
	result, err := synonymsMapValue(converted)
	if err != nil {
		panic(err)
	}
	return result
}

func indexSettingsStateModel(t *testing.T, state tfsdk.State) indexSettingsResourceModel {
	t.Helper()

	var model indexSettingsResourceModel
	if diagnostics := state.Get(context.Background(), &model); diagnostics.HasError() {
		t.Fatalf("index settings state diagnostics=%v", diagnostics)
	}
	return model
}

func assertIndexSettingsDiagnostic(t *testing.T, diagnostics interface{ HasError() bool }, substring string) {
	t.Helper()
	if !diagnostics.HasError() || !strings.Contains(fmt.Sprint(diagnostics), substring) {
		t.Fatalf("diagnostics=%v, expected error containing %q", diagnostics, substring)
	}
}

func TestIndexSettingsResourceOwnsOnlyConfiguredFieldsThroughLifecycle(t *testing.T) {
	fixture := newIndexSettingsHTTPFixture(t, "lifecycle-index")
	schema := indexSettingsResourceTestSchema(t)
	settings := configuredIndexSettingsTestResource(t, fixture, time.Second)

	createModel := indexSettingsTestModel(fixture.uid, nil)
	createModel.SearchableAttributes = indexSettingsTestList("title", "summary")
	createModel.FilterableAttributes = indexSettingsTestSet("category")
	createModel.StopWords = indexSettingsTestSet("the")
	createPlan := indexSettingsTestPlan(t, schema, createModel)
	createResponse := resource.CreateResponse{State: tfsdk.State{Schema: schema.Schema}}
	settings.Create(context.Background(), resource.CreateRequest{Plan: createPlan}, &createResponse)
	if createResponse.Diagnostics.HasError() {
		t.Fatalf("create diagnostics=%v", createResponse.Diagnostics)
	}

	created := indexSettingsStateModel(t, createResponse.State)
	if !created.ManagedFields.Equal(indexSettingsFieldSet([]string{settingsFieldSearchable, settingsFieldFilterable, settingsFieldStopWords})) {
		t.Fatalf("created ownership=%v", created.ManagedFields)
	}
	patches := fixture.patchSnapshot()
	if len(patches) != 1 {
		t.Fatalf("patch count=%d, expected one create patch", len(patches))
	}
	for _, key := range []string{"searchableAttributes", "filterableAttributes", "stopWords"} {
		if _, ok := patches[0][key]; !ok {
			t.Fatalf("create patch missing %q: %v", key, patches[0])
		}
	}
	for _, key := range []string{"displayedAttributes", "sortableAttributes", "rankingRules", "synonyms", "distinctAttribute"} {
		if _, ok := patches[0][key]; ok {
			t.Fatalf("create patch unexpectedly changed unowned %q: %v", key, patches[0])
		}
	}

	updateModel := indexSettingsTestModel(fixture.uid, nil)
	updateModel.SearchableAttributes = indexSettingsTestList("title", "body")
	updateModel.FilterableAttributes = indexSettingsTestSet("category", "brand")
	updatePlan := indexSettingsTestPlan(t, schema, updateModel)
	updateResponse := resource.UpdateResponse{State: createResponse.State}
	settings.Update(context.Background(), resource.UpdateRequest{Plan: updatePlan, State: createResponse.State}, &updateResponse)
	if updateResponse.Diagnostics.HasError() {
		t.Fatalf("update diagnostics=%v", updateResponse.Diagnostics)
	}

	updated := indexSettingsStateModel(t, updateResponse.State)
	if !updated.StopWords.IsNull() {
		t.Fatalf("stop_words=%v, expected null after ownership removal", updated.StopWords)
	}
	if !updated.ManagedFields.Equal(indexSettingsFieldSet([]string{settingsFieldSearchable, settingsFieldFilterable})) {
		t.Fatalf("updated ownership=%v", updated.ManagedFields)
	}
	patches = fixture.patchSnapshot()
	if len(patches) != 2 || string(patches[1]["stopWords"]) != "null" {
		t.Fatalf("update patches=%v, expected explicit stopWords null reset", patches)
	}
	if string(fixture.setting("displayedAttributes")) != `["title","summary"]` || string(fixture.setting("sortableAttributes")) != `["created_at"]` {
		t.Fatalf("unowned settings changed after update: displayed=%s sortable=%s", fixture.setting("displayedAttributes"), fixture.setting("sortableAttributes"))
	}

	deleteResponse := resource.DeleteResponse{State: updateResponse.State}
	settings.Delete(context.Background(), resource.DeleteRequest{State: updateResponse.State}, &deleteResponse)
	if deleteResponse.Diagnostics.HasError() {
		t.Fatalf("delete diagnostics=%v", deleteResponse.Diagnostics)
	}

	patches = fixture.patchSnapshot()
	if len(patches) != 3 {
		t.Fatalf("patch count=%d, expected create/update/delete", len(patches))
	}
	for _, key := range []string{"searchableAttributes", "filterableAttributes"} {
		if string(patches[2][key]) != "null" {
			t.Fatalf("destroy patch[%q]=%s, expected null", key, patches[2][key])
		}
	}
	for _, key := range []string{"stopWords", "displayedAttributes", "sortableAttributes", "rankingRules", "synonyms", "distinctAttribute"} {
		if _, ok := patches[2][key]; ok {
			t.Fatalf("destroy patch unexpectedly changed %q: %v", key, patches[2])
		}
	}
	if string(fixture.setting("displayedAttributes")) != `["title","summary"]` || string(fixture.setting("sortableAttributes")) != `["created_at"]` {
		t.Fatalf("unowned settings changed after destroy: displayed=%s sortable=%s", fixture.setting("displayedAttributes"), fixture.setting("sortableAttributes"))
	}
	fixture.mu.Lock()
	indexGets := fixture.indexGets
	deleteRequests := fixture.deleteRequests
	fixture.mu.Unlock()
	if indexGets == 0 || deleteRequests != 0 {
		t.Fatalf("index gets=%d delete requests=%d, expected parent preservation", indexGets, deleteRequests)
	}
}

func TestIndexSettingsOwnedDriftReadThenUpdatePreservesUnowned(t *testing.T) {
	fixture := newIndexSettingsHTTPFixture(t, "drift-index")
	schema := indexSettingsResourceTestSchema(t)
	settings := configuredIndexSettingsTestResource(t, fixture, time.Second)

	prior := indexSettingsTestModel(fixture.uid, []string{settingsFieldFilterable})
	prior.FilterableAttributes = indexSettingsTestSet("category")
	state := indexSettingsTestState(t, schema, prior)
	fixture.setSetting("filterableAttributes", json.RawMessage(`["drifted"]`))
	fixture.setSetting("displayedAttributes", json.RawMessage(`["out_of_band"]`))

	readResponse := resource.ReadResponse{State: state}
	settings.Read(context.Background(), resource.ReadRequest{State: state}, &readResponse)
	if readResponse.Diagnostics.HasError() {
		t.Fatalf("drift read diagnostics=%v", readResponse.Diagnostics)
	}
	drifted := indexSettingsStateModel(t, readResponse.State)
	if !drifted.FilterableAttributes.Equal(indexSettingsTestSet("drifted")) {
		t.Fatalf("read filterable_attributes=%v, expected remote drift", drifted.FilterableAttributes)
	}

	planModel := indexSettingsTestModel(fixture.uid, nil)
	planModel.FilterableAttributes = indexSettingsTestSet("category")
	plan := indexSettingsTestPlan(t, schema, planModel)
	updateResponse := resource.UpdateResponse{State: readResponse.State}
	settings.Update(context.Background(), resource.UpdateRequest{Plan: plan, State: readResponse.State}, &updateResponse)
	if updateResponse.Diagnostics.HasError() {
		t.Fatalf("drift repair diagnostics=%v", updateResponse.Diagnostics)
	}

	patches := fixture.patchSnapshot()
	if len(patches) != 1 || string(patches[0]["filterableAttributes"]) != `["category"]` {
		t.Fatalf("drift repair patches=%v, expected filterable repair", patches)
	}
	if _, ok := patches[0]["displayedAttributes"]; ok {
		t.Fatalf("drift repair unexpectedly changed unowned displayed_attributes: %v", patches[0])
	}
	if string(fixture.setting("displayedAttributes")) != `["out_of_band"]` {
		t.Fatalf("unowned displayed_attributes=%s, expected out-of-band value", fixture.setting("displayedAttributes"))
	}
	repaired := indexSettingsStateModel(t, updateResponse.State)
	if !repaired.FilterableAttributes.Equal(indexSettingsTestSet("category")) {
		t.Fatalf("repaired filterable_attributes=%v, expected configured value", repaired.FilterableAttributes)
	}
}

func TestIndexSettingsNoopUpdateRetainsConfiguredStopWordSpelling(t *testing.T) {
	fixture := newIndexSettingsHTTPFixture(t, "normalization-index")
	fixture.setSetting("stopWords", json.RawMessage(`["CAFE\u0301"]`))
	schema := indexSettingsResourceTestSchema(t)
	settings := configuredIndexSettingsTestResource(t, fixture, time.Second)

	prior := indexSettingsTestModel(fixture.uid, []string{settingsFieldStopWords})
	prior.StopWords = indexSettingsTestSet("CAFÉ")
	state := indexSettingsTestState(t, schema, prior)
	planModel := indexSettingsTestModel(fixture.uid, nil)
	planModel.StopWords = indexSettingsTestSet("CAFÉ")
	plan := indexSettingsTestPlan(t, schema, planModel)
	response := resource.UpdateResponse{State: state}
	settings.Update(context.Background(), resource.UpdateRequest{Plan: plan, State: state}, &response)
	if response.Diagnostics.HasError() {
		t.Fatalf("no-op update diagnostics=%v", response.Diagnostics)
	}
	if len(fixture.patchSnapshot()) != 0 {
		t.Fatalf("no-op update emitted patches=%v", fixture.patchSnapshot())
	}
	updated := indexSettingsStateModel(t, response.State)
	if !updated.StopWords.Equal(indexSettingsTestSet("CAFÉ")) {
		t.Fatalf("stop_words=%v, expected configured spelling", updated.StopWords)
	}
}

func TestIndexSettingsImportAdoptsAllCoreFields(t *testing.T) {
	fixture := newIndexSettingsHTTPFixture(t, "import-index")
	fixture.setSetting("distinctAttribute", json.RawMessage(`null`))
	schema := indexSettingsResourceTestSchema(t)
	settings := configuredIndexSettingsTestResource(t, fixture, time.Second)

	response := resource.ImportStateResponse{State: indexSettingsTestState(t, schema, indexSettingsTestModel(fixture.uid, nil))}
	settings.ImportState(context.Background(), resource.ImportStateRequest{ID: fixture.uid}, &response)
	if response.Diagnostics.HasError() {
		t.Fatalf("import diagnostics=%v", response.Diagnostics)
	}

	imported := indexSettingsStateModel(t, response.State)
	if !imported.ManagedFields.Equal(indexSettingsFieldSet(indexSettingsFields)) {
		t.Fatalf("imported ownership=%v, expected all core fields", imported.ManagedFields)
	}
	readResponse := resource.ReadResponse{State: response.State}
	settings.Read(context.Background(), resource.ReadRequest{State: response.State}, &readResponse)
	if readResponse.Diagnostics.HasError() {
		t.Fatalf("import refresh diagnostics=%v", readResponse.Diagnostics)
	}
	refreshed := indexSettingsStateModel(t, readResponse.State)
	for _, field := range indexSettingsFields {
		if field == settingsFieldDistinct {
			if !refreshed.DistinctAttribute.IsNull() {
				t.Fatalf("imported distinct_attribute=%v, expected null remote value", refreshed.DistinctAttribute)
			}
			continue
		}
		if indexSettingsModelField(refreshed, field).IsNull() {
			t.Fatalf("imported field %q remained null", field)
		}
	}
}

func TestIndexSettingsUnknownOwnershipFailsForPlanUpdateAndDestroy(t *testing.T) {
	fixture := newIndexSettingsHTTPFixture(t, "unknown-index")
	schema := indexSettingsResourceTestSchema(t)
	settings := configuredIndexSettingsTestResource(t, fixture, time.Second)

	unknownModel := indexSettingsTestModel(fixture.uid, nil)
	unknownModel.SearchableAttributes = types.ListUnknown(types.StringType)
	unknownPlan := indexSettingsTestPlan(t, schema, unknownModel)
	createResponse := resource.CreateResponse{State: tfsdk.State{Schema: schema.Schema}}
	settings.Create(context.Background(), resource.CreateRequest{Plan: unknownPlan}, &createResponse)
	assertIndexSettingsDiagnostic(t, createResponse.Diagnostics, "Unknown index settings ownership")

	config := tfsdk.Config{Raw: unknownPlan.Raw, Schema: schema.Schema}
	modifyResponse := resource.ModifyPlanResponse{Plan: unknownPlan}
	settings.ModifyPlan(context.Background(), resource.ModifyPlanRequest{Config: config, Plan: unknownPlan}, &modifyResponse)
	if modifyResponse.Diagnostics.HasError() {
		t.Fatalf("unknown plan modify diagnostics=%v", modifyResponse.Diagnostics)
	}
	var managed types.Set
	if diagnostics := modifyResponse.Plan.GetAttribute(context.Background(), path.Root("managed_fields"), &managed); diagnostics.HasError() || !managed.IsUnknown() {
		t.Fatalf("managed_fields=%v diagnostics=%v, expected unknown", managed, diagnostics)
	}

	knownStateModel := indexSettingsTestModel(fixture.uid, []string{settingsFieldSearchable})
	knownStateModel.SearchableAttributes = indexSettingsTestList("title")
	knownState := indexSettingsTestState(t, schema, knownStateModel)
	updateResponse := resource.UpdateResponse{State: knownState}
	settings.Update(context.Background(), resource.UpdateRequest{Plan: unknownPlan, State: knownState}, &updateResponse)
	assertIndexSettingsDiagnostic(t, updateResponse.Diagnostics, "Unknown index settings ownership")

	unknownStateModel := indexSettingsTestModel(fixture.uid, nil)
	unknownStateModel.ManagedFields = types.SetUnknown(types.StringType)
	unknownState := indexSettingsTestState(t, schema, unknownStateModel)
	deleteResponse := resource.DeleteResponse{State: unknownState}
	settings.Delete(context.Background(), resource.DeleteRequest{State: unknownState}, &deleteResponse)
	assertIndexSettingsDiagnostic(t, deleteResponse.Diagnostics, "Unknown index settings ownership")

	nullPlan := unknownPlan
	nullPlan.Raw = tftypes.NewValue(nullPlan.Raw.Type(), nil)
	nullModifyResponse := resource.ModifyPlanResponse{Plan: nullPlan}
	settings.ModifyPlan(context.Background(), resource.ModifyPlanRequest{Plan: nullPlan}, &nullModifyResponse)
	if nullModifyResponse.Diagnostics.HasError() || !nullModifyResponse.Plan.Raw.IsNull() {
		t.Fatalf("null plan diagnostics=%v plan=%v, expected unchanged null plan", nullModifyResponse.Diagnostics, nullModifyResponse.Plan.Raw)
	}

	fixture.mu.Lock()
	indexGets := fixture.indexGets
	fixture.mu.Unlock()
	if indexGets != 0 || len(fixture.patchSnapshot()) != 0 {
		t.Fatalf("unknown ownership made remote requests: index gets=%d patches=%v", indexGets, fixture.patchSnapshot())
	}
}

func TestIndexSettingsMissingParentLifecycleBehavior(t *testing.T) {
	const uid = "missing-index"
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method == http.MethodGet && request.URL.Path == "/indexes/"+uid {
			writeIndexSettingsTestJSON(writer, http.StatusNotFound, `{"code":"index_not_found"}`)
			return
		}
		writeIndexSettingsTestJSON(writer, http.StatusNotFound, `{"code":"unexpected_route"}`)
	}))
	defer server.Close()

	fixture := &indexSettingsHTTPFixture{server: server, uid: uid}
	schema := indexSettingsResourceTestSchema(t)
	settings := configuredIndexSettingsTestResource(t, fixture, time.Second)
	model := indexSettingsTestModel(uid, []string{settingsFieldSearchable})
	model.SearchableAttributes = indexSettingsTestList("title")
	plan := indexSettingsTestPlan(t, schema, model)

	createResponse := resource.CreateResponse{State: tfsdk.State{Schema: schema.Schema}}
	settings.Create(context.Background(), resource.CreateRequest{Plan: plan}, &createResponse)
	assertIndexSettingsDiagnostic(t, createResponse.Diagnostics, "Index settings parent not found")

	readState := indexSettingsTestState(t, schema, model)
	readResponse := resource.ReadResponse{State: readState}
	settings.Read(context.Background(), resource.ReadRequest{State: readState}, &readResponse)
	if readResponse.Diagnostics.HasError() || !readResponse.State.Raw.IsNull() {
		t.Fatalf("read diagnostics=%v state=%v, expected idempotent state removal", readResponse.Diagnostics, readResponse.State.Raw)
	}

	updateResponse := resource.UpdateResponse{State: readState}
	settings.Update(context.Background(), resource.UpdateRequest{Plan: plan, State: readState}, &updateResponse)
	assertIndexSettingsDiagnostic(t, updateResponse.Diagnostics, "Index settings parent not found")

	deleteResponse := resource.DeleteResponse{State: readState}
	settings.Delete(context.Background(), resource.DeleteRequest{State: readState}, &deleteResponse)
	if deleteResponse.Diagnostics.HasError() {
		t.Fatalf("delete diagnostics=%v, expected missing parent to be idempotent", deleteResponse.Diagnostics)
	}
}

func TestIndexSettingsAcceptedTimeoutRetainsIdentityAndOwnershipUnion(t *testing.T) {
	fixture := newIndexSettingsHTTPFixture(t, "accepted-index")
	fixture.taskTimeout = true
	schema := indexSettingsResourceTestSchema(t)
	settings := configuredIndexSettingsTestResource(t, fixture, 30*time.Millisecond)

	prior := indexSettingsTestModel(fixture.uid, []string{settingsFieldSearchable})
	prior.SearchableAttributes = indexSettingsTestList("title")
	state := indexSettingsTestState(t, schema, prior)
	planModel := indexSettingsTestModel(fixture.uid, nil)
	planModel.FilterableAttributes = indexSettingsTestSet("category")
	plan := indexSettingsTestPlan(t, schema, planModel)
	response := resource.UpdateResponse{State: state}
	settings.Update(context.Background(), resource.UpdateRequest{Plan: plan, State: state}, &response)
	assertIndexSettingsDiagnostic(t, response.Diagnostics, "operation deadline exceeded")

	retained := indexSettingsStateModel(t, response.State)
	if !retained.IndexUID.Equal(types.StringValue(fixture.uid)) || !retained.ID.Equal(types.StringValue(fixture.uid)) {
		t.Fatalf("retained identity index_uid=%v id=%v", retained.IndexUID, retained.ID)
	}
	if !retained.ManagedFields.Equal(indexSettingsFieldSet([]string{settingsFieldSearchable, settingsFieldFilterable})) {
		t.Fatalf("retained ownership=%v, expected desired/prior union", retained.ManagedFields)
	}
	if !retained.SearchableAttributes.Equal(prior.SearchableAttributes) || !retained.FilterableAttributes.Equal(planModel.FilterableAttributes) {
		t.Fatalf("retained pending values searchable=%v filterable=%v", retained.SearchableAttributes, retained.FilterableAttributes)
	}
}

func TestIndexSettingsAcceptedResponseTimeoutRetainsIdentity(t *testing.T) {
	fixture := newIndexSettingsHTTPFixture(t, "accepted-response-index")
	fixture.acceptedTimeout = true
	schema := indexSettingsResourceTestSchema(t)
	settings := configuredIndexSettingsTestResource(t, fixture, 30*time.Millisecond)

	prior := indexSettingsTestModel(fixture.uid, []string{settingsFieldSearchable})
	prior.SearchableAttributes = indexSettingsTestList("title")
	state := indexSettingsTestState(t, schema, prior)
	planModel := indexSettingsTestModel(fixture.uid, nil)
	planModel.FilterableAttributes = indexSettingsTestSet("category")
	plan := indexSettingsTestPlan(t, schema, planModel)
	response := resource.UpdateResponse{State: state}
	settings.Update(context.Background(), resource.UpdateRequest{Plan: plan, State: state}, &response)
	assertIndexSettingsDiagnostic(t, response.Diagnostics, "could not read the accepted settings task response")

	retained := indexSettingsStateModel(t, response.State)
	if !retained.IndexUID.Equal(types.StringValue(fixture.uid)) || !retained.ID.Equal(types.StringValue(fixture.uid)) {
		t.Fatalf("retained identity index_uid=%v id=%v", retained.IndexUID, retained.ID)
	}
	if !retained.ManagedFields.Equal(indexSettingsFieldSet([]string{settingsFieldSearchable, settingsFieldFilterable})) {
		t.Fatalf("retained ownership=%v, expected desired/prior union", retained.ManagedFields)
	}
}

func TestIndexSettingsUnsupportedAdvancedOwnedFieldRetainsOwnership(t *testing.T) {
	fixture := newIndexSettingsHTTPFixture(t, "advanced-index")
	fixture.advanced = true
	schema := indexSettingsResourceTestSchema(t)
	settings := configuredIndexSettingsTestResource(t, fixture, time.Second)

	prior := indexSettingsTestModel(fixture.uid, []string{settingsFieldFilterable})
	prior.FilterableAttributes = indexSettingsTestSet("configured_filter")
	state := indexSettingsTestState(t, schema, prior)
	response := resource.ReadResponse{State: state}
	settings.Read(context.Background(), resource.ReadRequest{State: state}, &response)
	assertIndexSettingsDiagnostic(t, response.Diagnostics, "Unsupported advanced filterable attributes")

	retained := indexSettingsStateModel(t, response.State)
	if !retained.FilterableAttributes.Equal(prior.FilterableAttributes) {
		t.Fatalf("filterable_attributes=%v, expected prior owned value", retained.FilterableAttributes)
	}
	if !retained.ManagedFields.Equal(prior.ManagedFields) {
		t.Fatalf("managed_fields=%v, expected ownership retention", retained.ManagedFields)
	}
	if !retained.SortableAttributes.IsNull() {
		t.Fatalf("unowned sortable_attributes=%v, expected null state", retained.SortableAttributes)
	}
	if string(fixture.setting("sortableAttributes")) != `["created_at"]` {
		t.Fatalf("unowned remote sortable_attributes=%s, expected preserved value", fixture.setting("sortableAttributes"))
	}
	if len(fixture.patchSnapshot()) != 0 {
		t.Fatalf("unsupported read emitted patches=%v", fixture.patchSnapshot())
	}
}

func TestIndexSettingsAdvancedFilterableUpdateAndDeletePreserveOwnership(t *testing.T) {
	fixture := newIndexSettingsHTTPFixture(t, "advanced-mutation-index")
	fixture.advanced = true
	schema := indexSettingsResourceTestSchema(t)
	settings := configuredIndexSettingsTestResource(t, fixture, time.Second)

	prior := indexSettingsTestModel(fixture.uid, []string{settingsFieldFilterable})
	prior.FilterableAttributes = indexSettingsTestSet("configured_filter")
	state := indexSettingsTestState(t, schema, prior)
	plan := indexSettingsTestPlan(t, schema, indexSettingsTestModel(fixture.uid, nil))

	updateResponse := resource.UpdateResponse{State: state}
	settings.Update(context.Background(), resource.UpdateRequest{Plan: plan, State: state}, &updateResponse)
	assertIndexSettingsDiagnostic(t, updateResponse.Diagnostics, "Unsupported advanced filterable attributes")
	if len(fixture.patchSnapshot()) != 0 {
		t.Fatalf("advanced update emitted patches=%v", fixture.patchSnapshot())
	}
	retained := indexSettingsStateModel(t, updateResponse.State)
	if !retained.FilterableAttributes.Equal(prior.FilterableAttributes) || !retained.ManagedFields.Equal(prior.ManagedFields) {
		t.Fatalf("advanced update state filterable=%v managed_fields=%v, expected prior ownership/value", retained.FilterableAttributes, retained.ManagedFields)
	}

	deleteResponse := resource.DeleteResponse{State: updateResponse.State}
	settings.Delete(context.Background(), resource.DeleteRequest{State: updateResponse.State}, &deleteResponse)
	assertIndexSettingsDiagnostic(t, deleteResponse.Diagnostics, "Unsupported advanced filterable attributes")
	if len(fixture.patchSnapshot()) != 0 {
		t.Fatalf("advanced delete emitted patches=%v", fixture.patchSnapshot())
	}
	retainedAfterDelete := indexSettingsStateModel(t, deleteResponse.State)
	if !retainedAfterDelete.FilterableAttributes.Equal(prior.FilterableAttributes) || !retainedAfterDelete.ManagedFields.Equal(prior.ManagedFields) {
		t.Fatalf("advanced delete state filterable=%v managed_fields=%v, expected prior ownership/value", retainedAfterDelete.FilterableAttributes, retainedAfterDelete.ManagedFields)
	}
}

func TestIndexSettingsAdvancedFilterableCreateRejectsTypedMutation(t *testing.T) {
	fixture := newIndexSettingsHTTPFixture(t, "advanced-create-index")
	fixture.advanced = true
	schema := indexSettingsResourceTestSchema(t)
	settings := configuredIndexSettingsTestResource(t, fixture, time.Second)

	model := indexSettingsTestModel(fixture.uid, nil)
	model.FilterableAttributes = indexSettingsTestSet("configured_filter")
	plan := indexSettingsTestPlan(t, schema, model)
	response := resource.CreateResponse{State: tfsdk.State{Schema: schema.Schema}}
	settings.Create(context.Background(), resource.CreateRequest{Plan: plan}, &response)
	assertIndexSettingsDiagnostic(t, response.Diagnostics, "Unsupported advanced filterable attributes")
	if len(fixture.patchSnapshot()) != 0 {
		t.Fatalf("advanced create emitted patches=%v", fixture.patchSnapshot())
	}
}

func TestAccIndexSettingsResourceLifecyclePreservesIndexAndDocuments(t *testing.T) {
	const uid = "settings-lifecycle-acceptance"
	const stopWordsConfig = `["the", "CAFÉ", "ك", "۱۲۳", "\u0002foo", "Å"]`
	client := meilisearch.New(testHost, meilisearch.WithAPIKey(testAPIKey))
	expectedDisplayed := []string{"title", "summary"}

	testresource.Test(t, testresource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		PreCheck: func() {
			client = prepareIndexSettingsAcceptanceIndex(t, uid)
			seedSettingsAcceptanceUnownedFields(t, client, uid)
		},
		CheckDestroy: testresource.TestCheckFunc(func(_ *terraform.State) error {
			index, err := client.GetIndex(uid)
			if err != nil {
				return fmt.Errorf("settings resource destroyed parent index: %w", err)
			}
			if index == nil {
				return fmt.Errorf("settings resource destroyed parent index")
			}
			stats, err := client.Index(uid).GetStats(nil)
			if err != nil {
				return fmt.Errorf("read preserved document stats: %w", err)
			}
			if stats.NumberOfDocuments != 1 {
				return fmt.Errorf("document count=%d, expected one preserved document", stats.NumberOfDocuments)
			}
			settings, err := client.Index(uid).GetSettings()
			if err != nil {
				return fmt.Errorf("read settings after destroy: %w", err)
			}
			if !reflect.DeepEqual(settings.DisplayedAttributes, expectedDisplayed) || !sameIndexSettingsStringSet(settings.SortableAttributes, []string{"created_at"}) {
				return fmt.Errorf("unowned settings changed after destroy: displayed=%v sortable=%v", settings.DisplayedAttributes, settings.SortableAttributes)
			}
			if len(settings.FilterableAttributes) != 0 || len(settings.StopWords) != 0 {
				return fmt.Errorf("owned settings were not reset after destroy: filterable=%v stop_words=%v", settings.FilterableAttributes, settings.StopWords)
			}
			if !reflect.DeepEqual(settings.RankingRules, []string{"words"}) || !reflect.DeepEqual(settings.Synonyms, map[string][]string{"phone": {"telephone"}}) || settings.DistinctAttribute == nil || *settings.DistinctAttribute != "group" {
				return fmt.Errorf("unowned ranking/synonym/distinct settings changed: ranking=%v synonyms=%v distinct=%v", settings.RankingRules, settings.Synonyms, settings.DistinctAttribute)
			}
			return nil
		}),
		Steps: []testresource.TestStep{
			{
				Config: providerConfig + fmt.Sprintf(`
resource "meilisearch_index_settings" "test" {
  index_uid = %q
  searchable_attributes = ["title", "summary"]
  filterable_attributes = ["category"]
  stop_words = %s
}

data "meilisearch_index_settings" "observed" {
  index_uid = %q
  depends_on = [meilisearch_index_settings.test]
}
`, uid, stopWordsConfig, uid),
				Check: testresource.ComposeAggregateTestCheckFunc(
					testresource.TestCheckResourceAttr("meilisearch_index_settings.test", "index_uid", uid),
					testresource.TestCheckResourceAttr("meilisearch_index_settings.test", "searchable_attributes.#", "2"),
					testresource.TestCheckResourceAttr("meilisearch_index_settings.test", "searchable_attributes.0", "title"),
					testresource.TestCheckTypeSetElemAttr("meilisearch_index_settings.test", "filterable_attributes.*", "category"),
					testresource.TestCheckTypeSetElemAttr("meilisearch_index_settings.test", "stop_words.*", "the"),
					testresource.TestCheckTypeSetElemAttr("meilisearch_index_settings.test", "stop_words.*", "CAFÉ"),
					testresource.TestCheckTypeSetElemAttr("meilisearch_index_settings.test", "stop_words.*", "ك"),
					testresource.TestCheckTypeSetElemAttr("meilisearch_index_settings.test", "stop_words.*", "۱۲۳"),
					testresource.TestCheckTypeSetElemAttr("meilisearch_index_settings.test", "stop_words.*", "\u0002foo"),
					testresource.TestCheckTypeSetElemAttr("meilisearch_index_settings.test", "stop_words.*", "Å"),
					testresource.TestCheckResourceAttr("data.meilisearch_index_settings.observed", "searchable_attributes.#", "2"),
					testresource.TestCheckResourceAttr("data.meilisearch_index_settings.observed", "filterable_attributes.#", "1"),
					testresource.TestCheckResourceAttr("data.meilisearch_index_settings.observed", "stop_words.#", "6"),
					checkIndexSettingsAcceptanceRemoteStopWords(client, uid, []string{"the", "CAFÉ", "ك", "۱۲۳", "\u0002foo", "Å"}),
				),
			},
			{
				PreConfig: func() {
					task, err := client.Index(uid).UpdateSettings(&meilisearch.Settings{
						DisplayedAttributes:  []string{"out_of_band"},
						FilterableAttributes: []string{"out_of_band_category"},
					})
					if err != nil {
						t.Fatalf("mutate acceptance settings out of band: %v", err)
					}
					if _, err := waitTask(context.Background(), client, task.TaskUID); err != nil {
						t.Fatalf("wait for out-of-band settings mutation: %v", err)
					}
					expectedDisplayed = []string{"out_of_band"}
				},
				Config: providerConfig + fmt.Sprintf(`
resource "meilisearch_index_settings" "test" {
  index_uid = %q
  searchable_attributes = ["title", "summary"]
  filterable_attributes = ["category"]
  stop_words = %s
}

data "meilisearch_index_settings" "observed" {
  index_uid = %q
  depends_on = [meilisearch_index_settings.test]
}
`, uid, stopWordsConfig, uid),
				ConfigPlanChecks: testresource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("meilisearch_index_settings.test", "Update"),
				}},
				Check: testresource.ComposeAggregateTestCheckFunc(
					testresource.TestCheckTypeSetElemAttr("meilisearch_index_settings.test", "filterable_attributes.*", "category"),
					testresource.TestCheckTypeSetElemAttr("meilisearch_index_settings.test", "stop_words.*", "CAFÉ"),
					testresource.TestCheckResourceAttr("data.meilisearch_index_settings.observed", "displayed_attributes.0", "out_of_band"),
					testresource.TestCheckResourceAttr("data.meilisearch_index_settings.observed", "stop_words.#", "6"),
					checkIndexSettingsAcceptanceRemoteWithDisplayed(client, uid, []string{"out_of_band"}, []string{"title", "summary"}, []string{"category"}, nil),
					checkIndexSettingsAcceptanceRemoteStopWords(client, uid, []string{"the", "CAFÉ", "ك", "۱۲۳", "\u0002foo", "Å"}),
				),
			},
			{
				Config: providerConfig + fmt.Sprintf(`
resource "meilisearch_index_settings" "test" {
  index_uid = %q
  searchable_attributes = ["title", "summary"]
  filterable_attributes = ["category"]
  stop_words = %s
}

data "meilisearch_index_settings" "observed" {
  index_uid = %q
  depends_on = [meilisearch_index_settings.test]
}
`, uid, stopWordsConfig, uid),
				ConfigPlanChecks: testresource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectEmptyPlan(),
				}},
				Check: testresource.ComposeAggregateTestCheckFunc(
					testresource.TestCheckTypeSetElemAttr("meilisearch_index_settings.test", "stop_words.*", "the"),
					testresource.TestCheckTypeSetElemAttr("meilisearch_index_settings.test", "stop_words.*", "CAFÉ"),
					testresource.TestCheckTypeSetElemAttr("meilisearch_index_settings.test", "stop_words.*", "ك"),
					testresource.TestCheckTypeSetElemAttr("meilisearch_index_settings.test", "stop_words.*", "۱۲۳"),
					testresource.TestCheckTypeSetElemAttr("meilisearch_index_settings.test", "stop_words.*", "\u0002foo"),
					testresource.TestCheckTypeSetElemAttr("meilisearch_index_settings.test", "stop_words.*", "Å"),
					testresource.TestCheckResourceAttr("data.meilisearch_index_settings.observed", "stop_words.#", "6"),
					checkIndexSettingsAcceptanceRemoteStopWords(client, uid, []string{"the", "CAFÉ", "ك", "۱۲۳", "\u0002foo", "Å"}),
				),
			},
			{
				Config: providerConfig + fmt.Sprintf(`
resource "meilisearch_index_settings" "test" {
  index_uid = %q
  searchable_attributes = ["title", "body"]
  filterable_attributes = ["category", "brand"]
}

data "meilisearch_index_settings" "observed" {
  index_uid = %q
  depends_on = [meilisearch_index_settings.test]
}
`, uid, uid),
				ConfigPlanChecks: testresource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("meilisearch_index_settings.test", "Update"),
				}},
				Check: testresource.ComposeAggregateTestCheckFunc(
					testresource.TestCheckResourceAttr("meilisearch_index_settings.test", "searchable_attributes.#", "2"),
					testresource.TestCheckResourceAttr("meilisearch_index_settings.test", "searchable_attributes.0", "title"),
					testresource.TestCheckTypeSetElemAttr("meilisearch_index_settings.test", "filterable_attributes.*", "brand"),
					testresource.TestCheckNoResourceAttr("meilisearch_index_settings.test", "stop_words"),
					testresource.TestCheckResourceAttr("data.meilisearch_index_settings.observed", "searchable_attributes.#", "2"),
					testresource.TestCheckResourceAttr("data.meilisearch_index_settings.observed", "filterable_attributes.#", "2"),
					testresource.TestCheckResourceAttr("data.meilisearch_index_settings.observed", "stop_words.#", "0"),
					checkIndexSettingsAcceptanceRemoteWithDisplayed(client, uid, []string{"out_of_band"}, []string{"title", "body"}, []string{"category", "brand"}, []string{}),
				),
			},
			{
				Config: providerConfig + fmt.Sprintf(`
resource "meilisearch_index_settings" "test" {
  index_uid = %q
  searchable_attributes = ["title", "body"]
  filterable_attributes = ["category", "brand"]
}

data "meilisearch_index_settings" "observed" {
  index_uid = %q
  depends_on = [meilisearch_index_settings.test]
}
`, uid, uid),
				ConfigPlanChecks: testresource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectEmptyPlan(),
				}},
				Check: checkIndexSettingsAcceptanceRemoteWithDisplayed(client, uid, []string{"out_of_band"}, []string{"title", "body"}, []string{"category", "brand"}, []string{}),
			},
		},
	})
}

func TestAccIndexSettingsResourceImportAllFields(t *testing.T) {
	const uid = "settings-import-acceptance"
	var client meilisearch.ServiceManager

	testresource.Test(t, testresource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		PreCheck: func() {
			client = prepareIndexSettingsAcceptanceIndex(t, uid)
			seedSettingsAcceptanceAllFields(t, client, uid)
		},
		Steps: []testresource.TestStep{
			{
				Config: providerConfig + fmt.Sprintf(`
resource "meilisearch_index_settings" "test" {
  index_uid = %q
  searchable_attributes = ["title", "summary"]
  displayed_attributes = ["title", "summary"]
  filterable_attributes = ["category"]
  sortable_attributes = ["created_at"]
  ranking_rules = ["words"]
  stop_words = ["the"]
  synonyms = { phone = ["telephone"] }
  distinct_attribute = "group"
}
`, uid),
			},
			{
				ResourceName: "meilisearch_index_settings.test",
				Config: providerConfig + fmt.Sprintf(`
resource "meilisearch_index_settings" "test" {
  index_uid = %q
  searchable_attributes = ["title", "summary"]
  displayed_attributes = ["title", "summary"]
  filterable_attributes = ["category"]
  sortable_attributes = ["created_at"]
  ranking_rules = ["words"]
  stop_words = ["the"]
  synonyms = { phone = ["telephone"] }
  distinct_attribute = "group"
}
`, uid),
				ImportStateId:     uid,
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateCheck: func(states []*terraform.InstanceState) error {
					if len(states) != 1 {
						return fmt.Errorf("imported state count=%d, expected one", len(states))
					}
					state := states[0]
					for _, field := range []string{"searchable_attributes.#", "displayed_attributes.#", "filterable_attributes.#", "sortable_attributes.#", "ranking_rules.#", "stop_words.#", "synonyms.%", "distinct_attribute"} {
						if state.Attributes[field] == "" {
							return fmt.Errorf("imported state missing %q", field)
						}
					}
					return nil
				},
				Check: testresource.ComposeAggregateTestCheckFunc(
					testresource.TestCheckResourceAttr("meilisearch_index_settings.test", "searchable_attributes.#", "2"),
					testresource.TestCheckResourceAttr("meilisearch_index_settings.test", "displayed_attributes.#", "2"),
					testresource.TestCheckTypeSetElemAttr("meilisearch_index_settings.test", "filterable_attributes.*", "category"),
					testresource.TestCheckTypeSetElemAttr("meilisearch_index_settings.test", "sortable_attributes.*", "created_at"),
					testresource.TestCheckResourceAttr("meilisearch_index_settings.test", "ranking_rules.0", "words"),
					testresource.TestCheckTypeSetElemAttr("meilisearch_index_settings.test", "stop_words.*", "the"),
					testresource.TestCheckResourceAttr("meilisearch_index_settings.test", "synonyms.phone.0", "telephone"),
					testresource.TestCheckResourceAttr("meilisearch_index_settings.test", "distinct_attribute", "group"),
				),
			},
		},
	})
}

func checkIndexSettingsAcceptanceRemoteWithDisplayed(client meilisearch.ServiceManager, uid string, displayed, searchable, filterable, stopWords []string) testresource.TestCheckFunc {
	return func(_ *terraform.State) error {
		settings, err := client.Index(uid).GetSettings()
		if err != nil {
			return fmt.Errorf("read remote index settings: %w", err)
		}
		if !reflect.DeepEqual(settings.SearchableAttributes, searchable) {
			return fmt.Errorf("remote searchable_attributes=%v, expected %v", settings.SearchableAttributes, searchable)
		}
		if !sameIndexSettingsStringSet(settings.FilterableAttributes, filterable) {
			return fmt.Errorf("remote filterable_attributes=%v, expected %v", settings.FilterableAttributes, filterable)
		}
		if stopWords != nil && !sameIndexSettingsStringSet(settings.StopWords, stopWords) {
			return fmt.Errorf("remote stop_words=%v, expected %v", settings.StopWords, stopWords)
		}
		if !reflect.DeepEqual(settings.DisplayedAttributes, displayed) || !sameIndexSettingsStringSet(settings.SortableAttributes, []string{"created_at"}) {
			return fmt.Errorf("remote unowned settings changed: displayed=%v sortable=%v", settings.DisplayedAttributes, settings.SortableAttributes)
		}
		if !reflect.DeepEqual(settings.RankingRules, []string{"words"}) || !reflect.DeepEqual(settings.Synonyms, map[string][]string{"phone": {"telephone"}}) || settings.DistinctAttribute == nil || *settings.DistinctAttribute != "group" {
			return fmt.Errorf("remote unowned ranking/synonym/distinct settings changed: ranking=%v synonyms=%v distinct=%v", settings.RankingRules, settings.Synonyms, settings.DistinctAttribute)
		}
		return nil
	}
}

func checkIndexSettingsAcceptanceRemoteStopWords(client meilisearch.ServiceManager, uid string, configured []string) testresource.TestCheckFunc {
	return func(_ *terraform.State) error {
		settings, err := client.Index(uid).GetSettings()
		if err != nil {
			return fmt.Errorf("read remote stop-word settings: %w", err)
		}
		if !sameIndexSettingsStringSet(normalizedStopWords(settings.StopWords), normalizedStopWords(configured)) {
			return fmt.Errorf("remote stop_words=%v, expected normalized values for %v", settings.StopWords, configured)
		}
		return nil
	}
}

func sameIndexSettingsStringSet(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	counts := make(map[string]int, len(left))
	for _, value := range left {
		counts[value]++
	}
	for _, value := range right {
		if counts[value] == 0 {
			return false
		}
		counts[value]--
	}
	return true
}

func prepareIndexSettingsAcceptanceIndex(t *testing.T, uid string) meilisearch.ServiceManager {
	t.Helper()

	client := meilisearch.New(testHost, meilisearch.WithAPIKey(testAPIKey))
	if _, err := client.GetIndex(uid); err == nil {
		task, deleteErr := client.DeleteIndex(uid)
		if deleteErr != nil {
			t.Fatalf("delete stale acceptance index: %v", deleteErr)
		}
		if _, waitErr := waitTask(context.Background(), client, task.TaskUID); waitErr != nil {
			t.Fatalf("wait for stale acceptance index deletion: %v", waitErr)
		}
	} else if !isMissingIndexError(err) {
		t.Fatalf("check stale acceptance index: %v", err)
	}

	task, err := client.CreateIndex(&meilisearch.IndexConfig{Uid: uid, PrimaryKey: "id"})
	if err != nil {
		t.Fatalf("create acceptance index: %v", err)
	}
	if _, err := waitTask(context.Background(), client, task.TaskUID); err != nil {
		t.Fatalf("wait for acceptance index creation: %v", err)
	}

	documentTask, err := client.Index(uid).AddDocuments([]map[string]interface{}{{"id": "settings-document", "title": "one", "category": "category"}}, nil)
	if err != nil {
		t.Fatalf("add acceptance document: %v", err)
	}
	if _, err := waitTask(context.Background(), client, documentTask.TaskUID); err != nil {
		t.Fatalf("wait for acceptance document: %v", err)
	}
	return client
}

func seedSettingsAcceptanceUnownedFields(t *testing.T, client meilisearch.ServiceManager, uid string) {
	t.Helper()

	distinct := "group"
	task, err := client.Index(uid).UpdateSettings(&meilisearch.Settings{
		DisplayedAttributes: []string{"title", "summary"},
		SortableAttributes:  []string{"created_at"},
		RankingRules:        []string{"words"},
		Synonyms:            map[string][]string{"phone": {"telephone"}},
		DistinctAttribute:   &distinct,
	})
	if err != nil {
		t.Fatalf("seed acceptance unowned settings: %v", err)
	}
	if _, err := waitTask(context.Background(), client, task.TaskUID); err != nil {
		t.Fatalf("wait for acceptance unowned settings: %v", err)
	}
}

func seedSettingsAcceptanceAllFields(t *testing.T, client meilisearch.ServiceManager, uid string) {
	t.Helper()

	distinct := "group"
	task, err := client.Index(uid).UpdateSettings(&meilisearch.Settings{
		SearchableAttributes: []string{"title", "summary"},
		DisplayedAttributes:  []string{"title", "summary"},
		FilterableAttributes: []string{"category"},
		SortableAttributes:   []string{"created_at"},
		RankingRules:         []string{"words"},
		StopWords:            []string{"the"},
		Synonyms:             map[string][]string{"phone": {"telephone"}},
		DistinctAttribute:    &distinct,
	})
	if err != nil {
		t.Fatalf("seed acceptance import settings: %v", err)
	}
	if _, err := waitTask(context.Background(), client, task.TaskUID); err != nil {
		t.Fatalf("wait for acceptance import settings: %v", err)
	}
}
