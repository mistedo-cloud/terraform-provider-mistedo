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

var _ datasource.DataSource = (*lbBackendServicesDataSource)(nil)

type lbBackendServicesDataSource struct {
	client *client.Client
}

func newLBBackendServicesDataSource() datasource.DataSource {
	return &lbBackendServicesDataSource{}
}

func (d *lbBackendServicesDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_lb_backend_services"
}

func (d *lbBackendServicesDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists compute services (instance groups) that can be attached to ALB routes (`GET /api/traefik_manager/v1/services`).",
		Attributes: map[string]schema.Attribute{
			"services": schema.ListNestedAttribute{
				Computed: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.Int64Attribute{
							Computed:            true,
							MarkdownDescription: "Internal service ID for `mistedo_lb_route` `services` blocks (`service_id`).",
						},
						"ext_id": schema.Int64Attribute{
							Computed:            true,
							MarkdownDescription: "External compute identifier.",
						},
						"name": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Service display name.",
						},
						"ipaddresses": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Comma-separated backend IPs.",
						},
						"owner": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Owner from the API.",
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

func (d *lbBackendServicesDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

type lbBackendServicesModel struct {
	Services types.List `tfsdk:"services"`
}

func (d *lbBackendServicesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	if d.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider did not supply an API client.")
		return
	}
	svcs, err := d.client.ListLBServices(ctx)
	if err != nil {
		resp.Diagnostics.Append(diagAPI("Could not list backend services", err)...)
		return
	}
	attrTypes := map[string]attr.Type{
		"id":          types.Int64Type,
		"ext_id":      types.Int64Type,
		"name":        types.StringType,
		"ipaddresses": types.StringType,
		"owner":       types.StringType,
		"account":     types.StringType,
	}
	elems := make([]attr.Value, 0, len(svcs))
	for _, s := range svcs {
		ov, diags := types.ObjectValue(attrTypes, map[string]attr.Value{
			"id":          types.Int64Value(int64(s.ID)),
			"ext_id":      types.Int64Value(s.ExtID),
			"name":        types.StringValue(s.Name),
			"ipaddresses": types.StringValue(s.IPAddresses),
			"owner":       types.StringValue(s.Owner),
			"account":     types.StringValue(s.Account),
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
	out := lbBackendServicesModel{Services: list}
	resp.Diagnostics.Append(resp.State.Set(ctx, &out)...)
}
