package provider

import (
	"context"
	"fmt"
	"sort"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/mistedo/terraform-provider-mistedo/internal/client"
)

var _ datasource.DataSource = (*s3PoolsDataSource)(nil)

type s3PoolsDataSource struct {
	client *client.Client
}

func newS3PoolsDataSource() datasource.DataSource {
	return &s3PoolsDataSource{}
}

func (d *s3PoolsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_s3_pools"
}

func (d *s3PoolsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists **S3-capable storage pools** (`GET /api/storage/v2/pools?type=s3`). " +
			"Use a pool **`name`** as `pool_name` on [`mistedo_s3_user`](../resources/s3_user.md).",
		Attributes: map[string]schema.Attribute{
			"pools": schema.ListNestedAttribute{
				Computed: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.Int64Attribute{
							Computed:            true,
							MarkdownDescription: "Numeric pool id (also exposed as computed `pool_id` on `mistedo_s3_user`).",
						},
						"name": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Pool name — pass to `mistedo_s3_user.pool_name`.",
						},
						"klass": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Pool class / tier from the API (`klass` in JSON).",
						},
						"type": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Pool type (always `s3` for this listing).",
						},
					},
				},
			},
		},
	}
}

func (d *s3PoolsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

type s3PoolsModel struct {
	Pools types.List `tfsdk:"pools"`
}

func (d *s3PoolsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	if d.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider did not supply an API client.")
		return
	}
	pools, err := d.client.ListS3Pools(ctx)
	if err != nil {
		resp.Diagnostics.Append(diagAPI("Could not list S3 pools", err)...)
		return
	}
	sort.Slice(pools, func(i, j int) bool {
		if pools[i].ID != pools[j].ID {
			return pools[i].ID < pools[j].ID
		}
		return pools[i].Name < pools[j].Name
	})
	attrTypes := map[string]attr.Type{
		"id":    types.Int64Type,
		"name":  types.StringType,
		"klass": types.StringType,
		"type":  types.StringType,
	}
	elems := make([]attr.Value, 0, len(pools))
	for _, p := range pools {
		ov, diags := types.ObjectValue(attrTypes, map[string]attr.Value{
			"id":    types.Int64Value(int64(p.ID)),
			"name":  types.StringValue(p.Name),
			"klass": types.StringValue(p.Class),
			"type":  types.StringValue(p.Type),
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
	out := s3PoolsModel{Pools: list}
	resp.Diagnostics.Append(resp.State.Set(ctx, &out)...)
}
