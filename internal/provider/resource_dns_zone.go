package provider

import (
	"context"
	"errors"
	"fmt"

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

var _ resource.Resource = (*dnsZoneResource)(nil)
var _ resource.ResourceWithImportState = (*dnsZoneResource)(nil)

type dnsZoneResource struct {
	client *client.Client
}

func newDNSZoneResource() resource.Resource {
	return &dnsZoneResource{}
}

func (r *dnsZoneResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dns_zone"
}

func (r *dnsZoneResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A public DNS hosted zone (similar in purpose to `aws_route53_zone` or `google_dns_managed_zone`). " +
			"The zone name must be a fully qualified domain name. Account context comes from the provider; the API associates the zone with that account.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The zone FQDN; identical to `name`.",
			},
			"name": schema.StringAttribute{
				Required: true,
				MarkdownDescription: "FQDN of the zone without a trailing dot (the API rejects a trailing dot). " +
					"Changing this forces replacement.",
				Validators: []validator.String{
					stringvalidator.LengthBetween(1, 1024),
					zoneFQDNNoTrailingDotValidator{},
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"account": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Account identifier returned by the API (metadata).",
			},
			"owner": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Owner email or principal from API metadata, if present.",
			},
		},
	}
}

func (r *dnsZoneResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *dnsZoneResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider did not supply an API client. Check provider configuration.")
		return
	}
	var plan dnsZoneModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	name := plan.Name.ValueString()
	z, err := r.client.CreateZone(ctx, name)
	if err != nil {
		resp.Diagnostics.Append(diagAPI("Could not create DNS zone", err)...)
		return
	}
	state := zoneModelFromAPI(z)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *dnsZoneResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider did not supply an API client.")
		return
	}
	var state dnsZoneModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	name := state.Name.ValueString()
	if name == "" {
		name = state.ID.ValueString()
	}
	z, err := r.client.GetZone(ctx, name)
	if err != nil {
		if errors.Is(err, client.ErrNotFound) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.Append(diagAPI("Could not read DNS zone", err)...)
		return
	}
	newState := zoneModelFromAPI(z)
	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func zoneModelFromAPI(z *client.Zone) dnsZoneModel {
	m := dnsZoneModel{
		ID:      types.StringValue(z.Name),
		Name:    types.StringValue(z.Name),
		Account: types.StringNull(),
		Owner:   types.StringNull(),
	}
	if z.Metadata != nil {
		if z.Metadata.Account != "" {
			m.Account = types.StringValue(z.Metadata.Account)
		}
		if z.Metadata.Owner != "" {
			m.Owner = types.StringValue(z.Metadata.Owner)
		}
	}
	return m
}

func (r *dnsZoneResource) Update(context.Context, resource.UpdateRequest, *resource.UpdateResponse) {
}

func (r *dnsZoneResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider did not supply an API client.")
		return
	}
	var state dnsZoneModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	name := state.Name.ValueString()
	if name == "" {
		name = state.ID.ValueString()
	}
	if err := r.client.DeleteZone(ctx, name); err != nil {
		resp.Diagnostics.Append(diagAPI("Could not delete DNS zone", err)...)
		return
	}
}

func (r *dnsZoneResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("name"), req, resp)
}

type dnsZoneModel struct {
	ID      types.String `tfsdk:"id"`
	Name    types.String `tfsdk:"name"`
	Account types.String `tfsdk:"account"`
	Owner   types.String `tfsdk:"owner"`
}
