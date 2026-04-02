package client

import (
	"encoding/json"
	"testing"
)

func TestCountSubnetNetworkPorts(t *testing.T) {
	raw := `{"cloud_subnets":[{"network_ports":[{"id":"1"},{"id":"2"}],"cloud_network_ports":[{"id":"3"}]}]}`
	var net Network
	if err := json.Unmarshal([]byte(raw), &net); err != nil {
		t.Fatal(err)
	}
	if n := countSubnetNetworkPorts(&net); n != 3 {
		t.Fatalf("got %d want 3", n)
	}
	if n := countSubnetNetworkPorts(nil); n != 0 {
		t.Fatalf("nil: got %d want 0", n)
	}
}

func TestDeleteNetworkTransientHTTP(t *testing.T) {
	if !deleteNetworkTransientHTTP(500) || !deleteNetworkTransientHTTP(409) {
		t.Fatal("expected 500 and 409 transient")
	}
	if deleteNetworkTransientHTTP(400) || deleteNetworkTransientHTTP(404) {
		t.Fatal("did not expect 400/404 transient")
	}
}
