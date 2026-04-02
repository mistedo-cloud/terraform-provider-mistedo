package provider

import (
	"context"
	"errors"
	"fmt"
	"strconv"

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

var _ resource.Resource = (*lbCertificateResource)(nil)
var _ resource.ResourceWithImportState = (*lbCertificateResource)(nil)

type lbCertificateResource struct {
	client *client.Client
}

func newLBCertificateResource() resource.Resource {
	return &lbCertificateResource{}
}

func (r *lbCertificateResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_lb_certificate"
}

type lbCertificateModel struct {
	ID         types.String `tfsdk:"id"`
	Name       types.String `tfsdk:"name"`
	Owner      types.String `tfsdk:"owner"`
	Account    types.String `tfsdk:"account"`
	CertPEM    types.String `tfsdk:"certificate_pem"`
	KeyPEM     types.String `tfsdk:"private_key_pem"`
	CAPEM      types.String `tfsdk:"ca_pem"`
	DestCAPEM  types.String `tfsdk:"dest_ca_pem"`
	CertPath   types.String `tfsdk:"cert_path"`
	KeyPath    types.String `tfsdk:"key_path"`
	CAPath     types.String `tfsdk:"ca_path"`
	DestCAPath types.String `tfsdk:"dest_ca_path"`
	CreatedAt  types.String `tfsdk:"created_at"`
	UpdatedAt  types.String `tfsdk:"updated_at"`
}

func (r *lbCertificateResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Uploads a TLS certificate and private key to the Mistedo ALB (Traefik Manager `POST/PUT /certificates`). " +
			"The request body must use the `certificate` JSON wrapper; the provider handles that.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Certificate ID in the API.",
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Certificate name (typically the primary hostname / FQDN).",
				Validators:          []validator.String{stringvalidator.LengthBetween(1, 1024)},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"certificate_pem": schema.StringAttribute{
				Required:            true,
				Sensitive:           true,
				MarkdownDescription: "PEM-encoded leaf certificate.",
			},
			"private_key_pem": schema.StringAttribute{
				Required:            true,
				Sensitive:           true,
				MarkdownDescription: "PEM-encoded private key matching the certificate.",
			},
			"ca_pem": schema.StringAttribute{
				Optional:            true,
				Sensitive:           true,
				MarkdownDescription: "Optional PEM chain / CA.",
			},
			"dest_ca_pem": schema.StringAttribute{
				Optional:            true,
				Sensitive:           true,
				MarkdownDescription: "Optional destination CA PEM for re-encrypt scenarios.",
			},
			"owner": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Owner email; if omitted, the API sets it from the account context.",
			},
			"account": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Account from the API.",
			},
			"cert_path": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Storage path for the certificate on the gateway.",
			},
			"key_path": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Storage path for the private key.",
			},
			"ca_path": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Storage path for the CA bundle.",
			},
			"dest_ca_path": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Storage path for the destination CA.",
			},
			"created_at": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Creation timestamp from the API.",
			},
			"updated_at": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Last update timestamp from the API.",
			},
		},
	}
}

func (r *lbCertificateResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *lbCertificateResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider did not supply an API client.")
		return
	}
	var plan lbCertificateModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	w := certWriteFromModel(&plan)
	out, err := r.client.CreateCertificate(ctx, w)
	if err != nil {
		resp.Diagnostics.Append(diagAPI("Could not create certificate", err)...)
		return
	}
	state := certModelFromAPI(out, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *lbCertificateResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider did not supply an API client.")
		return
	}
	var state lbCertificateModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id, err := strconv.Atoi(state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid id", state.ID.ValueString())
		return
	}
	cert, err := r.client.GetCertificate(ctx, id)
	if err != nil {
		if errors.Is(err, client.ErrNotFound) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.Append(diagAPI("Could not read certificate", err)...)
		return
	}
	next := certModelFromAPI(cert, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &next)...)
}

func (r *lbCertificateResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider did not supply an API client.")
		return
	}
	var plan lbCertificateModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id, err := strconv.Atoi(plan.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid id", plan.ID.ValueString())
		return
	}
	w := certWriteFromModel(&plan)
	out, err := r.client.UpdateCertificate(ctx, id, w)
	if err != nil {
		resp.Diagnostics.Append(diagAPI("Could not update certificate", err)...)
		return
	}
	state := certModelFromAPI(out, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *lbCertificateResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	if r.client == nil {
		return
	}
	var state lbCertificateModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id, err := strconv.Atoi(state.ID.ValueString())
	if err != nil {
		return
	}
	err = r.client.DeleteCertificate(ctx, id)
	if err != nil && !errors.Is(err, client.ErrNotFound) {
		resp.Diagnostics.Append(diagAPI("Could not delete certificate", err)...)
	}
}

func (r *lbCertificateResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func certWriteFromModel(m *lbCertificateModel) *client.CertificateWrite {
	w := &client.CertificateWrite{
		Name:   m.Name.ValueString(),
		Cert:   m.CertPEM.ValueString(),
		Key:    m.KeyPEM.ValueString(),
		CA:     m.CAPEM.ValueString(),
		DestCA: m.DestCAPEM.ValueString(),
	}
	if !m.Owner.IsNull() && !m.Owner.IsUnknown() {
		w.Owner = m.Owner.ValueString()
	}
	return w
}

func certModelFromAPI(c *client.Certificate, prior *lbCertificateModel) lbCertificateModel {
	m := lbCertificateModel{
		ID:        types.StringValue(strconv.Itoa(c.ID)),
		Name:      types.StringValue(c.Name),
		Account:   stringOrStringNull(c.Account),
		CreatedAt: stringOrStringNull(c.CreatedAt),
		UpdatedAt: stringOrStringNull(c.UpdatedAt),
	}
	m.Owner = stringOrStringNull(c.Owner)
	m.CertPath = stringOrStringNull(c.CertPath)
	m.KeyPath = stringOrStringNull(c.KeyPath)
	m.CAPath = stringOrStringNull(c.CAPath)
	m.DestCAPath = stringOrStringNull(c.DestCAPath)
	if c.Values != nil {
		m.CertPEM = types.StringValue(c.Values.Cert)
		m.KeyPEM = types.StringValue(c.Values.Key)
		m.CAPEM = stringOrStringNull(c.Values.CA)
		m.DestCAPEM = stringOrStringNull(c.Values.DestCA)
	} else if prior != nil {
		m.CertPEM = prior.CertPEM
		m.KeyPEM = prior.KeyPEM
		m.CAPEM = prior.CAPEM
		m.DestCAPEM = prior.DestCAPEM
	}
	return m
}

func stringOrStringNull(s string) types.String {
	if s == "" {
		return types.StringNull()
	}
	return types.StringValue(s)
}
