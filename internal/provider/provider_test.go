package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/provider"
)

func TestProviderMetadata(t *testing.T) {
	p := New("test")()
	var resp provider.MetadataResponse
	p.Metadata(context.Background(), provider.MetadataRequest{}, &resp)
	if resp.TypeName != "mistedo" {
		t.Errorf("expected TypeName mistedo, got %q", resp.TypeName)
	}
}

func TestProviderSchemaHasRequiredAndOptionalAttributes(t *testing.T) {
	p := New("test")()
	var resp provider.SchemaResponse
	p.Schema(context.Background(), provider.SchemaRequest{}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("schema has diagnostics: %v", resp.Diagnostics)
	}
	schema := resp.Schema
	attrs := []string{"username", "password", "account", "location", "role", "auth_url", "auth_realm", "auth_client"}
	for _, name := range attrs {
		if _, ok := schema.Attributes[name]; !ok {
			t.Errorf("expected schema attribute %q", name)
		}
	}
	if _, ok := schema.Attributes["password"]; !ok {
		t.Error("expected password attribute")
	}
}

func TestProviderResourcesAndDataSources(t *testing.T) {
	p := New("test")()
	resources := p.Resources(context.Background())
	datasources := p.DataSources(context.Background())
	if len(resources) != 13 {
		t.Errorf("expected 13 resources (dns_zone, dns_record, lb_route, lb_certificate, network, vpc_route, security_group, security_group_rule, instance_group, s3_user, s3_bucket, iscsi_disk, iscsi_client), got %d", len(resources))
	}
	if len(datasources) != 4 {
		t.Errorf("expected 4 data sources (load_balancers, lb_backend_services, instance_template, s3_pools), got %d", len(datasources))
	}
}
