package provider

import (
	"context"
	"errors"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/mistedo/terraform-provider-mistedo/internal/client"
)

var _ resource.Resource = (*s3BucketResource)(nil)
var _ resource.ResourceWithImportState = (*s3BucketResource)(nil)

type s3BucketResource struct {
	client *client.Client
}

func newS3BucketResource() resource.Resource {
	return &s3BucketResource{}
}

func (r *s3BucketResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_s3_bucket"
}

type s3BucketModel struct {
	ID              types.String `tfsdk:"id"`
	Name            types.String `tfsdk:"name"`
	Path            types.String `tfsdk:"path"`
	UserName        types.String `tfsdk:"user_name"`
	QuotaDataSizeMB types.Int64  `tfsdk:"quota_data_size_mb"`
	QuotaObjects    types.Int64  `tfsdk:"quota_objects"`
	UsageDataSizeMB types.Int64  `tfsdk:"usage_data_size_mb"`
	UsageObjects    types.Int64  `tfsdk:"usage_objects"`
}

func (r *s3BucketResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "An **S3 bucket** on the Mistedo Ceph RGW Storage API v2. " +
			"Comparable to `aws_s3_bucket` (namespace for objects; configuration is limited to what the Storage API exposes).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Full bucket path (`account/short-name`), unique in the storage service.",
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Short bucket name (unique within the account for the owning S3 user).",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"path": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Same as `id` — canonical path returned by the API.",
			},
			"user_name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Full S3 user name (e.g. `ha001$myapp`) from `mistedo_s3_user.full_name`.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"quota_data_size_mb": schema.Int64Attribute{
				Optional:            true,
				MarkdownDescription: "Per-bucket data quota (MiB). Omit or set `-1` for unlimited (API uses `-1` for the same).",
				Validators: []validator.Int64{
					int64validator.Any(
						int64validator.OneOf(-1),
						int64validator.AtLeast(0),
					),
				},
			},
			"quota_objects": schema.Int64Attribute{
				Optional:            true,
				MarkdownDescription: "Per-bucket object count quota. Omit or set `-1` for unlimited.",
				Validators: []validator.Int64{
					int64validator.Any(
						int64validator.OneOf(-1),
						int64validator.AtLeast(0),
					),
				},
			},
			"usage_data_size_mb": schema.Int64Attribute{
				Computed: true,
			},
			"usage_objects": schema.Int64Attribute{
				Computed: true,
			},
		},
	}
}

func (r *s3BucketResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Invalid provider data", fmt.Sprintf("Expected *client.Client, got %T.", req.ProviderData))
		return
	}
	r.client = c
}

func (r *s3BucketResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "")
		return
	}
	var plan s3BucketModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	b := &client.S3Bucket{
		Name:     plan.Name.ValueString(),
		UserName: plan.UserName.ValueString(),
	}
	if q := bucketQuotaFromPlan(plan); q != nil {
		b.Quota = q
	}
	out, err := r.client.CreateS3Bucket(ctx, b)
	if err != nil {
		resp.Diagnostics.Append(diagAPI("Could not create S3 bucket", err)...)
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, s3BucketModelFromAPI(out, plan))...)
}

func (r *s3BucketResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "")
		return
	}
	var state s3BucketModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	path := state.ID.ValueString()
	if path == "" {
		path = state.Path.ValueString()
	}
	u := state.UserName.ValueString()
	b, err := r.client.GetS3BucketByPath(ctx, path, u)
	if err != nil {
		if errors.Is(err, client.ErrNotFound) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.Append(diagAPI("Could not read S3 bucket", err)...)
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, s3BucketModelFromAPI(b, state))...)
}

func (r *s3BucketResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "")
		return
	}
	var plan s3BucketModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	var state s3BucketModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	path := state.Path.ValueString()
	if path == "" {
		path = state.ID.ValueString()
	}
	cur, err := r.client.GetS3BucketByPath(ctx, path, plan.UserName.ValueString())
	if err != nil {
		resp.Diagnostics.Append(diagAPI("Could not read S3 bucket for update", err)...)
		return
	}
	cur.Name = plan.Name.ValueString()
	cur.UserName = plan.UserName.ValueString()
	if q := bucketQuotaFromPlan(plan); q != nil {
		cur.Quota = q
	}
	out, err := r.client.UpdateS3Bucket(ctx, path, cur)
	if err != nil {
		resp.Diagnostics.Append(diagAPI("Could not update S3 bucket", err)...)
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, s3BucketModelFromAPI(out, plan))...)
}

func (r *s3BucketResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "")
		return
	}
	var state s3BucketModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	path := state.Path.ValueString()
	if path == "" {
		path = state.ID.ValueString()
	}
	if err := r.client.DeleteS3Bucket(ctx, path); err != nil {
		resp.Diagnostics.Append(diagAPI("Could not delete S3 bucket", err)...)
	}
}

func (r *s3BucketResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func bucketQuotaFromPlan(plan s3BucketModel) *client.BucketQuota {
	var q client.BucketQuota
	has := false
	if !plan.QuotaDataSizeMB.IsNull() {
		q.DataSizeMB = int(plan.QuotaDataSizeMB.ValueInt64())
		has = true
	}
	if !plan.QuotaObjects.IsNull() {
		q.Objects = int(plan.QuotaObjects.ValueInt64())
		has = true
	}
	if !has {
		return nil
	}
	return &q
}

// quotaIntFromAPIWithPrior maps Storage API quota integers to Terraform state.
// API -1 means unlimited. If the user omitted the attribute (prior null), store null;
// if they set -1 explicitly, keep -1 so plan and apply stay consistent.
func quotaIntFromAPIWithPrior(api int, prior types.Int64) types.Int64 {
	if api < 0 {
		if !prior.IsNull() && prior.ValueInt64() == -1 {
			return types.Int64Value(-1)
		}
		return types.Int64Null()
	}
	return types.Int64Value(int64(api))
}

func s3BucketModelFromAPI(b *client.S3Bucket, prior s3BucketModel) s3BucketModel {
	out := s3BucketModel{
		ID:       types.StringValue(b.Path),
		Path:     types.StringValue(b.Path),
		Name:     types.StringValue(b.Name),
		UserName: types.StringValue(b.UserName),
	}
	if b.Quota != nil {
		out.QuotaDataSizeMB = quotaIntFromAPIWithPrior(b.Quota.DataSizeMB, prior.QuotaDataSizeMB)
		out.QuotaObjects = quotaIntFromAPIWithPrior(b.Quota.Objects, prior.QuotaObjects)
	} else {
		out.QuotaDataSizeMB = types.Int64Null()
		out.QuotaObjects = types.Int64Null()
	}
	if b.Usage != nil {
		out.UsageDataSizeMB = types.Int64Value(int64(b.Usage.DataSizeMB))
		out.UsageObjects = types.Int64Value(int64(b.Usage.Objects))
	}
	return out
}
