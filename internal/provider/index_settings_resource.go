package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/meilisearch/meilisearch-go"
	"time"
)

var (
	_ resource.Resource                = &indexSettingsResource{}
	_ resource.ResourceWithConfigure   = &indexSettingsResource{}
	_ resource.ResourceWithImportState = &indexSettingsResource{}
	_ resource.ResourceWithModifyPlan  = &indexSettingsResource{}
)

func NewIndexSettingsResource() resource.Resource {
	return &indexSettingsResource{}
}

type indexSettingsResource struct {
	client           meilisearch.ServiceManager
	operationTimeout time.Duration
	provider         *providerData
}

func (r *indexSettingsResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_index_settings"
}

func (r *indexSettingsResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = indexSettingsResourceSchema()
}

func (r *indexSettingsResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	data, ok := req.ProviderData.(*providerData)
	if !ok || data == nil || data.client == nil {
		resp.Diagnostics.AddError(
			"Invalid provider data",
			"The Meilisearch index settings resource requires provider data created by the Meilisearch provider.",
		)
		return
	}

	r.client = data.client
	r.provider = data
	r.operationTimeout = data.operationTimeout
	if r.operationTimeout <= 0 {
		r.operationTimeout = defaultOperationTimeout
	}
}

func (r *indexSettingsResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.Plan.Raw.IsNull() {
		return
	}

	var config indexSettingsResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	managed, unknown := indexSettingsConfiguredFields(config)
	if unknown {
		resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, path.Root("managed_fields"), types.SetUnknown(types.StringType))...)
		return
	}

	resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, path.Root("managed_fields"), indexSettingsFieldSet(indexSettingsFieldNames(managed)))...)
}

func (r *indexSettingsResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan indexSettingsResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if !r.configured() {
		resp.Diagnostics.AddError("Index settings resource is not configured", "The Meilisearch provider did not supply a client.")
		return
	}

	managed, unknown := indexSettingsModelManagedFields(plan)
	if unknown {
		managed, unknown = indexSettingsConfiguredFields(plan)
	}
	if unknown {
		resp.Diagnostics.AddError("Unknown index settings ownership", "The provider cannot determine which settings are managed while configuration values are unknown.")
		return
	}

	operationCtx, cancel := operationContext(ctx, r.operationTimeout)
	defer cancel()

	uid := plan.IndexUID.ValueString()
	if err := r.ensureIndex(operationCtx, uid); err != nil {
		if isMissingIndexError(err) {
			resp.Diagnostics.AddError("Index settings parent not found", fmt.Sprintf("Could not find index %q: %s", uid, apiError(err)))
			return
		}
		resp.Diagnostics.AddError("Error checking index settings parent", apiError(err))
		return
	}

	patch, err := indexSettingsPatch(ctx, plan, nil, managed, nil)
	if err != nil {
		resp.Diagnostics.AddError("Invalid index settings", err.Error())
		return
	}

	pending := indexSettingsPendingState(plan, nil, managed, nil, uid)
	if len(patch) == 0 {
		resp.Diagnostics.Append(resp.State.Set(ctx, &pending)...)
		return
	}

	unsupportedBeforeMutation, err := r.checkFilterableMutation(operationCtx, uid, patch)
	if err != nil {
		resp.Diagnostics.AddError("Error checking index settings before mutation", indexSettingsErrorMessage(err))
		return
	}
	if unsupportedBeforeMutation {
		resp.Diagnostics.AddError("Unsupported advanced filterable attributes", "The index contains advanced filterable attribute rules that this resource cannot represent; settings were not changed and prior ownership is retained.")
		return
	}

	task, err := r.patchSettings(operationCtx, uid, patch)
	if err != nil {
		if indexSettingsResponseAccepted(err) {
			resp.Diagnostics.Append(resp.State.Set(ctx, &pending)...)
		}
		resp.Diagnostics.AddError("Error updating index settings", indexSettingsErrorMessage(err))
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &pending)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if task == nil || task.TaskUID == nil {
		resp.Diagnostics.AddError("Error updating index settings", "Meilisearch accepted the settings update without returning a task UID.")
		return
	}

	if _, err := waitTask(operationCtx, r.client, *task.TaskUID); err != nil {
		resp.Diagnostics.AddError("Error waiting for index settings update", apiError(err))
		return
	}

	refreshed, unsupported, err := r.refreshedState(operationCtx, uid, managed, &pending)
	if err != nil {
		resp.Diagnostics.AddError("Error reading index settings", apiError(err))
		return
	}
	if unsupported {
		resp.Diagnostics.AddError("Unsupported advanced filterable attributes", "The index contains advanced filterable attribute rules that this resource cannot represent; the field remains managed with its prior value.")
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &refreshed)...)
}

func (r *indexSettingsResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state indexSettingsResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if !r.configured() {
		resp.Diagnostics.AddError("Index settings resource is not configured", "The Meilisearch provider did not supply a client.")
		return
	}

	managed, unknown := indexSettingsModelManagedFields(state)
	if unknown {
		resp.Diagnostics.AddError("Unknown index settings ownership", "The provider cannot refresh index settings while managed_fields is unknown.")
		return
	}

	operationCtx, cancel := operationContext(ctx, r.operationTimeout)
	defer cancel()

	uid := state.IndexUID.ValueString()
	if err := r.ensureIndex(operationCtx, uid); err != nil {
		if isMissingIndexError(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error checking index settings parent", apiError(err))
		return
	}

	refreshed, unsupported, err := r.refreshedState(operationCtx, uid, managed, &state)
	if err != nil {
		if isMissingIndexError(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading index settings", apiError(err))
		return
	}
	if unsupported {
		resp.Diagnostics.AddError("Unsupported advanced filterable attributes", "The index contains advanced filterable attribute rules that this resource cannot represent; the field remains managed with its prior value.")
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &refreshed)...)
}

func (r *indexSettingsResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan indexSettingsResourceModel
	var state indexSettingsResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if !r.configured() {
		resp.Diagnostics.AddError("Index settings resource is not configured", "The Meilisearch provider did not supply a client.")
		return
	}

	desired, unknown := indexSettingsModelManagedFields(plan)
	if unknown {
		desired, unknown = indexSettingsConfiguredFields(plan)
	}
	if unknown {
		resp.Diagnostics.AddError("Unknown index settings ownership", "The provider cannot determine which settings are managed while configuration values are unknown.")
		return
	}
	owned, unknown := indexSettingsModelManagedFields(state)
	if unknown {
		resp.Diagnostics.AddError("Unknown index settings ownership", "The provider cannot update index settings while prior managed_fields is unknown.")
		return
	}

	operationCtx, cancel := operationContext(ctx, r.operationTimeout)
	defer cancel()

	uid := state.IndexUID.ValueString()
	if err := r.ensureIndex(operationCtx, uid); err != nil {
		if isMissingIndexError(err) {
			resp.Diagnostics.AddError("Index settings parent not found", fmt.Sprintf("Index %q no longer exists; settings were not changed.", uid))
			return
		}
		resp.Diagnostics.AddError("Error checking index settings parent", apiError(err))
		return
	}

	patch, err := indexSettingsPatch(ctx, plan, &state, desired, owned)
	if err != nil {
		resp.Diagnostics.AddError("Invalid index settings", err.Error())
		return
	}

	if len(patch) == 0 {
		plannedState := indexSettingsPendingState(plan, &state, desired, nil, uid)
		refreshed, unsupported, err := r.refreshedState(operationCtx, uid, desired, &plannedState)
		if err != nil {
			if isMissingIndexError(err) {
				resp.Diagnostics.AddError("Index settings parent not found", fmt.Sprintf("Index %q no longer exists; settings were not changed.", uid))
				return
			}
			resp.Diagnostics.AddError("Error reading index settings", apiError(err))
			return
		}
		if unsupported {
			resp.Diagnostics.AddError("Unsupported advanced filterable attributes", "The index contains advanced filterable attribute rules that this resource cannot represent; the field remains managed with its prior value.")
		}
		resp.Diagnostics.Append(resp.State.Set(ctx, &refreshed)...)
		return
	}

	unsupportedBeforeMutation, err := r.checkFilterableMutation(operationCtx, uid, patch)
	if err != nil {
		resp.Diagnostics.AddError("Error checking index settings before mutation", indexSettingsErrorMessage(err))
		return
	}
	if unsupportedBeforeMutation {
		resp.Diagnostics.AddError("Unsupported advanced filterable attributes", "The index contains advanced filterable attribute rules that this resource cannot represent; settings were not changed and prior ownership is retained.")
		return
	}

	task, err := r.patchSettings(operationCtx, uid, patch)
	if err != nil {
		if indexSettingsResponseAccepted(err) {
			pending := indexSettingsPendingState(plan, &state, desired, owned, uid)
			resp.Diagnostics.Append(resp.State.Set(ctx, &pending)...)
		}
		resp.Diagnostics.AddError("Error updating index settings", indexSettingsErrorMessage(err))
		return
	}

	pending := indexSettingsPendingState(plan, &state, desired, owned, uid)
	resp.Diagnostics.Append(resp.State.Set(ctx, &pending)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if task == nil || task.TaskUID == nil {
		resp.Diagnostics.AddError("Error updating index settings", "Meilisearch accepted the settings update without returning a task UID.")
		return
	}

	if _, err := waitTask(operationCtx, r.client, *task.TaskUID); err != nil {
		resp.Diagnostics.AddError("Error waiting for index settings update", apiError(err))
		return
	}

	refreshed, unsupported, err := r.refreshedState(operationCtx, uid, desired, &pending)
	if err != nil {
		if isMissingIndexError(err) {
			resp.Diagnostics.AddError("Index settings parent not found", fmt.Sprintf("Index %q disappeared while applying settings; verify the remote task before retrying.", uid))
			return
		}
		resp.Diagnostics.AddError("Error reading index settings", apiError(err))
		return
	}
	if unsupported {
		resp.Diagnostics.AddError("Unsupported advanced filterable attributes", "The index contains advanced filterable attribute rules that this resource cannot represent; the field remains managed with its prior value.")
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &refreshed)...)
}

func (r *indexSettingsResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state indexSettingsResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if !r.configured() {
		resp.Diagnostics.AddError("Index settings resource is not configured", "The Meilisearch provider did not supply a client.")
		return
	}

	owned, unknown := indexSettingsModelManagedFields(state)
	if unknown {
		resp.Diagnostics.AddError("Unknown index settings ownership", "The provider cannot delete index settings while managed_fields is unknown.")
		return
	}

	operationCtx, cancel := operationContext(ctx, r.operationTimeout)
	defer cancel()

	uid := state.IndexUID.ValueString()
	if err := r.ensureIndex(operationCtx, uid); err != nil {
		if isMissingIndexError(err) {
			return
		}
		resp.Diagnostics.AddError("Error checking index settings parent", apiError(err))
		return
	}

	patch := make(map[string]json.RawMessage, len(owned))
	for _, field := range indexSettingsFields {
		if owned[field] {
			patch[indexSettingsJSONFields[field]] = json.RawMessage("null")
		}
	}
	if len(patch) == 0 {
		return
	}

	unsupportedBeforeMutation, err := r.checkFilterableMutation(operationCtx, uid, patch)
	if err != nil {
		if isMissingIndexError(err) {
			return
		}
		resp.Diagnostics.AddError("Error checking index settings before mutation", indexSettingsErrorMessage(err))
		return
	}
	if unsupportedBeforeMutation {
		resp.Diagnostics.AddError("Unsupported advanced filterable attributes", "The index contains advanced filterable attribute rules that this resource cannot represent; settings were not changed and prior ownership is retained.")
		return
	}

	task, err := r.patchSettings(operationCtx, uid, patch)
	if err != nil {
		resp.Diagnostics.AddError("Error resetting index settings", indexSettingsErrorMessage(err))
		return
	}
	if task == nil || task.TaskUID == nil {
		resp.Diagnostics.AddError("Error resetting index settings", "Meilisearch accepted the settings reset without returning a task UID.")
		return
	}

	if _, err := waitTask(operationCtx, r.client, *task.TaskUID); err != nil {
		resp.Diagnostics.AddError("Error waiting for index settings reset", apiError(err))
	}
}

func (r *indexSettingsResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	if req.ID == "" {
		resp.Diagnostics.AddError("Invalid index settings import UID", "Provide a non-empty existing index UID.")
		return
	}

	resource.ImportStatePassthroughID(ctx, path.Root("index_uid"), req, resp)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("managed_fields"), indexSettingsFieldSet(indexSettingsFields))...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), types.StringValue(req.ID))...)
}

func (r *indexSettingsResource) configured() bool {
	return r.client != nil
}

func (r *indexSettingsResource) ensureIndex(ctx context.Context, uid string) error {
	if uid == "" {
		return errors.New("index_uid must be non-empty")
	}

	index, err := r.client.GetIndexWithContext(ctx, uid)
	if err != nil {
		return err
	}
	if index == nil || index.UID == "" {
		return errors.New("meilisearch returned no index data")
	}

	return nil
}

func (r *indexSettingsResource) refreshedState(ctx context.Context, uid string, managed map[string]bool, prior *indexSettingsResourceModel) (indexSettingsResourceModel, bool, error) {
	settings, err := r.getSettings(ctx, uid)
	if err != nil {
		return indexSettingsResourceModel{}, false, err
	}

	state, unsupported, err := indexSettingsStateFromValues(ctx, uid, settings, managed, prior)
	if err != nil {
		return indexSettingsResourceModel{}, false, err
	}

	return state, unsupported, nil
}

func (r *indexSettingsResource) checkFilterableMutation(ctx context.Context, uid string, patch map[string]json.RawMessage) (bool, error) {
	if _, changesFilterable := patch[indexSettingsJSONFields[settingsFieldFilterable]]; !changesFilterable {
		return false, nil
	}

	values, err := r.getSettings(ctx, uid)
	if err != nil {
		return false, err
	}

	return values.FilterableAdvanced, nil
}
