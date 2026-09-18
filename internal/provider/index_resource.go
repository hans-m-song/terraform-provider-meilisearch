package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/meilisearch/meilisearch-go"
)

var (
	_ resource.Resource                = &indexResource{}
	_ resource.ResourceWithConfigure   = &indexResource{}
	_ resource.ResourceWithImportState = &indexResource{}
)

func NewIndexResource() resource.Resource {
	return &indexResource{}
}

type indexResource struct {
	client           meilisearch.ServiceManager
	operationTimeout time.Duration
}

type indexResourceModel struct {
	UID        types.String `tfsdk:"uid"`
	PrimaryKey types.String `tfsdk:"primary_key"`
	CreatedAt  types.String `tfsdk:"created_at"`
	UpdatedAt  types.String `tfsdk:"updated_at"`
	ID         types.String `tfsdk:"id"`
}

func (r *indexResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_index"
}

func (r *indexResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Meilisearch index.",
		Attributes: map[string]schema.Attribute{
			"uid": schema.StringAttribute{
				Description: "Unique identifier of the index.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"primary_key": schema.StringAttribute{
				Description: "Primary key of the index. If omitted, Meilisearch chooses it when the first document is added.",
				Optional:    true,
				Computed:    true,
				Validators: []validator.String{
					nonEmptyPrimaryKeyValidator{},
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"created_at": schema.StringAttribute{
				Description: "Date and time when the index was created (RFC3339).",
				Computed:    true,
			},
			"updated_at": schema.StringAttribute{
				Description: "Date and time when the index was last updated (RFC3339).",
				Computed:    true,
			},
			"id": schema.StringAttribute{
				Description: "Remote index UID.",
				Computed:    true,
			},
		},
	}
}

func (r *indexResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	data, ok := req.ProviderData.(*providerData)
	if !ok || data == nil || data.client == nil {
		resp.Diagnostics.AddError(
			"Invalid provider data",
			"The Meilisearch index resource requires provider data created by the Meilisearch provider.",
		)
		return
	}

	r.client = data.client
	r.operationTimeout = data.operationTimeout
	if r.operationTimeout <= 0 {
		r.operationTimeout = defaultOperationTimeout
	}
}

func (r *indexResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan indexResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if r.client == nil {
		resp.Diagnostics.AddError("Index resource is not configured", "The Meilisearch provider did not supply a client.")
		return
	}
	if !plan.PrimaryKey.IsNull() && !plan.PrimaryKey.IsUnknown() && plan.PrimaryKey.ValueString() == "" {
		resp.Diagnostics.AddError("Invalid index primary key", "primary_key must be non-empty when configured.")
		return
	}

	operationCtx, cancel := operationContext(ctx, r.operationTimeout)
	defer cancel()

	config := &meilisearch.IndexConfig{
		Uid:        plan.UID.ValueString(),
		PrimaryKey: plan.PrimaryKey.ValueString(),
	}

	task, err := r.client.CreateIndexWithContext(operationCtx, config)
	if err != nil {
		resp.Diagnostics.AddError("Error creating index", "Could not create index: "+apiError(err))
		return
	}
	if task == nil {
		resp.Diagnostics.AddError("Error creating index", "Meilisearch returned no creation task.")
		return
	}

	uid := config.Uid
	if task.IndexUID != "" {
		uid = task.IndexUID
	}

	pendingState := indexResourceModel{
		UID: types.StringValue(uid),
		ID:  types.StringValue(uid),
	}
	if !plan.PrimaryKey.IsNull() && !plan.PrimaryKey.IsUnknown() {
		pendingState.PrimaryKey = plan.PrimaryKey
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &pendingState)...)
	if resp.Diagnostics.HasError() {
		return
	}

	completedTask, err := waitTask(operationCtx, r.client, task.TaskUID)
	if err != nil {
		if completedTask != nil && (completedTask.Status == meilisearch.TaskStatusFailed || completedTask.Status == meilisearch.TaskStatusCanceled) {
			resp.State.RemoveResource(ctx)
		}

		resp.Diagnostics.AddError("Error waiting for index creation", apiError(err))
		return
	}

	index, err := r.client.GetIndexWithContext(operationCtx, uid)
	if err != nil {
		resp.Diagnostics.AddError("Error reading created index", "Could not read index "+uid+": "+apiError(err))
		return
	}
	if index == nil || index.UID == "" {
		resp.Diagnostics.AddError("Error reading created index", "Meilisearch returned no index UID after creation.")
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, indexResourceState(index))...)
}

func (r *indexResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state indexResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if r.client == nil {
		resp.Diagnostics.AddError("Index resource is not configured", "The Meilisearch provider did not supply a client.")
		return
	}

	operationCtx, cancel := operationContext(ctx, r.operationTimeout)
	defer cancel()

	uid := state.UID.ValueString()
	index, err := r.client.GetIndexWithContext(operationCtx, uid)
	if err != nil {
		if isAPIError(err, "index_not_found") {
			resp.State.RemoveResource(ctx)
			return
		}

		resp.Diagnostics.AddError("Error reading Meilisearch index", "Could not read index "+uid+": "+apiError(err))
		return
	}
	if index == nil || index.UID == "" {
		resp.Diagnostics.AddError("Error reading Meilisearch index", "Meilisearch returned no index data.")
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, indexResourceState(index))...)
}

func (r *indexResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan indexResourceModel
	var state indexResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if r.client == nil {
		resp.Diagnostics.AddError("Index resource is not configured", "The Meilisearch provider did not supply a client.")
		return
	}
	if !plan.PrimaryKey.IsNull() && !plan.PrimaryKey.IsUnknown() && plan.PrimaryKey.ValueString() == "" {
		resp.Diagnostics.AddError("Invalid index primary key", "primary_key must be non-empty when configured.")
		return
	}

	uid := state.UID.ValueString()
	operationCtx, cancel := operationContext(ctx, r.operationTimeout)
	defer cancel()

	if primaryKeyChanged(state.PrimaryKey, plan.PrimaryKey) {
		primaryKey := plan.PrimaryKey.ValueString()
		if primaryKey == "" {
			resp.Diagnostics.AddError(
				"Invalid index primary key update",
				"The provider accepts only non-empty primary_key updates. Omit primary_key to retain the current value; the SDK cannot encode a null primary-key reset.",
			)
			return
		}

		indexManager := r.client.Index(uid)
		if indexManager == nil {
			resp.Diagnostics.AddError("Error checking index documents", "Meilisearch returned no index manager.")
			return
		}

		stats, err := indexManager.GetStatsWithContext(operationCtx, nil)
		if err != nil {
			resp.Diagnostics.AddError("Error checking index documents", "Could not determine whether index "+uid+" contains documents: "+apiError(err))
			return
		}
		if stats == nil {
			resp.Diagnostics.AddError("Error checking index documents", "Meilisearch returned no index statistics.")
			return
		}
		if stats.NumberOfDocuments > 0 {
			resp.Diagnostics.AddError(
				"Cannot change primary key on a populated index",
				fmt.Sprintf("Index %q contains %d documents. Meilisearch only permits this update before documents are added; no documents were deleted.", uid, stats.NumberOfDocuments),
			)
			return
		}

		task, err := indexManager.UpdateIndexWithContext(operationCtx, &meilisearch.UpdateIndexRequestParams{PrimaryKey: primaryKey})
		if err != nil {
			resp.Diagnostics.AddError("Error updating index primary key", "Could not update index "+uid+": "+apiError(err))
			return
		}
		if task == nil {
			resp.Diagnostics.AddError("Error updating index primary key", "Meilisearch returned no primary-key update task.")
			return
		}

		if _, err := waitTask(operationCtx, r.client, task.TaskUID); err != nil {
			resp.Diagnostics.AddError("Error waiting for index primary-key update", apiError(err))
			return
		}
	}

	index, err := r.client.GetIndexWithContext(operationCtx, uid)
	if err != nil {
		if isAPIError(err, "index_not_found") {
			resp.State.RemoveResource(ctx)
			return
		}

		resp.Diagnostics.AddError("Error reading updated index", "Could not read index "+uid+": "+apiError(err))
		return
	}
	if index == nil || index.UID == "" {
		resp.Diagnostics.AddError("Error reading updated index", "Meilisearch returned no index data.")
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, indexResourceState(index))...)
}

func (r *indexResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state indexResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if r.client == nil {
		resp.Diagnostics.AddError("Index resource is not configured", "The Meilisearch provider did not supply a client.")
		return
	}

	operationCtx, cancel := operationContext(ctx, r.operationTimeout)
	defer cancel()

	uid := state.UID.ValueString()
	task, err := r.client.DeleteIndexWithContext(operationCtx, uid)
	if err != nil {
		if isAPIError(err, "index_not_found") {
			return
		}

		resp.Diagnostics.AddError("Error deleting Meilisearch index", "Could not delete index "+uid+": "+apiError(err))
		return
	}
	if task == nil {
		resp.Diagnostics.AddError("Error deleting Meilisearch index", "Meilisearch returned no deletion task.")
		return
	}

	if _, err := waitTask(operationCtx, r.client, task.TaskUID); err != nil {
		if isAPIError(err, "index_not_found") {
			return
		}

		resp.Diagnostics.AddError("Error waiting for index deletion", apiError(err))
	}
}

func (r *indexResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("uid"), req, resp)
}

func indexResourceState(index *meilisearch.IndexResult) indexResourceModel {
	state := indexResourceModel{
		UID:       types.StringValue(index.UID),
		CreatedAt: timestampValue(index.CreatedAt),
		UpdatedAt: timestampValue(index.UpdatedAt),
		ID:        types.StringValue(index.UID),
	}
	if index.PrimaryKey != "" {
		state.PrimaryKey = types.StringValue(index.PrimaryKey)
	} else {
		state.PrimaryKey = types.StringNull()
	}

	return state
}

func primaryKeyChanged(state, plan types.String) bool {
	if plan.IsUnknown() || plan.IsNull() {
		return false
	}
	if state.IsNull() {
		return plan.ValueString() != ""
	}

	return state.ValueString() != plan.ValueString()
}

type nonEmptyPrimaryKeyValidator struct{}

func (nonEmptyPrimaryKeyValidator) Description(context.Context) string {
	return "primary_key must be non-empty when configured."
}

func (v nonEmptyPrimaryKeyValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (nonEmptyPrimaryKeyValidator) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() || req.ConfigValue.ValueString() != "" {
		return
	}

	resp.Diagnostics.AddAttributeError(req.Path, "Invalid index primary key", "primary_key must be non-empty when configured.")
}
