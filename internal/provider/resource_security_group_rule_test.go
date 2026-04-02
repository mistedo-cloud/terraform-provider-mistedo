package provider

import (
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/mistedo/terraform-provider-mistedo/internal/client"
)

func TestNormalizeOptionalAgainstPriorOmittedVsEmptyString(t *testing.T) {
	t.Parallel()
	out := types.StringValue("")
	prior := types.StringNull()
	normalizeOptionalAgainstPrior(&out, prior)
	if !out.IsNull() {
		t.Fatalf("want null when prior omitted and API empty, got %#v", out)
	}
	out = types.StringValue("")
	prior = types.StringValue("x")
	normalizeOptionalAgainstPrior(&out, prior)
	if out.ValueString() != "x" {
		t.Fatal(out)
	}
}

func TestRuleResourceIDIgnoresEMSCase(t *testing.T) {
	t.Parallel()
	a := ruleResourceID("81000000000115", "E04E9652-A61C-4A10-90C4-B4FB276EDA02")
	b := ruleResourceID("81000000000115", "e04e9652-a61c-4a10-90c4-b4fb276eda02")
	if a != b {
		t.Fatalf("%q vs %q", a, b)
	}
	if !strings.Contains(a, "e04e9652") {
		t.Fatalf("expected lowercase uuid in id: %q", a)
	}
}

func TestRuleModelFromAPIPreservesOptionalWhenAPIOmitsFields(t *testing.T) {
	t.Parallel()
	prior := securityGroupRuleModel{
		PortRange:       types.StringValue("22"),
		Protocol:        types.StringValue("tcp"),
		NetworkProtocol: types.StringValue("IPV4"),
		RemoteIPSubnet:  types.StringValue("0.0.0.0/0"),
		Direction:       types.StringValue("ingress"),
	}
	rule := client.FirewallRule{
		EmsRef:    "e1",
		Direction: "inbound",
	}
	out := ruleModelFromAPI("81000000000001", rule, prior)
	if out.PortRange.ValueString() != "22" {
		t.Fatalf("port_range: got %q", out.PortRange.ValueString())
	}
	if out.Protocol.ValueString() != "tcp" {
		t.Fatalf("protocol: got %q", out.Protocol.ValueString())
	}
	if out.NetworkProtocol.ValueString() != "IPV4" {
		t.Fatalf("network_protocol: got %q", out.NetworkProtocol.ValueString())
	}
	if out.RemoteIPSubnet.ValueString() != "0.0.0.0/0" {
		t.Fatalf("remote_ip_subnet: got %q", out.RemoteIPSubnet.ValueString())
	}
	if out.Direction.ValueString() != "ingress" {
		t.Fatalf("direction: got %q", out.Direction.ValueString())
	}
}
