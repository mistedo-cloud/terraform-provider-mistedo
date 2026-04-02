package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// ServiceTemplate is one row from GET /service_templates (ManageIQ catalog item).
type ServiceTemplate struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	GUID string `json:"guid"`
}

type serviceTemplateCollection struct {
	Resources []ServiceTemplate `json:"resources"`
}

func sqlSingleQuote(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}

// FindServiceTemplate returns the single catalog template that matches name and optional version.
//
// The API stores a full display name as "<name>:<version>" (e.g. "Ubuntu Server:24.04.1-240905").
// When version is non-empty, the filter is an exact match on name:version (case-sensitive).
// When version is empty, the filter uses SQL LIKE: %trimmedName% (also case-sensitive on the platform).
//
// If zero or more than one template matches, an error is returned.
func (c *Client) FindServiceTemplate(ctx context.Context, name, version string) (*ServiceTemplate, error) {
	name = strings.TrimSpace(name)
	version = strings.TrimSpace(version)
	if name == "" {
		return nil, fmt.Errorf("name is empty")
	}
	var filterExpr string
	if version != "" {
		filterExpr = fmt.Sprintf("name='%s'", sqlSingleQuote(name+":"+version))
	} else {
		filterExpr = fmt.Sprintf("name='%s'", sqlSingleQuote("%"+name+"%"))
	}
	q := url.Values{}
	q.Set("expand", "resources")
	q.Add("filter[]", filterExpr)
	path := ComputeAPIPrefix + "/service_templates?" + q.Encode()

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
	var coll serviceTemplateCollection
	if err := json.Unmarshal(b, &coll); err != nil {
		return nil, fmt.Errorf("decode service templates: %w", err)
	}
	switch n := len(coll.Resources); n {
	case 0:
		return nil, fmt.Errorf("no service template matched name=%q version=%q: %w", name, version, ErrNotFound)
	case 1:
		t := coll.Resources[0]
		return &t, nil
	default:
		var b strings.Builder
		fmt.Fprintf(&b, "multiple service templates matched (%d), expected exactly one. Refine argument \"name\" and/or set \"version\" (full catalog names are usually \"name:version\").\n\n", n)
		fmt.Fprintf(&b, "Your lookup: name=%q", name)
		if version != "" {
			fmt.Fprintf(&b, " version=%q", version)
		} else {
			b.WriteString(" (no version — LIKE match on %name%)")
		}
		b.WriteString("\n\nMatching templates:\n")
		for i, r := range coll.Resources {
			fmt.Fprintf(&b, "  %d) id=%s  full_name=%q\n", i+1, r.ID, r.Name)
		}
		return nil, fmt.Errorf("%s", b.String())
	}
}
