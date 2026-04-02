package provider

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/mistedo/terraform-provider-mistedo/internal/client"
)

var _ datasource.DataSource = (*instanceTemplateDataSource)(nil)

type instanceTemplateDataSource struct {
	client *client.Client
}

func newInstanceTemplateDataSource() datasource.DataSource {
	return &instanceTemplateDataSource{}
}

func (d *instanceTemplateDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_instance_template"
}

func (d *instanceTemplateDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Looks up one **service template** (VM catalog item) by name and optional version. " +
			"Calls `GET /api/compute/v1/service_templates` with a ManageIQ `filter[]` expression. " +
			"Matching is **case-sensitive** (both exact and LIKE filters on the platform).",
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Required: true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
				MarkdownDescription: "Template **product** segment: the part before `:` in the catalog display name " +
					"(e.g. `Ubuntu Server` for `Ubuntu Server:24.04.1-240905`). " +
					"Combined with `version` for an exact match, or used alone with a LIKE `%%name%%` filter. **Case-sensitive.**",
			},
			"version": schema.StringAttribute{
				Optional: true,
				MarkdownDescription: "Template **version** segment: the part after the first `:` in the display name " +
					"(e.g. `24.04.1-240905`). When set, the API match is **exact** on `name:version`. **Case-sensitive.**",
			},
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "ManageIQ service template id (use when ordering services / instance groups).",
			},
			"full_name": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Full catalog name returned by the API (usually `name:version`).",
			},
			"guid": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Template `guid` from the API when present.",
			},
		},
	}
}

func (d *instanceTemplateDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

type instanceTemplateModel struct {
	Name     types.String `tfsdk:"name"`
	Version  types.String `tfsdk:"version"`
	ID       types.String `tfsdk:"id"`
	FullName types.String `tfsdk:"full_name"`
	GUID     types.String `tfsdk:"guid"`
}

func (d *instanceTemplateDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	if d.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider did not supply an API client.")
		return
	}
	var data instanceTemplateModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	name := data.Name.ValueString()
	version := ""
	if !data.Version.IsNull() && !data.Version.IsUnknown() {
		version = data.Version.ValueString()
	}
	tpl, err := d.client.FindServiceTemplate(ctx, name, version)
	if err != nil {
		if errors.Is(err, client.ErrNotFound) {
			resp.Diagnostics.AddError(
				"No matching instance template",
				fmt.Sprintf("No catalog template matched name=%q version=%q. Check spelling and letter case.", name, version),
			)
			return
		}
		if strings.Contains(err.Error(), "multiple service templates matched") {
			resp.Diagnostics.AddError(
				"Several catalog templates match; narrow name and/or set version",
				err.Error(),
			)
			return
		}
		resp.Diagnostics.Append(diagAPI("Could not look up instance template", err)...)
		return
	}
	data.ID = types.StringValue(tpl.ID)
	data.FullName = types.StringValue(tpl.Name)
	if tpl.GUID != "" {
		data.GUID = types.StringValue(tpl.GUID)
	} else {
		data.GUID = types.StringNull()
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
