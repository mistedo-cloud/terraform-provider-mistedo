package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"
)

const networkDeleteWaitDeadline = 20 * time.Minute

// Network is a tenant private network from the regional compute API (ManageIQ cloud_network JSON type).
type Network struct {
	ID              string                 `json:"id,omitempty"`
	Href            string                 `json:"href,omitempty"`
	Name            string                 `json:"name"`
	EmsRef          string                 `json:"ems_ref,omitempty"`
	EmsID           string                 `json:"ems_id,omitempty"`
	Status          string                 `json:"status,omitempty"`
	CloudTenantID   string                 `json:"cloud_tenant_id,omitempty"`
	Type            string                 `json:"type,omitempty"`
	ExtraAttributes map[string]interface{} `json:"extra_attributes,omitempty"`
	Subnets         []NetworkSubnet        `json:"cloud_subnets,omitempty"`
}

// MTU returns extra_attributes.maximum_transmission_unit when present.
func (n *Network) MTU() int64 {
	if n == nil || len(n.ExtraAttributes) == 0 {
		return 0
	}
	v, ok := n.ExtraAttributes["maximum_transmission_unit"]
	if !ok || v == nil {
		return 0
	}
	switch x := v.(type) {
	case float64:
		return int64(x)
	case int:
		return int64(x)
	case int64:
		return x
	case json.Number:
		i, err := x.Int64()
		if err != nil {
			return 0
		}
		return i
	default:
		return 0
	}
}

// NetworkSubnet is the subnet attached to a network on this platform.
type NetworkSubnet struct {
	ID              string                 `json:"id,omitempty"`
	Href            string                 `json:"href,omitempty"`
	Name            string                 `json:"name"`
	EmsRef          string                 `json:"ems_ref,omitempty"`
	EmsID           string                 `json:"ems_id,omitempty"`
	ParentNetworkID string                 `json:"cloud_network_id,omitempty"`
	CloudTenantID   string                 `json:"cloud_tenant_id,omitempty"`
	Cidr            string                 `json:"cidr,omitempty"`
	Status          string                 `json:"status,omitempty"`
	DhcpEnabled     bool                   `json:"dhcp_enabled,omitempty"`
	Gateway         string                 `json:"gateway,omitempty"`
	NetworkProtocol string                 `json:"network_protocol,omitempty"`
	DnsNameservers  []string               `json:"dns_nameservers,omitempty"`
	ExtraAttributes map[string]interface{} `json:"extra_attributes,omitempty"`
	Type            string                 `json:"type,omitempty"`
	// Populated when loading attributes=cloud_subnets.network_ports (ManageIQ association).
	NetworkPorts []struct {
		ID string `json:"id"`
	} `json:"network_ports,omitempty"`
	CloudNetworkPorts []struct {
		ID string `json:"id"`
	} `json:"cloud_network_ports,omitempty"`
}

// IPVersionFromSubnetExtra returns extra_attributes.ip_version when set (e.g. 4).
func (s *NetworkSubnet) IPVersionFromSubnetExtra() int64 {
	if s == nil || len(s.ExtraAttributes) == 0 {
		return 0
	}
	v, ok := s.ExtraAttributes["ip_version"]
	if !ok || v == nil {
		return 0
	}
	switch x := v.(type) {
	case float64:
		return int64(x)
	case int:
		return int64(x)
	case int64:
		return x
	case json.Number:
		i, err := x.Int64()
		if err != nil {
			return 0
		}
		return i
	default:
		return 0
	}
}

type networkCollection struct {
	Resources []Network `json:"resources"`
}

type networkCreateBody struct {
	Action string          `json:"action"`
	Name   string          `json:"name"`
	Subnet networkSubnetIn `json:"subnet"`
}

type networkSubnetIn struct {
	Cidr            string   `json:"cidr"`
	IPVersion       int      `json:"ip_version"`
	NetworkProtocol string   `json:"network_protocol"`
	DnsNameservers  []string `json:"dns_nameservers,omitempty"`
	Name            string   `json:"name"`
}

type networkDeleteBody struct {
	Action string `json:"action"`
	ID     string `json:"id"`
}

// Fixed subnet stack for create (platform does not expose these for user changes).
const (
	networkCreateIPVersion       = 4
	networkCreateNetworkProtocol = "ipv4"
)

// NetworkSubnetCreateInput is the user-controlled subnet portion of a create-network request.
// IP version and network_protocol are fixed to match the live API contract.
type NetworkSubnetCreateInput struct {
	Cidr           string
	DnsNameservers []string
}

// ListNetworks returns GET /providers/{id}/cloud_networks?expand=resources.
func (c *Client) ListNetworks(ctx context.Context) ([]Network, error) {
	prov, err := c.NetworkManagerProviderID(ctx)
	if err != nil {
		return nil, err
	}
	path := fmt.Sprintf("%s/providers/%s/cloud_networks?expand=resources", ComputeAPIPrefix, prov)
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
	var coll networkCollection
	if err := decodeInto(b, &coll); err != nil {
		return nil, fmt.Errorf("decode networks: %w", err)
	}
	return coll.Resources, nil
}

// GetNetwork returns GET /cloud_networks/{id}?expand=resources&attributes=cloud_subnets.
func (c *Client) GetNetwork(ctx context.Context, id string) (*Network, error) {
	return c.getNetwork(ctx, id, "cloud_subnets")
}

func (c *Client) getNetwork(ctx context.Context, id, attributes string) (*Network, error) {
	path := fmt.Sprintf("%s/cloud_networks/%s?expand=resources&attributes=%s",
		ComputeAPIPrefix, id, attributes)
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
	var net Network
	if err := decodeInto(b, &net); err != nil {
		return nil, fmt.Errorf("decode network: %w", err)
	}
	if net.ID == "" {
		net.ID = id
	}
	return &net, nil
}

func countSubnetNetworkPorts(net *Network) int {
	if net == nil {
		return 0
	}
	n := 0
	for _, sn := range net.Subnets {
		n += len(sn.NetworkPorts) + len(sn.CloudNetworkPorts)
	}
	return n
}

func deleteNetworkTransientHTTP(code int) bool {
	switch code {
	case http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusConflict, http.StatusTooManyRequests:
		return true
	default:
		return false
	}
}

func (c *Client) networkIDsSnapshot(ctx context.Context) (map[string]bool, error) {
	list, err := c.ListNetworks(ctx)
	if err != nil {
		return nil, err
	}
	m := make(map[string]bool, len(list))
	for _, n := range list {
		if n.ID != "" {
			m[n.ID] = true
		}
	}
	return m, nil
}

// CreateNetwork creates a network via POST /providers/{id}/cloud_networks, waits for the task,
// then loads the new network once its subnet exists.
func (c *Client) CreateNetwork(ctx context.Context, name string, in NetworkSubnetCreateInput) (*Network, error) {
	prov, err := c.NetworkManagerProviderID(ctx)
	if err != nil {
		return nil, err
	}
	before, err := c.networkIDsSnapshot(ctx)
	if err != nil {
		return nil, err
	}
	body, err := json.Marshal(networkCreateBody{
		Action: "create",
		Name:   name,
		Subnet: networkSubnetIn{
			Cidr:            in.Cidr,
			IPVersion:       networkCreateIPVersion,
			NetworkProtocol: networkCreateNetworkProtocol,
			DnsNameservers:  in.DnsNameservers,
			Name:            name,
		},
	})
	if err != nil {
		return nil, err
	}
	path := fmt.Sprintf("%s/providers/%s/cloud_networks", ComputeAPIPrefix, prov)
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
	after, err := c.ListNetworks(ctx)
	if err != nil {
		return nil, err
	}
	var newID string
	for _, n := range after {
		if n.ID != "" && !before[n.ID] {
			newID = n.ID
			break
		}
	}
	if newID == "" {
		return nil, fmt.Errorf("could not find new network after create task (name %q)", name)
	}
	return c.waitNetworkReady(ctx, newID)
}

func (c *Client) waitNetworkReady(ctx context.Context, id string) (*Network, error) {
	deadline := time.Now().Add(manageIQTaskDeadline)
	for {
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("timeout waiting for subnet on network %s", id)
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		net, err := c.GetNetwork(ctx, id)
		if err != nil {
			return nil, err
		}
		if len(net.Subnets) > 0 {
			return net, nil
		}
		time.Sleep(manageIQTaskPollInterval)
	}
}

// DeleteNetwork deletes a network by id (POST .../cloud_networks with action delete).
// The platform rejects delete while VM network interfaces still reference the subnet; this method
// waits for subnet network_ports to clear when the API exposes them, and retries transient failures.
func (c *Client) DeleteNetwork(ctx context.Context, id string) error {
	deadline := time.Now().Add(networkDeleteWaitDeadline)
	portAttrSupported := true

	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("timeout deleting network %s (VM network interfaces may still be attached to the subnet; retry destroy after VMs are fully removed)", id)
		}

		if portAttrSupported {
			net, err := c.getNetwork(ctx, id, "cloud_subnets.network_ports")
			if errors.Is(err, ErrNotFound) {
				return nil
			}
			if err != nil {
				var api *APIError
				if errors.As(err, &api) {
					switch {
					case api.StatusCode == http.StatusBadRequest:
						portAttrSupported = false
					case deleteNetworkTransientHTTP(api.StatusCode):
						time.Sleep(manageIQTaskPollInterval)
						continue
					default:
						return err
					}
				} else {
					return err
				}
			} else if countSubnetNetworkPorts(net) > 0 {
				time.Sleep(manageIQTaskPollInterval)
				continue
			}
		}

		err := c.deleteNetworkOnce(ctx, id)
		if err == nil {
			return nil
		}
		if errors.Is(err, ErrNotFound) {
			return nil
		}
		var api *APIError
		if errors.As(err, &api) && deleteNetworkTransientHTTP(api.StatusCode) {
			time.Sleep(manageIQTaskPollInterval)
			continue
		}
		return err
	}
}

func (c *Client) deleteNetworkOnce(ctx context.Context, id string) error {
	prov, err := c.NetworkManagerProviderID(ctx)
	if err != nil {
		return err
	}
	body, err := json.Marshal(networkDeleteBody{Action: "delete", ID: id})
	if err != nil {
		return err
	}
	path := fmt.Sprintf("%s/providers/%s/cloud_networks", ComputeAPIPrefix, prov)
	resp, err := c.DoManageIQ(ctx, http.MethodPost, path, body)
	if err != nil {
		return err
	}
	rb, err := readResponseBody(resp)
	if err != nil {
		return err
	}
	if resp.StatusCode == http.StatusNotFound {
		return ErrNotFound
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
			return fmt.Errorf("delete network rejected by API")
		}
	}
	return nil
}
