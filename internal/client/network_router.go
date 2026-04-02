package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

// NetworkRouter is a tenant network router from GET /network_routers.
type NetworkRouter struct {
	ID              string           `json:"id,omitempty"`
	Name            string           `json:"name,omitempty"`
	EmsRef          string           `json:"ems_ref,omitempty"`
	CloudNetworkID  string           `json:"cloud_network_id,omitempty"`
	Status          string           `json:"status,omitempty"`
	ExtraAttributes routerExtraAttrs `json:"extra_attributes"`
}

type routerExtraAttrs struct {
	Routes []RouterRoute `json:"routes"`
}

// RouterRoute is one static route in extra_attributes.routes.
type RouterRoute struct {
	Destination string `json:"destination"`
	Nexthop     string `json:"nexthop"`
}

type networkRouterCollection struct {
	Resources []NetworkRouter `json:"resources"`
}

type routerRouteActionBody struct {
	Action      string `json:"action"`
	Destination string `json:"destination"`
	Nexthop     string `json:"nexthop"`
}

// ListNetworkRouters returns GET /network_routers?expand=resources.
func (c *Client) ListNetworkRouters(ctx context.Context) ([]NetworkRouter, error) {
	path := ComputeAPIPrefix + "/network_routers?expand=resources"
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
	var coll networkRouterCollection
	if err := decodeInto(b, &coll); err != nil {
		return nil, fmt.Errorf("decode network routers: %w", err)
	}
	return coll.Resources, nil
}

// GetNetworkRouter returns GET /network_routers/{id}?expand=resources.
func (c *Client) GetNetworkRouter(ctx context.Context, id string) (*NetworkRouter, error) {
	path := fmt.Sprintf("%s/network_routers/%s?expand=resources", ComputeAPIPrefix, id)
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
	var r NetworkRouter
	if err := decodeInto(b, &r); err != nil {
		return nil, fmt.Errorf("decode network router: %w", err)
	}
	if r.ID == "" {
		r.ID = id
	}
	return &r, nil
}

// ResolveDefaultNetworkRouterID picks the tenant default router: the only list entry, or the one named
// "{location}_{account}_default" (same pattern as the platform UI).
func (c *Client) ResolveDefaultNetworkRouterID(ctx context.Context) (string, error) {
	list, err := c.ListNetworkRouters(ctx)
	if err != nil {
		return "", err
	}
	if len(list) == 0 {
		return "", fmt.Errorf("no network routers returned by the API")
	}
	if len(list) == 1 {
		return strings.TrimSpace(list[0].ID), nil
	}
	loc := strings.ToLower(strings.TrimSpace(c.cfg.Location))
	acct := strings.ToLower(strings.TrimSpace(c.cfg.Account))
	want := loc + "_" + acct + "_default"
	for i := range list {
		if strings.TrimSpace(list[i].Name) == want {
			return strings.TrimSpace(list[i].ID), nil
		}
	}
	return "", fmt.Errorf("multiple network routers (%d) and none named %q; cannot resolve default router id", len(list), want)
}

// RouterHasRoute reports whether the router carries a route with the given destination and nexthop.
func RouterHasRoute(r *NetworkRouter, destination, nexthop string) bool {
	if r == nil {
		return false
	}
	dWant := strings.TrimSpace(destination)
	nWant := strings.TrimSpace(nexthop)
	for _, rt := range r.ExtraAttributes.Routes {
		if strings.TrimSpace(rt.Destination) == dWant && strings.TrimSpace(rt.Nexthop) == nWant {
			return true
		}
	}
	return false
}

// AddNetworkRouterRoute runs POST /network_routers/{id} with action add_route (synchronous in practice).
func (c *Client) AddNetworkRouterRoute(ctx context.Context, routerID, destination, nexthop string) error {
	body, err := json.Marshal(routerRouteActionBody{
		Action:      "add_route",
		Destination: strings.TrimSpace(destination),
		Nexthop:     strings.TrimSpace(nexthop),
	})
	if err != nil {
		return err
	}
	path := fmt.Sprintf("%s/network_routers/%s", ComputeAPIPrefix, strings.TrimSpace(routerID))
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
	return finishManageIQActionResponse(ctx, c, rb)
}

// RemoveNetworkRouterRoute runs POST /network_routers/{id} with action remove_route.
func (c *Client) RemoveNetworkRouterRoute(ctx context.Context, routerID, destination, nexthop string) error {
	body, err := json.Marshal(routerRouteActionBody{
		Action:      "remove_route",
		Destination: strings.TrimSpace(destination),
		Nexthop:     strings.TrimSpace(nexthop),
	})
	if err != nil {
		return err
	}
	path := fmt.Sprintf("%s/network_routers/%s", ComputeAPIPrefix, strings.TrimSpace(routerID))
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
	return finishManageIQActionResponse(ctx, c, rb)
}

// finishManageIQActionResponse handles empty 200 bodies and optional ManageIQ results envelopes.
func finishManageIQActionResponse(ctx context.Context, c *Client, rb []byte) error {
	rb = bytes.TrimSpace(rb)
	if len(rb) == 0 {
		return nil
	}
	var env miqTaskSubmitResult
	if err := json.Unmarshal(rb, &env); err != nil {
		return nil
	}
	if len(env.Results) == 0 {
		return nil
	}
	r0 := env.Results[0]
	if r0.TaskID != "" {
		return c.waitManageIQTask(ctx, r0.TaskID)
	}
	if !r0.Success {
		if r0.Message != "" {
			return errors.New(r0.Message)
		}
		return fmt.Errorf("API rejected the request")
	}
	return nil
}
