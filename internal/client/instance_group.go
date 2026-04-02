package client

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	instanceGroupCreateDeadline = 20 * time.Minute
	instanceGroupPollInterval   = 2 * time.Second
)

// SubnetResolveResult is built from cloud_networks / cloud_subnets for instance group ordering.
type SubnetResolveResult struct {
	DisplayName  string
	Cidr         string
	Vlan         string
	RegionNumber string
}

// InstanceGroupOrder is the user-facing input to build a cart service request.
type InstanceGroupOrder struct {
	ServiceName          string
	ServiceDescription   string
	TemplateID           string
	DesiredInstanceCount int64
	CPUCores             int64
	MemoryMiB            int64
	BootDiskSizeGiB      int64
	BootDiskType         string
	DataDiskSizeGiB      int64 // 0 = omit additional disk from order
	DataDiskType         string
	SubnetDisplayName    string
	ResolvedCIDR         string
	ResolvedVlan         string
	RegionNumber         string
	SecurityGroupID      string
	PassAuth             string
	Password             string
	UserSSHKeys          []string // full public keys
	SSHKeyPrimary        string   // often same as first key; API expects ssh_key
	PublicRemoteAccess   []string
	ManagedAccess        string
	UserData             string
	IPAssignPolicy       string // auto | manual
	IPSubnet             string // CIDR from subnet resolution
	IPAddress            string // empty for auto
}

type serviceCartResponse struct {
	Results []struct {
		Success          bool   `json:"success"`
		Message          string `json:"message"`
		ServiceRequestID string `json:"service_request_id"`
	} `json:"results"`
}

type serviceRequestTasks struct {
	MiqRequestTasks []struct {
		DestinationID   string `json:"destination_id"`
		DestinationType string `json:"destination_type"`
	} `json:"miq_request_tasks"`
}

// ServiceInstanceGroup is the service object from GET .../services/{id} with expanded attributes.
type ServiceInstanceGroup struct {
	ID             string           `json:"id"`
	Name           string           `json:"name"`
	LifecycleState string           `json:"lifecycle_state"`
	Vms            []serviceVMRef   `json:"vms"`
	Networks       []ServiceNetwork `json:"networks"`
}

type serviceVMRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// ServiceNetwork is one network attachment on a service with IP allocations.
type ServiceNetwork struct {
	Name        string              `json:"name"`
	Cidr        string              `json:"cidr"`
	Gateway     string              `json:"gateway"`
	Allocations []ServiceAllocation `json:"allocations"`
}

// ServiceAllocation is a VM network allocation on a service network.
type ServiceAllocation struct {
	Hostname string `json:"hostname"`
	IP       string `json:"ip"`
	Mac      string `json:"mac"`
	NicName  string `json:"nic_name"`
	Type     string `json:"type"`
	VmID     flexID `json:"vm_id"`
}

type flexID string

func (f *flexID) UnmarshalJSON(b []byte) error {
	b = trimQuotesJSON(b)
	if len(b) == 0 || string(b) == "null" {
		*f = ""
		return nil
	}
	var s string
	if err := json.Unmarshal(b, &s); err == nil {
		*f = flexID(s)
		return nil
	}
	var n json.Number
	if err := json.Unmarshal(b, &n); err == nil {
		*f = flexID(n.String())
		return nil
	}
	var x float64
	if err := json.Unmarshal(b, &x); err == nil {
		*f = flexID(strconv.FormatInt(int64(x), 10))
		return nil
	}
	return fmt.Errorf("flexID: cannot decode %s", string(b))
}

func trimQuotesJSON(b []byte) []byte {
	if len(b) >= 2 && b[0] == '"' {
		var s string
		if json.Unmarshal(b, &s) == nil {
			return []byte(s)
		}
	}
	return b
}

func (f flexID) String() string { return string(f) }

// VMInstanceGroupDetail is GET /vms/{id}?attributes=hardware,disks,...
type VMInstanceGroupDetail struct {
	ID       string          `json:"id"`
	Name     string          `json:"name"`
	Hardware vmHardwareBrief `json:"hardware"`
	Disks    []VmDiskBrief   `json:"disks"`
}

type vmHardwareBrief struct {
	MemoryMB      int `json:"memory_mb"`
	CPUTotalCores int `json:"cpu_total_cores"`
}

// VmDiskBrief is a disk row from GET /vms/{id}?attributes=disks.
type VmDiskBrief struct {
	ID         string  `json:"id"`
	Size       float64 `json:"size"` // bytes (API may send number)
	Filename   string  `json:"filename"`
	StorageID  string  `json:"storage_id"`
	DeviceType string  `json:"device_type"`
	DiskType   string  `json:"disk_type"`
}

// ResolveSubnetForInstanceGroup finds a cloud subnet by its **name** and returns CIDR, vlan string, region_number heuristic.
func (c *Client) ResolveSubnetForInstanceGroup(ctx context.Context, subnetName string) (*SubnetResolveResult, error) {
	subnetName = strings.TrimSpace(subnetName)
	if subnetName == "" {
		return nil, fmt.Errorf("subnet name is empty")
	}
	nets, err := c.ListNetworks(ctx)
	if err != nil {
		return nil, err
	}
	seen := make(map[string]bool)
	var matches []SubnetResolveResult
	for _, n := range nets {
		if n.ID == "" || seen[n.ID] {
			continue
		}
		seen[n.ID] = true
		full, err := c.GetNetwork(ctx, n.ID)
		if err != nil {
			return nil, err
		}
		for _, sn := range full.Subnets {
			if sn.Name != subnetName {
				continue
			}
			cidr := strings.TrimSpace(sn.Cidr)
			if cidr == "" {
				continue
			}
			rn, err := regionNumberFromIPv4CIDR(cidr)
			if err != nil {
				return nil, fmt.Errorf("subnet %q: %w", subnetName, err)
			}
			matches = append(matches, SubnetResolveResult{
				DisplayName:  sn.Name,
				Cidr:         cidr,
				Vlan:         fmt.Sprintf("%s (%s)", sn.Name, sn.Name),
				RegionNumber: rn,
			})
		}
	}
	switch len(matches) {
	case 0:
		return nil, fmt.Errorf("no subnet named %q found (check cloud network / subnet name)", subnetName)
	case 1:
		return &matches[0], nil
	default:
		return nil, fmt.Errorf("ambiguous subnet name %q: %d subnets match", subnetName, len(matches))
	}
}

func regionNumberFromIPv4CIDR(cidr string) (string, error) {
	_, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		return "", err
	}
	ip4 := ipNet.IP.To4()
	if ip4 == nil {
		return "", fmt.Errorf("only IPv4 subnets are supported for region_number derivation")
	}
	return strconv.Itoa(int(ip4[2])), nil
}

// IPv4InCIDR returns true if ipStr is a valid IPv4 inside cidrStr.
func IPv4InCIDR(ipStr, cidrStr string) bool {
	ip := net.ParseIP(strings.TrimSpace(ipStr))
	if ip == nil || ip.To4() == nil {
		return false
	}
	_, n, err := net.ParseCIDR(strings.TrimSpace(cidrStr))
	if err != nil {
		return false
	}
	return n.Contains(ip)
}

// SubmitInstanceGroupCart POSTs service_orders/cart/service_requests and returns service_request_id.
func (c *Client) SubmitInstanceGroupCart(ctx context.Context, ord InstanceGroupOrder) (string, error) {
	if ord.ResolvedCIDR == "" || ord.ResolvedVlan == "" {
		return "", fmt.Errorf("internal: subnet not resolved")
	}
	if ord.RegionNumber == "" {
		return "", fmt.Errorf("internal: region_number empty")
	}
	type cartRes struct {
		ServiceName         string   `json:"service_name"`
		ServiceDescription  string   `json:"service_description"`
		VmMemory            string   `json:"vm_memory"`
		CPU                 int64    `json:"cpu"`
		SystemDiskSize      int64    `json:"system_disk_size"`
		SystemDiskType      string   `json:"system_disk_type"`
		Vlan                string   `json:"vlan"`
		SecurityGroup       string   `json:"security_group,omitempty"`
		PassAuth            string   `json:"pass_auth"`
		Password            string   `json:"password"`
		UserSSHKeys         []string `json:"user_ssh_keys,omitempty"`
		SSHKey              string   `json:"ssh_key,omitempty"`
		PublicRemoteAccess  []string `json:"public_remote_access,omitempty"`
		ManagedAccess       string   `json:"managed_access,omitempty"`
		AdditionalDiskSize  *int64   `json:"additional_disk_size,omitempty"`
		AdditionalDiskType  string   `json:"additional_disk_type,omitempty"`
		NumberOfVms         string   `json:"number_of_vms"`
		UserData            string   `json:"user_data,omitempty"`
		IPAssignPolicy      string   `json:"ip_assign_policy"`
		IPSubnet            string   `json:"ip_subnet"`
		IPAddress           string   `json:"ipaddress"`
		ServiceTemplateHref string   `json:"service_template_href"`
		RegionNumber        string   `json:"region_number"`
	}
	cr := cartRes{
		ServiceName:         ord.ServiceName,
		ServiceDescription:  ord.ServiceDescription,
		VmMemory:            strconv.FormatInt(ord.MemoryMiB, 10),
		CPU:                 ord.CPUCores,
		SystemDiskSize:      ord.BootDiskSizeGiB,
		SystemDiskType:      ord.BootDiskType,
		Vlan:                ord.ResolvedVlan,
		PassAuth:            ord.PassAuth,
		Password:            ord.Password,
		NumberOfVms:         strconv.FormatInt(ord.DesiredInstanceCount, 10),
		IPAssignPolicy:      ord.IPAssignPolicy,
		IPSubnet:            ord.IPSubnet,
		IPAddress:           ord.IPAddress,
		ServiceTemplateHref: fmt.Sprintf("/api/service_templates/%s", strings.TrimSpace(ord.TemplateID)),
		RegionNumber:        ord.RegionNumber,
	}
	if ord.SecurityGroupID != "" {
		cr.SecurityGroup = ord.SecurityGroupID
	}
	if len(ord.UserSSHKeys) > 0 {
		cr.UserSSHKeys = ord.UserSSHKeys
	}
	if strings.TrimSpace(ord.SSHKeyPrimary) != "" {
		cr.SSHKey = strings.TrimSpace(ord.SSHKeyPrimary)
	}
	if len(ord.PublicRemoteAccess) > 0 {
		cr.PublicRemoteAccess = ord.PublicRemoteAccess
	}
	if ord.ManagedAccess != "" {
		cr.ManagedAccess = ord.ManagedAccess
	}
	if strings.TrimSpace(ord.UserData) != "" {
		cr.UserData = ord.UserData
	}
	if ord.DataDiskSizeGiB > 0 {
		sz := ord.DataDiskSizeGiB
		cr.AdditionalDiskSize = &sz
		cr.AdditionalDiskType = ord.DataDiskType
	}
	body, err := json.Marshal(map[string]interface{}{
		"action":    "add",
		"resources": []cartRes{cr},
	})
	if err != nil {
		return "", err
	}
	path := ComputeAPIPrefix + "/service_orders/cart/service_requests/"
	resp, err := c.DoManageIQ(ctx, http.MethodPost, path, body)
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
	var out serviceCartResponse
	if err := json.Unmarshal(b, &out); err != nil {
		return "", fmt.Errorf("decode cart response: %w", err)
	}
	if len(out.Results) == 0 {
		return "", fmt.Errorf("cart response: empty results")
	}
	r0 := out.Results[0]
	if !r0.Success && r0.Message != "" {
		return "", fmt.Errorf("cart rejected: %s", r0.Message)
	}
	if r0.ServiceRequestID == "" {
		return "", fmt.Errorf("cart response: missing service_request_id")
	}
	return r0.ServiceRequestID, nil
}

// PollServiceRequestDestination returns destination_id when DestinationType matches (e.g. "Service").
func (c *Client) PollServiceRequestDestination(ctx context.Context, serviceRequestID, wantType string) (string, error) {
	path := fmt.Sprintf("%s/service_requests/%s?expand=resources&attributes=miq_request_tasks",
		ComputeAPIPrefix, strings.TrimSpace(serviceRequestID))
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
	var tr serviceRequestTasks
	if err := json.Unmarshal(b, &tr); err != nil {
		return "", err
	}
	for _, t := range tr.MiqRequestTasks {
		if strings.EqualFold(t.DestinationType, wantType) && t.DestinationID != "" {
			return t.DestinationID, nil
		}
	}
	return "", nil
}

// GetServiceInstanceGroup GETs service with expanded attributes.
func (c *Client) GetServiceInstanceGroup(ctx context.Context, serviceID, attributes string) (*ServiceInstanceGroup, error) {
	path := fmt.Sprintf("%s/services/%s?expand=resources&attributes=%s",
		ComputeAPIPrefix, strings.TrimSpace(serviceID), attributes)
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
	var svc ServiceInstanceGroup
	if err := json.Unmarshal(b, &svc); err != nil {
		return nil, fmt.Errorf("decode service: %w", err)
	}
	return &svc, nil
}

// GetVMInstanceGroupDetail loads VM with disks for state.
func (c *Client) GetVMInstanceGroupDetail(ctx context.Context, vmID string) (*VMInstanceGroupDetail, error) {
	path := fmt.Sprintf("%s/vms/%s?expand=resources&attributes=hardware,disks,lans,ipaddresses",
		ComputeAPIPrefix, strings.TrimSpace(vmID))
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
	var vm VMInstanceGroupDetail
	if err := json.Unmarshal(b, &vm); err != nil {
		return nil, fmt.Errorf("decode vm: %w", err)
	}
	return &vm, nil
}

// WaitInstanceGroupReady polls until lifecycle is provisioned, VM count matches, and each VM has a primary IPv4 on a nic allocation.
func (c *Client) WaitInstanceGroupReady(ctx context.Context, serviceID string, wantVMs int) (*ServiceInstanceGroup, error) {
	deadline := time.Now().Add(instanceGroupCreateDeadline)
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("timeout waiting for instance group %s to provision (%d VMs)", serviceID, wantVMs)
		}
		svc, err := c.GetServiceInstanceGroup(ctx, serviceID, "vms,networks")
		if err != nil {
			time.Sleep(instanceGroupPollInterval)
			continue
		}
		if strings.ToLower(strings.TrimSpace(svc.LifecycleState)) != "provisioned" {
			time.Sleep(instanceGroupPollInterval)
			continue
		}
		if len(svc.Vms) < wantVMs {
			time.Sleep(instanceGroupPollInterval)
			continue
		}
		nicOK := countNicIPv4Allocations(svc.Networks, wantVMs)
		if nicOK < wantVMs {
			time.Sleep(instanceGroupPollInterval)
			continue
		}
		return svc, nil
	}
}

func countNicIPv4Allocations(nets []ServiceNetwork, want int) int {
	vmSeen := make(map[string]bool)
	for _, n := range nets {
		for _, a := range n.Allocations {
			if strings.ToLower(strings.TrimSpace(a.Type)) != "nic" {
				continue
			}
			ip := strings.TrimSpace(a.IP)
			if ip == "" || !isLikelyIPv4(ip) {
				continue
			}
			vid := a.VmID.String()
			if vid == "" {
				continue
			}
			vmSeen[vid] = true
			if len(vmSeen) >= want {
				return len(vmSeen)
			}
		}
	}
	return len(vmSeen)
}

func isLikelyIPv4(s string) bool {
	ip := net.ParseIP(s)
	return ip != nil && ip.To4() != nil && !ip.IsLinkLocalUnicast()
}

// PollUntilServiceIDCreated spins until Service destination id is available.
func (c *Client) PollUntilServiceIDCreated(ctx context.Context, serviceRequestID string) (string, error) {
	deadline := time.Now().Add(instanceGroupCreateDeadline)
	for {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		if time.Now().After(deadline) {
			return "", fmt.Errorf("timeout waiting for service_request %s to produce a Service id", serviceRequestID)
		}
		id, err := c.PollServiceRequestDestination(ctx, serviceRequestID, "Service")
		if err != nil {
			return "", err
		}
		if id != "" {
			return id, nil
		}
		time.Sleep(instanceGroupPollInterval)
	}
}

// RetireService POSTs request_retire on the service.
func (c *Client) RetireService(ctx context.Context, serviceID string) error {
	body, err := json.Marshal(map[string]string{"action": "request_retire"})
	if err != nil {
		return err
	}
	path := fmt.Sprintf("%s/services/%s", ComputeAPIPrefix, strings.TrimSpace(serviceID))
	resp, err := c.DoManageIQ(ctx, http.MethodPost, path, body)
	if err != nil {
		return err
	}
	b, err := readResponseBody(resp)
	if err != nil {
		return err
	}
	if resp.StatusCode == http.StatusNotFound {
		return nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return NewAPIError(resp, b)
	}
	return nil
}

// SortVMDisksByID sorts disk slice by id string (numeric-friendly).
func SortVMDisksByID(disks []VmDiskBrief) {
	sort.SliceStable(disks, func(i, j int) bool {
		return disks[i].ID < disks[j].ID
	})
}

// DiskSizeGiB converts API size in bytes to whole gibibytes (floor).
func DiskSizeGiB(sizeBytes float64) int64 {
	if sizeBytes <= 0 {
		return 0
	}
	return int64(sizeBytes) / (1 << 30)
}

// LoadInstanceGroupVMDetails fetches disk/hardware details for each VM on the service.
func (c *Client) LoadInstanceGroupVMDetails(ctx context.Context, svc *ServiceInstanceGroup) ([]VMInstanceGroupDetail, error) {
	if svc == nil {
		return nil, fmt.Errorf("service is nil")
	}
	out := make([]VMInstanceGroupDetail, 0, len(svc.Vms))
	for _, v := range svc.Vms {
		if strings.TrimSpace(v.ID) == "" {
			continue
		}
		d, err := c.GetVMInstanceGroupDetail(ctx, v.ID)
		if err != nil {
			return nil, err
		}
		out = append(out, *d)
	}
	return out, nil
}

// GenerateInstanceGroupPassword builds a random password acceptable for typical platform rules.
func GenerateInstanceGroupPassword() (string, error) {
	const chars = "ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnpqrstuvwxyz23456789"
	const n = 18
	var b strings.Builder
	for i := 0; i < n; i++ {
		x, err := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
		if err != nil {
			return "", err
		}
		b.WriteByte(chars[x.Int64()])
	}
	s := b.String()
	// ensure at least one upper, one lower, one digit
	if !strings.ContainsAny(s, "ABCDEFGHJKLMNPQRSTUVWXYZ") {
		s = "A" + s[1:]
	}
	if !strings.ContainsAny(s, "abcdefghijkmnpqrstuvwxyz") {
		s = s[:1] + "a" + s[2:]
	}
	if !strings.ContainsAny(s, "23456789") {
		s = s[:2] + "2" + s[3:]
	}
	return s, nil
}
