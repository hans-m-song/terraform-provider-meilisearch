package provider

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/meilisearch/meilisearch-go"
)

func TestIndexCreateTimeoutRetainsRemoteIdentity(t *testing.T) {
	var taskRequests atomic.Int32
	taskContextDone := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch {
		case request.Method == http.MethodPost && request.URL.Path == "/indexes":
			writeIndexFailureTestJSON(writer, http.StatusAccepted, `{"taskUid":101,"indexUid":"retained-index","status":"enqueued"}`)
		case request.Method == http.MethodGet && request.URL.Path == "/tasks/101":
			taskRequests.Add(1)
			<-request.Context().Done()
			close(taskContextDone)
		default:
			writeIndexFailureTestJSON(writer, http.StatusNotFound, `{"code":"unexpected_route"}`)
		}
	}))
	defer server.Close()

	schema := indexFailureTestSchema(t)
	index := newIndexFailureTestResource(t, server.URL, 25*time.Millisecond)
	response := resource.CreateResponse{State: tfsdk.State{Schema: schema.Schema}}
	started := time.Now()
	index.Create(context.Background(), resource.CreateRequest{
		Plan: indexFailureTestConfig(t, schema, "retained-index", types.StringNull()),
	}, &response)

	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("create exceeded bounded timeout: %s", elapsed)
	}
	if !response.Diagnostics.HasError() || !diagnosticsContain(response.Diagnostics, "deadline") {
		t.Fatalf("create diagnostics=%v, expected deadline error", response.Diagnostics)
	}
	if taskRequests.Load() != 1 {
		t.Fatalf("task requests=%d, expected one", taskRequests.Load())
	}
	select {
	case <-taskContextDone:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("task request context was not canceled")
	}

	var state indexResourceModel
	if diagnostics := response.State.Get(context.Background(), &state); diagnostics.HasError() {
		t.Fatalf("retained state diagnostics=%v", diagnostics)
	}
	if !state.UID.Equal(types.StringValue("retained-index")) || !state.ID.Equal(types.StringValue("retained-index")) {
		t.Fatalf("retained state uid=%v id=%v, expected remote identity", state.UID, state.ID)
	}
}

func TestIndexCreateTerminalFailureRemovesState(t *testing.T) {
	for _, test := range []struct {
		name   string
		status string
		uid    int
	}{
		{name: "failed", status: "failed", uid: 102},
		{name: "canceled", status: "canceled", uid: 103},
	} {
		t.Run(test.name, func(t *testing.T) {
			var taskRequests atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				switch {
				case request.Method == http.MethodPost && request.URL.Path == "/indexes":
					writeIndexFailureTestJSON(writer, http.StatusAccepted, fmt.Sprintf(`{"taskUid":%d,"indexUid":"terminal-index","status":"enqueued"}`, test.uid))
				case request.Method == http.MethodGet && request.URL.Path == fmt.Sprintf("/tasks/%d", test.uid):
					taskRequests.Add(1)
					writeIndexFailureTestJSON(writer, http.StatusOK, fmt.Sprintf(`{"uid":%d,"status":%q,"error":{"code":"index_already_exists"}}`, test.uid, test.status))
				default:
					writeIndexFailureTestJSON(writer, http.StatusNotFound, `{"code":"unexpected_route"}`)
				}
			}))

			schema := indexFailureTestSchema(t)
			index := newIndexFailureTestResource(t, server.URL, time.Second)
			response := resource.CreateResponse{State: tfsdk.State{Schema: schema.Schema}}
			index.Create(context.Background(), resource.CreateRequest{
				Plan: indexFailureTestConfig(t, schema, "terminal-index", types.StringNull()),
			}, &response)
			server.Close()

			if !response.Diagnostics.HasError() {
				t.Fatal("terminal create failure must produce a diagnostic")
			}
			if taskRequests.Load() != 1 {
				t.Fatalf("task requests=%d, expected one", taskRequests.Load())
			}
			if !response.State.Raw.IsNull() {
				t.Fatalf("state=%v, expected resource state removal", response.State.Raw)
			}
		})
	}
}

func TestIndexReadNotFoundRemovesState(t *testing.T) {
	var readRequests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet || request.URL.Path != "/indexes/missing-index" {
			writeIndexFailureTestJSON(writer, http.StatusNotFound, `{"code":"unexpected_route"}`)
			return
		}

		readRequests.Add(1)
		writeIndexFailureTestJSON(writer, http.StatusNotFound, `{"code":"index_not_found","message":"missing-index"}`)
	}))
	defer server.Close()

	schema := indexFailureTestSchema(t)
	index := newIndexFailureTestResource(t, server.URL, time.Second)
	state := indexFailureTestState(t, schema, "missing-index", types.StringNull())
	response := resource.ReadResponse{State: state}
	index.Read(context.Background(), resource.ReadRequest{State: state}, &response)

	if response.Diagnostics.HasError() {
		t.Fatalf("read diagnostics=%v, expected idempotent missing state removal", response.Diagnostics)
	}
	if readRequests.Load() != 1 {
		t.Fatalf("read requests=%d, expected one", readRequests.Load())
	}
	if !response.State.Raw.IsNull() {
		t.Fatalf("state=%v, expected resource state removal", response.State.Raw)
	}
}

func TestIndexPrimaryKeyUpdateRejectsPopulatedIndexWithoutMutation(t *testing.T) {
	var statsRequests atomic.Int32
	var updateRequests atomic.Int32
	var mutationRequests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method == http.MethodPost || request.Method == http.MethodPut || request.Method == http.MethodPatch || request.Method == http.MethodDelete {
			mutationRequests.Add(1)
		}

		switch {
		case request.Method == http.MethodGet && request.URL.Path == "/indexes/populated-index/stats":
			statsRequests.Add(1)
			writeIndexFailureTestJSON(writer, http.StatusOK, `{"numberOfDocuments":3}`)
		case request.Method == http.MethodPatch && request.URL.Path == "/indexes/populated-index":
			updateRequests.Add(1)
			writeIndexFailureTestJSON(writer, http.StatusAccepted, `{"taskUid":104,"indexUid":"populated-index","status":"enqueued"}`)
		default:
			writeIndexFailureTestJSON(writer, http.StatusNotFound, `{"code":"unexpected_route"}`)
		}
	}))
	defer server.Close()

	schema := indexFailureTestSchema(t)
	index := newIndexFailureTestResource(t, server.URL, time.Second)
	state := indexFailureTestState(t, schema, "populated-index", types.StringValue("old_id"))
	plan := indexFailureTestConfig(t, schema, "populated-index", types.StringValue("new_id"))
	response := resource.UpdateResponse{State: state}
	index.Update(context.Background(), resource.UpdateRequest{Plan: plan, State: state}, &response)

	if !response.Diagnostics.HasError() || !diagnosticsContain(response.Diagnostics, "no documents were deleted") {
		t.Fatalf("update diagnostics=%v, expected populated-index rejection", response.Diagnostics)
	}
	if statsRequests.Load() != 1 {
		t.Fatalf("stats requests=%d, expected one", statsRequests.Load())
	}
	if updateRequests.Load() != 0 {
		t.Fatalf("update requests=%d, expected no mutation", updateRequests.Load())
	}
	if mutationRequests.Load() != 0 {
		t.Fatalf("mutation requests=%d, expected no mutation", mutationRequests.Load())
	}
}

func TestIndexDeleteTaskNotFoundIsIdempotent(t *testing.T) {
	var deleteRequests atomic.Int32
	var taskRequests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch {
		case request.Method == http.MethodDelete && request.URL.Path == "/indexes/deleted-index":
			deleteRequests.Add(1)
			writeIndexFailureTestJSON(writer, http.StatusAccepted, `{"taskUid":105,"indexUid":"deleted-index","status":"enqueued"}`)
		case request.Method == http.MethodGet && request.URL.Path == "/tasks/105":
			taskRequests.Add(1)
			writeIndexFailureTestJSON(writer, http.StatusOK, `{"uid":105,"status":"failed","error":{"code":"index_not_found"}}`)
		default:
			writeIndexFailureTestJSON(writer, http.StatusNotFound, `{"code":"unexpected_route"}`)
		}
	}))
	defer server.Close()

	schema := indexFailureTestSchema(t)
	index := newIndexFailureTestResource(t, server.URL, time.Second)
	state := indexFailureTestState(t, schema, "deleted-index", types.StringNull())
	response := resource.DeleteResponse{State: state}
	index.Delete(context.Background(), resource.DeleteRequest{State: state}, &response)

	if response.Diagnostics.HasError() {
		t.Fatalf("delete diagnostics=%v, expected task-level not-found idempotence", response.Diagnostics)
	}
	if deleteRequests.Load() != 1 || taskRequests.Load() != 1 {
		t.Fatalf("delete requests=%d task requests=%d, expected one each", deleteRequests.Load(), taskRequests.Load())
	}
}

func indexFailureTestSchema(t *testing.T) resource.SchemaResponse {
	t.Helper()

	var response resource.SchemaResponse
	NewIndexResource().Schema(context.Background(), resource.SchemaRequest{}, &response)
	if response.Diagnostics.HasError() {
		t.Fatalf("index schema diagnostics=%v", response.Diagnostics)
	}

	return response
}

func indexFailureTestConfig(t *testing.T, schema resource.SchemaResponse, uid string, primaryKey types.String) tfsdk.Plan {
	t.Helper()

	plan := tfsdk.Plan{Schema: schema.Schema}
	diagnostics := plan.Set(context.Background(), &indexResourceModel{
		UID:        types.StringValue(uid),
		PrimaryKey: primaryKey,
	})
	if diagnostics.HasError() {
		t.Fatalf("index config diagnostics=%v", diagnostics)
	}

	return plan
}

func indexFailureTestState(t *testing.T, schema resource.SchemaResponse, uid string, primaryKey types.String) tfsdk.State {
	t.Helper()

	state := tfsdk.State{Schema: schema.Schema}
	diagnostics := state.Set(context.Background(), &indexResourceModel{
		UID:        types.StringValue(uid),
		PrimaryKey: primaryKey,
		ID:         types.StringValue(uid),
	})
	if diagnostics.HasError() {
		t.Fatalf("index state diagnostics=%v", diagnostics)
	}

	return state
}

func newIndexFailureTestResource(t *testing.T, host string, timeout time.Duration) *indexResource {
	t.Helper()

	configured, ok := NewIndexResource().(*indexResource)
	if !ok {
		t.Fatal("index resource has unexpected type")
	}

	var response resource.ConfigureResponse
	configured.Configure(context.Background(), resource.ConfigureRequest{ProviderData: &providerData{
		client:           meilisearch.New(host, meilisearch.WithAPIKey("synthetic-test-key")),
		operationTimeout: timeout,
	}}, &response)
	if response.Diagnostics.HasError() {
		t.Fatalf("index configure diagnostics=%v", response.Diagnostics)
	}

	return configured
}

func writeIndexFailureTestJSON(writer http.ResponseWriter, status int, body string) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_, _ = writer.Write([]byte(body))
}

func diagnosticsContain(diagnostics interface{ HasError() bool }, substring string) bool {
	return diagnostics.HasError() && strings.Contains(fmt.Sprint(diagnostics), substring)
}
