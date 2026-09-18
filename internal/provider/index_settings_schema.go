package provider

import (
	"context"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func indexSettingsResourceSchema() schema.Schema {
	return schema.Schema{
		Description: "Manages selected core settings for one existing Meilisearch index; use one settings resource per index. The provider preflights the index and performs no deliberate index creation or deletion; use a settings-scoped credential without indexes.create to prevent implicit creation during concurrent deletion.\n\nInitially omitted or null fields remain unmanaged and appear as null in resource state; use the data source to read their server values. Removing a previously managed field sends an explicit null reset to the server default and relinquishes ownership. Destroy resets only owned fields and retains the index and documents.\n\nUID import adopts all eight typed core fields. Review configuration before applying: omitted imported fields will reset. Advanced filterable rules remain outside this resource's typed contract and are diagnosed when owned or read through the data source.",
		Attributes: map[string]schema.Attribute{
			"index_uid": schema.StringAttribute{
				Description: "UID of the existing Meilisearch index.",
				Required:    true,
				Validators:  []validator.String{indexSettingsNonEmptyStringValidator{}},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"searchable_attributes": indexSettingsListAttribute("Unique searchable attributes in priority order. The wildcard must be the only list entry when used."),
			"displayed_attributes":  indexSettingsListAttribute("Unique attributes returned in search results. The wildcard must be the only list entry when used."),
			"filterable_attributes": indexSettingsSetAttribute("Attributes available to filtering. Advanced filterable rules are not supported by this field."),
			"sortable_attributes":   indexSettingsSetAttribute("Attributes available to sorting."),
			"ranking_rules":         indexSettingsListAttribute("Ranking rules in evaluation order."),
			"stop_words":            indexSettingsStopWordsAttribute("Stop words ignored during search. Values must be distinct after Meilisearch stop-word normalization; equivalent server normalization preserves configured spelling."),
			"synonyms": schema.MapAttribute{
				Description: "Synonym groups keyed by their source term.",
				ElementType: types.ListType{ElemType: types.StringType},
				Optional:    true,
			},
			"distinct_attribute": schema.StringAttribute{
				Description: "Attribute used to deduplicate search results.",
				Optional:    true,
				Validators:  []validator.String{indexSettingsNonEmptyStringValidator{}},
			},
			"managed_fields": schema.SetAttribute{
				Description: "Core settings currently owned by this resource.",
				ElementType: types.StringType,
				Computed:    true,
			},
			"id": schema.StringAttribute{
				Description: "Remote index UID.",
				Computed:    true,
			},
		},
	}
}

func indexSettingsListAttribute(description string) schema.ListAttribute {
	return schema.ListAttribute{
		Description: description,
		ElementType: types.StringType,
		Optional:    true,
		Validators:  []validator.List{indexSettingsOrderedListValidator{}},
	}
}

func indexSettingsSetAttribute(description string) schema.SetAttribute {
	return schema.SetAttribute{
		Description: description,
		ElementType: types.StringType,
		Optional:    true,
	}
}

func indexSettingsStopWordsAttribute(description string) schema.SetAttribute {
	attribute := indexSettingsSetAttribute(description)
	attribute.Validators = []validator.Set{indexSettingsStopWordsValidator{}}
	return attribute
}

type indexSettingsOrderedListValidator struct{}

func (indexSettingsOrderedListValidator) Description(context.Context) string {
	return "The list must not contain duplicates or combine the wildcard with other attributes."
}

func (v indexSettingsOrderedListValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (indexSettingsOrderedListValidator) ValidateList(_ context.Context, req validator.ListRequest, resp *validator.ListResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	seen := make(map[string]struct{})
	wildcard := false
	for _, element := range req.ConfigValue.Elements() {
		value, ok := element.(types.String)
		if !ok || value.IsUnknown() || value.IsNull() {
			return
		}
		if _, exists := seen[value.ValueString()]; exists {
			resp.Diagnostics.AddAttributeError(req.Path, "Invalid ordered index settings list", "The list must not contain duplicate attributes or rules.")
			return
		}
		seen[value.ValueString()] = struct{}{}
		if value.ValueString() == "*" {
			wildcard = true
		}
	}
	if wildcard && len(seen) > 1 {
		resp.Diagnostics.AddAttributeError(req.Path, "Invalid wildcard index settings list", "The wildcard must be the only value in searchable_attributes or displayed_attributes.")
	}
}

type indexSettingsNonEmptyStringValidator struct{}

func (indexSettingsNonEmptyStringValidator) Description(context.Context) string {
	return "The value must be non-empty when configured."
}

type indexSettingsStopWordsValidator struct{}

func (indexSettingsStopWordsValidator) Description(context.Context) string {
	return "Stop words must not contain values that normalize to the same server value."
}

func (v indexSettingsStopWordsValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (indexSettingsStopWordsValidator) ValidateSet(_ context.Context, req validator.SetRequest, resp *validator.SetResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	seen := make(map[string]struct{})
	for _, element := range req.ConfigValue.Elements() {
		value, ok := element.(types.String)
		if !ok || value.IsUnknown() || value.IsNull() {
			return
		}
		normalized := indexSettingsStopWordKey(value.ValueString())
		if _, exists := seen[normalized]; exists {
			resp.Diagnostics.AddAttributeError(req.Path, "Invalid stop_words", "Stop words that normalize to the same server value cannot be managed together.")
			return
		}
		seen[normalized] = struct{}{}
	}
}

func (v indexSettingsNonEmptyStringValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (indexSettingsNonEmptyStringValidator) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if !req.ConfigValue.IsNull() && !req.ConfigValue.IsUnknown() && req.ConfigValue.ValueString() == "" {
		resp.Diagnostics.AddAttributeError(req.Path, "Invalid index settings string", "The value must be non-empty when configured.")
	}
}
