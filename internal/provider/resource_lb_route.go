package provider

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/float64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/mistedo/terraform-provider-mistedo/internal/client"
)

var _ resource.Resource = (*lbRouteResource)(nil)
var _ resource.ResourceWithImportState = (*lbRouteResource)(nil)

type lbRouteResource struct {
	client *client.Client
}

func newLBRouteResource() resource.Resource {
	return &lbRouteResource{}
}

func (r *lbRouteResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_lb_route"
}

type lbRouteModel struct {
	ID               types.String `tfsdk:"id"`
	Name             types.String `tfsdk:"name"`
	Hostname         types.String `tfsdk:"hostname"`
	Path             types.String `tfsdk:"path"`
	TargetPort       types.Int64  `tfsdk:"target_port"`
	CloudGatewayID   types.Int64  `tfsdk:"cloud_gateway_id"`
	IPVersion        types.String `tfsdk:"ip_version"`
	Insecure         types.String `tfsdk:"insecure"`
	TLSTermination   types.String `tfsdk:"tls_termination"`
	CertificateID    types.Int64  `tfsdk:"certificate_id"`
	Services         types.List   `tfsdk:"services"`
	Healthcheck      types.Object `tfsdk:"healthcheck"`
	SourceProto      types.String `tfsdk:"source_proto"`
	DestinationProto types.String `tfsdk:"destination_proto"`
	Labels           types.String `tfsdk:"labels"`
	Owner            types.String `tfsdk:"owner"`
	GatewayName      types.String `tfsdk:"gateway_name"`
}

type lbRouteServiceModel struct {
	ServiceID   types.Int64   `tfsdk:"service_id"`
	Weight      types.Float64 `tfsdk:"weight"`
	BalanceType types.String  `tfsdk:"balance_type"`
}

type hcModel struct {
	Path            types.String `tfsdk:"path"`
	Scheme          types.String `tfsdk:"scheme"`
	Hostname        types.String `tfsdk:"hostname"`
	Port            types.Int64  `tfsdk:"port"`
	Interval        types.Int64  `tfsdk:"interval"`
	Timeout         types.Int64  `tfsdk:"timeout"`
	Method          types.String `tfsdk:"method"`
	FollowRedirects types.Bool   `tfsdk:"follow_redirects"`
	Headers         types.Map    `tfsdk:"headers"`
}

func (r *lbRouteResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	stateUnknown := []planmodifier.String{stringplanmodifier.UseStateForUnknown()}
	stateUnknownInt := []planmodifier.Int64{int64planmodifier.UseStateForUnknown()}
	stateUnknownBool := []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()}
	stateUnknownMap := []planmodifier.Map{mapplanmodifier.UseStateForUnknown()}

	hcAttrs := map[string]schema.Attribute{
		"path": schema.StringAttribute{
			Optional:            true,
			Computed:            true,
			MarkdownDescription: "HTTP path for the health check.",
			PlanModifiers:       stateUnknown,
		},
		"scheme": schema.StringAttribute{
			Optional:            true,
			Computed:            true,
			MarkdownDescription: "URL scheme (e.g. `http`, `https`).",
			PlanModifiers:       stateUnknown,
		},
		"hostname": schema.StringAttribute{
			Optional:            true,
			Computed:            true,
			MarkdownDescription: "Host header for the health check request.",
			PlanModifiers:       stateUnknown,
		},
		"port": schema.Int64Attribute{
			Optional:            true,
			Computed:            true,
			MarkdownDescription: "Port to probe.",
			PlanModifiers:       stateUnknownInt,
		},
		"interval": schema.Int64Attribute{
			Optional:            true,
			Computed:            true,
			MarkdownDescription: "Interval between checks in seconds.",
			PlanModifiers:       stateUnknownInt,
		},
		"timeout": schema.Int64Attribute{
			Optional:            true,
			Computed:            true,
			MarkdownDescription: "Timeout in seconds.",
			PlanModifiers:       stateUnknownInt,
		},
		"method": schema.StringAttribute{
			Optional:            true,
			Computed:            true,
			MarkdownDescription: "HTTP method (e.g. `GET`).",
			PlanModifiers:       stateUnknown,
		},
		"follow_redirects": schema.BoolAttribute{
			Optional:            true,
			Computed:            true,
			MarkdownDescription: "Whether to follow HTTP redirects.",
			PlanModifiers:       stateUnknownBool,
		},
		"headers": schema.MapAttribute{
			ElementType: types.StringType,
			Optional:    true,
			Computed:    true,
			MarkdownDescription: "Arbitrary HTTP header names to values for the probe (any number of entries). " +
				"Example: `headers = { Auth = \"token\", \"X-Custom\" = \"v\" }`.",
			PlanModifiers: stateUnknownMap,
		},
	}

	resp.Schema = schema.Schema{
		MarkdownDescription: "An HTTP(S) routing rule on the Mistedo Application Load Balancer (Traefik Manager API). " +
			"Maps to `POST/PUT /api/traefik_manager/v1/routes`. Always sends a full route body on update; do not rely on partial patches.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Route ID returned by the API.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Display name of the route.",
				Validators:          []validator.String{stringvalidator.LengthBetween(1, 1024)},
			},
			"hostname": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Public hostname this route matches.",
				Validators:          []validator.String{stringvalidator.LengthBetween(1, 1024)},
			},
			"path": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "URL path prefix. Defaults to `/`.",
				Default:             stringdefault.StaticString("/"),
			},
			"target_port": schema.Int64Attribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Backend port on the service. Defaults to 80.",
				Default:             int64default.StaticInt64(80),
			},
			"cloud_gateway_id": schema.Int64Attribute{
				Required:            true,
				MarkdownDescription: "Cloud Gateway ID (`GET /gateways` or data source `mistedo_load_balancers`).",
			},
			"ip_version": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "`4` or `6`. Defaults to `4`.",
				Default:             stringdefault.StaticString("4"),
				Validators: []validator.String{
					stringvalidator.OneOf("4", "6"),
				},
			},
			"insecure": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "HTTP/HTTPS handling (API-specific enum). Leave unset unless required; invalid values return 422.",
			},
			"tls_termination": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "TLS termination mode when using HTTPS (API-specific enum).",
			},
			"certificate_id": schema.Int64Attribute{
				Optional:            true,
				MarkdownDescription: "Optional uploaded certificate ID (`mistedo_lb_certificate`).",
			},
			"services": schema.ListNestedAttribute{
				Required: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"service_id": schema.Int64Attribute{
							Required:            true,
							MarkdownDescription: "Backend service ID from `mistedo_lb_backend_services`.",
						},
						"weight": schema.Float64Attribute{
							Optional:            true,
							Computed:            true,
							MarkdownDescription: "Weight for `weighted` balancing (API field `value`).",
							PlanModifiers:       []planmodifier.Float64{float64planmodifier.UseStateForUnknown()},
						},
						"balance_type": schema.StringAttribute{
							Optional:            true,
							Computed:            true,
							MarkdownDescription: "Balancing mode for this backend (e.g. `weighted`, `loadbalancer`).",
							PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
						},
					},
				},
				MarkdownDescription: "Backend services and optional per-target weights (see API `routes_services`). At least one entry.",
				Validators: []validator.List{
					listvalidator.SizeAtLeast(1),
				},
			},
			"healthcheck": schema.SingleNestedAttribute{
				Optional: true,
				MarkdownDescription: "If set, enables health checks (`healthcheck_enabled` in the API). " +
					"Nested attributes are optional in requests; the API may fill defaults — unset fields are reflected in state after apply.",
				Attributes: hcAttrs,
			},
			"source_proto": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Source protocol (e.g. `tcp`) when applicable.",
			},
			"destination_proto": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Destination protocol when applicable.",
			},
			"labels": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Optional labels string from the API.",
			},
			"owner": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Owner from the API, if set.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"gateway_name": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Name of the resolved Cloud Gateway from the API.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
		},
	}
}

func (r *lbRouteResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *lbRouteResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider did not supply an API client.")
		return
	}
	var plan lbRouteModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	w, diags := routeWriteFromModel(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	out, err := r.client.CreateRoute(ctx, w)
	if err != nil {
		resp.Diagnostics.Append(diagAPI("Could not create load balancer route", err)...)
		return
	}
	fresh, err := r.client.GetRoute(ctx, out.ID)
	if err != nil {
		resp.Diagnostics.Append(diagAPI("Could not read load balancer route after create", err)...)
		return
	}
	state := routeModelFromAPI(fresh)
	mergeRouteServicesFillNulls(ctx, &state, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *lbRouteResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider did not supply an API client.")
		return
	}
	var state lbRouteModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	idStr := state.ID.ValueString()
	if idStr == "" {
		resp.Diagnostics.AddError("Missing id", "Cannot read route without id.")
		return
	}
	id, err := strconv.Atoi(idStr)
	if err != nil {
		resp.Diagnostics.AddError("Invalid id", idStr)
		return
	}
	rt, err := r.client.GetRoute(ctx, id)
	if err != nil {
		if errors.Is(err, client.ErrNotFound) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.Append(diagAPI("Could not read load balancer route", err)...)
		return
	}
	next := routeModelFromAPI(rt)
	mergeRouteServicesFillNulls(ctx, &next, &state)
	mergeHealthcheckFillNulls(ctx, &next, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &next)...)
}

func (r *lbRouteResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider did not supply an API client.")
		return
	}
	var plan lbRouteModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id, err := strconv.Atoi(plan.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid id", plan.ID.ValueString())
		return
	}
	w, diags := routeWriteFromModel(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	_, err = r.client.UpdateRoute(ctx, id, w)
	if err != nil {
		resp.Diagnostics.Append(diagAPI("Could not update load balancer route", err)...)
		return
	}
	fresh, err := r.client.GetRoute(ctx, id)
	if err != nil {
		resp.Diagnostics.Append(diagAPI("Could not read load balancer route after update", err)...)
		return
	}
	state := routeModelFromAPI(fresh)
	mergeRouteServicesFillNulls(ctx, &state, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *lbRouteResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	if r.client == nil {
		return
	}
	var state lbRouteModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id, err := strconv.Atoi(state.ID.ValueString())
	if err != nil {
		return
	}
	err = r.client.DeleteRoute(ctx, id)
	if err != nil && !errors.Is(err, client.ErrNotFound) {
		resp.Diagnostics.Append(diagAPI("Could not delete load balancer route", err)...)
	}
}

func (r *lbRouteResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func routeWriteFromModel(ctx context.Context, m *lbRouteModel) (*client.RouteWrite, diag.Diagnostics) {
	var diags diag.Diagnostics
	var rows []lbRouteServiceModel
	diags.Append(m.Services.ElementsAs(ctx, &rows, false)...)
	if diags.HasError() {
		return nil, diags
	}
	refs := make([]client.ServiceRef, 0, len(rows))
	for _, row := range rows {
		ref := client.ServiceRef{ID: int(row.ServiceID.ValueInt64())}
		if !row.BalanceType.IsNull() && !row.BalanceType.IsUnknown() {
			ref.BalanceType = row.BalanceType.ValueString()
		}
		if !row.Weight.IsNull() && !row.Weight.IsUnknown() {
			w := row.Weight.ValueFloat64()
			ref.Value = &w
		}
		refs = append(refs, ref)
	}

	hcEnabled := !m.Healthcheck.IsNull() && !m.Healthcheck.IsUnknown()

	w := &client.RouteWrite{
		Name:               m.Name.ValueString(),
		Hostname:           m.Hostname.ValueString(),
		Path:               m.Path.ValueString(),
		TargetPort:         int(m.TargetPort.ValueInt64()),
		CloudGatewayID:     int(m.CloudGatewayID.ValueInt64()),
		IPVersion:          m.IPVersion.ValueString(),
		Services:           refs,
		HealthcheckEnabled: hcEnabled,
	}
	if !m.Insecure.IsNull() && !m.Insecure.IsUnknown() {
		s := m.Insecure.ValueString()
		w.Insecure = &s
	}
	if !m.TLSTermination.IsNull() && !m.TLSTermination.IsUnknown() {
		s := m.TLSTermination.ValueString()
		w.TLSTermination = &s
	}
	if !m.CertificateID.IsNull() && !m.CertificateID.IsUnknown() {
		v := int(m.CertificateID.ValueInt64())
		w.CertificateID = &v
	}
	if !m.SourceProto.IsNull() && !m.SourceProto.IsUnknown() {
		s := m.SourceProto.ValueString()
		w.SourceProto = &s
	}
	if !m.DestinationProto.IsNull() && !m.DestinationProto.IsUnknown() {
		s := m.DestinationProto.ValueString()
		w.DestinationProto = &s
	}
	if !m.Labels.IsNull() && !m.Labels.IsUnknown() {
		s := m.Labels.ValueString()
		w.Labels = &s
	}

	if hcEnabled {
		var hcm hcModel
		diags.Append(m.Healthcheck.As(ctx, &hcm, basetypes.ObjectAsOptions{})...)
		if diags.HasError() {
			return nil, diags
		}
		hc := &client.HealthcheckInput{}
		if !hcm.Path.IsNull() && !hcm.Path.IsUnknown() {
			s := hcm.Path.ValueString()
			hc.Path = &s
		}
		if !hcm.Scheme.IsNull() && !hcm.Scheme.IsUnknown() {
			s := hcm.Scheme.ValueString()
			hc.Scheme = &s
		}
		if !hcm.Hostname.IsNull() && !hcm.Hostname.IsUnknown() {
			s := hcm.Hostname.ValueString()
			hc.Hostname = &s
		}
		if !hcm.Port.IsNull() && !hcm.Port.IsUnknown() {
			p := int(hcm.Port.ValueInt64())
			hc.Port = &p
		}
		if !hcm.Interval.IsNull() && !hcm.Interval.IsUnknown() {
			i := int(hcm.Interval.ValueInt64())
			hc.Interval = &i
		}
		if !hcm.Timeout.IsNull() && !hcm.Timeout.IsUnknown() {
			t := int(hcm.Timeout.ValueInt64())
			hc.Timeout = &t
		}
		if !hcm.Method.IsNull() && !hcm.Method.IsUnknown() {
			s := hcm.Method.ValueString()
			hc.Method = &s
		}
		if !hcm.FollowRedirects.IsNull() && !hcm.FollowRedirects.IsUnknown() {
			b := hcm.FollowRedirects.ValueBool()
			hc.FollowRedirects = &b
		}
		if !hcm.Headers.IsNull() && !hcm.Headers.IsUnknown() {
			var hdr map[string]string
			diags.Append(hcm.Headers.ElementsAs(ctx, &hdr, false)...)
			if len(hdr) > 0 {
				hc.Headers = hdr
			}
		}
		w.Healthcheck = hc
	}

	return w, diags
}

// mergeRouteServicesFillNulls copies weight/balance_type from src (plan or prior state)
// into base when GET /routes/{id} left those attributes null (API omits routes_services.value).
func mergeRouteServicesFillNulls(ctx context.Context, base *lbRouteModel, src *lbRouteModel) {
	if src.Services.IsNull() || src.Services.IsUnknown() || base.Services.IsNull() {
		return
	}
	var srcRows []lbRouteServiceModel
	diags := src.Services.ElementsAs(ctx, &srcRows, false)
	if diags.HasError() {
		return
	}
	var baseRows []lbRouteServiceModel
	diags = base.Services.ElementsAs(ctx, &baseRows, false)
	if diags.HasError() {
		return
	}
	for i := range baseRows {
		if i >= len(srcRows) {
			break
		}
		if srcRows[i].ServiceID != baseRows[i].ServiceID {
			continue
		}
		if !srcRows[i].Weight.IsNull() && !srcRows[i].Weight.IsUnknown() && baseRows[i].Weight.IsNull() {
			baseRows[i].Weight = srcRows[i].Weight
		}
		if !srcRows[i].BalanceType.IsNull() && !srcRows[i].BalanceType.IsUnknown() && baseRows[i].BalanceType.IsNull() {
			baseRows[i].BalanceType = srcRows[i].BalanceType
		}
	}
	objT := types.ObjectType{AttrTypes: serviceEntryAttrTypes()}
	elems := make([]attr.Value, len(baseRows))
	for i := range baseRows {
		elems[i] = types.ObjectValueMust(serviceEntryAttrTypes(), map[string]attr.Value{
			"service_id":   baseRows[i].ServiceID,
			"weight":       baseRows[i].Weight,
			"balance_type": baseRows[i].BalanceType,
		})
	}
	base.Services = types.ListValueMust(objT, elems)
}

func mergeHealthcheckFillNulls(ctx context.Context, base *lbRouteModel, prior *lbRouteModel) {
	if base.Healthcheck.IsNull() || prior.Healthcheck.IsNull() {
		return
	}
	var b, pr hcModel
	if diags := base.Healthcheck.As(ctx, &b, basetypes.ObjectAsOptions{}); diags.HasError() {
		return
	}
	if diags := prior.Healthcheck.As(ctx, &pr, basetypes.ObjectAsOptions{}); diags.HasError() {
		return
	}
	if b.Scheme.IsNull() && !pr.Scheme.IsNull() {
		b.Scheme = pr.Scheme
	}
	if b.Hostname.IsNull() && !pr.Hostname.IsNull() {
		b.Hostname = pr.Hostname
	}
	if b.Port.IsNull() && !pr.Port.IsNull() {
		b.Port = pr.Port
	}
	if b.Interval.IsNull() && !pr.Interval.IsNull() {
		b.Interval = pr.Interval
	}
	if b.Timeout.IsNull() && !pr.Timeout.IsNull() {
		b.Timeout = pr.Timeout
	}
	if b.Method.IsNull() && !pr.Method.IsNull() {
		b.Method = pr.Method
	}
	if b.FollowRedirects.IsNull() && !pr.FollowRedirects.IsNull() {
		b.FollowRedirects = pr.FollowRedirects
	}
	if b.Headers.IsNull() && !pr.Headers.IsNull() {
		b.Headers = pr.Headers
	}
	base.Healthcheck = hcModelToObject(b)
}

func hcModelToObject(h hcModel) types.Object {
	return types.ObjectValueMust(healthcheckAttrTypes(), map[string]attr.Value{
		"path":             h.Path,
		"scheme":           h.Scheme,
		"hostname":         h.Hostname,
		"port":             h.Port,
		"interval":         h.Interval,
		"timeout":          h.Timeout,
		"method":           h.Method,
		"follow_redirects": h.FollowRedirects,
		"headers":          h.Headers,
	})
}

func routeModelFromAPI(rt *client.Route) lbRouteModel {
	m := lbRouteModel{
		ID:             types.StringValue(strconv.Itoa(rt.ID)),
		Name:           types.StringValue(rt.Name),
		Hostname:       types.StringValue(rt.Hostname),
		Path:           types.StringValue(rt.Path),
		TargetPort:     types.Int64Value(int64(rt.TargetPort)),
		CloudGatewayID: types.Int64Value(int64(rt.CloudGatewayID)),
		IPVersion:      types.StringValue(rt.IPVersion),
		Services:       servicesListFromAPI(rt),
	}
	if rt.Insecure != "" {
		m.Insecure = types.StringValue(rt.Insecure)
	} else {
		m.Insecure = types.StringNull()
	}
	if rt.TLSTermination != "" {
		m.TLSTermination = types.StringValue(rt.TLSTermination)
	} else {
		m.TLSTermination = types.StringNull()
	}
	if rt.CertificateID != nil {
		m.CertificateID = types.Int64Value(int64(*rt.CertificateID))
	} else {
		m.CertificateID = types.Int64Null()
	}

	if rt.Owner != "" {
		m.Owner = types.StringValue(rt.Owner)
	} else {
		m.Owner = types.StringNull()
	}
	if rt.CloudGateway != nil && rt.CloudGateway.Name != "" {
		m.GatewayName = types.StringValue(rt.CloudGateway.Name)
	} else {
		m.GatewayName = types.StringNull()
	}

	if rt.Healthcheck != nil {
		m.Healthcheck = healthcheckObjectFromState(rt.Healthcheck)
	} else {
		m.Healthcheck = types.ObjectNull(healthcheckAttrTypes())
	}

	if rt.SourceProto != "" {
		m.SourceProto = types.StringValue(rt.SourceProto)
	} else {
		m.SourceProto = types.StringNull()
	}
	if rt.DestinationProto != "" {
		m.DestinationProto = types.StringValue(rt.DestinationProto)
	} else {
		m.DestinationProto = types.StringNull()
	}
	if rt.Labels != nil && *rt.Labels != "" {
		m.Labels = types.StringValue(*rt.Labels)
	} else {
		m.Labels = types.StringNull()
	}

	return m
}

func serviceEntryAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"service_id":   types.Int64Type,
		"weight":       types.Float64Type,
		"balance_type": types.StringType,
	}
}

func servicesListFromAPI(rt *client.Route) types.List {
	objT := types.ObjectType{AttrTypes: serviceEntryAttrTypes()}
	if len(rt.RoutesServices) > 0 {
		vals := make([]attr.Value, 0, len(rt.RoutesServices))
		for _, rs := range rt.RoutesServices {
			obj := map[string]attr.Value{
				"service_id":   types.Int64Value(int64(rs.ServiceID)),
				"weight":       types.Float64Null(),
				"balance_type": types.StringNull(),
			}
			if rs.Value != nil {
				obj["weight"] = types.Float64Value(*rs.Value)
			}
			if rs.BalanceType != "" {
				obj["balance_type"] = types.StringValue(rs.BalanceType)
			}
			vals = append(vals, types.ObjectValueMust(serviceEntryAttrTypes(), obj))
		}
		return types.ListValueMust(objT, vals)
	}
	vals := make([]attr.Value, 0, len(rt.Services))
	for _, s := range rt.Services {
		obj := map[string]attr.Value{
			"service_id":   types.Int64Value(int64(s.ID)),
			"weight":       types.Float64Null(),
			"balance_type": types.StringNull(),
		}
		vals = append(vals, types.ObjectValueMust(serviceEntryAttrTypes(), obj))
	}
	if len(vals) == 0 {
		return types.ListNull(objT)
	}
	return types.ListValueMust(objT, vals)
}

func stringOrNullAttr(s string) attr.Value {
	if s == "" {
		return types.StringNull()
	}
	return types.StringValue(s)
}

func int64OrNullAttr(v int) attr.Value {
	if v == 0 {
		return types.Int64Null()
	}
	return types.Int64Value(int64(v))
}

func healthcheckAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"path":             types.StringType,
		"scheme":           types.StringType,
		"hostname":         types.StringType,
		"port":             types.Int64Type,
		"interval":         types.Int64Type,
		"timeout":          types.Int64Type,
		"method":           types.StringType,
		"follow_redirects": types.BoolType,
		"headers":          types.MapType{ElemType: types.StringType},
	}
}

func healthcheckObjectFromState(h *client.HealthcheckState) types.Object {
	var portAttr attr.Value
	if h.Port != nil {
		portAttr = types.Int64Value(int64(*h.Port))
	} else {
		portAttr = types.Int64Null()
	}
	vals := map[string]attr.Value{
		"path":             stringOrNullAttr(h.Path),
		"scheme":           stringOrNullAttr(h.Scheme),
		"hostname":         stringOrNullAttr(h.Hostname),
		"port":             portAttr,
		"interval":         int64OrNullAttr(h.Interval),
		"timeout":          int64OrNullAttr(h.Timeout),
		"method":           stringOrNullAttr(h.Method),
		"follow_redirects": types.BoolValue(h.FollowRedirects),
		"headers":          headersMapAttr(h.Headers),
	}
	return types.ObjectValueMust(healthcheckAttrTypes(), vals)
}

func headersMapAttr(m map[string]string) attr.Value {
	if len(m) == 0 {
		return types.MapNull(types.StringType)
	}
	elems := make(map[string]attr.Value, len(m))
	for k, v := range m {
		elems[k] = types.StringValue(v)
	}
	return types.MapValueMust(types.StringType, elems)
}
