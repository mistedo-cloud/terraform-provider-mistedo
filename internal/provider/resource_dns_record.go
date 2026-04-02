package provider

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/mistedo/terraform-provider-mistedo/internal/client"
)

var _ resource.Resource = (*dnsRecordResource)(nil)
var _ resource.ResourceWithImportState = (*dnsRecordResource)(nil)

type dnsRecordResource struct {
	client *client.Client
}

func newDNSRecordResource() resource.Resource {
	return &dnsRecordResource{}
}

func (r *dnsRecordResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dns_record"
}

func (r *dnsRecordResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A DNS resource record inside a hosted zone (similar to `aws_route53_record` or `google_dns_record_set`). " +
			"Use `content` for the RDATA (what many providers call the record value). " +
			"Changing `zone`, `name`, or `type` replaces the record; other attributes are updated in place when the API allows (delete + recreate under the hood).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				MarkdownDescription: "Composite id: `<zone_name>/<record_path>`. " +
					"`record_path` is the API identifier (`[server-assigned-id.]relative_name`).",
			},
			"zone": schema.StringAttribute{
				Required: true,
				MarkdownDescription: "FQDN of the hosted zone (must match an existing `mistedo_dns_zone`), without a trailing dot. " +
					"Changing this forces replacement.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.LengthBetween(1, 1024),
					zoneFQDNNoTrailingDotValidator{},
				},
			},
			"name": schema.StringAttribute{
				Required: true,
				MarkdownDescription: "Owner name relative to the zone (e.g. `www`, `ns.dns`, or `@` for apex if supported by the API). " +
					"Changing this forces replacement.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.LengthBetween(1, 1024),
				},
			},
			"type": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "DNS record type. Changing this forces replacement.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					dnsRecordTypeValidator{},
				},
			},
			"content": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Record data (RDATA): IPv4/IPv6, hostname, TXT text, etc., depending on `type`.",
			},
			"ttl": schema.Int64Attribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Time to live in seconds. Defaults to 300 if omitted.",
				Default:             int64default.StaticInt64(300),
				Validators: []validator.Int64{
					int64validator.Between(30, 2147483647),
				},
			},
			"priority": schema.Int64Attribute{
				Optional:            true,
				MarkdownDescription: "Priority for `MX` and `SRV` records.",
				Validators: []validator.Int64{
					int64validator.Between(0, 65535),
				},
			},
			"weight": schema.Int64Attribute{
				Optional:            true,
				MarkdownDescription: "SRV weight.",
				Validators: []validator.Int64{
					int64validator.Between(0, 65535),
				},
			},
			"port": schema.Int64Attribute{
				Optional:            true,
				MarkdownDescription: "SRV port.",
				Validators: []validator.Int64{
					int64validator.Between(0, 65535),
				},
			},
			"record_path": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Opaque API path segment for this record (read-only).",
			},
			"group": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Optional grouping key returned by the API.",
			},
		},
	}
}

func (r *dnsRecordResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func recordFromModel(m dnsRecordModel) client.Record {
	rec := client.Record{
		Name: m.Name.ValueString(),
		Type: strings.ToUpper(strings.TrimSpace(m.Type.ValueString())),
		Data: m.Content.ValueString(),
		TTL:  int(m.TTL.ValueInt64()),
	}
	if !m.Priority.IsNull() {
		rec.Priority = int(m.Priority.ValueInt64())
	}
	if !m.Weight.IsNull() {
		rec.Weight = int(m.Weight.ValueInt64())
	}
	if !m.Port.IsNull() {
		rec.Port = int(m.Port.ValueInt64())
	}
	return rec
}

func (r *dnsRecordResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider did not supply an API client.")
		return
	}
	var plan dnsRecordModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	zone := plan.Zone.ValueString()
	payload := recordFromModel(plan)
	out, err := r.client.CreateRecord(ctx, zone, payload)
	if err != nil {
		resp.Diagnostics.Append(diagAPI("Could not create DNS record", err)...)
		return
	}
	recordPath := client.RecordPathInZone(zone, out)
	if recordPath == "" {
		found, err := r.client.FindMatchingRecord(ctx, zone, payload.Type, payload.Name, payload.Data)
		if err != nil {
			resp.Diagnostics.AddError(
				"DNS record may have been created but could not be resolved",
				"The API did not return a full record body. Listing records to locate the new record failed: "+err.Error(),
			)
			return
		}
		recordPath = client.RecordPathInZone(zone, found)
		out = found
	}
	state := recordModelFromAPI(zone, recordPath, out, payload.Type)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func recordModelFromAPI(zone, recordPath string, rec *client.Record, canonicalType string) dnsRecordModel {
	outType := strings.ToUpper(strings.TrimSpace(rec.Type))
	if canonicalType != "" {
		outType = strings.ToUpper(strings.TrimSpace(canonicalType))
	}
	outName := canonicalRecordNameForState(zone, recordPath, rec)
	contentType := outType
	if canonicalType != "" {
		contentType = strings.ToUpper(strings.TrimSpace(canonicalType))
	}
	outContent := client.CanonicalRecordContent(contentType, client.RecordRData(rec))
	id := composeRecordID(zone, recordPath)
	st := dnsRecordModel{
		ID:         types.StringValue(id),
		Zone:       types.StringValue(zone),
		Name:       types.StringValue(outName),
		Type:       types.StringValue(outType),
		Content:    types.StringValue(outContent),
		TTL:        types.Int64Value(int64(rec.TTL)),
		RecordPath: types.StringValue(recordPath),
		Group:      types.StringNull(),
		Priority:   types.Int64Null(),
		Weight:     types.Int64Null(),
		Port:       types.Int64Null(),
	}
	if rec.Group != "" {
		st.Group = types.StringValue(rec.Group)
	}
	if rec.Priority != 0 {
		st.Priority = types.Int64Value(int64(rec.Priority))
	}
	if rec.Weight != 0 {
		st.Weight = types.Int64Value(int64(rec.Weight))
	}
	if rec.Port != 0 {
		st.Port = types.Int64Value(int64(rec.Port))
	}
	return st
}

func canonicalRecordNameForState(zone, recordPath string, rec *client.Record) string {
	zone = client.NormalizeZoneName(zone)
	name := client.NormalizeZoneName(strings.TrimSpace(rec.Name))
	if name == "" {
		name = recordPathName(recordPath)
	}
	// Some API responses return full FQDN; keep Terraform field as relative owner name.
	sfx := "." + strings.ToLower(zone)
	lowerName := strings.ToLower(name)
	if zone != "" && strings.HasSuffix(lowerName, sfx) {
		name = name[:len(name)-len(sfx)]
	}
	// If API returns path-like name ("x1.www"), remove id prefix.
	if strings.Contains(name, ".") {
		if idPrefix, _, ok := splitRecordPath(recordPath); ok && strings.HasPrefix(strings.ToLower(name), strings.ToLower(idPrefix+".")) {
			name = name[len(idPrefix)+1:]
		}
	}
	return name
}

func recordPathName(recordPath string) string {
	if _, rest, ok := splitRecordPath(recordPath); ok {
		return rest
	}
	return recordPath
}

func splitRecordPath(recordPath string) (idPrefix, rest string, ok bool) {
	i := strings.Index(recordPath, ".")
	if i <= 0 || i >= len(recordPath)-1 {
		return "", "", false
	}
	id := recordPath[:i]
	if len(id) >= 2 && (id[0] == 'x' || id[0] == 'X') {
		allDigits := true
		for _, ch := range id[1:] {
			if ch < '0' || ch > '9' {
				allDigits = false
				break
			}
		}
		if allDigits {
			return id, recordPath[i+1:], true
		}
	}
	return "", "", false
}

func (r *dnsRecordResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider did not supply an API client.")
		return
	}
	var state dnsRecordModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	zone := state.Zone.ValueString()
	recordPath := state.RecordPath.ValueString()
	if recordPath == "" {
		_, rp, ok := parseRecordID(state.ID.ValueString())
		if !ok {
			resp.Diagnostics.AddError("Invalid resource state", "Set `record_path` or a valid composite `id` (`zone/record_path`).")
			return
		}
		recordPath = rp
	}
	rec, err := r.client.GetRecord(ctx, zone, recordPath)
	if err != nil {
		var apiErr *client.APIError
		if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
			resp.State.RemoveResource(ctx)
			return
		}
		if errors.Is(err, client.ErrNotFound) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.Append(diagAPI("Could not read DNS record", err)...)
		return
	}
	rp := client.RecordPathInZone(zone, rec)
	newState := recordModelFromAPI(zone, rp, rec, state.Type.ValueString())
	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *dnsRecordResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider did not supply an API client.")
		return
	}
	var plan, prior dnsRecordModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
	if resp.Diagnostics.HasError() {
		return
	}
	zone := prior.Zone.ValueString()
	oldPath := prior.RecordPath.ValueString()
	if oldPath == "" {
		_, rp, ok := parseRecordID(prior.ID.ValueString())
		if !ok {
			resp.Diagnostics.AddError("Cannot update DNS record", "Previous state is missing `record_path`.")
			return
		}
		oldPath = rp
	}
	if err := r.client.DeleteRecord(ctx, zone, oldPath); err != nil {
		var apiErr *client.APIError
		if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
			// Record already gone; continue to recreate from plan.
		} else {
			resp.Diagnostics.Append(diagAPI("Could not delete DNS record before update", err)...)
			return
		}
	}
	payload := recordFromModel(plan)
	out, err := r.client.CreateRecord(ctx, zone, payload)
	if err != nil {
		resp.Diagnostics.Append(diagAPI("DNS record update failed after delete", err)...)
		return
	}
	recordPath := client.RecordPathInZone(zone, out)
	if recordPath == "" {
		found, err := r.client.FindMatchingRecord(ctx, zone, payload.Type, payload.Name, payload.Data)
		if err != nil {
			resp.Diagnostics.Append(diagAPI("DNS record recreated but could not resolve new record path", err)...)
			return
		}
		recordPath = client.RecordPathInZone(zone, found)
		out = found
	}
	newState := recordModelFromAPI(zone, recordPath, out, payload.Type)
	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *dnsRecordResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider did not supply an API client.")
		return
	}
	var state dnsRecordModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	zone := state.Zone.ValueString()
	recordPath := state.RecordPath.ValueString()
	if recordPath == "" {
		_, rp, ok := parseRecordID(state.ID.ValueString())
		if !ok {
			resp.Diagnostics.AddError("Cannot delete DNS record", "State is missing `record_path`.")
			return
		}
		recordPath = rp
	}
	if err := r.client.DeleteRecord(ctx, zone, recordPath); err != nil {
		resp.Diagnostics.Append(diagAPI("Could not delete DNS record", err)...)
		return
	}
}

func (r *dnsRecordResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	zone, recordPath, ok := parseRecordID(req.ID)
	if !ok {
		resp.Diagnostics.AddError(
			"Invalid import id",
			"Expected `<zone_fqdn>/<record_path>` where `record_path` matches the API (for example `x1.ns.dns` or `www`). "+
				"You can copy `record_path` from the API or from an existing Terraform state.",
		)
		return
	}
	zone = client.NormalizeZoneName(zone)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("zone"), types.StringValue(zone))...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("record_path"), types.StringValue(recordPath))...)
}

type dnsRecordModel struct {
	ID         types.String `tfsdk:"id"`
	Zone       types.String `tfsdk:"zone"`
	Name       types.String `tfsdk:"name"`
	Type       types.String `tfsdk:"type"`
	Content    types.String `tfsdk:"content"`
	TTL        types.Int64  `tfsdk:"ttl"`
	Priority   types.Int64  `tfsdk:"priority"`
	Weight     types.Int64  `tfsdk:"weight"`
	Port       types.Int64  `tfsdk:"port"`
	RecordPath types.String `tfsdk:"record_path"`
	Group      types.String `tfsdk:"group"`
}

func composeRecordID(zone, recordPath string) string {
	return zone + "/" + recordPath
}

func parseRecordID(id string) (zone, recordPath string, ok bool) {
	i := strings.Index(id, "/")
	if i <= 0 || i >= len(id)-1 {
		return "", "", false
	}
	return id[:i], id[i+1:], true
}
