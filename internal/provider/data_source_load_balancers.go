package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/mistedo/terraform-provider-mistedo/internal/client"
)

var _ datasource.DataSource = (*loadBalancersDataSource)(nil)

type loadBalancersDataSource struct {
	client *client.Client
}

func newLoadBalancersDataSource() datasource.DataSource {
	return &loadBalancersDataSource{}
}

func (d *loadBalancersDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_load_balancers"
}

func (d *loadBalancersDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists ALB Cloud Gateways (`GET /api/traefik_manager/v1/gateways`).",
		Attributes: map[string]schema.Attribute{
			"gateways": schema.ListNestedAttribute{
				Computed: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.Int64Attribute{
							Computed:            true,
							MarkdownDescription: "Gateway numeric ID (use as `cloud_gateway_id` on `mistedo_lb_route`).",
						},
						"name": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Gateway name (e.g. `alb-default`).",
						},
						"cloudgw_id": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "UUID of the CloudGateway VM.",
						},
						"cloudgw_instance": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Display name of the gateway instance.",
						},
						"account": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Account identifier.",
						},
					},
				},
			},
		},
	}
}

func (d *loadBalancersDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Invalid provider data", fmt.Sprintf("Expected a configured Mistedo client, got %T.", req.ProviderData))
		return
	}
	d.client = c
}

type loadBalancersModel struct {
	Gateways types.List `tfsdk:"gateways"`
}

func (d *loadBalancersDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	if d.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider did not supply an API client.")
		return
	}
	gws, err := d.client.ListGateways(ctx)
	if err != nil {
		resp.Diagnostics.Append(diagAPI("Could not list load balancers", err)...)
		return
	}
	attrTypes := map[string]attr.Type{
		"id":               types.Int64Type,
		"name":             types.StringType,
		"cloudgw_id":       types.StringType,
		"cloudgw_instance": types.StringType,
		"account":          types.StringType,
	}
	elems := make([]attr.Value, 0, len(gws))
	for _, g := range gws {
		ov, diags := types.ObjectValue(attrTypes, map[string]attr.Value{
			"id":               types.Int64Value(int64(g.ID)),
			"name":             types.StringValue(g.Name),
			"cloudgw_id":       types.StringValue(g.CloudgwID),
			"cloudgw_instance": types.StringValue(g.CloudgwInstance),
			"account":          types.StringValue(g.Account),
		})
		resp.Diagnostics.Append(diags...)
		if diags.HasError() {
			return
		}
		elems = append(elems, ov)
	}
	list, diags := types.ListValue(types.ObjectType{AttrTypes: attrTypes}, elems)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	out := loadBalancersModel{Gateways: list}
	resp.Diagnostics.Append(resp.State.Set(ctx, &out)...)
}
