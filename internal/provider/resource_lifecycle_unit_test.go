package provider

import (
	"context"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	resourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/meilisearch/meilisearch-go"
)

func TestIndexSchemaAllowsServerSelectedPrimaryKey(t *testing.T) {
	var response resource.SchemaResponse
	NewIndexResource().Schema(context.Background(), resource.SchemaRequest{}, &response)

	primaryKey, ok := response.Schema.Attributes["primary_key"].(resourceschema.StringAttribute)
	if !ok {
		t.Fatal("primary_key schema attribute has unexpected type")
	}
	if !primaryKey.Optional || !primaryKey.Computed {
		t.Fatalf("primary_key schema=%+v, expected optional and computed", primaryKey)
	}
}

func TestKeyMetadataRemovalIsPlannable(t *testing.T) {
	var response resource.SchemaResponse
	NewKeyResource().Schema(context.Background(), resource.SchemaRequest{}, &response)

	for _, name := range []string{"name", "description"} {
		attribute, ok := response.Schema.Attributes[name].(resourceschema.StringAttribute)
		if !ok {
			t.Fatalf("%s schema attribute has unexpected type", name)
		}
		if !attribute.Optional || attribute.Computed {
			t.Fatalf("%s schema=%+v, expected optional-only metadata", name, attribute)
		}
	}
}

func TestKeyExpirationSchemaRequiresReplacementAndValidatesConfiguredValues(t *testing.T) {
	var response resource.SchemaResponse
	NewKeyResource().Schema(context.Background(), resource.SchemaRequest{}, &response)

	expiresAt, ok := response.Schema.Attributes["expires_at"].(resourceschema.StringAttribute)
	if !ok {
		t.Fatal("expires_at schema attribute has unexpected type")
	}
	if !expiresAt.Optional || expiresAt.Computed || len(expiresAt.PlanModifiers) != 1 {
		t.Fatalf("expires_at schema=%+v, expected optional-only replacement attribute", expiresAt)
	}
	if len(expiresAt.Validators) != 1 {
		t.Fatalf("expires_at validators=%d, expected one RFC3339 validator", len(expiresAt.Validators))
	}

	for _, test := range []struct {
		name      string
		value     types.String
		wantError bool
	}{
		{name: "null", value: types.StringNull()},
		{name: "unknown", value: types.StringUnknown()},
		{name: "valid", value: types.StringValue("2042-04-02T00:42:42+10:00")},
		{name: "empty", value: types.StringValue(""), wantError: true},
		{name: "malformed", value: types.StringValue("not-a-timestamp"), wantError: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			var validation validator.StringResponse
			expiresAt.Validators[0].ValidateString(context.Background(), validator.StringRequest{
				Path:        path.Root("expires_at"),
				ConfigValue: test.value,
			}, &validation)
			if validation.Diagnostics.HasError() != test.wantError {
				t.Fatalf("diagnostics=%v, wantError=%t", validation.Diagnostics, test.wantError)
			}
		})
	}

	for _, test := range []struct {
		name      string
		value     types.String
		wantError bool
	}{
		{name: "null", value: types.StringNull()},
		{name: "unknown", value: types.StringUnknown()},
		{name: "empty", value: types.StringValue(""), wantError: true},
		{name: "malformed", value: types.StringValue("not-a-timestamp"), wantError: true},
	} {
		t.Run("parse_"+test.name, func(t *testing.T) {
			parsed, err := parseExpiresAt(test.value)
			if (err != nil) != test.wantError {
				t.Fatalf("parseExpiresAt() error=%v, wantError=%t", err, test.wantError)
			}
			if !test.wantError && !parsed.IsZero() {
				t.Fatalf("parseExpiresAt()=%s, expected no-expiry zero time", parsed)
			}
		})
	}
}

func TestResourceConfigureRejectsInvalidProviderData(t *testing.T) {
	for _, constructor := range []func() resource.Resource{NewIndexResource, NewKeyResource} {
		var response resource.ConfigureResponse
		configuredResource, ok := constructor().(resource.ResourceWithConfigure)
		if !ok {
			t.Fatal("resource does not implement ResourceWithConfigure")
		}
		configuredResource.Configure(context.Background(), resource.ConfigureRequest{ProviderData: "invalid"}, &response)
		if !response.Diagnostics.HasError() {
			t.Fatal("invalid provider data must produce a diagnostic")
		}
	}
}

func TestKeyStateNormalizesOptionalValuesAndPreservesConfiguredOffset(t *testing.T) {
	ctx := context.Background()
	expiresAt := time.Date(2026, time.September, 18, 3, 0, 0, 0, time.FixedZone("AEST", 10*60*60))
	configuredExpiry := "2026-09-18T03:00:00+10:00"
	configured := &keyResourceModel{
		Name:        types.StringValue("configured-name"),
		Description: types.StringValue(""),
		ExpiresAt:   types.StringValue(configuredExpiry),
		Actions:     types.SetValueMust(types.StringType, []attr.Value{types.StringValue("search")}),
		Indexes:     types.SetValueMust(types.StringType, []attr.Value{types.StringValue("fixture")}),
	}

	state, diags := keyResourceState(ctx, &meilisearch.Key{
		UID:         "fixture-key",
		Name:        "",
		Description: "",
		Actions:     []string{"search"},
		Indexes:     []string{"fixture"},
		ExpiresAt:   expiresAt,
	}, configured, true)
	if diags.HasError() {
		t.Fatalf("keyResourceState() diagnostics=%v", diags)
	}
	if !state.Name.IsNull() {
		t.Fatalf("name=%v, expected remote empty value to remain null", state.Name)
	}
	if !state.Description.Equal(types.StringValue("")) {
		t.Fatalf("description=%v, expected explicit empty string", state.Description)
	}
	if !state.ExpiresAt.Equal(types.StringValue(configuredExpiry)) {
		t.Fatalf("expires_at=%v, expected configured offset spelling", state.ExpiresAt)
	}
	if !state.ID.Equal(types.StringValue("fixture-key")) {
		t.Fatalf("id=%v, expected remote UID", state.ID)
	}
}

func TestOperationContextUsesDefaultTimeout(t *testing.T) {
	ctx, cancel := operationContext(context.Background(), 0)
	defer cancel()

	deadline, ok := ctx.Deadline()
	if !ok {
		t.Fatal("operationContext() did not set a deadline")
	}
	if remaining := time.Until(deadline); remaining <= 0 || remaining > defaultOperationTimeout {
		t.Fatalf("remaining timeout=%s, expected at most %s", remaining, defaultOperationTimeout)
	}
}
