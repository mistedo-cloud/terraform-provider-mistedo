package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

// ALBAPIPrefix is the path prefix for the Traefik Manager (ALB) API.
const ALBAPIPrefix = "/api/traefik_manager/v1"

// --- Gateways (data sources) ---

// Gateway is an ALB Cloud Gateway (see OpenAPI Gateway schema).
type Gateway struct {
	ID              int    `json:"id"`
	Name            string `json:"name"`
	Account         string `json:"account,omitempty"`
	CloudgwID       string `json:"cloudgw_id,omitempty"`
	CloudgwInstance string `json:"cloudgw_instance,omitempty"`
	CreatedAt       string `json:"created_at,omitempty"`
	UpdatedAt       string `json:"updated_at,omitempty"`
}

// ListGateways returns GET /gateways.
func (c *Client) ListGateways(ctx context.Context) ([]Gateway, error) {
	resp, err := c.Do(ctx, http.MethodGet, ALBAPIPrefix+"/gateways", nil)
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
	var out []Gateway
	if err := decodeInto(b, &out); err != nil {
		return nil, fmt.Errorf("decode gateways: %w", err)
	}
	return out, nil
}

// --- Backend services (data sources) ---

// LBService is a compute service (instance group) exposed to ALB routes.
type LBService struct {
	ID          int    `json:"id"`
	Name        string `json:"name,omitempty"`
	ExtID       int64  `json:"ext_id"`
	IPAddresses string `json:"ipaddresses,omitempty"`
	Owner       string `json:"owner,omitempty"`
	Account     string `json:"account,omitempty"`
	CreatedAt   string `json:"created_at,omitempty"`
	UpdatedAt   string `json:"updated_at,omitempty"`
}

// ListLBServices returns GET /services.
func (c *Client) ListLBServices(ctx context.Context) ([]LBService, error) {
	resp, err := c.Do(ctx, http.MethodGet, ALBAPIPrefix+"/services", nil)
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
	var out []LBService
	if err := decodeInto(b, &out); err != nil {
		return nil, fmt.Errorf("decode services: %w", err)
	}
	return out, nil
}

// --- Routes ---

// ServiceRef identifies a backend service on create/update (matches services[] in route body).
type ServiceRef struct {
	ID          int      `json:"id"`
	BalanceType string   `json:"balance_type,omitempty"`
	Value       *float64 `json:"value,omitempty"`
}

// HealthcheckInput is the request payload for healthcheck on a route.
type HealthcheckInput struct {
	Path            *string           `json:"path,omitempty"`
	Scheme          *string           `json:"scheme,omitempty"`
	Hostname        *string           `json:"hostname,omitempty"`
	Port            *int              `json:"port,omitempty"`
	Interval        *int              `json:"interval,omitempty"`
	Timeout         *int              `json:"timeout,omitempty"`
	Method          *string           `json:"method,omitempty"`
	FollowRedirects *bool             `json:"follow_redirects,omitempty"`
	Headers         map[string]string `json:"headers,omitempty"`
}

// Route is a web route returned by the API (GET/POST/PUT).
type Route struct {
	ID                 int               `json:"id,omitempty"`
	Name               string            `json:"name,omitempty"`
	Hostname           string            `json:"hostname,omitempty"`
	Path               string            `json:"path,omitempty"`
	TargetPort         int               `json:"target_port,omitempty"`
	Insecure           string            `json:"insecure,omitempty"`
	TLSTermination     string            `json:"tls_termination,omitempty"`
	Labels             *string           `json:"labels,omitempty"`
	CloudGatewayID     int               `json:"cloud_gateway_id,omitempty"`
	CertificateID      *int              `json:"certificate_id,omitempty"`
	CreatedAt          string            `json:"created_at,omitempty"`
	UpdatedAt          string            `json:"updated_at,omitempty"`
	Owner              string            `json:"owner,omitempty"`
	SourceProto        string            `json:"source_proto,omitempty"`
	DestinationProto   string            `json:"destination_proto,omitempty"`
	IPVersion          string            `json:"ip_version,omitempty"`
	HealthcheckEnabled bool              `json:"healthcheck_enabled,omitempty"`
	Services           []LBService       `json:"services,omitempty"`
	RoutesServices     []RouteServiceRow `json:"routes_services,omitempty"`
	CloudGateway       *Gateway          `json:"cloud_gateway,omitempty"`
	Healthcheck        *HealthcheckState `json:"healthcheck,omitempty"`
}

// RouteServiceRow is the join row between routes and services.
type RouteServiceRow struct {
	ID          int      `json:"id,omitempty"`
	RouteID     int      `json:"route_id,omitempty"`
	ServiceID   int      `json:"service_id,omitempty"`
	BalanceType string   `json:"balance_type,omitempty"`
	Value       *float64 `json:"value,omitempty"`
	Name        string   `json:"name,omitempty"`
	CreatedAt   string   `json:"created_at,omitempty"`
	UpdatedAt   string   `json:"updated_at,omitempty"`
}

// UnmarshalJSON accepts API quirks: value as int/float/string, or alternate key "weight".
func (r *RouteServiceRow) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	type rowCore struct {
		ID          int    `json:"id"`
		RouteID     int    `json:"route_id"`
		ServiceID   int    `json:"service_id"`
		BalanceType string `json:"balance_type"`
		Name        string `json:"name"`
		CreatedAt   string `json:"created_at"`
		UpdatedAt   string `json:"updated_at"`
	}
	var core rowCore
	if err := json.Unmarshal(data, &core); err != nil {
		return err
	}
	r.ID = core.ID
	r.RouteID = core.RouteID
	r.ServiceID = core.ServiceID
	r.BalanceType = core.BalanceType
	r.Name = core.Name
	r.CreatedAt = core.CreatedAt
	r.UpdatedAt = core.UpdatedAt
	r.Value = parseJSONWeight(raw["value"])
	if r.Value == nil {
		r.Value = parseJSONWeight(raw["weight"])
	}
	return nil
}

func parseJSONWeight(b json.RawMessage) *float64 {
	if len(b) == 0 || string(b) == "null" {
		return nil
	}
	var f float64
	if err := json.Unmarshal(b, &f); err == nil {
		return &f
	}
	var i int64
	if err := json.Unmarshal(b, &i); err == nil {
		f := float64(i)
		return &f
	}
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return nil
	}
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return nil
	}
	return &v
}

// HealthcheckState is healthcheck as returned by the API.
type HealthcheckState struct {
	ID              int               `json:"id,omitempty"`
	Path            string            `json:"path,omitempty"`
	Scheme          string            `json:"scheme,omitempty"`
	Hostname        string            `json:"hostname,omitempty"`
	Port            *int              `json:"port,omitempty"`
	Interval        int               `json:"interval,omitempty"`
	Timeout         int               `json:"timeout,omitempty"`
	Method          string            `json:"method,omitempty"`
	FollowRedirects bool              `json:"follow_redirects,omitempty"`
	Headers         map[string]string `json:"headers,omitempty"`
	RouteID         int               `json:"route_id,omitempty"`
}

// RouteWrite is the JSON body for POST /routes and PUT /routes/{id} (flat object).
type RouteWrite struct {
	Name               string            `json:"name"`
	Hostname           string            `json:"hostname"`
	Path               string            `json:"path,omitempty"`
	TargetPort         int               `json:"target_port"`
	CloudGatewayID     int               `json:"cloud_gateway_id"`
	IPVersion          string            `json:"ip_version"`
	Insecure           *string           `json:"insecure,omitempty"`
	TLSTermination     *string           `json:"tls_termination,omitempty"`
	CertificateID      *int              `json:"certificate_id,omitempty"`
	Services           []ServiceRef      `json:"services"`
	HealthcheckEnabled bool              `json:"healthcheck_enabled"`
	Healthcheck        *HealthcheckInput `json:"healthcheck,omitempty"`
	SourceProto        *string           `json:"source_proto,omitempty"`
	DestinationProto   *string           `json:"destination_proto,omitempty"`
	Labels             *string           `json:"labels,omitempty"`
}

func decodeRouteResponse(b []byte) (*Route, error) {
	if len(b) == 0 {
		return nil, fmt.Errorf("empty route response")
	}
	var wrapped struct {
		Route Route `json:"route"`
	}
	if err := json.Unmarshal(b, &wrapped); err == nil && (wrapped.Route.ID != 0 || wrapped.Route.Name != "" || wrapped.Route.Hostname != "") {
		return &wrapped.Route, nil
	}
	var r Route
	if err := json.Unmarshal(b, &r); err != nil {
		return nil, err
	}
	return &r, nil
}

// ListRoutes returns GET /routes.
func (c *Client) ListRoutes(ctx context.Context) ([]Route, error) {
	resp, err := c.Do(ctx, http.MethodGet, ALBAPIPrefix+"/routes", nil)
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
	var out []Route
	if err := decodeInto(b, &out); err != nil {
		return nil, fmt.Errorf("decode routes: %w", err)
	}
	return out, nil
}

// GetRoute returns GET /routes/{id}.
func (c *Client) GetRoute(ctx context.Context, id int) (*Route, error) {
	path := ALBAPIPrefix + "/routes/" + strconv.Itoa(id)
	resp, err := c.Do(ctx, http.MethodGet, path, nil)
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
	return decodeRouteResponse(b)
}

// CreateRoute POST /routes with a flat JSON body.
func (c *Client) CreateRoute(ctx context.Context, w *RouteWrite) (*Route, error) {
	raw, err := json.Marshal(w)
	if err != nil {
		return nil, err
	}
	resp, err := c.Do(ctx, http.MethodPost, ALBAPIPrefix+"/routes", raw)
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
	return decodeRouteResponse(b)
}

// UpdateRoute PUT /routes/{id} with a full flat JSON body (never send empty {}).
func (c *Client) UpdateRoute(ctx context.Context, id int, w *RouteWrite) (*Route, error) {
	raw, err := json.Marshal(w)
	if err != nil {
		return nil, err
	}
	path := ALBAPIPrefix + "/routes/" + strconv.Itoa(id)
	resp, err := c.Do(ctx, http.MethodPut, path, raw)
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
	return decodeRouteResponse(b)
}

// DeleteRoute DELETE /routes/{id}.
func (c *Client) DeleteRoute(ctx context.Context, id int) error {
	path := ALBAPIPrefix + "/routes/" + strconv.Itoa(id)
	resp, err := c.Do(ctx, http.MethodDelete, path, nil)
	if err != nil {
		return err
	}
	b, err := readResponseBody(resp)
	if err != nil {
		return err
	}
	if resp.StatusCode == http.StatusNotFound {
		return ErrNotFound
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return NewAPIError(resp, b)
	}
	return nil
}

// --- Certificates ---

// CertificateValues holds PEM material in API responses.
type CertificateValues struct {
	Cert   string `json:"cert,omitempty"`
	Key    string `json:"key,omitempty"`
	CA     string `json:"ca,omitempty"`
	DestCA string `json:"dest_ca,omitempty"`
}

// Certificate is a TLS certificate record.
type Certificate struct {
	ID         int                `json:"id,omitempty"`
	Name       string             `json:"name,omitempty"`
	Owner      string             `json:"owner,omitempty"`
	Account    string             `json:"account,omitempty"`
	CertPath   string             `json:"cert_path,omitempty"`
	KeyPath    string             `json:"key_path,omitempty"`
	CAPath     string             `json:"ca_path,omitempty"`
	DestCAPath string             `json:"dest_ca_path,omitempty"`
	CreatedAt  string             `json:"created_at,omitempty"`
	UpdatedAt  string             `json:"updated_at,omitempty"`
	Values     *CertificateValues `json:"values,omitempty"`
}

type certificateEnvelope struct {
	Certificate CertificateWrite `json:"certificate"`
}

// CertificateWrite is the inner object for POST/PUT /certificates.
type CertificateWrite struct {
	Name   string `json:"name"`
	Owner  string `json:"owner,omitempty"`
	Cert   string `json:"cert"`
	Key    string `json:"key"`
	CA     string `json:"ca,omitempty"`
	DestCA string `json:"dest_ca,omitempty"`
}

// ListCertificates GET /certificates.
func (c *Client) ListCertificates(ctx context.Context) ([]Certificate, error) {
	resp, err := c.Do(ctx, http.MethodGet, ALBAPIPrefix+"/certificates", nil)
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
	var out []Certificate
	if err := decodeInto(b, &out); err != nil {
		return nil, fmt.Errorf("decode certificates: %w", err)
	}
	return out, nil
}

// GetCertificate GET /certificates/{id}.
func (c *Client) GetCertificate(ctx context.Context, id int) (*Certificate, error) {
	path := ALBAPIPrefix + "/certificates/" + strconv.Itoa(id)
	resp, err := c.Do(ctx, http.MethodGet, path, nil)
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
	var cert Certificate
	if err := json.Unmarshal(b, &cert); err != nil {
		return nil, err
	}
	return &cert, nil
}

// CreateCertificate POST /certificates with {"certificate":{...}}.
func (c *Client) CreateCertificate(ctx context.Context, w *CertificateWrite) (*Certificate, error) {
	env := certificateEnvelope{Certificate: *w}
	raw, err := json.Marshal(env)
	if err != nil {
		return nil, err
	}
	resp, err := c.Do(ctx, http.MethodPost, ALBAPIPrefix+"/certificates", raw)
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
	var cert Certificate
	if err := json.Unmarshal(b, &cert); err != nil {
		return nil, fmt.Errorf("decode certificate: %w", err)
	}
	return &cert, nil
}

// UpdateCertificate PUT /certificates/{id} with {"certificate":{...}}.
func (c *Client) UpdateCertificate(ctx context.Context, id int, w *CertificateWrite) (*Certificate, error) {
	env := certificateEnvelope{Certificate: *w}
	raw, err := json.Marshal(env)
	if err != nil {
		return nil, err
	}
	path := ALBAPIPrefix + "/certificates/" + strconv.Itoa(id)
	resp, err := c.Do(ctx, http.MethodPut, path, raw)
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
	var cert Certificate
	if err := json.Unmarshal(b, &cert); err != nil {
		return nil, fmt.Errorf("decode certificate: %w", err)
	}
	return &cert, nil
}

// DeleteCertificate DELETE /certificates/{id}.
func (c *Client) DeleteCertificate(ctx context.Context, id int) error {
	path := ALBAPIPrefix + "/certificates/" + strconv.Itoa(id)
	resp, err := c.Do(ctx, http.MethodDelete, path, nil)
	if err != nil {
		return err
	}
	b, err := readResponseBody(resp)
	if err != nil {
		return err
	}
	if resp.StatusCode == http.StatusNotFound {
		return ErrNotFound
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return NewAPIError(resp, b)
	}
	return nil
}
