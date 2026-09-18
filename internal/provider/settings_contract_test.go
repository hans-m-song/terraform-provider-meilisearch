package provider

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

const settingsContractResponse = `{"searchableAttributes":["*"],"displayedAttributes":["*"],"filterableAttributes":[],"sortableAttributes":[],"rankingRules":["words"],"stopWords":[],"synonyms":{},"distinctAttribute":null}`

func TestSettingsContractRejectsMalformedReads(t *testing.T) {
	for _, body := range []string{"null", "{}", strings.Replace(settingsContractResponse, `"stopWords":[]`, `"stopWords":null`, 1), strings.Replace(settingsContractResponse, `"stopWords":[]`, `"stopWords":[null]`, 1), strings.Replace(settingsContractResponse, `"filterableAttributes":[]`, `"filterableAttributes":[42]`, 1)} {
		if _, err := decodeIndexSettings([]byte(body)); err == nil {
			t.Fatalf("accepted malformed response %s", body)
		}
	}

	values, err := decodeIndexSettings([]byte(settingsContractResponse))
	if err != nil || values.StopWords.IsNull() || values.Synonyms.IsNull() || !values.DistinctAttribute.IsNull() {
		t.Fatalf("valid empty/null settings decoded incorrectly: %v", err)
	}
}

func TestSettingsContractEmptyValuesRemainNonNull(t *testing.T) {
	for _, test := range []struct {
		field string
		value attr.Value
		want  string
	}{
		{settingsFieldSearchable, types.ListValueMust(types.StringType, []attr.Value{}), "[]"},
		{settingsFieldStopWords, types.SetValueMust(types.StringType, []attr.Value{}), "[]"},
		{settingsFieldSynonyms, types.MapValueMust(types.ListType{ElemType: types.StringType}, map[string]attr.Value{}), "{}"},
	} {
		encoded, err := encodeIndexSettingsField(context.Background(), test.field, test.value)
		if err != nil || string(encoded) != test.want {
			t.Fatalf("%s encoded as %s; error=%v", test.field, encoded, err)
		}
	}

	patch, err := indexSettingsPatch(context.Background(), indexSettingsResourceModel{}, nil, nil, map[string]bool{settingsFieldStopWords: true})
	if err != nil || string(patch["stopWords"]) != "null" || len(patch) != 1 {
		t.Fatalf("owned field removal patch=%v error=%v", patch, err)
	}
}

func TestSettingsContractAcceptedResponseFailures(t *testing.T) {
	for _, body := range []string{`{`, `{}`, `{"taskUid":null}`, "short-read", strings.Repeat("x", maxIndexSettingsResponseBytes+1)} {
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			if body == "short-read" {
				writer.Header().Set("Content-Length", "100")
			}
			writer.WriteHeader(http.StatusAccepted)
			if body == "short-read" {
				_, _ = writer.Write([]byte("{"))
				return
			}
			_, _ = writer.Write([]byte(body))
		}))
		settings := indexSettingsResource{provider: &providerData{host: server.URL, httpClient: server.Client()}}
		_, err := settings.patchSettings(context.Background(), "test", map[string]json.RawMessage{"stopWords": json.RawMessage("[]")})
		server.Close()
		if err == nil || !indexSettingsResponseAccepted(err) {
			t.Fatalf("accepted response failure lost its acceptance marker: %v", err)
		}
	}

	if strings.Contains(indexSettingsErrorMessage(errors.New("synthetic-sensitive-detail")), "synthetic-sensitive-detail") {
		t.Fatal("unknown errors expose raw details")
	}
}

func TestSettingsContractStopWordNormalization(t *testing.T) {
	prior := types.SetValueMust(types.StringType, []attr.Value{types.StringValue("CAFÉ"), types.StringValue("ﬀ")})
	remote := types.SetValueMust(types.StringType, []attr.Value{types.StringValue("CAFE\u0301"), types.StringValue("ff")})
	if !canonicalStopWords(remote, prior).Equal(prior) {
		t.Fatal("normalization changed configured spelling")
	}

	request := validator.SetRequest{Path: path.Root("stop_words"), ConfigValue: types.SetValueMust(types.StringType, []attr.Value{types.StringValue("ﬀ"), types.StringValue("ff")})}
	var response validator.SetResponse
	indexSettingsStopWordsValidator{}.ValidateSet(context.Background(), request, &response)
	if !response.Diagnostics.HasError() {
		t.Fatal("normalization collisions should be rejected before apply")
	}
}

func TestSettingsContractLosslessStopWordTransformations(t *testing.T) {
	for _, test := range []struct {
		configured string
		remote     string
	}{
		{"ك", "ک"},
		{"يى", "یی"},
		{"۱۲۳۴۵۶۷۸۹۰", "1234567890"},
		{"،؟", ",?"},
		{"\u0002foo\u200c", "foo"},
		{"ÅÄÖ", "A\u030aA\u0308O\u0308"},
		{"\tfoo\n", "\tfoo\n"},
	} {
		prior := types.SetValueMust(types.StringType, []attr.Value{types.StringValue(test.configured)})
		remote := types.SetValueMust(types.StringType, []attr.Value{types.StringValue(test.remote)})
		if !canonicalStopWords(remote, prior).Equal(prior) {
			t.Fatalf("did not preserve configured spelling %q for remote %q", test.configured, test.remote)
		}
	}

	request := validator.SetRequest{Path: path.Root("stop_words"), ConfigValue: types.SetValueMust(types.StringType, []attr.Value{types.StringValue("ك"), types.StringValue("ک")})}
	var response validator.SetResponse
	indexSettingsStopWordsValidator{}.ValidateSet(context.Background(), request, &response)
	if !response.Diagnostics.HasError() {
		t.Fatal("Persian normalization collisions should be rejected")
	}

	if _, err := encodeIndexSettingsField(context.Background(), settingsFieldStopWords, request.ConfigValue); err == nil {
		t.Fatal("resolved normalization collisions must be rejected before sending an update")
	}
}
