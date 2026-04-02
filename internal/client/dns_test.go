package client

import (
	"encoding/json"
	"testing"
)

func TestDecodeInto_envelope(t *testing.T) {
	raw := []byte(`{"status":200,"data":{"name":"z.example.com.","metadata":{"account":"a1"}}}`)
	var z Zone
	if err := decodeInto(raw, &z); err != nil {
		t.Fatal(err)
	}
	if z.Name != "z.example.com." {
		t.Fatalf("name = %q", z.Name)
	}
	if z.Metadata == nil || z.Metadata.Account != "a1" {
		t.Fatalf("metadata: %+v", z.Metadata)
	}
}

func TestDecodeInto_plain(t *testing.T) {
	raw := []byte(`{"name":"plain.example.com.","account":"a1"}`)
	var z Zone
	if err := decodeInto(raw, &z); err != nil {
		t.Fatal(err)
	}
	if z.Name != "plain.example.com." {
		t.Fatalf("name = %q", z.Name)
	}
}

func TestParseZoneList_emptyBody(t *testing.T) {
	zones, err := parseZoneList(nil)
	if err != nil || len(zones) != 0 {
		t.Fatalf("parseZoneList(nil) = %v %v", zones, err)
	}
	zones, err = parseZoneList([]byte{})
	if err != nil || len(zones) != 0 {
		t.Fatalf("parseZoneList(empty) = %v %v", zones, err)
	}
}

func TestParseRecordList_emptyBody(t *testing.T) {
	recs, err := parseRecordList(nil)
	if err != nil || len(recs) != 0 {
		t.Fatalf("parseRecordList(nil) = %v %v", recs, err)
	}
}

func TestParseZoneList_envelopeArray(t *testing.T) {
	raw := []byte(`{"status":200,"data":[{"name":"a.example.com."},{"name":"b.example.com."}]}`)
	zones, err := parseZoneList(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(zones) != 2 {
		t.Fatalf("len = %d", len(zones))
	}
}

func TestParseRecordList_envelope(t *testing.T) {
	raw := []byte(`{"status":200,"data":[{"id":"x1","name":"www","type":"A","data":"1.2.3.4","ttl":300}]}`)
	recs, err := parseRecordList(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 1 || recs[0].Name != "www" {
		t.Fatalf("%+v", recs)
	}
}

func TestDecodeRecord(t *testing.T) {
	b := []byte(`{"status":200,"data":{"id":"x1","name":"ns","type":"NS","data":"1.2.3.4","ttl":1800}}`)
	r, err := decodeRecord(b)
	if err != nil {
		t.Fatal(err)
	}
	if r.ID != "x1" || r.Type != "NS" {
		t.Fatalf("%+v", r)
	}
}

func TestDecodeRecord_nestedRecordEnvelope(t *testing.T) {
	b := []byte(`{"status":200,"data":{"record":{"id":"x9","name":"ns-deleg-demo","type":"NS","data":"ns.example.net"}}}`)
	r, err := decodeRecord(b)
	if err != nil {
		t.Fatal(err)
	}
	if r.ID != "x9" || r.Name != "ns-deleg-demo" {
		t.Fatalf("%+v", r)
	}
}

func TestRecordPathInZone(t *testing.T) {
	zone := "z.example.com"
	if got := RecordPathInZone(zone, &Record{ID: "x1", Name: "ns"}); got != "x1.ns" {
		t.Fatalf("got %q", got)
	}
	if got := RecordPathInZone(zone, &Record{Name: "apex"}); got != "apex" {
		t.Fatalf("got %q", got)
	}
	// API may return owner as FQDN under the zone.
	if got := RecordPathInZone(zone, &Record{ID: "x1", Name: "www.z.example.com"}); got != "x1.www" {
		t.Fatalf("got %q", got)
	}
	// Duplicate id prefix in name (some responses).
	if got := RecordPathInZone(zone, &Record{ID: "x1", Name: "x1.www.z.example.com"}); got != "x1.www" {
		t.Fatalf("got %q", got)
	}
}

func TestRecordOwnerNameForAPIPath(t *testing.T) {
	zone := "tf-e2e.ha001.dev.mistedo.by"
	if got := RecordOwnerNameForAPIPath(zone, "x1", "a-demo.tf-e2e.ha001.dev.mistedo.by"); got != "a-demo" {
		t.Fatalf("got %q", got)
	}
	if got := RecordOwnerNameForAPIPath(zone, "x1", "x1.a-demo.tf-e2e.ha001.dev.mistedo.by"); got != "a-demo" {
		t.Fatalf("got %q", got)
	}
}

func TestRecordRData(t *testing.T) {
	if got := RecordRData(&Record{Data: "plain"}); got != "plain" {
		t.Fatalf("got %q", got)
	}
	if got := RecordRData(&Record{Text: "from-text"}); got != "from-text" {
		t.Fatalf("got %q", got)
	}
	if got := RecordRData(&Record{Texts: []string{"a", "b"}}); got != "ab" {
		t.Fatalf("got %q", got)
	}
	if got := RecordRData(&Record{Host: "192.0.2.1"}); got != "192.0.2.1" {
		t.Fatalf("got %q", got)
	}
}

func TestDecodeInto_mapProbeInvalidJSON(t *testing.T) {
	var z Zone
	if err := decodeInto([]byte(`not json`), &z); err == nil {
		t.Fatal("expected error")
	}
}

func TestParseSingleZone(t *testing.T) {
	b, _ := json.Marshal(Zone{Name: "z.test.", Account: "acc"})
	z, err := parseSingleZone(b)
	if err != nil || z.Name != "z.test" {
		t.Fatalf("err=%v z=%+v", err, z)
	}
}

func TestNormalizeZoneName(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"example.com.", "example.com"},
		{"example.com", "example.com"},
		{"  z.example.com.  ", "z.example.com"},
		{"", ""},
	}
	for _, tt := range tests {
		if got := NormalizeZoneName(tt.in); got != tt.want {
			t.Errorf("NormalizeZoneName(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestCanonicalRecordData(t *testing.T) {
	tests := []struct {
		recordType string
		in         string
		want       string
	}{
		{recordType: "NS", in: "ns1.example.com.", want: "ns1.example.com"},
		{recordType: "CNAME", in: " target.example.com. ", want: "target.example.com"},
		{recordType: "PTR", in: "ptr.example.com.", want: "ptr.example.com"},
		{recordType: "A", in: " 1.2.3.4 ", want: "1.2.3.4"},
		{recordType: "TXT", in: "\"value.\"", want: "value."},
	}
	for _, tt := range tests {
		if got := canonicalRecordData(tt.recordType, tt.in); got != tt.want {
			t.Errorf("canonicalRecordData(%q, %q) = %q, want %q", tt.recordType, tt.in, got, tt.want)
		}
	}
}

func TestCanonicalRecordName(t *testing.T) {
	zone := "tf-e2e.ha001.dev.mistedo.by"
	tests := []struct {
		in   string
		want string
	}{
		{in: "ns-deleg-demo", want: "ns-deleg-demo"},
		{in: "ns-deleg-demo.tf-e2e.ha001.dev.mistedo.by", want: "ns-deleg-demo"},
		{in: "ns-deleg-demo.tf-e2e.ha001.dev.mistedo.by.", want: "ns-deleg-demo"},
	}
	for _, tt := range tests {
		if got := canonicalRecordName(tt.in, zone); got != tt.want {
			t.Errorf("canonicalRecordName(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestParseCreateRecordGroup(t *testing.T) {
	g, ok := parseCreateRecordGroup([]byte(`{"status":200,"data":{"group":"ns-demo.example.com","host":"ns.example.net"}}`))
	if !ok || g != "ns-demo.example.com" {
		t.Fatalf("group parse failed: ok=%v g=%q", ok, g)
	}
}

