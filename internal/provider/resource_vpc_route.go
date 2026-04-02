package provider

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/mistedo/terraform-provider-mistedo/internal/client"
)

var _ resource.Resource = (*vpcRouteResource)(nil)
var _ resource.ResourceWithImportState = (*vpcRouteResource)(nil)

type vpcRouteResource struct {
	client *client.Client
}

func newVPCRouteResource() resource.Resource {
	return &vpcRouteResource{}
}

func (r *vpcRouteResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_vpc_route"
}

type vpcRouteModel struct {
	ID          types.String `tfsdk:"id"`
	RouterID    types.String `tfsdk:"router_id"`
	Destination types.String `tfsdk:"destination"`
	Nexthop     types.String `tfsdk:"nexthop"`
}

func vpcRouteResourceID(routerID, destination, nexthop string) string {
	return strings.TrimSpace(routerID) + "|" + strings.TrimSpace(destination) + "|" + strings.TrimSpace(nexthop)
}

func (r *vpcRouteResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A **static VPC route** on the tenant **default network router** (the platform provisions one router per account; it cannot be created or deleted in Terraform). " +
			"The provider **resolves the router id** from the API; you only set **destination** (CIDR) and **nexthop**. " +
			"Terraform **id** is `router_id|destination|nexthop` (stable surrogate because the API does not assign route ids).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Surrogate id: `<router_id>|<destination>|<nexthop>`.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"router_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Cloud id of the default network router (from `GET .../network_routers`); filled by the provider.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"destination": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Route destination CIDR (e.g. `10.220.0.0/16`).",
				Validators: []validator.String{
					stringvalidator.LengthBetween(1, 128),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"nexthop": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Next hop IP address for the route.",
				Validators: []validator.String{
					stringvalidator.LengthBetween(1, 128),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
		},
	}
}

func (r *vpcRouteResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *vpcRouteResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider did not supply an API client.")
		return
	}
	var plan vpcRouteModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	dest := strings.TrimSpace(plan.Destination.ValueString())
	hop := strings.TrimSpace(plan.Nexthop.ValueString())
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	routerID, err := r.client.ResolveDefaultNetworkRouterID(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Could not resolve default network router", err.Error())
		return
	}
	if err := r.client.AddNetworkRouterRoute(ctx, routerID, dest, hop); err != nil {
		resp.Diagnostics.Append(diagAPI("Could not add VPC route", err)...)
		return
	}
	rt, err := r.client.GetNetworkRouter(ctx, routerID)
	if err != nil {
		resp.Diagnostics.Append(diagAPI("Could not read router after add", err)...)
		return
	}
	if !client.RouterHasRoute(rt, dest, hop) {
		resp.Diagnostics.AddError("Route missing after create", "POST succeeded but GET router does not list the new route yet; retry apply.")
		return
	}
	state := vpcRouteModel{
		ID:          types.StringValue(vpcRouteResourceID(routerID, dest, hop)),
		RouterID:    types.StringValue(routerID),
		Destination: plan.Destination,
		Nexthop:     plan.Nexthop,
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *vpcRouteResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider did not supply an API client.")
		return
	}
	var state vpcRouteModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	routerID := strings.TrimSpace(state.RouterID.ValueString())
	dest := strings.TrimSpace(state.Destination.ValueString())
	hop := strings.TrimSpace(state.Nexthop.ValueString())
	if routerID == "" || dest == "" || hop == "" {
		resp.Diagnostics.AddError("Invalid state", "router_id, destination, and nexthop must be set.")
		return
	}
	rt, err := r.client.GetNetworkRouter(ctx, routerID)
	if err != nil {
		if errors.Is(err, client.ErrNotFound) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.Append(diagAPI("Could not read network router", err)...)
		return
	}
	if !client.RouterHasRoute(rt, dest, hop) {
		resp.State.RemoveResource(ctx)
		return
	}
	next := vpcRouteModel{
		ID:          types.StringValue(vpcRouteResourceID(routerID, dest, hop)),
		RouterID:    types.StringValue(routerID),
		Destination: state.Destination,
		Nexthop:     state.Nexthop,
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &next)...)
}

func (r *vpcRouteResource) Update(ctx context.Context, _ resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError(
		"Update not supported",
		"Change destination or nexthop to replace the route (remove + add).",
	)
	_ = ctx
}

func (r *vpcRouteResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider did not supply an API client.")
		return
	}
	var state vpcRouteModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	routerID := strings.TrimSpace(state.RouterID.ValueString())
	dest := strings.TrimSpace(state.Destination.ValueString())
	hop := strings.TrimSpace(state.Nexthop.ValueString())
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	err := r.client.RemoveNetworkRouterRoute(ctx, routerID, dest, hop)
	if err != nil {
		resp.Diagnostics.Append(diagAPI("Could not remove VPC route", err)...)
	}
}

func (r *vpcRouteResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider did not supply an API client.")
		return
	}
	dest, hop, err := parseVPCRouteImportID(req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Invalid import id", err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	routerID, err := r.client.ResolveDefaultNetworkRouterID(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Could not resolve default network router", err.Error())
		return
	}
	rt, err := r.client.GetNetworkRouter(ctx, routerID)
	if err != nil {
		resp.Diagnostics.Append(diagAPI("Could not read network router for import", err)...)
		return
	}
	if !client.RouterHasRoute(rt, dest, hop) {
		resp.Diagnostics.AddError(
			"Route not found",
			fmt.Sprintf("No route destination=%q nexthop=%q on default router %s", dest, hop, routerID),
		)
		return
	}
	state := vpcRouteModel{
		ID:          types.StringValue(vpcRouteResourceID(routerID, dest, hop)),
		RouterID:    types.StringValue(routerID),
		Destination: types.StringValue(dest),
		Nexthop:     types.StringValue(hop),
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// parseVPCRouteImportID accepts destination|nexthop (single | separator; CIDRs use /, not |).
func parseVPCRouteImportID(id string) (destination, nexthop string, err error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return "", "", fmt.Errorf("import id is empty")
	}
	parts := strings.SplitN(id, "|", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
		return "", "", fmt.Errorf("import id must be destination|nexthop (e.g. 10.220.0.0/16|10.220.0.2)")
	}
	return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]), nil
}
