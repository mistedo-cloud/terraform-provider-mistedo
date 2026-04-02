package provider

import (
	"context"
	"errors"
	"fmt"
	"strconv"

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

var _ resource.Resource = (*iscsiDiskResource)(nil)
var _ resource.ResourceWithImportState = (*iscsiDiskResource)(nil)

type iscsiDiskResource struct {
	client *client.Client
}

func newIscsiDiskResource() resource.Resource {
	return &iscsiDiskResource{}
}

func (r *iscsiDiskResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_iscsi_disk"
}

type iscsiDiskModel struct {
	ID         types.String `tfsdk:"id"`
	Name       types.String `tfsdk:"name"`
	Owner      types.String `tfsdk:"owner"`
	SizeGB     types.Int64  `tfsdk:"size_gb"`
	PoolName   types.String `tfsdk:"pool_name"`
	ConfigID   types.Int64  `tfsdk:"config_id"`
	TargetIQN  types.String `tfsdk:"target_iqn"`
	ConfigName types.String `tfsdk:"config_name"`
}

func (r *iscsiDiskResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "An **iSCSI disk** (block volume) in the Storage API v2. Disks are created inside an existing iSCSI target configuration for a pool (configs are provisioned by the platform).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Storage service numeric disk id (string).",
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Disk name.",
				Validators:          []validator.String{stringvalidator.LengthBetween(1, 256)},
			},
			"owner": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Owner email (responsible person).",
				Validators:          []validator.String{stringvalidator.LengthBetween(1, 320)},
			},
			"size_gb": schema.Int64Attribute{
				Required:            true,
				MarkdownDescription: "Disk size in GiB.",
				Validators:          []validator.Int64{int64validator.AtLeast(1)},
			},
			"pool_name": schema.StringAttribute{
				Required: true,
				MarkdownDescription: "Storage pool **name** for the iSCSI tier (see `GET /api/storage/v2/pools?type=iscsi`). " +
					"The provider resolves the iSCSI config for this pool. Changing pool replaces the disk.",
				Validators:    []validator.String{stringvalidator.LengthBetween(1, 256)},
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"config_id": schema.Int64Attribute{
				Computed:            true,
				MarkdownDescription: "Numeric iSCSI config id the disk belongs to.",
			},
			"target_iqn": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Target IQN from the iSCSI config (read-only).",
			},
			"config_name": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Config file name from the API (read-only).",
			},
		},
	}
}

func (r *iscsiDiskResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *iscsiDiskResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "")
		return
	}
	var plan iscsiDiskModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	cfg, err := r.client.FindIscsiConfigByPoolName(ctx, plan.PoolName.ValueString())
	if err != nil {
		resp.Diagnostics.Append(diagAPI("Could not resolve iSCSI config for pool", err)...)
		return
	}
	in := client.IscsiDiskCreate{
		Name:   plan.Name.ValueString(),
		Owner:  plan.Owner.ValueString(),
		SizeGB: int(plan.SizeGB.ValueInt64()),
	}
	d, err := r.client.CreateIscsiDisk(ctx, cfg.ID, in)
	if err != nil {
		resp.Diagnostics.Append(diagAPI("Could not create iSCSI disk", err)...)
		return
	}
	state := iscsiDiskModelFromAPI(d, cfg, plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *iscsiDiskResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "")
		return
	}
	var state iscsiDiskModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	diskID, err := strconv.Atoi(state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid state id", err.Error())
		return
	}
	var cid int
	var d *client.IscsiDisk
	if !state.ConfigID.IsNull() {
		cid = int(state.ConfigID.ValueInt64())
		d, err = r.client.GetIscsiDiskInConfig(ctx, cid, diskID)
	} else {
		cid, d, err = r.client.FindIscsiDiskConfig(ctx, diskID)
	}
	if err != nil {
		if errors.Is(err, client.ErrNotFound) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.Append(diagAPI("Could not read iSCSI disk", err)...)
		return
	}
	cfg, err := r.client.GetIscsiConfig(ctx, cid)
	if err != nil {
		cfg = &client.IscsiConfig{ID: cid}
	}
	out := iscsiDiskModelFromAPI(d, cfg, state)
	resp.Diagnostics.Append(resp.State.Set(ctx, out)...)
}

func (r *iscsiDiskResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "")
		return
	}
	var plan iscsiDiskModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	var state iscsiDiskModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	diskID, _ := strconv.Atoi(state.ID.ValueString())
	cid := int(state.ConfigID.ValueInt64())
	cur, err := r.client.GetIscsiDiskInConfig(ctx, cid, diskID)
	if err != nil {
		resp.Diagnostics.Append(diagAPI("Could not read iSCSI disk for update", err)...)
		return
	}
	cur.Name = plan.Name.ValueString()
	cur.Owner = plan.Owner.ValueString()
	cur.SizeGB = int(plan.SizeGB.ValueInt64())
	out, err := r.client.UpdateIscsiDisk(ctx, cid, cur)
	if err != nil {
		resp.Diagnostics.Append(diagAPI("Could not update iSCSI disk", err)...)
		return
	}
	cfg, err := r.client.GetIscsiConfig(ctx, cid)
	if err != nil {
		cfg = &client.IscsiConfig{ID: cid}
	}
	st := iscsiDiskModelFromAPI(out, cfg, plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, st)...)
}

func (r *iscsiDiskResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "")
		return
	}
	var state iscsiDiskModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	diskID, _ := strconv.Atoi(state.ID.ValueString())
	cid := int(state.ConfigID.ValueInt64())
	if err := r.client.DeleteIscsiDisk(ctx, cid, diskID); err != nil {
		resp.Diagnostics.Append(diagAPI("Could not delete iSCSI disk", err)...)
	}
}

func (r *iscsiDiskResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func iscsiDiskModelFromAPI(d *client.IscsiDisk, cfg *client.IscsiConfig, prior iscsiDiskModel) iscsiDiskModel {
	out := iscsiDiskModel{
		ID:     types.StringValue(strconv.Itoa(d.ID)),
		Name:   types.StringValue(d.Name),
		Owner:  types.StringValue(d.Owner),
		SizeGB: types.Int64Value(int64(d.SizeGB)),
	}
	if cfg != nil {
		out.ConfigID = types.Int64Value(int64(cfg.ID))
		if cfg.TargetIQN != "" {
			out.TargetIQN = types.StringValue(cfg.TargetIQN)
		} else {
			out.TargetIQN = types.StringNull()
		}
		if cfg.Name != "" {
			out.ConfigName = types.StringValue(cfg.Name)
		} else {
			out.ConfigName = types.StringNull()
		}
	} else {
		out.ConfigID = types.Int64Null()
		out.TargetIQN = types.StringNull()
		out.ConfigName = types.StringNull()
	}
	if !prior.PoolName.IsNull() && prior.PoolName.ValueString() != "" {
		out.PoolName = prior.PoolName
	} else if cfg != nil && cfg.Pool != nil && cfg.Pool.Name != "" {
		out.PoolName = types.StringValue(cfg.Pool.Name)
	} else {
		out.PoolName = types.StringNull()
	}
	return out
}
