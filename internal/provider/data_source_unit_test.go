package provider

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/meilisearch/meilisearch-go"
)

func TestDataSourceReads(t *testing.T) {
	for _, test := range []struct {
		name     string
		new      func() datasource.DataSource
		model    any
		route    string
		response string
		expect   map[string]types.String
	}{
		{
			name: "index without primary key", new: NewIndexDataSource,
			model: indexDataSourceModel{UID: types.StringValue("fixture")}, route: "/indexes/fixture",
			response: `{"uid":"fixture","primaryKey":null,"createdAt":"2026-09-18T00:00:00Z","updatedAt":"2026-09-18T00:00:01Z"}`,
			expect: map[string]types.String{
				"id": types.StringValue("fixture"), "primary_key": types.StringNull(),
				"created_at": types.StringValue("2026-09-18T00:00:00Z"),
			},
		},
		{
			name: "key without expiry", new: NewKeyDataSource,
			model: keyDataSourceModel{UID: types.StringValue("fixture-key"), Actions: types.SetNull(types.StringType), Indexes: types.SetNull(types.StringType)}, route: "/keys/fixture-key",
			response: `{"uid":"fixture-key","key":"synthetic-key","name":"fixture","description":"fixture","actions":["search"],"indexes":["fixture"],"expiresAt":null,"createdAt":"2026-09-18T00:00:00Z","updatedAt":"2026-09-18T00:00:01Z"}`,
			expect: map[string]types.String{
				"id": types.StringValue("fixture-key"), "expires_at": types.StringNull(),
				"created_at": types.StringValue("2026-09-18T00:00:00Z"),
			},
		},
		{
			name: "version", new: NewVersionDataSource,
			model: versionDataSourceModel{}, route: "/version",
			response: `{"commitSha":"fixture-commit","commitDate":"2026-09-18T00:00:00Z","pkgVersion":"1.53.2"}`,
			expect:   map[string]types.String{"id": types.StringValue("1.53.2")},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != test.route || r.Method != http.MethodGet {
					t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
				}

				w.Header().Set("Content-Type", "application/json")
				_, _ = fmt.Fprint(w, test.response)
			}))
			defer server.Close()

			dataSource := test.new()

			var schemaResponse datasource.SchemaResponse

			dataSource.Schema(context.Background(), datasource.SchemaRequest{}, &schemaResponse)

			config := tfsdk.State{Schema: schemaResponse.Schema}
			if diagnostics := config.Set(context.Background(), test.model); diagnostics.HasError() {
				t.Fatalf("fixture configuration: %v", diagnostics)
			}

			var configureResponse datasource.ConfigureResponse

			configurable, ok := dataSource.(datasource.DataSourceWithConfigure)
			if !ok {
				t.Fatal("data source does not implement DataSourceWithConfigure")
			}

			configurable.Configure(context.Background(), datasource.ConfigureRequest{
				ProviderData: &providerData{client: meilisearch.New(server.URL, meilisearch.WithAPIKey("synthetic-key")), operationTimeout: time.Second},
			}, &configureResponse)

			if configureResponse.Diagnostics.HasError() {
				t.Fatalf("configure: %v", configureResponse.Diagnostics)
			}

			readResponse := datasource.ReadResponse{State: tfsdk.State{Schema: schemaResponse.Schema}}

			dataSource.Read(context.Background(), datasource.ReadRequest{Config: tfsdk.Config(config)}, &readResponse)

			if readResponse.Diagnostics.HasError() {
				t.Fatalf("read: %v", readResponse.Diagnostics)
			}

			for name, expected := range test.expect {
				var actual types.String

				if diagnostics := readResponse.State.GetAttribute(context.Background(), path.Root(name), &actual); diagnostics.HasError() {
					t.Fatalf("attribute %s: %v", name, diagnostics)
				}

				if !actual.Equal(expected) {
					t.Errorf("%s=%v, expected %v", name, actual, expected)
				}
			}
		})
	}
}

func TestDataSourceConfigureRejectsInvalidClient(t *testing.T) {
	for _, constructor := range []func() datasource.DataSource{NewIndexDataSource, NewKeyDataSource, NewVersionDataSource} {
		for _, value := range []any{"invalid", (*providerData)(nil), &providerData{}} {
			var response datasource.ConfigureResponse

			configurable, ok := constructor().(datasource.DataSourceWithConfigure)
			if !ok {
				t.Fatal("data source does not implement DataSourceWithConfigure")
			}

			configurable.Configure(context.Background(), datasource.ConfigureRequest{ProviderData: value}, &response)

			if !response.Diagnostics.HasError() {
				t.Fatal("invalid provider data must produce a diagnostic")
			}
		}
	}
}

func TestDataSourceReadDeadline(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer server.Close()

	dataSource := &versionDataSource{}

	var schemaResponse datasource.SchemaResponse

	dataSource.Schema(context.Background(), datasource.SchemaRequest{}, &schemaResponse)

	var configureResponse datasource.ConfigureResponse

	dataSource.Configure(context.Background(), datasource.ConfigureRequest{
		ProviderData: &providerData{client: meilisearch.New(server.URL, meilisearch.WithAPIKey("synthetic-key")), operationTimeout: 20 * time.Millisecond},
	}, &configureResponse)

	response := datasource.ReadResponse{State: tfsdk.State{Schema: schemaResponse.Schema}}
	started := time.Now()

	dataSource.Read(context.Background(), datasource.ReadRequest{}, &response)

	if !response.Diagnostics.HasError() || time.Since(started) > time.Second {
		t.Fatalf("read must fail promptly on deadline: %v", response.Diagnostics)
	}

	if !strings.Contains(response.Diagnostics[0].Detail(), "deadline") {
		t.Fatalf("deadline diagnostic: %v", response.Diagnostics)
	}
}
