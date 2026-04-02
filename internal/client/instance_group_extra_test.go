package client

import (
	"encoding/json"
	"testing"
)

func TestRegionNumberFromIPv4CIDR(t *testing.T) {
	t.Parallel()
	s, err := regionNumberFromIPv4CIDR("10.207.81.0/24")
	if err != nil || s != "81" {
		t.Fatalf("got %q err %v", s, err)
	}
}

func TestIPv4InCIDR(t *testing.T) {
	t.Parallel()
	if !IPv4InCIDR("10.207.81.50", "10.207.81.0/24") {
		t.Fatal("expected inside")
	}
	if IPv4InCIDR("10.207.82.1", "10.207.81.0/24") {
		t.Fatal("expected outside")
	}
}

func TestFlexIDUnmarshal(t *testing.T) {
	t.Parallel()
	var s struct {
		V flexID `json:"vm_id"`
	}
	if err := json.Unmarshal([]byte(`{"vm_id":81000000000091}`), &s); err != nil {
		t.Fatal(err)
	}
	if s.V.String() != "81000000000091" {
		t.Fatalf("got %q", s.V)
	}
	if err := json.Unmarshal([]byte(`{"vm_id":"81000000000091"}`), &s); err != nil {
		t.Fatal(err)
	}
	if s.V.String() != "81000000000091" {
		t.Fatalf("got %q", s.V)
	}
}
