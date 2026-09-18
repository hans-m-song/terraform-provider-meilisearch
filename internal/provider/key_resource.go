package provider

import (
	"context"
	"errors"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/meilisearch/meilisearch-go"
)

var (
	_ resource.Resource                = &keyResource{}
	_ resource.ResourceWithConfigure   = &keyResource{}
	_ resource.ResourceWithImportState = &keyResource{}
)

func NewKeyResource() resource.Resource {
	return &keyResource{}
}

type keyResource struct {
	client           meilisearch.ServiceManager
	operationTimeout time.Duration
	provider         *providerData
}

type keyResourceModel struct {
	UID         types.String `tfsdk:"uid"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	Key         types.String `tfsdk:"key"`
	Actions     types.Set    `tfsdk:"actions"`
	Indexes     types.Set    `tfsdk:"indexes"`
	ExpiresAt   types.String `tfsdk:"expires_at"`
	CreatedAt   types.String `tfsdk:"created_at"`
	UpdatedAt   types.String `tfsdk:"updated_at"`
	ID          types.String `tfsdk:"id"`
}

func (r *keyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_key"
}

func (r *keyResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Meilisearch API key.",
		Attributes: map[string]schema.Attribute{
			"uid": schema.StringAttribute{
				Description: "UID used by Meilisearch to identify the key.",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				Description: "Name of the key.",
				Optional:    true,
			},
			"description": schema.StringAttribute{
				Description: "Description of the key.",
				Optional:    true,
			},
			"key": schema.StringAttribute{
				Description: "Actual key value.",
				Computed:    true,
				Sensitive:   true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"actions": schema.SetAttribute{
				Description: "Actions permitted for the key.",
				ElementType: types.StringType,
				Required:    true,
				PlanModifiers: []planmodifier.Set{
					setplanmodifier.RequiresReplace(),
				},
			},
			"indexes": schema.SetAttribute{
				Description: "Indexes the key is authorized to act on.",
				ElementType: types.StringType,
				Required:    true,
				PlanModifiers: []planmodifier.Set{
					setplanmodifier.RequiresReplace(),
				},
			},
			"expires_at": schema.StringAttribute{
				Description: "Optional expiration timestamp (RFC3339). Removing it recreates the key without an expiration.",
				Optional:    true,
				Validators: []validator.String{
					expiresAtValidator{},
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"created_at": schema.StringAttribute{
				Description: "Date and time when the key was created (RFC3339).",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"updated_at": schema.StringAttribute{
				Description: "Date and time when the key was last updated (RFC3339).",
				Computed:    true,
			},
			"id": schema.StringAttribute{
				Description: "Remote API key UID.",
				Computed:    true,
			},
		},
	}
}

func (r *keyResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	data, ok := req.ProviderData.(*providerData)
	if !ok || data == nil || data.client == nil {
		resp.Diagnostics.AddError(
			"Invalid provider data",
			"The Meilisearch key resource requires provider data created by the Meilisearch provider.",
		)
		return
	}

	r.client = data.client
	r.operationTimeout = data.operationTimeout
	r.provider = data
	if r.operationTimeout <= 0 {
		r.operationTimeout = defaultOperationTimeout
	}
}

func (r *keyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan keyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if r.client == nil {
		resp.Diagnostics.AddError("Key resource is not configured", "The Meilisearch provider did not supply a client.")
		return
	}

	var actions []string
	var indexes []string
	resp.Diagnostics.Append(plan.Actions.ElementsAs(ctx, &actions, false)...)
	resp.Diagnostics.Append(plan.Indexes.ElementsAs(ctx, &indexes, false)...)
	if resp.Diagnostics.HasError() {
		return
	}

	expiresAt, err := parseExpiresAt(plan.ExpiresAt)
	if err != nil {
		resp.Diagnostics.AddError("Invalid key expiration", "expires_at must be an RFC3339 timestamp.")
		return
	}

	operationCtx, cancel := operationContext(ctx, r.operationTimeout)
	defer cancel()

	key, err := r.client.CreateKeyWithContext(operationCtx, &meilisearch.Key{
		UID:         plan.UID.ValueString(),
		Name:        plan.Name.ValueString(),
		Description: plan.Description.ValueString(),
		Actions:     actions,
		Indexes:     indexes,
		ExpiresAt:   expiresAt,
	})
	if err != nil {
		resp.Diagnostics.AddError("Error creating key", "Could not create key: "+apiError(err))
		return
	}
	if key == nil || key.UID == "" {
		resp.Diagnostics.AddError("Error creating key", "Meilisearch returned no API key UID.")
		return
	}

	state, diags := keyResourceState(ctx, key, &plan, true)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *keyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state keyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if r.client == nil {
		resp.Diagnostics.AddError("Key resource is not configured", "The Meilisearch provider did not supply a client.")
		return
	}

	operationCtx, cancel := operationContext(ctx, r.operationTimeout)
	defer cancel()

	uid := state.UID.ValueString()
	key, err := r.client.GetKeyWithContext(operationCtx, uid)
	if err != nil {
		if isAPIError(err, "api_key_not_found") || isAPIError(err, "key_not_found") {
			resp.State.RemoveResource(ctx)
			return
		}

		resp.Diagnostics.AddError("Error reading Meilisearch API key", "Could not read key "+uid+": "+apiError(err))
		return
	}
	if key == nil || key.UID == "" {
		resp.Diagnostics.AddError("Error reading Meilisearch API key", "Meilisearch returned no API key UID.")
		return
	}

	refreshedState, diags := keyResourceState(ctx, key, &state, false)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &refreshedState)...)
}

func (r *keyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan keyResourceModel
	var state keyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if r.client == nil {
		resp.Diagnostics.AddError("Key resource is not configured", "The Meilisearch provider did not supply a client.")
		return
	}

	operationCtx, cancel := operationContext(ctx, r.operationTimeout)
	defer cancel()

	key, err := r.updateKeyWithContext(operationCtx, state.UID.ValueString(), stringPointer(plan.Name), stringPointer(plan.Description))
	if err != nil {
		resp.Diagnostics.AddError("Error updating Meilisearch API key", "Could not update key: "+apiError(err))
		return
	}
	if key == nil || key.UID == "" {
		key, err = r.client.GetKeyWithContext(operationCtx, state.UID.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Error reading updated Meilisearch API key", "Could not read key "+state.UID.ValueString()+": "+apiError(err))
			return
		}
	}
	if key == nil || key.UID == "" {
		resp.Diagnostics.AddError("Error updating Meilisearch API key", "Meilisearch returned no API key UID.")
		return
	}

	refreshedState, diags := keyResourceState(ctx, key, &plan, true)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &refreshedState)...)
}

func (r *keyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state keyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if r.client == nil {
		resp.Diagnostics.AddError("Key resource is not configured", "The Meilisearch provider did not supply a client.")
		return
	}

	operationCtx, cancel := operationContext(ctx, r.operationTimeout)
	defer cancel()

	_, err := r.client.DeleteKeyWithContext(operationCtx, state.UID.ValueString())
	if err != nil && !isAPIError(err, "api_key_not_found") && !isAPIError(err, "key_not_found") {
		resp.Diagnostics.AddError("Error deleting Meilisearch API key", "Could not delete key "+state.UID.ValueString()+": "+apiError(err))
	}
}

func (r *keyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("uid"), req, resp)
}

func parseExpiresAt(value types.String) (time.Time, error) {
	if value.IsNull() || value.IsUnknown() {
		return time.Time{}, nil
	}
	if value.ValueString() == "" {
		return time.Time{}, errors.New("expires_at must be a non-empty RFC3339 timestamp")
	}

	return time.Parse(time.RFC3339, value.ValueString())
}

type expiresAtValidator struct{}

func (expiresAtValidator) Description(context.Context) string {
	return "expires_at must be a non-empty RFC3339 timestamp when configured."
}

func (v expiresAtValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (expiresAtValidator) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	if req.ConfigValue.ValueString() == "" {
		resp.Diagnostics.AddAttributeError(req.Path, "Invalid key expiration", "expires_at must be a non-empty RFC3339 timestamp when configured.")
		return
	}

	if _, err := time.Parse(time.RFC3339, req.ConfigValue.ValueString()); err != nil {
		resp.Diagnostics.AddAttributeError(req.Path, "Invalid key expiration", "expires_at must be a non-empty RFC3339 timestamp when configured.")
	}
}

func keyResourceState(ctx context.Context, key *meilisearch.Key, configured *keyResourceModel, preserveConfiguredScopes bool) (keyResourceModel, diag.Diagnostics) {
	state := keyResourceModel{
		UID:         types.StringValue(key.UID),
		Name:        optionalStringValue(key.Name, configuredString(configured, func(model *keyResourceModel) types.String { return model.Name })),
		Description: optionalStringValue(key.Description, configuredString(configured, func(model *keyResourceModel) types.String { return model.Description })),
		CreatedAt:   timestampValue(key.CreatedAt),
		UpdatedAt:   timestampValue(key.UpdatedAt),
		ExpiresAt:   expiresAtValue(key.ExpiresAt, configuredString(configured, func(model *keyResourceModel) types.String { return model.ExpiresAt })),
		ID:          types.StringValue(key.UID),
	}

	if key.Key != "" {
		state.Key = types.StringValue(key.Key)
	} else if configured != nil && !configured.Key.IsNull() && !configured.Key.IsUnknown() {
		state.Key = configured.Key
	} else {
		state.Key = types.StringNull()
	}

	scopes := types.SetNull(types.StringType)
	if preserveConfiguredScopes {
		scopes = configuredSet(configured, func(model *keyResourceModel) types.Set { return model.Actions })
	}
	actions, diags := keySetValue(ctx, key.Actions, scopes)
	state.Actions = actions
	allDiags := diags

	if preserveConfiguredScopes {
		scopes = configuredSet(configured, func(model *keyResourceModel) types.Set { return model.Indexes })
	} else {
		scopes = types.SetNull(types.StringType)
	}
	indexes, diags := keySetValue(ctx, key.Indexes, scopes)
	state.Indexes = indexes
	allDiags.Append(diags...)

	return state, allDiags
}

func configuredString(model *keyResourceModel, get func(*keyResourceModel) types.String) types.String {
	if model == nil {
		return types.StringNull()
	}

	return get(model)
}

func configuredSet(model *keyResourceModel, get func(*keyResourceModel) types.Set) types.Set {
	if model == nil {
		return types.SetNull(types.StringType)
	}

	return get(model)
}

func optionalStringValue(remote string, configured types.String) types.String {
	if remote != "" {
		return types.StringValue(remote)
	}
	if !configured.IsNull() && !configured.IsUnknown() && configured.ValueString() == "" {
		return configured
	}

	return types.StringNull()
}

func stringPointer(value types.String) *string {
	if value.IsNull() || value.IsUnknown() {
		return nil
	}

	stringValue := value.ValueString()
	return &stringValue
}

func timestampValue(value time.Time) types.String {
	if value.IsZero() {
		return types.StringNull()
	}

	return types.StringValue(value.Format(time.RFC3339))
}

func expiresAtValue(remote time.Time, configured types.String) types.String {
	if remote.IsZero() {
		return types.StringNull()
	}

	if !configured.IsNull() && !configured.IsUnknown() && configured.ValueString() != "" {
		if parsed, err := time.Parse(time.RFC3339, configured.ValueString()); err == nil && parsed.Equal(remote) {
			return configured
		}
	}

	return types.StringValue(remote.Format(time.RFC3339))
}

func keySetValue(ctx context.Context, remote []string, configured types.Set) (types.Set, diag.Diagnostics) {
	if remote == nil && !configured.IsNull() && !configured.IsUnknown() {
		var fallback []string
		diags := configured.ElementsAs(ctx, &fallback, false)
		if diags.HasError() {
			return types.SetNull(types.StringType), diags
		}

		remote = fallback
	}

	return types.SetValueFrom(ctx, types.StringType, remote)
}
