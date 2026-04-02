package provider

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/mistedo/terraform-provider-mistedo/internal/client"
)

// chapPasswordRegexp: length 12–16; only letters, digits, . : @ _ - /
var chapPasswordRegexp = regexp.MustCompile(`^[a-zA-Z0-9.:@_\-/]{12,16}$`)

var _ resource.Resource = (*iscsiClientResource)(nil)
var _ resource.ResourceWithImportState = (*iscsiClientResource)(nil)

type iscsiClientResource struct {
	client *client.Client
}

func newIscsiClientResource() resource.Resource {
	return &iscsiClientResource{}
}

func (r *iscsiClientResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_iscsi_client"
}

type iscsiClientDiskTF struct {
	ID   types.Int64  `tfsdk:"id"`
	Name types.String `tfsdk:"name"`
}

type iscsiClientModel struct {
	ID           types.String `tfsdk:"id"`
	Name         types.String `tfsdk:"name"`
	Owner        types.String `tfsdk:"owner"`
	IQN          types.String `tfsdk:"iqn"`
	ChapUsername types.String `tfsdk:"chap_username"`
	ChapPassword types.String `tfsdk:"chap_password"`
	Disks        types.List   `tfsdk:"disks"`
}

func (r *iscsiClientResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "An **iSCSI client** (initiator) in the Storage API v2. CHAP credentials are required. " +
			"Optional **`disks`** attach volumes via a second API call (POST assign); changing **`disks`** adds or removes assignments without using PUT.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Storage service numeric client id (string).",
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Human-readable name for this client.",
				Validators:          []validator.String{stringvalidator.LengthBetween(1, 256)},
			},
			"owner": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Owner email (responsible person).",
				Validators:          []validator.String{stringvalidator.LengthBetween(1, 320)},
			},
			"iqn": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Initiator IQN (e.g. `iqn.1994-05.com.redhat:...`).",
				Validators:          []validator.String{stringvalidator.LengthBetween(1, 512)},
			},
			"chap_username": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "CHAP username.",
				Validators:          []validator.String{stringvalidator.LengthBetween(1, 256)},
			},
			"chap_password": schema.StringAttribute{
				Required:            true,
				Sensitive:           true,
				MarkdownDescription: "CHAP password: 12–16 characters; only letters, digits, and `. : @ _ - /`.",
				Validators: []validator.String{
					stringvalidator.RegexMatches(
						chapPasswordRegexp,
						"must be 12–16 characters and use only letters, digits, and . : @ _ - /",
					),
				},
			},
			"disks": schema.ListNestedAttribute{
				Optional: true,
				MarkdownDescription: "Disks to attach to this client (POST `/iscsi/clients/{id}/disks`). " +
					"Each entry needs **`id`** and **`name`** from the disk resource. Omit for a client with no attachments yet.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.Int64Attribute{
							Required:            true,
							MarkdownDescription: "Numeric disk id (`mistedo_iscsi_disk.id`).",
						},
						"name": schema.StringAttribute{
							Required:            true,
							MarkdownDescription: "Disk short name (must match the disk in the API).",
						},
					},
				},
			},
		},
	}
}

func (r *iscsiClientResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func planDiskRefs(ctx context.Context, plan iscsiClientModel) ([]client.IscsiClientDiskRef, diag.Diagnostics) {
	var diags diag.Diagnostics
	if plan.Disks.IsNull() || plan.Disks.IsUnknown() {
		return nil, diags
	}
	var rows []iscsiClientDiskTF
	diags.Append(plan.Disks.ElementsAs(ctx, &rows, false)...)
	if diags.HasError() {
		return nil, diags
	}
	out := make([]client.IscsiClientDiskRef, 0, len(rows))
	for _, row := range rows {
		out = append(out, client.IscsiClientDiskRef{
			ID:   int(row.ID.ValueInt64()),
			Name: row.Name.ValueString(),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, diags
}

// diskRefsToList maps API disk refs to Terraform state. If there are no disks, use ListNull
// when priorDisks was null (attribute omitted in config) so plan and apply stay consistent.
// If priorDisks was set (including explicit empty list), use an empty ListValue.
func diskRefsToList(refs []client.IscsiClientDiskRef, priorDisks types.List) (types.List, diag.Diagnostics) {
	var diags diag.Diagnostics
	attrTypes := map[string]attr.Type{
		"id":   types.Int64Type,
		"name": types.StringType,
	}
	ot := types.ObjectType{AttrTypes: attrTypes}
	if len(refs) == 0 {
		if priorDisks.IsNull() || priorDisks.IsUnknown() {
			return types.ListNull(ot), diags
		}
		list, dgs := types.ListValue(ot, []attr.Value{})
		diags.Append(dgs...)
		return list, diags
	}
	sort.Slice(refs, func(i, j int) bool { return refs[i].ID < refs[j].ID })
	elems := make([]attr.Value, 0, len(refs))
	for _, d := range refs {
		ov, dgs := types.ObjectValue(attrTypes, map[string]attr.Value{
			"id":   types.Int64Value(int64(d.ID)),
			"name": types.StringValue(d.Name),
		})
		diags.Append(dgs...)
		if dgs.HasError() {
			return types.ListNull(ot), diags
		}
		elems = append(elems, ov)
	}
	list, dgs := types.ListValue(ot, elems)
	diags.Append(dgs...)
	return list, diags
}

func diskDiffToAdd(want, have []client.IscsiClientDiskRef) []client.IscsiClientDiskRef {
	haveID := make(map[int]struct{}, len(have))
	for _, d := range have {
		haveID[d.ID] = struct{}{}
	}
	var add []client.IscsiClientDiskRef
	for _, d := range want {
		if _, ok := haveID[d.ID]; !ok {
			add = append(add, d)
		}
	}
	sort.Slice(add, func(i, j int) bool { return add[i].ID < add[j].ID })
	return add
}

func diskDiffToRemove(want, have []client.IscsiClientDiskRef) []int {
	wantID := make(map[int]struct{}, len(want))
	for _, d := range want {
		wantID[d.ID] = struct{}{}
	}
	var rem []int
	for _, d := range have {
		if _, ok := wantID[d.ID]; !ok {
			rem = append(rem, d.ID)
		}
	}
	sort.Ints(rem)
	return rem
}

func (r *iscsiClientResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "")
		return
	}
	var plan iscsiClientModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	wantDisks, d := planDiskRefs(ctx, plan)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}
	in := &client.IscsiClient{
		Name:         plan.Name.ValueString(),
		Owner:        plan.Owner.ValueString(),
		IQN:          plan.IQN.ValueString(),
		ChapUsername: plan.ChapUsername.ValueString(),
		ChapPassword: plan.ChapPassword.ValueString(),
		AccountName:  r.client.Account(),
	}
	out, err := r.client.CreateIscsiClient(ctx, in)
	if err != nil {
		resp.Diagnostics.Append(diagAPI("Could not create iSCSI client", err)...)
		return
	}
	if len(wantDisks) > 0 {
		if err := r.client.AssignIscsiClientDisks(ctx, out.ID, wantDisks); err != nil {
			resp.Diagnostics.Append(diagAPI("Could not assign disks to iSCSI client", err)...)
			return
		}
	}
	u, err := r.client.GetIscsiClient(ctx, out.ID)
	if err != nil {
		resp.Diagnostics.Append(diagAPI("Could not read iSCSI client after create", err)...)
		return
	}
	st, d := iscsiClientModelFromAPI(u, plan)
	resp.Diagnostics.Append(d...)
	resp.Diagnostics.Append(resp.State.Set(ctx, st)...)
}

func (r *iscsiClientResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "")
		return
	}
	var state iscsiClientModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id, err := strconv.Atoi(state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid state id", err.Error())
		return
	}
	u, err := r.client.GetIscsiClient(ctx, id)
	if err != nil {
		if errors.Is(err, client.ErrNotFound) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.Append(diagAPI("Could not read iSCSI client", err)...)
		return
	}
	out, d := iscsiClientModelFromAPI(u, state)
	resp.Diagnostics.Append(d...)
	resp.Diagnostics.Append(resp.State.Set(ctx, out)...)
}

func (r *iscsiClientResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "")
		return
	}
	var plan iscsiClientModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	var state iscsiClientModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id, _ := strconv.Atoi(state.ID.ValueString())
	wantDisks, d := planDiskRefs(ctx, plan)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}
	cur, err := r.client.GetIscsiClient(ctx, id)
	if err != nil {
		resp.Diagnostics.Append(diagAPI("Could not read iSCSI client for update", err)...)
		return
	}
	cur.Name = plan.Name.ValueString()
	cur.Owner = plan.Owner.ValueString()
	cur.IQN = plan.IQN.ValueString()
	cur.ChapUsername = plan.ChapUsername.ValueString()
	cur.ChapPassword = plan.ChapPassword.ValueString()
	if _, err := r.client.UpdateIscsiClient(ctx, cur); err != nil {
		resp.Diagnostics.Append(diagAPI("Could not update iSCSI client", err)...)
		return
	}
	fresh, err := r.client.GetIscsiClient(ctx, id)
	if err != nil {
		resp.Diagnostics.Append(diagAPI("Could not read iSCSI client after update", err)...)
		return
	}
	have := fresh.Disks
	toRem := diskDiffToRemove(wantDisks, have)
	for _, diskID := range toRem {
		if err := r.client.UnassignIscsiClientDisk(ctx, id, diskID); err != nil {
			resp.Diagnostics.Append(diagAPI(fmt.Sprintf("Could not unassign disk %d", diskID), err)...)
			return
		}
	}
	toAdd := diskDiffToAdd(wantDisks, have)
	if len(toAdd) > 0 {
		if err := r.client.AssignIscsiClientDisks(ctx, id, toAdd); err != nil {
			resp.Diagnostics.Append(diagAPI("Could not assign disks to iSCSI client", err)...)
			return
		}
	}
	final, err := r.client.GetIscsiClient(ctx, id)
	if err != nil {
		resp.Diagnostics.Append(diagAPI("Could not read iSCSI client after disk sync", err)...)
		return
	}
	st, di := iscsiClientModelFromAPI(final, plan)
	resp.Diagnostics.Append(di...)
	resp.Diagnostics.Append(resp.State.Set(ctx, st)...)
}

func (r *iscsiClientResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "")
		return
	}
	var state iscsiClientModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id, _ := strconv.Atoi(state.ID.ValueString())
	if err := r.client.DeleteIscsiClient(ctx, id); err != nil {
		resp.Diagnostics.Append(diagAPI("Could not delete iSCSI client", err)...)
	}
}

func (r *iscsiClientResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func iscsiClientModelFromAPI(u *client.IscsiClient, prior iscsiClientModel) (iscsiClientModel, diag.Diagnostics) {
	var diags diag.Diagnostics
	out := iscsiClientModel{
		ID:           types.StringValue(strconv.Itoa(u.ID)),
		Name:         types.StringValue(u.Name),
		Owner:        types.StringValue(u.Owner),
		IQN:          types.StringValue(u.IQN),
		ChapUsername: types.StringValue(u.ChapUsername),
	}
	if u.ChapPassword != "" {
		out.ChapPassword = types.StringValue(u.ChapPassword)
	} else if !prior.ChapPassword.IsNull() {
		out.ChapPassword = prior.ChapPassword
	} else {
		out.ChapPassword = types.StringNull()
	}
	refs := u.Disks
	if refs == nil {
		refs = []client.IscsiClientDiskRef{}
	}
	list, d := diskRefsToList(refs, prior.Disks)
	diags.Append(d...)
	out.Disks = list
	return out, diags
}
