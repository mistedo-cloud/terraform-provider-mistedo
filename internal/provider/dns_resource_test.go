package provider

import (
	"testing"

	"github.com/mistedo/terraform-provider-mistedo/internal/client"
)

func TestParseRecordID(t *testing.T) {
	z, rp, ok := parseRecordID("ha001.at.dev.mistedo.by/x1.ns.dns")
	if !ok || z != "ha001.at.dev.mistedo.by" || rp != "x1.ns.dns" {
		t.Fatalf("got %q %q %v", z, rp, ok)
	}
	if _, _, ok := parseRecordID("no-slash"); ok {
		t.Fatal("expected false")
	}
	if _, _, ok := parseRecordID("/onlysecond"); ok {
		t.Fatal("expected false")
	}
}

func TestComposeRecordID(t *testing.T) {
	id := composeRecordID("z.example.com", "x1.www")
	if id != "z.example.com/x1.www" {
		t.Fatal(id)
	}
}

func TestRecordModelFromAPI_UsesCanonicalType(t *testing.T) {
	rec := &client.Record{
		ID:   "x1",
		Name: "ns-deleg-demo",
		Type: "cname",
		Data: "ns.example.net",
		TTL:  300,
	}
	st := recordModelFromAPI("z.example.com", "x1.ns-deleg-demo", rec, "NS")
	if st.Type.ValueString() != "NS" {
		t.Fatalf("type = %q", st.Type.ValueString())
	}
}

func TestRecordModelFromAPI_NormalizesNameAndContent(t *testing.T) {
	rec := &client.Record{
		Name: "x1.a-demo.tf-e2e.ha001.dev.mistedo.by",
		Host: "192.0.2.10",
		TTL:  300,
	}
	st := recordModelFromAPI("tf-e2e.ha001.dev.mistedo.by", "x1.a-demo", rec, "A")
	if st.Name.ValueString() != "a-demo" {
		t.Fatalf("name = %q", st.Name.ValueString())
	}
	if st.Content.ValueString() != "192.0.2.10" {
		t.Fatalf("content = %q", st.Content.ValueString())
	}
	if st.RecordPath.ValueString() != "x1.a-demo" {
		t.Fatalf("record_path = %q", st.RecordPath.ValueString())
	}
}

func TestRecordModelFromAPI_TXTContentFromTextField(t *testing.T) {
	rec := &client.Record{
		ID:   "x1",
		Name: "txt-demo",
		Type: "txt",
		Text: `"terraform-e2e=ok"`,
		TTL:  300,
	}
	st := recordModelFromAPI("tf-e2e.ha001.dev.mistedo.by", "x1.txt-demo", rec, "TXT")
	if st.Content.ValueString() != "terraform-e2e=ok" {
		t.Fatalf("content = %q", st.Content.ValueString())
	}
}
