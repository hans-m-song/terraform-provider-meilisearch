package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"sort"
	"strings"
)

const (
	maxIndexSettingsResponseBytes = 1 << 20
	settingsFieldSearchable       = "searchable_attributes"
	settingsFieldDisplayed        = "displayed_attributes"
	settingsFieldFilterable       = "filterable_attributes"
	settingsFieldSortable         = "sortable_attributes"
	settingsFieldRanking          = "ranking_rules"
	settingsFieldStopWords        = "stop_words"
	settingsFieldSynonyms         = "synonyms"
	settingsFieldDistinct         = "distinct_attribute"
)

var indexSettingsFields = []string{
	settingsFieldSearchable,
	settingsFieldDisplayed,
	settingsFieldFilterable,
	settingsFieldSortable,
	settingsFieldRanking,
	settingsFieldStopWords,
	settingsFieldSynonyms,
	settingsFieldDistinct,
}

var indexSettingsJSONFields = map[string]string{
	settingsFieldSearchable: "searchableAttributes",
	settingsFieldDisplayed:  "displayedAttributes",
	settingsFieldFilterable: "filterableAttributes",
	settingsFieldSortable:   "sortableAttributes",
	settingsFieldRanking:    "rankingRules",
	settingsFieldStopWords:  "stopWords",
	settingsFieldSynonyms:   "synonyms",
	settingsFieldDistinct:   "distinctAttribute",
}

type indexSettingsResourceModel struct {
	IndexUID             types.String `tfsdk:"index_uid"`
	SearchableAttributes types.List   `tfsdk:"searchable_attributes"`
	DisplayedAttributes  types.List   `tfsdk:"displayed_attributes"`
	FilterableAttributes types.Set    `tfsdk:"filterable_attributes"`
	SortableAttributes   types.Set    `tfsdk:"sortable_attributes"`
	RankingRules         types.List   `tfsdk:"ranking_rules"`
	StopWords            types.Set    `tfsdk:"stop_words"`
	Synonyms             types.Map    `tfsdk:"synonyms"`
	DistinctAttribute    types.String `tfsdk:"distinct_attribute"`
	ManagedFields        types.Set    `tfsdk:"managed_fields"`
	ID                   types.String `tfsdk:"id"`
}

type indexSettingsValues struct {
	SearchableAttributes types.List
	DisplayedAttributes  types.List
	FilterableAttributes types.Set
	SortableAttributes   types.Set
	RankingRules         types.List
	StopWords            types.Set
	Synonyms             types.Map
	DistinctAttribute    types.String
	FilterableAdvanced   bool
}

func decodeIndexSettings(body []byte) (indexSettingsValues, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return indexSettingsValues{}, errors.New("meilisearch returned an invalid index settings response")
	}
	if raw == nil {
		return indexSettingsValues{}, errors.New("meilisearch returned an invalid index settings response")
	}

	for _, field := range indexSettingsFields {
		value, exists := raw[indexSettingsJSONFields[field]]
		if !exists || (field != settingsFieldDistinct && strings.TrimSpace(string(value)) == "null") {
			return indexSettingsValues{}, fmt.Errorf("meilisearch returned no valid %s setting", indexSettingsJSONFields[field])
		}
	}

	values := indexSettingsValues{
		SearchableAttributes: types.ListNull(types.StringType),
		DisplayedAttributes:  types.ListNull(types.StringType),
		FilterableAttributes: types.SetNull(types.StringType),
		SortableAttributes:   types.SetNull(types.StringType),
		RankingRules:         types.ListNull(types.StringType),
		StopWords:            types.SetNull(types.StringType),
		Synonyms:             types.MapNull(types.ListType{ElemType: types.StringType}),
		DistinctAttribute:    types.StringNull(),
	}

	var err error
	values.SearchableAttributes, err = decodeStringList(raw[indexSettingsJSONFields[settingsFieldSearchable]])
	if err != nil {
		return indexSettingsValues{}, fmt.Errorf("decode searchableAttributes: %w", err)
	}
	values.DisplayedAttributes, err = decodeStringList(raw[indexSettingsJSONFields[settingsFieldDisplayed]])
	if err != nil {
		return indexSettingsValues{}, fmt.Errorf("decode displayedAttributes: %w", err)
	}
	values.FilterableAttributes, values.FilterableAdvanced, err = decodeFilterable(raw[indexSettingsJSONFields[settingsFieldFilterable]])
	if err != nil {
		return indexSettingsValues{}, fmt.Errorf("decode filterableAttributes: %w", err)
	}
	values.SortableAttributes, err = decodeStringSet(raw[indexSettingsJSONFields[settingsFieldSortable]])
	if err != nil {
		return indexSettingsValues{}, fmt.Errorf("decode sortableAttributes: %w", err)
	}
	values.RankingRules, err = decodeStringList(raw[indexSettingsJSONFields[settingsFieldRanking]])
	if err != nil {
		return indexSettingsValues{}, fmt.Errorf("decode rankingRules: %w", err)
	}
	values.StopWords, err = decodeStringSet(raw[indexSettingsJSONFields[settingsFieldStopWords]])
	if err != nil {
		return indexSettingsValues{}, fmt.Errorf("decode stopWords: %w", err)
	}
	values.Synonyms, err = decodeSynonyms(raw[indexSettingsJSONFields[settingsFieldSynonyms]])
	if err != nil {
		return indexSettingsValues{}, fmt.Errorf("decode synonyms: %w", err)
	}
	values.DistinctAttribute, err = decodeString(raw[indexSettingsJSONFields[settingsFieldDistinct]])
	if err != nil {
		return indexSettingsValues{}, fmt.Errorf("decode distinctAttribute: %w", err)
	}

	return values, nil
}

func decodeSettingsStringArray(raw json.RawMessage) ([]string, error) {
	var elements []json.RawMessage
	if err := json.Unmarshal(raw, &elements); err != nil || elements == nil {
		return nil, errors.New("expected an array of strings")
	}

	values := make([]string, 0, len(elements))
	for _, element := range elements {
		var value string
		if strings.TrimSpace(string(element)) == "null" {
			return nil, errors.New("expected an array of strings")
		}
		if err := json.Unmarshal(element, &value); err != nil {
			return nil, errors.New("expected an array of strings")
		}
		values = append(values, value)
	}

	return values, nil
}

func decodeStringList(raw json.RawMessage) (types.List, error) {
	if len(raw) == 0 || strings.TrimSpace(string(raw)) == "null" {
		return types.ListNull(types.StringType), nil
	}

	values, err := decodeSettingsStringArray(raw)
	if err != nil {
		return types.List{}, err
	}

	result, diags := types.ListValueFrom(context.Background(), types.StringType, values)
	if diags.HasError() {
		return types.List{}, errors.New("invalid string list")
	}
	return result, nil
}

func decodeStringSet(raw json.RawMessage) (types.Set, error) {
	if len(raw) == 0 || strings.TrimSpace(string(raw)) == "null" {
		return types.SetNull(types.StringType), nil
	}

	values, err := decodeSettingsStringArray(raw)
	if err != nil {
		return types.Set{}, err
	}

	result, diags := types.SetValueFrom(context.Background(), types.StringType, values)
	if diags.HasError() {
		return types.Set{}, errors.New("invalid string set")
	}
	return result, nil
}

func decodeFilterable(raw json.RawMessage) (types.Set, bool, error) {
	if len(raw) == 0 || strings.TrimSpace(string(raw)) == "null" {
		return types.SetNull(types.StringType), false, nil
	}

	var values []json.RawMessage
	if err := json.Unmarshal(raw, &values); err != nil || values == nil {
		return types.Set{}, false, errors.New("expected an array of strings or advanced filter rules")
	}

	stringsOnly := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(string(value))
		if strings.HasPrefix(trimmed, "{") {
			return types.SetNull(types.StringType), true, nil
		}
		var attribute string
		if err := json.Unmarshal(value, &attribute); err != nil || trimmed == "null" {
			return types.Set{}, false, errors.New("expected strings or advanced filter rule objects")
		}
		stringsOnly = append(stringsOnly, attribute)
	}

	set, diags := types.SetValueFrom(context.Background(), types.StringType, stringsOnly)
	if diags.HasError() {
		return types.Set{}, false, errors.New("invalid filterable attribute set")
	}
	return set, false, nil
}

func decodeSynonyms(raw json.RawMessage) (types.Map, error) {
	if len(raw) == 0 || strings.TrimSpace(string(raw)) == "null" {
		return types.MapNull(types.ListType{ElemType: types.StringType}), nil
	}

	var values map[string]json.RawMessage
	if err := json.Unmarshal(raw, &values); err != nil || values == nil {
		return types.Map{}, errors.New("expected an object of string arrays")
	}

	result := make(map[string]attrValueList, len(values))
	for key, value := range values {
		synonyms, err := decodeSettingsStringArray(value)
		if err != nil {
			return types.Map{}, errors.New("expected each synonym value to be an array of strings")
		}
		result[key] = attrValueList(synonyms)
	}

	return synonymsMapValue(result)
}

func decodeString(raw json.RawMessage) (types.String, error) {
	if len(raw) == 0 || strings.TrimSpace(string(raw)) == "null" {
		return types.StringNull(), nil
	}

	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return types.String{}, errors.New("expected a string or null")
	}

	return types.StringValue(value), nil
}

func indexSettingsFieldSet(fields []string) types.Set {
	elements := make([]attr.Value, 0, len(fields))
	for _, field := range fields {
		elements = append(elements, types.StringValue(field))
	}
	return types.SetValueMust(types.StringType, elements)
}

func indexSettingsFieldNames(fields map[string]bool) []string {
	result := make([]string, 0, len(fields))
	for _, field := range indexSettingsFields {
		if fields[field] {
			result = append(result, field)
		}
	}
	return result
}

func indexSettingsConfiguredFields(model indexSettingsResourceModel) (map[string]bool, bool) {
	managed := make(map[string]bool, len(indexSettingsFields))
	unknown := false
	for _, field := range indexSettingsFields {
		value := indexSettingsModelField(model, field)
		if value.IsUnknown() {
			unknown = true
			continue
		}
		if !value.IsNull() {
			managed[field] = true
		}
	}
	return managed, unknown
}

func indexSettingsModelManagedFields(model indexSettingsResourceModel) (map[string]bool, bool) {
	if model.ManagedFields.IsNull() {
		return nil, true
	}
	if model.ManagedFields.IsUnknown() {
		return nil, true
	}

	managed := make(map[string]bool)
	for _, value := range model.ManagedFields.Elements() {
		field, ok := value.(types.String)
		if !ok || field.IsUnknown() || field.IsNull() {
			return nil, true
		}
		managed[field.ValueString()] = true
	}
	return managed, false
}

func indexSettingsModelField(model indexSettingsResourceModel, field string) attr.Value {
	switch field {
	case settingsFieldSearchable:
		return model.SearchableAttributes
	case settingsFieldDisplayed:
		return model.DisplayedAttributes
	case settingsFieldFilterable:
		return model.FilterableAttributes
	case settingsFieldSortable:
		return model.SortableAttributes
	case settingsFieldRanking:
		return model.RankingRules
	case settingsFieldStopWords:
		return model.StopWords
	case settingsFieldSynonyms:
		return model.Synonyms
	case settingsFieldDistinct:
		return model.DistinctAttribute
	default:
		return types.StringNull()
	}
}

func indexSettingsSetModelField(model *indexSettingsResourceModel, field string, value attr.Value) bool {
	var valid bool

	switch field {
	case settingsFieldSearchable:
		model.SearchableAttributes, valid = value.(types.List)
	case settingsFieldDisplayed:
		model.DisplayedAttributes, valid = value.(types.List)
	case settingsFieldFilterable:
		model.FilterableAttributes, valid = value.(types.Set)
	case settingsFieldSortable:
		model.SortableAttributes, valid = value.(types.Set)
	case settingsFieldRanking:
		model.RankingRules, valid = value.(types.List)
	case settingsFieldStopWords:
		model.StopWords, valid = value.(types.Set)
	case settingsFieldSynonyms:
		model.Synonyms, valid = value.(types.Map)
	case settingsFieldDistinct:
		model.DistinctAttribute, valid = value.(types.String)
	}
	return valid
}

func indexSettingsPatch(ctx context.Context, plan indexSettingsResourceModel, state *indexSettingsResourceModel, desired, previous map[string]bool) (map[string]json.RawMessage, error) {
	patch := make(map[string]json.RawMessage)
	for _, field := range indexSettingsFields {
		want := desired[field]
		had := previous[field]
		if want {
			value := indexSettingsModelField(plan, field)
			if value.IsUnknown() || value.IsNull() {
				if state != nil && had {
					value = indexSettingsModelField(*state, field)
				} else {
					return nil, fmt.Errorf("%s must be known and non-null when managed", field)
				}
			}
			if !had || state == nil || !indexSettingsValueEqual(field, value, indexSettingsModelField(*state, field)) {
				encoded, err := encodeIndexSettingsField(ctx, field, value)
				if err != nil {
					return nil, err
				}
				patch[indexSettingsJSONFields[field]] = encoded
			}
			continue
		}

		if had {
			patch[indexSettingsJSONFields[field]] = json.RawMessage("null")
		}
	}

	return patch, nil
}

func encodeIndexSettingsField(ctx context.Context, field string, value attr.Value) (json.RawMessage, error) {
	if value.IsNull() || value.IsUnknown() {
		return nil, fmt.Errorf("%s must be known and non-null when managed", field)
	}

	var encoded any
	switch field {
	case settingsFieldSearchable, settingsFieldDisplayed, settingsFieldRanking:
		list, ok := value.(types.List)
		if !ok {
			return nil, fmt.Errorf("%s has an invalid Terraform type", field)
		}
		var values []string
		if diags := list.ElementsAs(ctx, &values, false); diags.HasError() {
			return nil, fmt.Errorf("%s must contain only strings", field)
		}
		if values == nil {
			values = []string{}
		}

		seen := make(map[string]bool, len(values))
		for _, value := range values {
			if seen[value] {
				return nil, fmt.Errorf("%s must not contain duplicate values", field)
			}
			seen[value] = true
		}
		if (field == settingsFieldSearchable || field == settingsFieldDisplayed) && seen["*"] && len(values) > 1 {
			return nil, fmt.Errorf("%s must use the wildcard as its only value", field)
		}

		encoded = values
	case settingsFieldFilterable, settingsFieldSortable, settingsFieldStopWords:
		set, ok := value.(types.Set)
		if !ok {
			return nil, fmt.Errorf("%s has an invalid Terraform type", field)
		}
		var values []string
		if diags := set.ElementsAs(ctx, &values, false); diags.HasError() {
			return nil, fmt.Errorf("%s must contain only strings", field)
		}
		if values == nil {
			values = []string{}
		}

		if field == settingsFieldStopWords {
			seen := make(map[string]bool, len(values))
			for _, value := range values {
				key := indexSettingsStopWordKey(value)
				if seen[key] {
					return nil, errors.New("stop_words must be distinct after server normalization")
				}
				seen[key] = true
			}
		}

		encoded = values
	case settingsFieldSynonyms:
		m, ok := value.(types.Map)
		if !ok {
			return nil, fmt.Errorf("%s has an invalid Terraform type", field)
		}
		values := make(map[string][]string, len(m.Elements()))
		for key, element := range m.Elements() {
			list, ok := element.(types.List)
			if !ok || list.IsNull() || list.IsUnknown() {
				return nil, fmt.Errorf("%s values must be lists of strings", field)
			}
			var synonyms []string
			if diags := list.ElementsAs(ctx, &synonyms, false); diags.HasError() {
				return nil, fmt.Errorf("%s values must contain only strings", field)
			}
			if synonyms == nil {
				synonyms = []string{}
			}
			values[key] = synonyms
		}
		encoded = values
	case settingsFieldDistinct:
		stringValue, ok := value.(types.String)
		if !ok || stringValue.ValueString() == "" {
			return nil, errors.New("distinct_attribute must be non-empty when configured")
		}
		encoded = stringValue.ValueString()
	default:
		return nil, fmt.Errorf("unsupported index settings field %q", field)
	}

	return json.Marshal(encoded)
}

func indexSettingsPendingState(plan indexSettingsResourceModel, prior *indexSettingsResourceModel, desired, previous map[string]bool, uid string) indexSettingsResourceModel {
	state := indexSettingsResourceModel{
		IndexUID:             types.StringValue(uid),
		SearchableAttributes: types.ListNull(types.StringType),
		DisplayedAttributes:  types.ListNull(types.StringType),
		FilterableAttributes: types.SetNull(types.StringType),
		SortableAttributes:   types.SetNull(types.StringType),
		RankingRules:         types.ListNull(types.StringType),
		StopWords:            types.SetNull(types.StringType),
		Synonyms:             types.MapNull(types.ListType{ElemType: types.StringType}),
		DistinctAttribute:    types.StringNull(),
		ID:                   types.StringValue(uid),
		ManagedFields:        indexSettingsFieldSet(indexSettingsFieldsFromMaps(desired, previous)),
	}
	for _, field := range indexSettingsFields {
		if desired[field] {
			value := indexSettingsModelField(plan, field)
			if (value.IsNull() || value.IsUnknown()) && prior != nil {
				value = indexSettingsModelField(*prior, field)
			}
			if value != nil {
				indexSettingsSetModelField(&state, field, value)
			}
		} else if prior != nil && previous[field] {
			indexSettingsSetModelField(&state, field, indexSettingsModelField(*prior, field))
		}
	}
	return state
}

func indexSettingsFieldsFromMaps(desired, previous map[string]bool) []string {
	fields := make([]string, 0, len(indexSettingsFields))
	for _, field := range indexSettingsFields {
		if desired[field] || previous[field] {
			fields = append(fields, field)
		}
	}
	return fields
}

func indexSettingsStateFromValues(_ context.Context, uid string, values indexSettingsValues, managed map[string]bool, prior *indexSettingsResourceModel) (indexSettingsResourceModel, bool, error) {
	state := indexSettingsResourceModel{
		IndexUID:             types.StringValue(uid),
		SearchableAttributes: types.ListNull(types.StringType),
		DisplayedAttributes:  types.ListNull(types.StringType),
		FilterableAttributes: types.SetNull(types.StringType),
		SortableAttributes:   types.SetNull(types.StringType),
		RankingRules:         types.ListNull(types.StringType),
		StopWords:            types.SetNull(types.StringType),
		Synonyms:             types.MapNull(types.ListType{ElemType: types.StringType}),
		DistinctAttribute:    types.StringNull(),
		ID:                   types.StringValue(uid),
		ManagedFields:        indexSettingsFieldSet(indexSettingsFieldsFromMaps(managed, nil)),
	}

	unsupported := false
	for _, field := range indexSettingsFields {
		if !managed[field] {
			continue
		}
		if field == settingsFieldFilterable && values.FilterableAdvanced {
			unsupported = true
			if prior != nil {
				state.FilterableAttributes = prior.FilterableAttributes
			}
			continue
		}
		value := indexSettingsValuesField(values, field)
		if field == settingsFieldStopWords && prior != nil {
			remoteWords, valid := value.(types.Set)
			if !valid {
				return indexSettingsResourceModel{}, false, errors.New("invalid stop-word state type")
			}
			value = canonicalStopWords(remoteWords, prior.StopWords)
		}
		if !indexSettingsSetModelField(&state, field, value) {
			return indexSettingsResourceModel{}, false, fmt.Errorf("invalid state type for %s", field)
		}
	}

	return state, unsupported, nil
}

func indexSettingsValuesField(values indexSettingsValues, field string) attr.Value {
	switch field {
	case settingsFieldSearchable:
		return values.SearchableAttributes
	case settingsFieldDisplayed:
		return values.DisplayedAttributes
	case settingsFieldFilterable:
		return values.FilterableAttributes
	case settingsFieldSortable:
		return values.SortableAttributes
	case settingsFieldRanking:
		return values.RankingRules
	case settingsFieldStopWords:
		return values.StopWords
	case settingsFieldSynonyms:
		return values.Synonyms
	case settingsFieldDistinct:
		return values.DistinctAttribute
	default:
		return types.StringNull()
	}
}

func indexSettingsValueEqual(field string, left, right attr.Value) bool {
	if field != settingsFieldStopWords || left.IsNull() || left.IsUnknown() || right.IsNull() || right.IsUnknown() {
		return left.Equal(right)
	}

	leftSet, leftOK := left.(types.Set)
	rightSet, rightOK := right.(types.Set)
	if !leftOK || !rightOK {
		return left.Equal(right)
	}

	leftWords, leftValid := stopWordsValues(leftSet)
	rightWords, rightValid := stopWordsValues(rightSet)
	if !leftValid || !rightValid {
		return left.Equal(right)
	}
	return normalizedStopWordsEqual(leftWords, rightWords)
}

func canonicalStopWords(remote, prior types.Set) types.Set {
	if remote.IsNull() || remote.IsUnknown() || prior.IsNull() || prior.IsUnknown() {
		return remote
	}

	remoteWords, remoteOK := stopWordsValues(remote)
	priorWords, priorOK := stopWordsValues(prior)
	if !remoteOK || !priorOK {
		return remote
	}

	priorByNormalized := make(map[string]string, len(priorWords))
	for _, word := range priorWords {
		priorByNormalized[indexSettingsStopWordKey(word)] = word
	}
	canonical := make([]string, 0, len(remoteWords))
	for _, word := range remoteWords {
		if priorWord, ok := priorByNormalized[indexSettingsStopWordKey(word)]; ok {
			canonical = append(canonical, priorWord)
			continue
		}
		return remote
	}

	if len(canonical) != len(priorWords) {
		return remote
	}
	result, diags := types.SetValueFrom(context.Background(), types.StringType, canonical)
	if diags.HasError() {
		return remote
	}
	return result
}

func stopWordsValues(value types.Set) ([]string, bool) {
	var values []string
	if diags := value.ElementsAs(context.Background(), &values, false); diags.HasError() {
		return nil, false
	}
	return values, true
}

func normalizedStopWords(values []string) []string {
	normalized := make([]string, len(values))
	for index, value := range values {
		normalized[index] = indexSettingsStopWordKey(value)
	}
	sort.Strings(normalized)
	return normalized
}

func normalizedStopWordsEqual(left, right []string) bool {
	leftNormalized := normalizedStopWords(left)
	rightNormalized := normalizedStopWords(right)
	if len(leftNormalized) != len(rightNormalized) {
		return false
	}
	for index := range leftNormalized {
		if leftNormalized[index] != rightNormalized[index] {
			return false
		}
	}
	return true
}

type attrValueList []string

func synonymsMapValue(values map[string]attrValueList) (types.Map, error) {
	elements := make(map[string]attr.Value, len(values))
	for key, synonyms := range values {
		list, diags := types.ListValueFrom(context.Background(), types.StringType, []string(synonyms))
		if diags.HasError() {
			return types.Map{}, errors.New("invalid synonym value")
		}
		elements[key] = list
	}

	result, diags := types.MapValue(types.ListType{ElemType: types.StringType}, elements)
	if diags.HasError() {
		return types.Map{}, errors.New("invalid synonym map")
	}
	return result, nil
}
