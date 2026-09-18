package provider

import (
	"context"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func providerTestConfig(t *testing.T, host, key, timeout any) tfsdk.Config {
	t.Helper()

	var response provider.SchemaResponse

	New("test")().Schema(context.Background(), provider.SchemaRequest{}, &response)

	return tfsdk.Config{
		Schema: response.Schema,
		Raw: tftypes.NewValue(tftypes.Object{AttributeTypes: map[string]tftypes.Type{
			"host":              tftypes.String,
			"api_key":           tftypes.String,
			"operation_timeout": tftypes.String,
		}}, map[string]tftypes.Value{
			"host":              tftypes.NewValue(tftypes.String, host),
			"api_key":           tftypes.NewValue(tftypes.String, key),
			"operation_timeout": tftypes.NewValue(tftypes.String, timeout),
		}),
	}
}

func TestProviderConfiguration(t *testing.T) {
	for _, test := range []struct {
		name    string
		host    any
		key     any
		timeout any
		invalid bool
	}{
		{name: "defaults", host: "http://localhost:17700", key: "synthetic-test-key"},
		{name: "custom timeout", host: "https://example.invalid/search", key: "synthetic-test-key", timeout: "30s"},
		{name: "unknown host", host: tftypes.UnknownValue, key: "synthetic-test-key", invalid: true},
		{name: "unknown key", host: "http://localhost:17700", key: tftypes.UnknownValue, invalid: true},
		{name: "unknown timeout", host: "http://localhost:17700", key: "synthetic-test-key", timeout: tftypes.UnknownValue, invalid: true},
		{name: "empty key", host: "http://localhost:17700", key: "", invalid: true},
		{name: "relative URL", host: "localhost:17700", key: "synthetic-test-key", invalid: true},
		{name: "invalid scheme", host: "file:///tmp/meili", key: "synthetic-test-key", invalid: true},
		{name: "URL query", host: "https://example.invalid?token=synthetic", key: "synthetic-test-key", invalid: true},
		{name: "URL fragment", host: "https://example.invalid#synthetic", key: "synthetic-test-key", invalid: true},
		{name: "zero timeout", host: "http://localhost:17700", key: "synthetic-test-key", timeout: "0s", invalid: true},
		{name: "negative timeout", host: "http://localhost:17700", key: "synthetic-test-key", timeout: "-1m", invalid: true},
		{name: "invalid timeout", host: "http://localhost:17700", key: "synthetic-test-key", timeout: "long", invalid: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			var response provider.ConfigureResponse

			New("test")().Configure(context.Background(), provider.ConfigureRequest{
				Config: providerTestConfig(t, test.host, test.key, test.timeout),
			}, &response)

			if response.Diagnostics.HasError() != test.invalid {
				t.Fatalf("invalid=%t, diagnostics=%v", test.invalid, response.Diagnostics)
			}

			if test.invalid {
				if response.ResourceData != nil || response.DataSourceData != nil {
					t.Fatal("invalid configuration exposed a client")
				}

				return
			}

			data, ok := response.ResourceData.(*providerData)
			if !ok || data.client == nil || response.DataSourceData != data {
				t.Fatal("resources and data sources must share the configured provider data")
			}

			expected := defaultOperationTimeout
			if test.timeout != nil {
				expected = 30 * time.Second
			}

			if data.operationTimeout != expected {
				t.Fatalf("timeout=%s, expected %s", data.operationTimeout, expected)
			}
		})
	}
}

func TestProviderValidationDefersUnknownValues(t *testing.T) {
	var response provider.ValidateConfigResponse

	configuredProvider := &MeilisearchProvider{version: "test"}

	configuredProvider.ValidateConfig(context.Background(), provider.ValidateConfigRequest{
		Config: providerTestConfig(t, tftypes.UnknownValue, tftypes.UnknownValue, tftypes.UnknownValue),
	}, &response)

	if response.Diagnostics.HasError() {
		t.Fatalf("validation must defer unknown values: %v", response.Diagnostics)
	}
}

func TestConfiguredOperationTimeout(t *testing.T) {
	timeout, err := configuredOperationTimeout(types.StringNull())
	if err != nil || timeout != 5*time.Minute {
		t.Fatalf("default timeout=%s, error=%v", timeout, err)
	}
}
