package provider

import (
	"context"
	"errors"
	"fmt"
	"time"

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

var _ resource.Resource = (*securityGroupResource)(nil)
var _ resource.ResourceWithImportState = (*securityGroupResource)(nil)

type securityGroupResource struct {
	client *client.Client
}

func newSecurityGroupResource() resource.Resource {
	return &securityGroupResource{}
}

func (r *securityGroupResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_security_group"
}

type securityGroupModel struct {
	ID            types.String `tfsdk:"id"`
	Name          types.String `tfsdk:"name"`
	CanonicalName types.String `tfsdk:"canonical_name"`
	EmsRef        types.String `tfsdk:"ems_ref"`
}

func (r *securityGroupResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A compute security group (ManageIQ firewall group). " +
			"API calls use `x-miq-group` (`<account>.<role>`) and async tasks; the provider waits for task completion. " +
			"Two default egress rules (IPv4/IPv6) may appear with a delay; the provider waits for them, then removes all automatic rules so you can manage access with `mistedo_security_group_rule`.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Cloud security group id returned by the compute API.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required: true,
				MarkdownDescription: "Name you send when creating the group (short label). The API may add a prefix; see `canonical_name`. This value stays in state as configured so Terraform does not report a false mismatch after apply.",
				Validators: []validator.String{
					stringvalidator.LengthBetween(1, 256),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"canonical_name": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Full group name returned by the API (e.g. `<location>_<account>_<name>`). Used for delete requests.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"ems_ref": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "ManageIQ `ems_ref` for the security group (used by the API for rule payloads).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *securityGroupResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Invalid provider data", fmt.Sprintf("Expected a configured Mistedo client, got %T.", req.ProviderData))
		return
	}
	r.client = c
}

func (r *securityGroupResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider did not supply an API client.")
		return
	}
	var plan securityGroupModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()
	sg, err := r.client.CreateSecurityGroup(ctx, plan.Name.ValueString())
	if err != nil {
		resp.Diagnostics.Append(diagAPI("Could not create security group", err)...)
		return
	}
	state := securityGroupModel{
		ID:            types.StringValue(sg.ID),
		Name:          plan.Name,
		CanonicalName: types.StringValue(sg.Name),
		EmsRef:        types.StringValue(sg.EmsRef),
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *securityGroupResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider did not supply an API client.")
		return
	}
	var state securityGroupModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id := state.ID.ValueString()
	if id == "" {
		resp.Diagnostics.AddError("Missing id", "Security group id is empty in state.")
		return
	}
	sg, err := r.client.GetSecurityGroup(ctx, id)
	if err != nil {
		if errors.Is(err, client.ErrNotFound) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.Append(diagAPI("Could not read security group", err)...)
		return
	}
	nameInState := state.Name
	if nameInState.IsNull() || nameInState.ValueString() == "" {
		nameInState = types.StringValue(sg.Name)
	}
	next := securityGroupModel{
		ID:            types.StringValue(sg.ID),
		Name:          nameInState,
		CanonicalName: types.StringValue(sg.Name),
		EmsRef:        types.StringValue(sg.EmsRef),
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &next)...)
}

func (r *securityGroupResource) Update(ctx context.Context, _ resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError(
		"Update not supported",
		"Security groups are replaced when `name` changes. No other attributes are updatable in this resource.",
	)
	_ = ctx
}

func (r *securityGroupResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider did not supply an API client.")
		return
	}
	var state securityGroupModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()
	apiName := state.CanonicalName.ValueString()
	if apiName == "" {
		apiName = state.Name.ValueString()
	}
	err := r.client.DeleteSecurityGroup(ctx, state.ID.ValueString(), apiName)
	if err != nil {
		if errors.Is(err, client.ErrNotFound) {
			return
		}
		resp.Diagnostics.Append(diagAPI("Could not delete security group", err)...)
	}
}

func (r *securityGroupResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

