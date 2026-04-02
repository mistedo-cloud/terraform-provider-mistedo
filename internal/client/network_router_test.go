package client

import "testing"

func TestRouterHasRoute(t *testing.T) {
	t.Parallel()
	r := &NetworkRouter{
		ExtraAttributes: routerExtraAttrs{
			Routes: []RouterRoute{
				{Destination: "10.220.0.0/16", Nexthop: "10.220.0.2"},
			},
		},
	}
	if !RouterHasRoute(r, "10.220.0.0/16", "10.220.0.2") {
		t.Fatal("expected match")
	}
	if RouterHasRoute(r, "10.221.0.0/16", "10.220.0.2") {
		t.Fatal("expected no match on destination")
	}
	if RouterHasRoute(nil, "0.0.0.0/0", "1.1.1.1") {
		t.Fatal("nil router")
	}
}
