package provider

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/mistedo/terraform-provider-mistedo/internal/client"
)

var _ resource.Resource = (*networkResource)(nil)
var _ resource.ResourceWithImportState = (*networkResource)(nil)

type networkResource struct {
	client *client.Client
}

func newNetworkResource() resource.Resource {
	return &networkResource{}
}

func (r *networkResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_network"
}

type networkModel struct {
	ID             types.String `tfsdk:"id"`
	Name           types.String `tfsdk:"name"`
	CanonicalName  types.String `tfsdk:"canonical_name"`
	Cidr           types.String `tfsdk:"cidr"`
	DnsNameservers types.List   `tfsdk:"dns_nameservers"`
	EmsRef         types.String `tfsdk:"ems_ref"`
	Status         types.String `tfsdk:"status"`
	Mtu            types.Int64  `tfsdk:"mtu"`
	SubnetID       types.String `tfsdk:"subnet_id"`
	SubnetEmsRef   types.String `tfsdk:"subnet_ems_ref"`
	Gateway        types.String `tfsdk:"gateway"`
}

func replaceOnChangeList() []planmodifier.List {
	return []planmodifier.List{listplanmodifier.RequiresReplace()}
}

func (r *networkResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A tenant **private network** (IPv4) in the regional compute API. " +
			"The platform fixes **IP version** (`4`) and **network protocol** (`ipv4`) in the create payload; only **`name`**, **`cidr`**, and **`dns_nameservers`** are user-controlled. " +
			"Create and delete run as **asynchronous** jobs; the provider waits for the task and until the first subnet exists. " +
			"Changing name, CIDR, or DNS servers **replaces** the network.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Cloud network id from the compute API.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required: true,
				MarkdownDescription: "Name for the network and for the subnet in the API create body (the same string is sent as top-level `name` and as `subnet.name`). " +
					"The API may add a prefix to the stored network name; see `canonical_name`. " +
					"This value stays in state as configured so Terraform does not report a false mismatch after apply.",
				Validators: []validator.String{
					stringvalidator.LengthBetween(1, 256),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"canonical_name": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Full network name returned by the API (often `<location>_<account>_<name>`).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"cidr": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Subnet CIDR for the single subnet created with the network (e.g. `10.0.0.0/24`).",
				Validators: []validator.String{
					stringvalidator.LengthBetween(1, 64),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"dns_nameservers": schema.ListAttribute{
				ElementType: types.StringType,
				Optional:    true,
				MarkdownDescription: "Optional DNS servers for the subnet. " +
					"If you omit this argument, it stays unset in state even if the platform fills in servers (avoids perpetual drift). " +
					"Set it explicitly when you want Terraform to track and refresh the list from the API.",
				Validators: []validator.List{
					listvalidator.ValueStringsAre(stringvalidator.LengthBetween(1, 256)),
				},
				PlanModifiers: replaceOnChangeList(),
			},
			"ems_ref": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "ManageIQ `ems_ref` for the network.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"status": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Status string from the API when present.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"mtu": schema.Int64Attribute{
				Computed:            true,
				MarkdownDescription: "`maximum_transmission_unit` from `extra_attributes` when the API exposes it.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"subnet_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Cloud id of the first (only) subnet on this network.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"subnet_ems_ref": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "ManageIQ `ems_ref` for the subnet when present.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"gateway": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Subnet gateway from the API when present.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *networkResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *networkResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider did not supply an API client.")
		return
	}
	var plan networkModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	dnsSlice, d := expandOptionalStringList(ctx, plan.DnsNameservers)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}
	in := client.NetworkSubnetCreateInput{
		Cidr:           plan.Cidr.ValueString(),
		DnsNameservers: dnsSlice,
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()
	net, err := r.client.CreateNetwork(ctx, plan.Name.ValueString(), in)
	if err != nil {
		resp.Diagnostics.Append(diagAPI("Could not create network", err)...)
		return
	}
	prior := networkModel{
		Cidr:           plan.Cidr,
		DnsNameservers: plan.DnsNameservers,
	}
	next, diags := networkModelFromAPI(net, plan.Name, prior)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &next)...)
}

func (r *networkResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider did not supply an API client.")
		return
	}
	var state networkModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id := state.ID.ValueString()
	if id == "" {
		resp.Diagnostics.AddError("Missing id", "Network id is empty in state.")
		return
	}
	net, err := r.client.GetNetwork(ctx, id)
	if err != nil {
		if errors.Is(err, client.ErrNotFound) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.Append(diagAPI("Could not read network", err)...)
		return
	}
	nameInState := state.Name
	if nameInState.IsNull() || nameInState.ValueString() == "" {
		nameInState = types.StringValue(net.Name)
	}
	next, diags := networkModelFromAPI(net, nameInState, state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &next)...)
}

func (r *networkResource) Update(ctx context.Context, _ resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError(
		"Update not supported",
		"Private networks are replaced when you change in-place attributes handled by this resource. Destroy and re-create is required for other API changes.",
	)
	_ = ctx
}

func (r *networkResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider did not supply an API client.")
		return
	}
	var state networkModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()
	err := r.client.DeleteNetwork(ctx, state.ID.ValueString())
	if err != nil {
		if errors.Is(err, client.ErrNotFound) {
			return
		}
		resp.Diagnostics.Append(diagAPI("Could not delete network", err)...)
	}
}

func (r *networkResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func primarySubnet(net *client.Network) *client.NetworkSubnet {
	if net == nil || len(net.Subnets) == 0 {
		return nil
	}
	return &net.Subnets[0]
}

func expandOptionalStringList(ctx context.Context, l types.List) ([]string, diag.Diagnostics) {
	var diags diag.Diagnostics
	if l.IsNull() || l.IsUnknown() {
		return nil, diags
	}
	var out []string
	diags.Append(l.ElementsAs(ctx, &out, false)...)
	return out, diags
}

func listFromStrings(elems []string) (types.List, diag.Diagnostics) {
	var diags diag.Diagnostics
	if len(elems) == 0 {
		l, d := types.ListValue(types.StringType, []attr.Value{})
		diags.Append(d...)
		return l, diags
	}
	vals := make([]attr.Value, len(elems))
	for i, s := range elems {
		vals[i] = types.StringValue(s)
	}
	l, d := types.ListValue(types.StringType, vals)
	diags.Append(d...)
	return l, diags
}

// networkModelFromAPI maps a GET network into Terraform state. prior controls whether dns_nameservers
// stays null when omitted in config.
func networkModelFromAPI(net *client.Network, nameInState types.String, prior networkModel) (networkModel, diag.Diagnostics) {
	var diags diag.Diagnostics
	sub := primarySubnet(net)
	displayName := nameInState
	if displayName.IsNull() || displayName.ValueString() == "" {
		if net != nil {
			displayName = types.StringValue(net.Name)
		}
	}
	var dns types.List
	if prior.DnsNameservers.IsNull() {
		dns = types.ListNull(types.StringType)
	} else {
		if sub == nil {
			var d diag.Diagnostics
			dns, d = listFromStrings(nil)
			diags.Append(d...)
		} else {
			var d diag.Diagnostics
			dns, d = listFromStrings(sub.DnsNameservers)
			diags.Append(d...)
		}
	}
	cidrVal := prior.Cidr
	if sub != nil && sub.Cidr != "" {
		cidrVal = types.StringValue(sub.Cidr)
	}
	gw := prior.Gateway
	if sub != nil && sub.Gateway != "" {
		gw = types.StringValue(sub.Gateway)
	}
	subID := prior.SubnetID
	if sub != nil && sub.ID != "" {
		subID = types.StringValue(sub.ID)
	}
	subEms := prior.SubnetEmsRef
	if sub != nil && sub.EmsRef != "" {
		subEms = types.StringValue(sub.EmsRef)
	}
	mtu := prior.Mtu
	if net != nil {
		if m := net.MTU(); m != 0 {
			mtu = types.Int64Value(m)
		}
	}
	status := prior.Status
	if net != nil && strings.TrimSpace(net.Status) != "" {
		status = types.StringValue(strings.TrimSpace(net.Status))
	}
	ems := prior.EmsRef
	if net != nil && net.EmsRef != "" {
		ems = types.StringValue(net.EmsRef)
	}
	canonical := prior.CanonicalName
	if net != nil {
		canonical = types.StringValue(net.Name)
	}
	idVal := prior.ID
	if net != nil && net.ID != "" {
		idVal = types.StringValue(net.ID)
	}
	return networkModel{
		ID:             idVal,
		Name:           displayName,
		CanonicalName:  canonical,
		Cidr:           cidrVal,
		DnsNameservers: dns,
		EmsRef:         ems,
		Status:         status,
		Mtu:            mtu,
		SubnetID:       subID,
		SubnetEmsRef:   subEms,
		Gateway:        gw,
	}, diags
}
