package client

import (
	"encoding/json"
	"testing"
)

func TestParseMIQTaskSubmit(t *testing.T) {
	t.Parallel()
	id, err := parseMIQTaskSubmit([]byte(`{"results":[{"task_id":"81000000000099","success":true}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if id != "81000000000099" {
		t.Fatalf("task id: got %q", id)
	}
}

func TestParseMIQTaskSubmitFailureMessage(t *testing.T) {
	t.Parallel()
	_, err := parseMIQTaskSubmit([]byte(`{"results":[{"success":false,"message":"denied"}]}`))
	if err == nil || err.Error() != "denied" {
		t.Fatalf("expected denied error, got %v", err)
	}
}

func TestSplitPortRangeSinglePort(t *testing.T) {
	t.Parallel()
	min, max := splitPortRange("22")
	if min != "22" || max != "22" {
		t.Fatalf("got min=%q max=%q", min, max)
	}
	min, max = splitPortRange(" 8080 ")
	if min != "8080" || max != "8080" {
		t.Fatalf("trim: got min=%q max=%q", min, max)
	}
	min, max = splitPortRange("10-20")
	if min != "10" || max != "20" {
		t.Fatalf("range: got min=%q max=%q", min, max)
	}
}

func TestFirewallRuleGETShape(t *testing.T) {
	t.Parallel()
	p := 22
	r := FirewallRule{
		Direction:       "inbound",
		Port:            &p,
		EndPort:         &p,
		HostProtocol:    "TCP",
		NetworkProtocol: "ipv4",
	}
	if g, w := r.DirectionNormalized(), "ingress"; g != w {
		t.Fatalf("DirectionNormalized: got %q want %q", g, w)
	}
	if g, w := r.PortRangeMinEffective(), "22"; g != w {
		t.Fatalf("PortRangeMinEffective: got %q want %q", g, w)
	}
	if g, w := r.ProtocolEffective(), "tcp"; g != w {
		t.Fatalf("ProtocolEffective: got %q want %q", g, w)
	}
}

func TestSecurityGroupRulesFallback(t *testing.T) {
	t.Parallel()
	g := SecurityGroup{SecurityGroupRules: []FirewallRule{{EmsRef: "a"}}}
	if len(g.Rules()) != 1 || g.Rules()[0].EmsRef != "a" {
		t.Fatalf("Rules(): %+v", g.Rules())
	}
	g2 := SecurityGroup{FirewallRules: []FirewallRule{{EmsRef: "b"}}}
	if len(g2.Rules()) != 1 || g2.Rules()[0].EmsRef != "b" {
		t.Fatalf("Rules() should prefer firewall_rules")
	}
}

func TestPickNewRuleAfterAddSingle(t *testing.T) {
	t.Parallel()
	before := map[string]bool{"1": true}
	rules := []FirewallRule{
		{ID: "1", Direction: "inbound", Port: ptrInt(22), HostProtocol: "tcp"},
		{ID: "2", Direction: "inbound", Port: ptrInt(443), HostProtocol: "tcp"},
	}
	in := FirewallRuleInput{Direction: "ingress", PortRange: "443", Protocol: "tcp"}
	got, err := pickNewRuleAfterAdd(before, rules, in)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "2" {
		t.Fatalf("got id %q", got.ID)
	}
}

func TestPickNewRuleAfterAddOneCandidate(t *testing.T) {
	t.Parallel()
	before := map[string]bool{"1": true}
	rules := []FirewallRule{
		{ID: "1", EmsRef: "a"},
		{ID: "2", EmsRef: "b", Direction: "ingress", PortRangeMin: "22", PortRangeMax: "22", Protocol: "tcp"},
	}
	in := FirewallRuleInput{Direction: "ingress", PortRange: "22", Protocol: "tcp"}
	got, err := pickNewRuleAfterAdd(before, rules, in)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "2" {
		t.Fatalf("got id %q", got.ID)
	}
}

func TestPickNewRuleAfterAddAmbiguous(t *testing.T) {
	t.Parallel()
	before := map[string]bool{"1": true}
	rules := []FirewallRule{
		{ID: "1", EmsRef: "a"},
		{ID: "2", EmsRef: "b", Direction: "ingress", PortRangeMin: "22", PortRangeMax: "22", Protocol: "tcp"},
		{ID: "3", EmsRef: "c", Direction: "ingress", PortRangeMin: "22", PortRangeMax: "22", Protocol: "tcp"},
	}
	in := FirewallRuleInput{Direction: "ingress", PortRange: "22", Protocol: "tcp"}
	_, err := pickNewRuleAfterAdd(before, rules, in)
	if err == nil {
		t.Fatal("expected ambiguous error")
	}
}

func ptrInt(n int) *int {
	return &n
}

func TestMiqRuleResponseWithEmbeddedRule(t *testing.T) {
	t.Parallel()
	const sample = `{"success":"true","message":"Rule was added successfully","rule":{"id":"81000000000651","host_protocol":"TCP","direction":"inbound","port":59991,"end_port":59991,"source_ip_range":"203.0.113.0/24","ems_ref":"0eaa3486-f176-4a7a-a709-1a633f7502ff","network_protocol":"ipv4"}}`
	var tr miqRuleResponse
	if err := json.Unmarshal([]byte(sample), &tr); err != nil {
		t.Fatal(err)
	}
	if tr.Rule == nil || tr.Rule.EmsRef != "0eaa3486-f176-4a7a-a709-1a633f7502ff" {
		t.Fatalf("rule: %+v", tr.Rule)
	}
	if tr.Rule.ID != "81000000000651" {
		t.Fatalf("id: %q", tr.Rule.ID)
	}
}

