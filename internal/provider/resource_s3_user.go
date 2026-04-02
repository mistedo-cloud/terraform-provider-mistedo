package provider

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/mistedo/terraform-provider-mistedo/internal/client"
)

var _ resource.Resource = (*s3UserResource)(nil)
var _ resource.ResourceWithImportState = (*s3UserResource)(nil)

type s3UserResource struct {
	client *client.Client
}

func newS3UserResource() resource.Resource {
	return &s3UserResource{}
}

func (r *s3UserResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_s3_user"
}

type s3UserModel struct {
	ID              types.String `tfsdk:"id"`
	Name            types.String `tfsdk:"name"`
	FullName        types.String `tfsdk:"full_name"`
	PoolID          types.Int64  `tfsdk:"pool_id"`
	Owner           types.String `tfsdk:"owner"`
	Description     types.String `tfsdk:"description"`
	QuotaBuckets    types.Int64  `tfsdk:"quota_buckets"`
	QuotaDataSizeMB types.Int64  `tfsdk:"quota_data_size_mb"`
	QuotaObjects    types.Int64  `tfsdk:"quota_objects"`
	S3AccessKey     types.String `tfsdk:"s3_access_key"`
	S3SecretKey     types.String `tfsdk:"s3_secret_key"`
	SwiftSecretKey  types.String `tfsdk:"swift_secret_key"`
	Status          types.String `tfsdk:"status"`
	UsageBuckets    types.Int64  `tfsdk:"usage_buckets"`
	UsageDataSizeMB types.Int64  `tfsdk:"usage_data_size_mb"`
	UsageObjects    types.Int64  `tfsdk:"usage_objects"`
	PoolName        types.String `tfsdk:"pool_name"`
}

func (r *s3UserResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A Ceph RGW **S3 user** (technical account with access keys) on the Mistedo Storage API v2. " +
			"Similar in role to a dedicated principal for object storage (compare `google_storage_hmac_key` parent, or an IAM user used only for S3).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Storage service numeric id (string).",
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Short logical name; the API prefixes it with the account id (e.g. `ha001$<name>`).",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"full_name": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Full Ceph user name including account prefix.",
			},
			"pool_name": schema.StringAttribute{
				Required: true,
				MarkdownDescription: "Storage pool **name** as returned by `GET /api/storage/v2/pools?type=s3` (e.g. `nvme`, `hdd-cold-21`). " +
					"Matching is case-insensitive; the provider resolves the numeric pool id automatically.",
				Validators: []validator.String{
					stringvalidator.LengthBetween(1, 256),
				},
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"pool_id": schema.Int64Attribute{
				Computed:            true,
				MarkdownDescription: "Numeric pool id from the API after resolution.",
			},
			"owner": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Owner email (responsible person).",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"description": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Optional description.",
			},
			"quota_buckets": schema.Int64Attribute{
				Optional:            true,
				MarkdownDescription: "Max buckets for this S3 user. Omit or `-1` for unlimited.",
				Validators: []validator.Int64{
					int64validator.Any(
						int64validator.OneOf(-1),
						int64validator.AtLeast(0),
					),
				},
			},
			"quota_data_size_mb": schema.Int64Attribute{
				Optional:            true,
				MarkdownDescription: "Max total data size (MiB) for this user. Omit or `-1` for unlimited.",
				Validators: []validator.Int64{
					int64validator.Any(
						int64validator.OneOf(-1),
						int64validator.AtLeast(0),
					),
				},
			},
			"quota_objects": schema.Int64Attribute{
				Optional:            true,
				MarkdownDescription: "Max number of objects for this user. Omit or `-1` for unlimited.",
				Validators: []validator.Int64{
					int64validator.Any(
						int64validator.OneOf(-1),
						int64validator.AtLeast(0),
					),
				},
			},
			"s3_access_key": schema.StringAttribute{
				Computed:            true,
				Sensitive:           true,
				MarkdownDescription: "S3 access key (stored in Terraform state).",
			},
			"s3_secret_key": schema.StringAttribute{
				Computed:            true,
				Sensitive:           true,
				MarkdownDescription: "S3 secret key (stored in Terraform state).",
			},
			"swift_secret_key": schema.StringAttribute{
				Computed:            true,
				Sensitive:           true,
				MarkdownDescription: "Swift secret key, if returned by the API.",
			},
			"status": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "User status from the API (e.g. active).",
			},
			"usage_buckets": schema.Int64Attribute{
				Computed: true,
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

func (r *s3UserResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *s3UserResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "")
		return
	}
	var plan s3UserModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	pool, err := r.client.FindS3PoolByName(ctx, plan.PoolName.ValueString())
	if err != nil {
		resp.Diagnostics.Append(diagAPI("Could not resolve S3 pool by name", err)...)
		return
	}
	in := client.S3UserCreate{
		PoolID:      pool.ID,
		Owner:       plan.Owner.ValueString(),
		Name:        plan.Name.ValueString(),
		Description: plan.Description.ValueString(),
	}
	if q := quotaFromPlan(plan); q != nil {
		in.Quota = q
	}
	u, err := r.client.CreateS3User(ctx, in)
	if err != nil {
		resp.Diagnostics.Append(diagAPI("Could not create S3 user", err)...)
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, s3UserModelFromAPI(u, plan))...)
}

func (r *s3UserResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "")
		return
	}
	var state s3UserModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id, err := strconv.Atoi(state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid id in state", err.Error())
		return
	}
	u, err := r.client.GetS3User(ctx, id)
	if err != nil {
		if errors.Is(err, client.ErrNotFound) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.Append(diagAPI("Could not read S3 user", err)...)
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, s3UserModelFromAPI(u, state))...)
}

func (r *s3UserResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "")
		return
	}
	var plan s3UserModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	var state s3UserModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id, err := strconv.Atoi(state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid id in state", err.Error())
		return
	}
	cur, err := r.client.GetS3User(ctx, id)
	if err != nil {
		resp.Diagnostics.Append(diagAPI("Could not read S3 user for update", err)...)
		return
	}
	mergeS3UserFromPlan(cur, plan)
	cur.Keys = nil
	cur.Usage = nil
	cur.Account = nil
	cur.Pool = nil
	out, err := r.client.UpdateS3User(ctx, id, cur)
	if err != nil {
		resp.Diagnostics.Append(diagAPI("Could not update S3 user", err)...)
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, s3UserModelFromAPI(out, plan))...)
}

func (r *s3UserResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "")
		return
	}
	var state s3UserModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id, err := strconv.Atoi(state.ID.ValueString())
	if err != nil {
		return
	}
	if err := r.client.DeleteS3User(ctx, id); err != nil {
		resp.Diagnostics.Append(diagAPI("Could not delete S3 user", err)...)
	}
}

func (r *s3UserResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func quotaFromPlan(plan s3UserModel) *client.S3UserQuota {
	var q client.S3UserQuota
	has := false
	if !plan.QuotaBuckets.IsNull() {
		q.Buckets = int(plan.QuotaBuckets.ValueInt64())
		has = true
	}
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

func s3UserModelFromAPI(u *client.S3User, prior s3UserModel) s3UserModel {
	out := s3UserModel{
		ID:          types.StringValue(strconv.Itoa(u.ID)),
		FullName:    types.StringValue(u.Name),
		Owner:       types.StringValue(u.Owner),
		Status:      types.StringValue(u.Status),
		S3AccessKey: types.StringNull(),
		S3SecretKey: types.StringNull(),
	}
	if !prior.Name.IsNull() && strings.TrimSpace(prior.Name.ValueString()) != "" {
		out.Name = prior.Name
	} else if i := strings.Index(u.Name, "$"); i >= 0 && i+1 < len(u.Name) {
		out.Name = types.StringValue(u.Name[i+1:])
	} else {
		out.Name = types.StringValue(u.Name)
	}
	if !prior.PoolName.IsNull() && strings.TrimSpace(prior.PoolName.ValueString()) != "" {
		out.PoolName = prior.PoolName
	} else if u.Pool != nil {
		out.PoolName = types.StringValue(u.Pool.Name)
	} else {
		out.PoolName = types.StringNull()
	}
	if u.PoolID != 0 {
		out.PoolID = types.Int64Value(int64(u.PoolID))
	} else if u.Pool != nil {
		out.PoolID = types.Int64Value(int64(u.Pool.ID))
	} else {
		out.PoolID = types.Int64Null()
	}
	if strings.TrimSpace(u.Description) != "" {
		out.Description = types.StringValue(u.Description)
	} else {
		out.Description = types.StringNull()
	}
	if u.Quota != nil {
		out.QuotaBuckets = quotaIntFromAPIWithPrior(u.Quota.Buckets, prior.QuotaBuckets)
		out.QuotaDataSizeMB = quotaIntFromAPIWithPrior(u.Quota.DataSizeMB, prior.QuotaDataSizeMB)
		out.QuotaObjects = quotaIntFromAPIWithPrior(u.Quota.Objects, prior.QuotaObjects)
	} else {
		out.QuotaBuckets = types.Int64Null()
		out.QuotaDataSizeMB = types.Int64Null()
		out.QuotaObjects = types.Int64Null()
	}
	if u.Usage != nil {
		out.UsageBuckets = types.Int64Value(int64(u.Usage.Buckets))
		out.UsageDataSizeMB = types.Int64Value(int64(u.Usage.DataSizeMB))
		out.UsageObjects = types.Int64Value(int64(u.Usage.Objects))
	}
	if u.Keys != nil && u.Keys.S3 != nil {
		out.S3AccessKey = types.StringValue(u.Keys.S3.AccessKey)
		out.S3SecretKey = types.StringValue(u.Keys.S3.SecretKey)
	}
	if u.Keys != nil && u.Keys.Swift != nil {
		out.SwiftSecretKey = types.StringValue(u.Keys.Swift.SecretKey)
	} else {
		out.SwiftSecretKey = types.StringNull()
	}
	return out
}

func mergeS3UserFromPlan(cur *client.S3User, plan s3UserModel) {
	if !plan.Description.IsNull() {
		cur.Description = plan.Description.ValueString()
	}
	if q := quotaFromPlan(plan); q != nil {
		cur.Quota = q
	}
}
