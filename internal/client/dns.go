package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// NormalizeZoneName trims whitespace and removes a single trailing dot from an FQDN.
// The Mistedo DNS API rejects names with a trailing dot and may return HTTP 500 with an empty body.
func NormalizeZoneName(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimSuffix(s, ".")
	return s
}

// Metadata mirrors read-only metadata from the DNS API (OpenAPI).
type Metadata struct {
	Account string `json:"account,omitempty"`
	Owner   string `json:"owner,omitempty"`
	Service bool   `json:"service,omitempty"`
	Zone    bool   `json:"zone,omitempty"`
}

// Zone is a DNS hosted zone (OpenAPI).
type Zone struct {
	Name     string    `json:"name"`
	Account  string    `json:"account,omitempty"`
	Metadata *Metadata `json:"metadata,omitempty"`
}

// ZoneCollection is the list-zones response.
type ZoneCollection struct {
	Count int    `json:"count"`
	Data  []Zone `json:"data"`
}

// Record is a DNS record (OpenAPI).
type Record struct {
	ID       string    `json:"id,omitempty"`
	Name     string    `json:"name"`
	Type     string    `json:"type,omitempty"`
	Data     string    `json:"data,omitempty"`
	Host     string    `json:"host,omitempty"`
	Text     string    `json:"text,omitempty"`
	Value    string    `json:"value,omitempty"`
	Texts    []string  `json:"texts,omitempty"`
	TTL      int       `json:"ttl,omitempty"`
	Priority int       `json:"priority,omitempty"`
	Weight   int       `json:"weight,omitempty"`
	Port     int       `json:"port,omitempty"`
	Group    string    `json:"group,omitempty"`
	Metadata *Metadata `json:"metadata,omitempty"`
}

// RecordRData returns the record payload (RDATA) for Terraform `content`.
// Some responses put A/AAAA in `host`, TXT in `text` / `texts` instead of `data`.
func RecordRData(rec *Record) string {
	if rec == nil {
		return ""
	}
	if s := strings.TrimSpace(rec.Data); s != "" {
		return s
	}
	if len(rec.Texts) > 0 {
		return strings.Join(rec.Texts, "")
	}
	if s := strings.TrimSpace(rec.Text); s != "" {
		return s
	}
	if s := strings.TrimSpace(rec.Value); s != "" {
		return s
	}
	return strings.TrimSpace(rec.Host)
}

type zoneCreateBody struct {
	Zone Zone `json:"zone"`
}

type recordCreateBody struct {
	Record Record `json:"record"`
}

const createRecordFindRetries = 5
const createRecordFindRetryDelay = 250 * time.Millisecond

func readResponseBody(resp *http.Response) ([]byte, error) {
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

// decodeInto unmarshals JSON, accepting either a raw object or API envelopes like {"status":200,"data":{...}}.
func decodeInto(b []byte, target interface{}) error {
	if len(b) == 0 {
		return fmt.Errorf("empty response body")
	}
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(b, &probe); err == nil {
		if raw, ok := probe["data"]; ok && len(raw) > 0 && (raw[0] == '{' || raw[0] == '[') {
			return json.Unmarshal(raw, target)
		}
	}
	return json.Unmarshal(b, target)
}

func decodeRecord(b []byte) (*Record, error) {
	var r Record
	if err := decodeInto(b, &r); err != nil {
		// continue to fallback decoding shapes used by some API gateways
	} else if r.Name != "" || r.ID != "" {
		return &r, nil
	}
	// Fallback: some deployments wrap the record under additional keys, e.g.
	// {"status":200,"data":{"record":{...}}} or return arrays.
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(b, &obj); err == nil {
		candidates := []json.RawMessage{obj["record"], obj["data"]}
		for _, cand := range candidates {
			if len(cand) == 0 {
				continue
			}
			// Direct candidate as record object.
			var one Record
			if err := json.Unmarshal(cand, &one); err == nil && (one.Name != "" || one.ID != "") {
				return &one, nil
			}
			// Nested candidate with "record" key.
			var nested struct {
				Record Record `json:"record"`
			}
			if err := json.Unmarshal(cand, &nested); err == nil && (nested.Record.Name != "" || nested.Record.ID != "") {
				return &nested.Record, nil
			}
			// Candidate as list; choose first meaningful element.
			var many []Record
			if err := json.Unmarshal(cand, &many); err == nil {
				for i := range many {
					if many[i].Name != "" || many[i].ID != "" {
						return &many[i], nil
					}
				}
			}
		}
	}
	return nil, fmt.Errorf("invalid record in response")
}

// CreateZone creates a hosted zone (POST /zones).
func (c *Client) CreateZone(ctx context.Context, name string) (*Zone, error) {
	name = NormalizeZoneName(name)
	if name == "" {
		return nil, fmt.Errorf("zone name is empty")
	}
	account := c.cfg.Account
	if account == "" {
		return nil, fmt.Errorf("account is required to create a zone")
	}
	body := zoneCreateBody{
		Zone: Zone{Name: name, Account: account},
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	resp, err := c.Do(ctx, http.MethodPost, DNSAPIPrefix+"/zones", raw)
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
	// Do not decode JSON when the body is empty (gateways may strip payloads on 200/201).
	if len(b) == 0 {
		return &Zone{Name: name, Account: account}, nil
	}
	z, err := parseSingleZone(b)
	if err != nil {
		return nil, err
	}
	return z, nil
}

// DeleteZone deletes a hosted zone by name.
func (c *Client) DeleteZone(ctx context.Context, zoneName string) error {
	zoneName = NormalizeZoneName(zoneName)
	if zoneName == "" {
		return fmt.Errorf("zone name is empty")
	}
	path := DNSAPIPrefix + "/zones/" + url.PathEscape(zoneName)
	resp, err := c.Do(ctx, http.MethodDelete, path, nil)
	if err != nil {
		return err
	}
	b, err := readResponseBody(resp)
	if err != nil {
		return err
	}
	if resp.StatusCode == http.StatusUnauthorized {
		return NewAPIError(resp, b)
	}
	if resp.StatusCode == http.StatusNotFound {
		return NewAPIError(resp, b)
	}
	if resp.StatusCode != http.StatusNoContent && (resp.StatusCode < 200 || resp.StatusCode >= 300) {
		return NewAPIError(resp, b)
	}
	return nil
}

// GetZone returns a zone by name using GET /zones (list).
func (c *Client) GetZone(ctx context.Context, name string) (*Zone, error) {
	want := NormalizeZoneName(name)
	zones, err := c.ListZones(ctx)
	if err != nil {
		return nil, err
	}
	for i := range zones {
		if NormalizeZoneName(zones[i].Name) == want {
			return &zones[i], nil
		}
	}
	return nil, fmt.Errorf("%w: DNS zone %q", ErrNotFound, want)
}

// ListZones returns all zones for the account.
func (c *Client) ListZones(ctx context.Context) ([]Zone, error) {
	resp, err := c.Do(ctx, http.MethodGet, DNSAPIPrefix+"/zones", nil)
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
	if len(b) == 0 {
		return []Zone{}, nil
	}
	return parseZoneList(b)
}

func parseZoneList(b []byte) ([]Zone, error) {
	if len(b) == 0 {
		return []Zone{}, nil
	}
	var out []Zone
	if len(b) > 0 && b[0] == '{' {
		var coll ZoneCollection
		if err := json.Unmarshal(b, &coll); err == nil {
			if coll.Data != nil {
				out = coll.Data
			} else {
				out = []Zone{}
			}
		}
	}
	if out == nil {
		var direct []Zone
		if err := json.Unmarshal(b, &direct); err != nil {
			return nil, fmt.Errorf("decode zones: %w", err)
		}
		out = direct
	}
	for i := range out {
		out[i].Name = NormalizeZoneName(out[i].Name)
	}
	return out, nil
}

func parseSingleZone(b []byte) (*Zone, error) {
	var z Zone
	if err := decodeInto(b, &z); err != nil {
		return nil, fmt.Errorf("decode zone: %w", err)
	}
	z.Name = NormalizeZoneName(z.Name)
	if z.Name == "" {
		return nil, fmt.Errorf("decode zone: missing name")
	}
	return &z, nil
}

// RecordOwnerNameForAPIPath returns the relative owner name (labels under the zone) used in API paths.
// The DNS API sometimes returns name as a full FQDN or with a leading id prefix; normalize before building [id.]owner.
func RecordOwnerNameForAPIPath(zoneName, id, rawName string) string {
	zoneName = NormalizeZoneName(zoneName)
	n := NormalizeZoneName(strings.TrimSpace(rawName))
	if zoneName != "" {
		sfx := "." + strings.ToLower(zoneName)
		if strings.HasSuffix(strings.ToLower(n), sfx) {
			n = n[:len(n)-len(sfx)]
		}
	}
	if id != "" {
		prefix := id + "."
		if strings.HasPrefix(strings.ToLower(n), strings.ToLower(prefix)) {
			n = n[len(prefix):]
		}
	}
	return n
}

// RecordPathInZone returns the path segment for GET/DELETE records in the form [id.]owner (per API).
func RecordPathInZone(zoneName string, r *Record) string {
	if r == nil {
		return ""
	}
	owner := RecordOwnerNameForAPIPath(zoneName, r.ID, r.Name)
	if r.ID != "" {
		return r.ID + "." + owner
	}
	return owner
}

// CreateRecord creates a DNS record in a zone. If the API returns a full record, it is returned;
// otherwise the caller may list records to resolve the record path.
func (c *Client) CreateRecord(ctx context.Context, zoneName string, rec Record) (*Record, error) {
	zoneName = NormalizeZoneName(zoneName)
	if zoneName == "" {
		return nil, fmt.Errorf("zone name is empty")
	}
	body := recordCreateBody{Record: rec}
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	path := DNSAPIPrefix + "/zones/" + url.PathEscape(zoneName) + "/records"
	resp, err := c.Do(ctx, http.MethodPost, path, raw)
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
	// Do not decode JSON when the body is empty (200/201); 204 is not used for POST create here.
	// Many deployments also return a non-empty body without a full record object (e.g. KrakenD).
	// In those cases resolve the created record by listing the zone.
	if len(b) > 0 {
		out, err := decodeRecord(b)
		if err == nil && (out.Name != "" || out.ID != "") {
			return out, nil
		}
		// Some deployments return only host/group metadata on create.
		if group, ok := parseCreateRecordGroup(b); ok {
			found, findErr := c.findRecordByGroupWithRetry(ctx, zoneName, group)
			if findErr == nil {
				return found, nil
			}
		}
	}
	var lastErr error
	for attempt := 0; attempt < createRecordFindRetries; attempt++ {
		found, findErr := c.FindMatchingRecord(ctx, zoneName, rec.Type, rec.Name, rec.Data)
		if findErr == nil {
			return found, nil
		}
		lastErr = findErr
		// Retry only for "not found yet" to tolerate eventual consistency right after create.
		if !errors.Is(findErr, ErrNotFound) || attempt == createRecordFindRetries-1 {
			break
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(createRecordFindRetryDelay):
		}
	}
	return nil, fmt.Errorf(
		"record create returned HTTP %d (%d-byte body) but response was not a decodable record; listing the zone did not find a matching new record after %d attempts: %v",
		resp.StatusCode, len(b), createRecordFindRetries, lastErr,
	)
}

func parseCreateRecordGroup(b []byte) (string, bool) {
	var wrapped struct {
		Data struct {
			Group string `json:"group"`
		} `json:"data"`
		Group string `json:"group"`
	}
	if err := json.Unmarshal(b, &wrapped); err != nil {
		return "", false
	}
	if wrapped.Data.Group != "" {
		return wrapped.Data.Group, true
	}
	if wrapped.Group != "" {
		return wrapped.Group, true
	}
	return "", false
}

func (c *Client) findRecordByGroupWithRetry(ctx context.Context, zoneName, group string) (*Record, error) {
	var lastErr error
	for attempt := 0; attempt < createRecordFindRetries; attempt++ {
		out, err := c.FindRecordByGroup(ctx, zoneName, group)
		if err == nil {
			return out, nil
		}
		lastErr = err
		if !errors.Is(err, ErrNotFound) || attempt == createRecordFindRetries-1 {
			break
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(createRecordFindRetryDelay):
		}
	}
	return nil, lastErr
}

// ListRecords returns all records in a zone.
func (c *Client) ListRecords(ctx context.Context, zoneName string) ([]Record, error) {
	zoneName = NormalizeZoneName(zoneName)
	if zoneName == "" {
		return nil, fmt.Errorf("zone name is empty")
	}
	path := DNSAPIPrefix + "/zones/" + url.PathEscape(zoneName) + "/records"
	resp, err := c.Do(ctx, http.MethodGet, path, nil)
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
	if len(b) == 0 {
		return []Record{}, nil
	}
	return parseRecordList(b)
}

func parseRecordList(b []byte) ([]Record, error) {
	if len(b) == 0 {
		return []Record{}, nil
	}
	if len(b) > 0 && b[0] == '{' {
		var wrapped struct {
			Data []Record `json:"data"`
		}
		if err := json.Unmarshal(b, &wrapped); err == nil {
			if wrapped.Data != nil {
				return wrapped.Data, nil
			}
			return []Record{}, nil
		}
	}
	var direct []Record
	if err := json.Unmarshal(b, &direct); err != nil {
		return nil, fmt.Errorf("decode records list: %w", err)
	}
	return direct, nil
}

// GetRecord fetches a single record by zone and record path segment.
func (c *Client) GetRecord(ctx context.Context, zoneName, recordPath string) (*Record, error) {
	zoneName = NormalizeZoneName(zoneName)
	if zoneName == "" {
		return nil, fmt.Errorf("zone name is empty")
	}
	path := DNSAPIPrefix + "/zones/" + url.PathEscape(zoneName) + "/records/" + url.PathEscape(recordPath)
	resp, err := c.Do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	b, err := readResponseBody(resp)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("%w: DNS record %q in zone %q", ErrNotFound, recordPath, zoneName)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, NewAPIError(resp, b)
	}
	if len(b) == 0 {
		return nil, fmt.Errorf("GET record returned HTTP %d with empty body", resp.StatusCode)
	}
	out, err := decodeRecord(b)
	if err != nil {
		return nil, fmt.Errorf("decode record: %w", err)
	}
	return out, nil
}

// DeleteRecord deletes a record by zone and record path segment.
func (c *Client) DeleteRecord(ctx context.Context, zoneName, recordPath string) error {
	zoneName = NormalizeZoneName(zoneName)
	if zoneName == "" {
		return fmt.Errorf("zone name is empty")
	}
	path := DNSAPIPrefix + "/zones/" + url.PathEscape(zoneName) + "/records/" + url.PathEscape(recordPath)
	resp, err := c.Do(ctx, http.MethodDelete, path, nil)
	if err != nil {
		return err
	}
	b, err := readResponseBody(resp)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusNoContent && (resp.StatusCode < 200 || resp.StatusCode >= 300) {
		return NewAPIError(resp, b)
	}
	return nil
}

// FindMatchingRecord lists records and returns the first matching type, name, and content (API data).
func (c *Client) FindMatchingRecord(ctx context.Context, zoneName, recordType, name, content string) (*Record, error) {
	zoneName = NormalizeZoneName(zoneName)
	if zoneName == "" {
		return nil, fmt.Errorf("zone name is empty")
	}
	list, err := c.ListRecords(ctx, zoneName)
	if err != nil {
		return nil, err
	}
	t := strings.ToUpper(strings.TrimSpace(recordType))
	wantName := canonicalRecordName(strings.TrimSpace(name), zoneName)
	wantContent := canonicalRecordData(t, content)
	for i := range list {
		r := &list[i]
		gotType := strings.ToUpper(strings.TrimSpace(r.Type))
		gotName := canonicalRecordName(strings.TrimSpace(r.Name), zoneName)
		gotContent := canonicalRecordData(gotType, RecordRData(r))
		if gotType == t && strings.EqualFold(gotName, wantName) && recordDataEqual(t, gotContent, wantContent) {
			return r, nil
		}
	}
	return nil, fmt.Errorf("%w: no record matching type %s, name %q, content %q in zone %q", ErrNotFound, t, name, content, zoneName)
}

// FindRecordByGroup returns the first record with an exact API group value.
func (c *Client) FindRecordByGroup(ctx context.Context, zoneName, group string) (*Record, error) {
	zoneName = NormalizeZoneName(zoneName)
	group = strings.TrimSpace(group)
	if zoneName == "" {
		return nil, fmt.Errorf("zone name is empty")
	}
	if group == "" {
		return nil, fmt.Errorf("group is empty")
	}
	list, err := c.ListRecords(ctx, zoneName)
	if err != nil {
		return nil, err
	}
	for i := range list {
		if strings.EqualFold(strings.TrimSpace(list[i].Group), group) {
			return &list[i], nil
		}
	}
	return nil, fmt.Errorf("%w: no record with group %q in zone %q", ErrNotFound, group, zoneName)
}

func canonicalRecordData(recordType, data string) string {
	v := strings.TrimSpace(data)
	switch strings.ToUpper(strings.TrimSpace(recordType)) {
	case "CNAME", "NS", "PTR":
		// API may normalize target FQDN by removing a trailing dot.
		return strings.TrimSuffix(v, ".")
	case "TXT":
		return unquoteTXT(v)
	default:
		return v
	}
}

// CanonicalRecordContent normalizes RDATA for Terraform state (same rules as FindMatchingRecord).
func CanonicalRecordContent(recordType, raw string) string {
	return canonicalRecordData(recordType, raw)
}

func canonicalRecordName(name, zoneName string) string {
	n := NormalizeZoneName(name)
	z := NormalizeZoneName(zoneName)
	if z == "" {
		return n
	}
	sfx := "." + z
	if strings.HasSuffix(strings.ToLower(n), strings.ToLower(sfx)) {
		return n[:len(n)-len(sfx)]
	}
	return n
}

func recordDataEqual(recordType, got, want string) bool {
	switch strings.ToUpper(strings.TrimSpace(recordType)) {
	case "CNAME", "NS", "PTR":
		return strings.EqualFold(got, want)
	case "TXT":
		return got == want
	default:
		return got == want
	}
}

func unquoteTXT(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}

