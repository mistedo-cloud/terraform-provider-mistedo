package provider

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

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

var _ resource.Resource = (*securityGroupRuleResource)(nil)
var _ resource.ResourceWithImportState = (*securityGroupRuleResource)(nil)

type securityGroupRuleResource struct {
	client *client.Client
}

func newSecurityGroupRuleResource() resource.Resource {
	return &securityGroupRuleResource{}
}

func (r *securityGroupRuleResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_security_group_rule"
}

type securityGroupRuleModel struct {
	ID              types.String `tfsdk:"id"`
	SecurityGroupID types.String `tfsdk:"security_group_id"`
	EmsRef          types.String `tfsdk:"ems_ref"`
	Direction       types.String `tfsdk:"direction"`
	PortRange       types.String `tfsdk:"port_range"`
	Protocol        types.String `tfsdk:"protocol"`
	NetworkProtocol types.String `tfsdk:"network_protocol"`
	RemoteGroupID   types.String `tfsdk:"remote_group_id"`
	RemoteIPSubnet  types.String `tfsdk:"remote_ip_subnet"`
}

func replaceOnChangeString() []planmodifier.String {
	return []planmodifier.String{stringplanmodifier.RequiresReplace()}
}

func (r *securityGroupRuleResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A single ingress/egress firewall rule on a `mistedo_security_group`. " +
			"The resource id is `<security_group_id>/<rule_ems_ref>` (used for import). " +
			"Changing any rule attribute forces replacement (remove + add).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Terraform id: `<security_group_id>/<ems_ref>`.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"security_group_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Id of the parent security group (`mistedo_security_group.id`).",
				PlanModifiers:       replaceOnChangeString(),
			},
			"ems_ref": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "ManageIQ reference for this rule; required by the API to delete the rule.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"direction": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Traffic direction, e.g. `ingress` or `egress` (values as accepted by the compute API).",
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
				PlanModifiers: replaceOnChangeString(),
			},
			"port_range": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Single port or `min-max` range as a string.",
				PlanModifiers:       replaceOnChangeString(),
			},
			"protocol": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Protocol field sent to the API (e.g. `tcp`, `udp`, `icmp`).",
				PlanModifiers:       replaceOnChangeString(),
			},
			"network_protocol": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Optional `network_protocol` in the API payload when required by the platform.",
				PlanModifiers:       replaceOnChangeString(),
			},
			"remote_group_id": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Remote security group reference when the rule targets another group.",
				PlanModifiers:       replaceOnChangeString(),
			},
			"remote_ip_subnet": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Source CIDR / IP range (`source_ip_range` in the API).",
				PlanModifiers:       replaceOnChangeString(),
			},
		},
	}
}

func (r *securityGroupRuleResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *securityGroupRuleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider did not supply an API client.")
		return
	}
	var plan securityGroupRuleModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	in := client.FirewallRuleInput{
		Direction:       plan.Direction.ValueString(),
		PortRange:       plan.PortRange.ValueString(),
		Protocol:        plan.Protocol.ValueString(),
		NetworkProtocol: plan.NetworkProtocol.ValueString(),
		RemoteGroupID:   plan.RemoteGroupID.ValueString(),
		RemoteIPSubnet:  plan.RemoteIPSubnet.ValueString(),
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()
	rule, err := r.client.AddFirewallRule(ctx, plan.SecurityGroupID.ValueString(), in)
	if err != nil {
		resp.Diagnostics.Append(diagAPI("Could not add security group rule", err)...)
		return
	}
	if rule.EmsRef == "" {
		resp.Diagnostics.AddError("Invalid API response", "Created rule has empty ems_ref.")
		return
	}
	emsNorm := normalizeRuleEMSRef(rule.EmsRef)
	tfID := ruleResourceID(plan.SecurityGroupID.ValueString(), emsNorm)
	state := securityGroupRuleModel{
		ID:              types.StringValue(tfID),
		SecurityGroupID: plan.SecurityGroupID,
		EmsRef:          types.StringValue(emsNorm),
		Direction:       plan.Direction,
		PortRange:       plan.PortRange,
		Protocol:        plan.Protocol,
		NetworkProtocol: plan.NetworkProtocol,
		RemoteGroupID:   plan.RemoteGroupID,
		RemoteIPSubnet:  plan.RemoteIPSubnet,
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *securityGroupRuleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider did not supply an API client.")
		return
	}
	var state securityGroupRuleModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	groupID, emsRef, err := parseRuleResourceID(state.ID.ValueString(), state.SecurityGroupID.ValueString(), state.EmsRef.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid state", err.Error())
		return
	}
	sg, err := r.client.GetSecurityGroup(ctx, groupID)
	if err != nil {
		if errors.Is(err, client.ErrNotFound) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.Append(diagAPI("Could not read security group for rule", err)...)
		return
	}
	var found *client.FirewallRule
	for i := range sg.Rules() {
		rule := sg.Rules()[i]
		if normalizeRuleEMSRef(rule.EmsRef) == emsRef {
			found = &rule
			break
		}
	}
	if found == nil {
		resp.State.RemoveResource(ctx)
		return
	}
	next := ruleModelFromAPI(groupID, *found, state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &next)...)
}

func (r *securityGroupRuleResource) Update(ctx context.Context, _ resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError(
		"Update not supported",
		"Rule attributes use replace-on-change; Terraform will destroy and recreate the rule when you change them.",
	)
	_ = ctx
}

func (r *securityGroupRuleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider did not supply an API client.")
		return
	}
	var state securityGroupRuleModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	groupID, emsRef, err := parseRuleResourceID(state.ID.ValueString(), state.SecurityGroupID.ValueString(), state.EmsRef.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid state", err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()
	err = r.client.RemoveFirewallRule(ctx, groupID, normalizeRuleEMSRef(emsRef))
	if err != nil {
		if errors.Is(err, client.ErrNotFound) {
			return
		}
		resp.Diagnostics.Append(diagAPI("Could not remove security group rule", err)...)
	}
}

func (r *securityGroupRuleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func ruleResourceID(groupID, emsRef string) string {
	return strings.TrimSpace(groupID) + "/" + normalizeRuleEMSRef(emsRef)
}

// normalizeRuleEMSRef makes ems_ref and composite id stable across refreshes (UUID case varies in some APIs).
func normalizeRuleEMSRef(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

func parseRuleResourceID(id, groupFallback, emsFallback string) (groupID, emsRef string, err error) {
	if id != "" {
		parts := strings.SplitN(id, "/", 2)
		if len(parts) == 2 && parts[0] != "" && parts[1] != "" {
			return strings.TrimSpace(parts[0]), normalizeRuleEMSRef(parts[1]), nil
		}
	}
	if groupFallback != "" && emsFallback != "" {
		return strings.TrimSpace(groupFallback), normalizeRuleEMSRef(emsFallback), nil
	}
	return "", "", fmt.Errorf("need id as <security_group_id>/<ems_ref> or both security_group_id and ems_ref in state")
}

func ruleModelFromAPI(groupID string, rule client.FirewallRule, prior securityGroupRuleModel) securityGroupRuleModel {
	ems := normalizeRuleEMSRef(rule.EmsRef)
	out := securityGroupRuleModel{
		ID:              types.StringValue(ruleResourceID(groupID, ems)),
		SecurityGroupID: types.StringValue(strings.TrimSpace(groupID)),
		EmsRef:          types.StringValue(ems),
		Direction:       types.StringValue(rule.DirectionNormalized()),
		PortRange:       portRangeFromRule(rule),
		Protocol:        types.StringValue(rule.ProtocolEffective()),
		NetworkProtocol: types.StringValue(strings.ToUpper(strings.TrimSpace(rule.NetworkProtocol))),
		RemoteGroupID:   types.StringValue(rule.RemoteGroupID),
		RemoteIPSubnet:  types.StringValue(rule.SourceIPRange),
	}
	// GET rule objects often omit fields that POST accepted. Optional attributes use
	// RequiresReplace — any plan diff forces replacement. Keep prior non-null values
	// when the API gives no equivalent (null/empty/unknown).
	preserveOptionalRuleFields(&out, prior)
	return out
}

func preserveOptionalRuleFields(out *securityGroupRuleModel, prior securityGroupRuleModel) {
	// Optional + RequiresReplace: Terraform stores omitted arguments as null, but JSON decode
	// often becomes "". Null and "" differ → perpetual replacement. Align empty API values
	// with prior: copy prior when set, else use null (not "").
	normalizeOptionalAgainstPrior(&out.PortRange, prior.PortRange)
	normalizeOptionalAgainstPrior(&out.Protocol, prior.Protocol)
	normalizeOptionalAgainstPrior(&out.NetworkProtocol, prior.NetworkProtocol)
	normalizeOptionalAgainstPrior(&out.RemoteGroupID, prior.RemoteGroupID)
	normalizeOptionalAgainstPrior(&out.RemoteIPSubnet, prior.RemoteIPSubnet)
	if stringAttrEmpty(out.Direction) && !prior.Direction.IsNull() && !prior.Direction.IsUnknown() {
		out.Direction = prior.Direction
	}
}

func normalizeOptionalAgainstPrior(out *types.String, prior types.String) {
	if !stringAttrEmpty(*out) {
		return
	}
	if !prior.IsNull() && !prior.IsUnknown() {
		*out = prior
		return
	}
	*out = types.StringNull()
}

func stringAttrEmpty(s types.String) bool {
	return s.IsNull() || s.IsUnknown() || strings.TrimSpace(s.ValueString()) == ""
}

func portRangeFromRule(rule client.FirewallRule) types.String {
	min := strings.TrimSpace(rule.PortRangeMinEffective())
	max := strings.TrimSpace(rule.PortRangeMaxEffective())
	if strings.EqualFold(max, "null") {
		max = ""
	}
	if min == "" && max == "" {
		return types.StringNull()
	}
	if max == "" || min == max {
		return types.StringValue(min)
	}
	return types.StringValue(min + "-" + max)
}
