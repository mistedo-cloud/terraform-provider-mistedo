package provider

import (
	"context"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/mistedo/terraform-provider-mistedo/internal/client"
)

var _ provider.Provider = (*mistedoProvider)(nil)

type mistedoProvider struct {
	version string
}

type mistedoProviderModel struct {
	Username   types.String `tfsdk:"username"`
	Password   types.String `tfsdk:"password"`
	Account    types.String `tfsdk:"account"`
	Location   types.String `tfsdk:"location"`
	Role       types.String `tfsdk:"role"`
	AuthURL    types.String `tfsdk:"auth_url"`
	AuthRealm  types.String `tfsdk:"auth_realm"`
	AuthClient types.String `tfsdk:"auth_client"`
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &mistedoProvider{version: version}
	}
}

func (p *mistedoProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "mistedo"
}

func (p *mistedoProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"username": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Username for Keycloak authentication.",
			},
			"password": schema.StringAttribute{
				Required:            true,
				Sensitive:           true,
				MarkdownDescription: "Password for Keycloak authentication.",
			},
			"account": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Account identifier.",
			},
			"location": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Location code (used in API host `api.<location>.mistedo.by`).",
			},
			"role": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "API role (sent as `x-auth-role` with `x-auth-account` / `x-auth-group`).",
			},
			"auth_url": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Keycloak base URL including `/realms` (e.g. `https://auth.mistedo.by/realms`).",
			},
			"auth_realm": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Keycloak realm. Defaults to \"master\".",
			},
			"auth_client": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Keycloak OAuth2 client_id. Defaults to \"cloud-console\" (Mistedo cloud console).",
			},
		},
	}
}

func (p *mistedoProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var config mistedoProviderModel
	diags := req.Config.Get(ctx, &config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	username := os.Getenv("MISTEDO_USERNAME")
	password := os.Getenv("MISTEDO_PASSWORD")
	account := os.Getenv("MISTEDO_ACCOUNT")
	location := os.Getenv("MISTEDO_LOCATION")
	role := os.Getenv("MISTEDO_ROLE")

	if !config.Username.IsNull() {
		username = config.Username.ValueString()
	}
	if !config.Password.IsNull() {
		password = config.Password.ValueString()
	}
	if !config.Account.IsNull() {
		account = config.Account.ValueString()
	}
	if !config.Location.IsNull() {
		location = config.Location.ValueString()
	}
	if !config.Role.IsNull() {
		role = config.Role.ValueString()
	}

	var authURL, authRealm, authClientID string
	if !config.AuthURL.IsNull() {
		authURL = config.AuthURL.ValueString()
	} else {
		authURL = "https://auth.mistedo.by/realms"
	}
	if !config.AuthRealm.IsNull() {
		authRealm = config.AuthRealm.ValueString()
	} else {
		authRealm = "master"
	}
	if !config.AuthClient.IsNull() {
		authClientID = config.AuthClient.ValueString()
	} else {
		authClientID = "cloud-console"
	}

	if username == "" {
		resp.Diagnostics.AddError("Missing username", "MISTEDO_USERNAME or provider username must be set.")
		return
	}
	if password == "" {
		resp.Diagnostics.AddError("Missing password", "MISTEDO_PASSWORD or provider password must be set.")
		return
	}
	if account == "" {
		resp.Diagnostics.AddError("Missing account", "MISTEDO_ACCOUNT or provider account must be set.")
		return
	}
	if location == "" {
		resp.Diagnostics.AddError("Missing location", "MISTEDO_LOCATION or provider location must be set.")
		return
	}
	if role == "" {
		resp.Diagnostics.AddError("Missing role", "MISTEDO_ROLE or provider role must be set.")
		return
	}

	cfg := &client.Config{
		Username:     username,
		Password:     password,
		Account:      account,
		Location:     location,
		Role:         role,
		AuthURL:      authURL,
		AuthRealm:    authRealm,
		AuthClientID: authClientID,
	}
	c, err := client.New(cfg)
	if err != nil {
		resp.Diagnostics.AddError("Unable to create Mistedo API client", err.Error())
		return
	}
	if err := c.EnsureToken(ctx); err != nil {
		resp.Diagnostics.AddError("Authentication failed", "Could not obtain token from Keycloak: "+err.Error())
		return
	}
	resp.ResourceData = c
	resp.DataSourceData = c
}

func (p *mistedoProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		newLoadBalancersDataSource,
		newLBBackendServicesDataSource,
		newInstanceTemplateDataSource,
		newS3PoolsDataSource,
	}
}

func (p *mistedoProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		newDNSZoneResource,
		newDNSRecordResource,
		newLBRouteResource,
		newLBCertificateResource,
		newNetworkResource,
		newVPCRouteResource,
		newSecurityGroupResource,
		newSecurityGroupRuleResource,
		newInstanceGroupResource,
		newS3UserResource,
		newS3BucketResource,
		newIscsiDiskResource,
		newIscsiClientResource,
	}
}
