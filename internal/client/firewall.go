package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ComputeAPIPrefix is the ManageIQ-backed compute API (security groups, tasks, shared helpers).
const ComputeAPIPrefix = "/api/compute/v1"

const (
	manageIQTaskPollInterval = 2 * time.Second
	manageIQTaskDeadline     = 15 * time.Minute
	// minDefaultEgressRulesAfterCreate is how many rules the platform adds by default (IPv4 + IPv6 egress)
	// before user-defined rules; they may appear only after a delay, so we must wait for them then remove.
	minDefaultEgressRulesAfterCreate = 2
)

// SecurityGroup is a compute security group (firewall group) from the API.
type SecurityGroup struct {
	ID             string         `json:"id,omitempty"`
	Name           string         `json:"name"`
	EmsRef         string         `json:"ems_ref"`
	FirewallRules  []FirewallRule `json:"firewall_rules,omitempty"`
	AssignedVMs    []any          `json:"assigned_vms,omitempty"`
	SecurityGroupRules []FirewallRule `json:"security_group_rules,omitempty"` // legacy key
}

// Rules returns firewall rules from either JSON key used by the API.
func (g *SecurityGroup) Rules() []FirewallRule {
	if len(g.FirewallRules) > 0 {
		return g.FirewallRules
	}
	return g.SecurityGroupRules
}

// FirewallRule is one ingress/egress rule (ManageIQ firewall rule).
// GET responses often use port/end_port (numbers), host_protocol, and direction inbound/outbound;
// POST bodies use port_range_min/max, protocol, direction ingress/egress.
type FirewallRule struct {
	Action           string `json:"action,omitempty"`
	ID               string `json:"id,omitempty"`
	EmsRef           string `json:"ems_ref,omitempty"`
	Direction        string `json:"direction,omitempty"`
	PortRangeMin     string `json:"port_range_min,omitempty"`
	PortRangeMax     string `json:"port_range_max,omitempty"`
	Port             *int   `json:"port,omitempty"`
	EndPort          *int   `json:"end_port,omitempty"`
	Protocol         string `json:"protocol,omitempty"`
	HostProtocol     string `json:"host_protocol,omitempty"`
	NetworkProtocol  string `json:"network_protocol,omitempty"`
	RemoteGroupID    string `json:"remote_group_id,omitempty"`
	SecurityGroupID  string `json:"security_group_id,omitempty"`
	SourceIPRange    string `json:"source_ip_range,omitempty"`
}

// DirectionNormalized maps API values (inbound/outbound) to Terraform / POST vocabulary (ingress/egress).
func (r *FirewallRule) DirectionNormalized() string {
	d := strings.ToLower(strings.TrimSpace(r.Direction))
	switch d {
	case "inbound", "ingress":
		return "ingress"
	case "outbound", "egress":
		return "egress"
	default:
		return strings.TrimSpace(r.Direction)
	}
}

// PortRangeMinEffective prefers port_range_min from POST-shaped JSON, else numeric port from GET-shaped JSON.
func (r *FirewallRule) PortRangeMinEffective() string {
	if s := strings.TrimSpace(r.PortRangeMin); s != "" {
		return s
	}
	if r.Port != nil {
		return strconv.Itoa(*r.Port)
	}
	return ""
}

// PortRangeMaxEffective prefers port_range_max, else end_port or port.
func (r *FirewallRule) PortRangeMaxEffective() string {
	if s := strings.TrimSpace(r.PortRangeMax); s != "" {
		return s
	}
	if r.EndPort != nil {
		return strconv.Itoa(*r.EndPort)
	}
	if r.Port != nil {
		return strconv.Itoa(*r.Port)
	}
	return ""
}

// ProtocolEffective prefers protocol, else host_protocol (GET), lowercased.
func (r *FirewallRule) ProtocolEffective() string {
	if s := strings.TrimSpace(r.Protocol); s != "" {
		return strings.ToLower(s)
	}
	if s := strings.TrimSpace(r.HostProtocol); s != "" {
		return strings.ToLower(s)
	}
	return ""
}

type securityGroupCollection struct {
	Resources []SecurityGroup `json:"resources"`
}

type miqTaskSubmitResult struct {
	Results []struct {
		TaskID  string `json:"task_id"`
		Success bool   `json:"success"`
		Message string `json:"message"`
	} `json:"results"`
}

type miqTaskStatus struct {
	State  string `json:"state"`
	Status string `json:"status"`
}

type miqRuleResponse struct {
	Success string        `json:"success"`
	Message string        `json:"message"`
	Rule    *FirewallRule `json:"rule,omitempty"` // present on success for add_firewall_rule (ManageIQ synchronous body)
}

type miqRuleDeleteResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// NetworkManagerProviderID returns the ManageIQ provider id for Red Hat NetworkManager (one per site).
func (c *Client) NetworkManagerProviderID(ctx context.Context) (string, error) {
	c.muManageIQ.Lock()
	defer c.muManageIQ.Unlock()
	if c.manageIQProv != "" {
		return c.manageIQProv, nil
	}
	path := ComputeAPIPrefix + "/providers?expand=resources&filter[]=type=ManageIQ::Providers::Redhat::NetworkManager"
	resp, err := c.DoManageIQ(ctx, http.MethodGet, path, nil)
	if err != nil {
		return "", err
	}
	b, err := readResponseBody(resp)
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", NewAPIError(resp, b)
	}
	var parsed struct {
		Resources []struct {
			ID string `json:"id"`
		} `json:"resources"`
	}
	if err := decodeInto(b, &parsed); err != nil {
		return "", fmt.Errorf("decode providers: %w", err)
	}
	if len(parsed.Resources) == 0 {
		return "", fmt.Errorf("no ManageIQ Red Hat NetworkManager provider in API response")
	}
	c.manageIQProv = parsed.Resources[0].ID
	return c.manageIQProv, nil
}

// ListSecurityGroups returns GET /security_groups?expand=resources.
func (c *Client) ListSecurityGroups(ctx context.Context) ([]SecurityGroup, error) {
	path := ComputeAPIPrefix + "/security_groups?expand=resources"
	resp, err := c.DoManageIQ(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	b, err := readResponseBody(resp)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, NewAPIError(resp, b)
	}
	var coll securityGroupCollection
	if err := decodeInto(b, &coll); err != nil {
		return nil, fmt.Errorf("decode security groups: %w", err)
	}
	return coll.Resources, nil
}

// GetSecurityGroup returns GET /security_groups/{id}?expand=resources&attributes=firewall_rules,assigned_vms.
func (c *Client) GetSecurityGroup(ctx context.Context, id string) (*SecurityGroup, error) {
	path := fmt.Sprintf("%s/security_groups/%s?expand=resources&attributes=firewall_rules,assigned_vms",
		ComputeAPIPrefix, id)
	resp, err := c.DoManageIQ(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	b, err := readResponseBody(resp)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrNotFound
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, NewAPIError(resp, b)
	}
	var g SecurityGroup
	if err := decodeInto(b, &g); err != nil {
		return nil, fmt.Errorf("decode security group: %w", err)
	}
	if g.ID == "" {
		g.ID = id
	}
	return &g, nil
}

type securityGroupWrite struct {
	Action string `json:"action"`
	Name   string `json:"name"`
	ID     string `json:"id,omitempty"`
}

// CreateSecurityGroup creates a group via POST /providers/{providerId}/security_groups, waits for the async task,
// discovers the new id, and removes default firewall rules.
func (c *Client) CreateSecurityGroup(ctx context.Context, name string) (*SecurityGroup, error) {
	prov, err := c.NetworkManagerProviderID(ctx)
	if err != nil {
		return nil, err
	}
	before, err := c.securityGroupIDsSnapshot(ctx)
	if err != nil {
		return nil, err
	}
	body, err := json.Marshal(securityGroupWrite{Action: "create", Name: name})
	if err != nil {
		return nil, err
	}
	path := fmt.Sprintf("%s/providers/%s/security_groups", ComputeAPIPrefix, prov)
	resp, err := c.DoManageIQ(ctx, http.MethodPost, path, body)
	if err != nil {
		return nil, err
	}
	rb, err := readResponseBody(resp)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, NewAPIError(resp, rb)
	}
	taskID, err := parseMIQTaskSubmit(rb)
	if err != nil {
		return nil, err
	}
	if taskID != "" {
		if err := c.waitManageIQTask(ctx, taskID); err != nil {
			return nil, err
		}
	}
	after, err := c.ListSecurityGroups(ctx)
	if err != nil {
		return nil, err
	}
	var created *SecurityGroup
	for i := range after {
		if !before[after[i].ID] {
			created = &after[i]
			break
		}
	}
	if created == nil {
		return nil, fmt.Errorf("could not find new security group %q after create task", name)
	}
	full, err := c.GetSecurityGroup(ctx, created.ID)
	if err != nil {
		return nil, err
	}
	if err := c.stripDefaultFirewallRulesAfterCreate(ctx, full.ID); err != nil {
		return nil, fmt.Errorf("strip default firewall rules: %w", err)
	}
	return c.GetSecurityGroup(ctx, full.ID)
}

func (c *Client) securityGroupIDsSnapshot(ctx context.Context) (map[string]bool, error) {
	list, err := c.ListSecurityGroups(ctx)
	if err != nil {
		return nil, err
	}
	m := make(map[string]bool, len(list))
	for _, g := range list {
		m[g.ID] = true
	}
	return m, nil
}

func parseMIQTaskSubmit(b []byte) (taskID string, err error) {
	var env miqTaskSubmitResult
	if err := json.Unmarshal(b, &env); err != nil {
		return "", err
	}
	if len(env.Results) == 0 {
		return "", fmt.Errorf("empty task results in API response")
	}
	r := env.Results[0]
	if r.TaskID != "" {
		return r.TaskID, nil
	}
	if !r.Success {
		if r.Message != "" {
			return "", errors.New(r.Message)
		}
		return "", fmt.Errorf("task submission failed")
	}
	return "", nil
}

func (c *Client) waitManageIQTask(ctx context.Context, taskID string) error {
	deadline := time.Now().Add(manageIQTaskDeadline)
	for {
		if time.Now().After(deadline) {
			return fmt.Errorf("timeout waiting for ManageIQ task %s", taskID)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		path := fmt.Sprintf("%s/tasks/%s", ComputeAPIPrefix, taskID)
		resp, err := c.DoManageIQ(ctx, http.MethodGet, path, nil)
		if err != nil {
			return err
		}
		b, err := readResponseBody(resp)
		if err != nil {
			return err
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return NewAPIError(resp, b)
		}
		var t miqTaskStatus
		if err := decodeInto(b, &t); err != nil {
			return err
		}
		if t.State == "Finished" {
			if strings.EqualFold(t.Status, "Ok") {
				return nil
			}
			return fmt.Errorf("ManageIQ task %s finished with status %q", taskID, t.Status)
		}
		time.Sleep(manageIQTaskPollInterval)
	}
}

// stripDefaultFirewallRulesAfterCreate waits until the platform has attached the usual two default egress
// rules (IPv4 and IPv6), then removes every firewall rule until the group is empty. Early empty list is
// treated as “not ready yet”, not success.
func (c *Client) stripDefaultFirewallRulesAfterCreate(ctx context.Context, groupID string) error {
	deadline := time.Now().Add(manageIQTaskDeadline)
	seenMinimumDefaults := false

	for {
		if time.Now().After(deadline) {
			if !seenMinimumDefaults {
				return fmt.Errorf("timeout waiting for default egress rules (expected at least %d) on security group %s",
					minDefaultEgressRulesAfterCreate, groupID)
			}
			return fmt.Errorf("timeout removing default firewall rules from security group %s", groupID)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		g, err := c.GetSecurityGroup(ctx, groupID)
		if err != nil {
			return err
		}
		rules := g.Rules()

		if !seenMinimumDefaults {
			if len(rules) < minDefaultEgressRulesAfterCreate {
				time.Sleep(manageIQTaskPollInterval)
				continue
			}
			seenMinimumDefaults = true
		}

		if len(rules) == 0 {
			return nil
		}

		removedAny := false
		for _, r := range rules {
			if r.EmsRef == "" {
				continue
			}
			removedAny = true
			if err := c.RemoveFirewallRule(ctx, groupID, r.EmsRef); err != nil {
				return err
			}
		}
		if !removedAny {
			// Rules are listed but ems_ref not yet populated — keep polling.
			time.Sleep(manageIQTaskPollInterval)
			continue
		}
		time.Sleep(manageIQTaskPollInterval)
	}
}

// DeleteSecurityGroup removes a security group (POST /providers/{id}/security_groups with action remove).
func (c *Client) DeleteSecurityGroup(ctx context.Context, id, name string) error {
	prov, err := c.NetworkManagerProviderID(ctx)
	if err != nil {
		return err
	}
	body, err := json.Marshal(securityGroupWrite{Action: "remove", Name: name, ID: id})
	if err != nil {
		return err
	}
	path := fmt.Sprintf("%s/providers/%s/security_groups", ComputeAPIPrefix, prov)
	resp, err := c.DoManageIQ(ctx, http.MethodPost, path, body)
	if err != nil {
		return err
	}
	rb, err := readResponseBody(resp)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return NewAPIError(resp, rb)
	}
	var env miqTaskSubmitResult
	if err := json.Unmarshal(rb, &env); err != nil {
		return err
	}
	if len(env.Results) > 0 {
		r := env.Results[0]
		if r.TaskID != "" {
			return c.waitManageIQTask(ctx, r.TaskID)
		}
		if !r.Success {
			if r.Message != "" {
				return errors.New(r.Message)
			}
			return fmt.Errorf("delete security group rejected by API")
		}
	}
	return nil
}

// ruleAddMutex returns a per–security-group mutex so parallel AddFirewallRule calls
// cannot share the same pre-POST snapshot and both return the first "new" rule in the list.
func (c *Client) ruleAddMutex(groupID string) *sync.Mutex {
	c.muRuleAdd.Lock()
	defer c.muRuleAdd.Unlock()
	if c.ruleAddMu == nil {
		c.ruleAddMu = make(map[string]*sync.Mutex)
	}
	if m, ok := c.ruleAddMu[groupID]; ok {
		return m
	}
	m := &sync.Mutex{}
	c.ruleAddMu[groupID] = m
	return m
}

// AddFirewallRule adds a rule to an existing group. groupID is the cloud security group id (path segment);
// the request body uses the group's ems_ref as security_group_id (ManageIQ convention).
func (c *Client) AddFirewallRule(ctx context.Context, groupID string, in FirewallRuleInput) (*FirewallRule, error) {
	mu := c.ruleAddMutex(groupID)
	mu.Lock()
	defer mu.Unlock()

	sg, err := c.GetSecurityGroup(ctx, groupID)
	if err != nil {
		return nil, err
	}
	before := ruleIDsSnapshot(sg.Rules())

	min, max := splitPortRange(in.PortRange)
	rule := FirewallRule{
		Action:          "add_firewall_rule",
		Direction:       in.Direction,
		PortRangeMin:    min,
		PortRangeMax:    max,
		Protocol:        in.Protocol,
		NetworkProtocol: in.NetworkProtocol,
		RemoteGroupID:   in.RemoteGroupID,
		SecurityGroupID: sg.EmsRef,
		SourceIPRange:   in.RemoteIPSubnet,
	}
	body, err := json.Marshal(rule)
	if err != nil {
		return nil, err
	}
	path := fmt.Sprintf("%s/security_groups/%s", ComputeAPIPrefix, groupID)
	resp, err := c.DoManageIQ(ctx, http.MethodPost, path, body)
	if err != nil {
		return nil, err
	}
	rb, err := readResponseBody(resp)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, NewAPIError(resp, rb)
	}
	var tr miqRuleResponse
	if err := json.Unmarshal(rb, &tr); err != nil {
		return nil, err
	}
	if tr.Success != "" && !strings.EqualFold(tr.Success, "true") {
		if tr.Message != "" {
			return nil, errors.New(tr.Message)
		}
		return nil, fmt.Errorf("add firewall rule failed")
	}
	// API returns the created rule inline (GET-shaped: inbound, port, host_protocol, ems_ref).
	// No results[].task_id in this path — no need to poll /tasks/{id} for identity.
	if tr.Rule != nil && strings.TrimSpace(tr.Rule.EmsRef) != "" {
		return tr.Rule, nil
	}
	sg, err = c.GetSecurityGroup(ctx, groupID)
	if err != nil {
		return nil, err
	}
	return pickNewRuleAfterAdd(before, sg.Rules(), in)
}

// FirewallRuleInput is provider input for AddFirewallRule.
type FirewallRuleInput struct {
	Direction        string
	PortRange        string
	Protocol         string
	NetworkProtocol  string
	RemoteGroupID    string
	RemoteIPSubnet   string
}

func splitPortRange(s string) (min, max string) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", ""
	}
	parts := strings.SplitN(s, "-", 2)
	if len(parts) == 1 {
		p := strings.TrimSpace(parts[0])
		if p == "" {
			return "", ""
		}
		// Single port: API expects the same value for min and max; sending only min
		// can make the platform store max as null and later show ranges like "22-null".
		return p, p
	}
	return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
}

func ruleIDsSnapshot(rules []FirewallRule) map[string]bool {
	m := make(map[string]bool)
	for _, r := range rules {
		if r.ID != "" {
			m[r.ID] = true
		}
	}
	return m
}

func normalizeInputDirection(in string) string {
	d := strings.ToLower(strings.TrimSpace(in))
	switch d {
	case "inbound", "ingress":
		return "ingress"
	case "outbound", "egress":
		return "egress"
	default:
		return d
	}
}

// ruleMatchesInput compares API rule shape to the POST body fields (GET may use port vs port_range_min).
func ruleMatchesInput(r FirewallRule, in FirewallRuleInput) bool {
	if normalizeInputDirection(in.Direction) != r.DirectionNormalized() {
		return false
	}
	inMin, inMax := splitPortRange(in.PortRange)
	apiMin := r.PortRangeMinEffective()
	apiMax := r.PortRangeMaxEffective()
	if inMax == "" && inMin != "" {
		inMax = inMin
	}
	if apiMax == "" && apiMin != "" {
		apiMax = apiMin
	}
	if strings.TrimSpace(inMin) != strings.TrimSpace(apiMin) || strings.TrimSpace(inMax) != strings.TrimSpace(apiMax) {
		return false
	}
	if strings.TrimSpace(in.Protocol) != "" && !strings.EqualFold(strings.TrimSpace(in.Protocol), r.ProtocolEffective()) {
		return false
	}
	if strings.TrimSpace(in.NetworkProtocol) != "" && !strings.EqualFold(strings.TrimSpace(in.NetworkProtocol), strings.TrimSpace(r.NetworkProtocol)) {
		return false
	}
	if strings.TrimSpace(in.RemoteGroupID) != "" && strings.TrimSpace(in.RemoteGroupID) != strings.TrimSpace(r.RemoteGroupID) {
		return false
	}
	if strings.TrimSpace(in.RemoteIPSubnet) != "" && strings.TrimSpace(in.RemoteIPSubnet) != strings.TrimSpace(r.SourceIPRange) {
		return false
	}
	return true
}

func pickNewRuleAfterAdd(before map[string]bool, rules []FirewallRule, in FirewallRuleInput) (*FirewallRule, error) {
	var candidates []FirewallRule
	for _, r := range rules {
		if r.ID == "" {
			continue
		}
		if !before[r.ID] {
			candidates = append(candidates, r)
		}
	}
	if len(candidates) == 0 {
		return nil, fmt.Errorf("could not locate new firewall rule after add")
	}
	if len(candidates) == 1 {
		return &candidates[0], nil
	}
	var matches []FirewallRule
	for _, r := range candidates {
		if ruleMatchesInput(r, in) {
			matches = append(matches, r)
		}
	}
	if len(matches) == 1 {
		return &matches[0], nil
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("could not locate new firewall rule after add (%d new rules, attributes did not match request)", len(candidates))
	}
	return nil, fmt.Errorf("ambiguous new firewall rule after add (%d new rules match the same attributes)", len(matches))
}

// RemoveFirewallRule removes a rule by its ems_ref (ManageIQ reference).
func (c *Client) RemoveFirewallRule(ctx context.Context, groupID, ruleEmsRef string) error {
	rule := FirewallRule{
		Action: "remove_firewall_rule",
		ID:     ruleEmsRef,
	}
	body, err := json.Marshal(rule)
	if err != nil {
		return err
	}
	path := fmt.Sprintf("%s/security_groups/%s", ComputeAPIPrefix, groupID)
	resp, err := c.DoManageIQ(ctx, http.MethodPost, path, body)
	if err != nil {
		return err
	}
	rb, err := readResponseBody(resp)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return NewAPIError(resp, rb)
	}
	var out miqRuleDeleteResponse
	if err := json.Unmarshal(rb, &out); err != nil {
		return err
	}
	if !out.Success {
		if out.Message != "" {
			return errors.New(out.Message)
		}
		return fmt.Errorf("remove firewall rule failed")
	}
	return nil
}
