package client

import (
	"encoding/json"
	"testing"
)

func TestDecodeRouteResponse_wrapped(t *testing.T) {
	const raw = `{"route":{"id":7,"name":"n","hostname":"h.example.com","path":"/","target_port":80,"cloud_gateway_id":5,"ip_version":"4"}}`
	r, err := decodeRouteResponse([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	if r.ID != 7 || r.Name != "n" || r.Hostname != "h.example.com" {
		t.Fatalf("unexpected: %+v", r)
	}
}

func TestDecodeRouteResponse_flat(t *testing.T) {
	const raw = `{"id":7,"name":"n","hostname":"h.example.com"}`
	r, err := decodeRouteResponse([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	if r.ID != 7 {
		t.Fatalf("got %+v", r)
	}
}

func TestDecodeRouteResponse_routesServicesAndHealthcheck(t *testing.T) {
	const raw = `{"route":{"id":7,"healthcheck_enabled":true,"routes_services":[
		{"service_id":10,"balance_type":"weighted","value":1.0},
		{"service_id":10,"balance_type":"weighted","value":3.0}
	],"healthcheck":{"path":"/","port":null,"headers":{"Auth":"123"}}}}`
	r, err := decodeRouteResponse([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	if len(r.RoutesServices) != 2 || r.RoutesServices[0].Value == nil || *r.RoutesServices[0].Value != 1 {
		t.Fatalf("routes_services: %+v", r.RoutesServices)
	}
	if r.Healthcheck == nil || r.Healthcheck.Path != "/" || r.Healthcheck.Port != nil {
		t.Fatalf("healthcheck: %+v", r.Healthcheck)
	}
	if r.Healthcheck.Headers["Auth"] != "123" {
		t.Fatalf("headers: %+v", r.Healthcheck.Headers)
	}
}

func TestDecodeRouteResponse_healthcheckAllFieldsAndMultipleHeaders(t *testing.T) {
	// Shape aligned with GET /routes/7 (dev): all probe fields + several headers
	const raw = `{"route":{"id":7,"healthcheck":{"path":"/","scheme":"https","port":22001,"interval":30,"timeout":5,"hostname":"host321","headers":{"Auth":"123","Header2":"321","X-Third":"x"},"method":"GET","follow_redirects":true}}}`
	r, err := decodeRouteResponse([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	h := r.Healthcheck
	if h == nil {
		t.Fatal("nil healthcheck")
	}
	if h.Path != "/" || h.Scheme != "https" || h.Hostname != "host321" || h.Method != "GET" {
		t.Fatalf("basic fields: %+v", h)
	}
	if h.Port == nil || *h.Port != 22001 || h.Interval != 30 || h.Timeout != 5 || !h.FollowRedirects {
		t.Fatalf("numeric/bool: port=%v interval=%d timeout=%d fr=%v", h.Port, h.Interval, h.Timeout, h.FollowRedirects)
	}
	if len(h.Headers) != 3 || h.Headers["Auth"] != "123" || h.Headers["Header2"] != "321" || h.Headers["X-Third"] != "x" {
		t.Fatalf("headers map: %+v", h.Headers)
	}
}

func TestRouteServiceRow_unmarshalValueIntOrWeightKey(t *testing.T) {
	t.Run("integer value", func(t *testing.T) {
		const raw = `{"service_id":10,"balance_type":"weighted","value":3}`
		var rs RouteServiceRow
		if err := json.Unmarshal([]byte(raw), &rs); err != nil {
			t.Fatal(err)
		}
		if rs.Value == nil || *rs.Value != 3 {
			t.Fatalf("value: %+v", rs.Value)
		}
	})
	t.Run("weight key", func(t *testing.T) {
		const raw = `{"service_id":10,"balance_type":"weighted","weight":2.5}`
		var rs RouteServiceRow
		if err := json.Unmarshal([]byte(raw), &rs); err != nil {
			t.Fatal(err)
		}
		if rs.Value == nil || *rs.Value != 2.5 {
			t.Fatalf("value: %+v", rs.Value)
		}
	})
}
