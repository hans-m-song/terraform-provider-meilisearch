package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestUpdateKeyWithContextPreservesNullableMetadata(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPatch || request.URL.EscapedPath() != "/keys/key%2Fuid" {
			t.Fatalf("request=%s %s, expected PATCH /keys/key%%2Fuid", request.Method, request.URL.EscapedPath())
		}
		if got := request.Header.Get("Authorization"); got != "Bearer test-api-key" {
			t.Fatalf("authorization=%q", got)
		}

		var body map[string]any
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		want := map[string]any{"name": nil, "description": ""}
		if !reflect.DeepEqual(body, want) {
			t.Fatalf("request body=%v, expected %v", body, want)
		}

		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"uid":"key/uid","name":"","description":"","actions":["search"],"indexes":["*"],"createdAt":"2026-01-01T00:00:00Z","updatedAt":"2026-01-01T00:00:00Z"}`))
	}))
	defer server.Close()

	resource := &keyResource{provider: &providerData{
		host:       server.URL,
		apiKey:     "test-api-key",
		httpClient: server.Client(),
	}}

	name := types.StringNull()
	description := types.StringValue("")
	key, err := resource.updateKeyWithContext(context.Background(), "key/uid", stringPointer(name), stringPointer(description))
	if err != nil {
		t.Fatalf("updateKeyWithContext() error=%v", err)
	}
	if key == nil || key.UID != "key/uid" {
		t.Fatalf("updateKeyWithContext() key=%+v, expected key/uid", key)
	}
}

func TestKeyAPIErrorDoesNotExposeResponseBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusNotFound)
		_, _ = writer.Write([]byte(`{"code":"api_key_not_found","message":"secret-value-must-not-escape"}`))
	}))
	defer server.Close()

	resource := &keyResource{provider: &providerData{
		host:       server.URL,
		apiKey:     "test-api-key",
		httpClient: server.Client(),
	}}

	_, err := resource.updateKeyWithContext(context.Background(), "missing", stringPointer(types.StringValue("name")), stringPointer(types.StringValue("description")))
	if err == nil {
		t.Fatal("updateKeyWithContext() error=nil, expected API error")
	}
	if !isAPIError(err, "api_key_not_found") {
		t.Fatalf("isAPIError() false for %v", err)
	}
	message := apiError(err)
	if message == "" || containsAny(message, "secret-value-must-not-escape", server.URL) {
		t.Fatalf("apiError()=%q, expected redacted API diagnostic", message)
	}
}

func containsAny(value string, needles ...string) bool {
	for _, needle := range needles {
		if needle != "" && strings.Contains(value, needle) {
			return true
		}
	}
	return false
}
