package client

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func newTestClientForComputeAPI(t *testing.T, api http.Handler) (*Client, func()) {
	t.Helper()
	authSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"access_token": "mock-token"})
	}))
	apiSrv := httptest.NewServer(api)
	cfg := &Config{
		Username:  "u",
		Password:  "p",
		Account:   "a",
		Location:  "l",
		Role:      "owner",
		AuthURL:   authSrv.URL,
		AuthRealm: "test",
	}
	c, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	c.baseURL = strings.TrimSuffix(apiSrv.URL, "/")
	return c, func() { authSrv.Close(); apiSrv.Close() }
}

func TestFindServiceTemplateExactFilter(t *testing.T) {
	t.Parallel()
	var gotPath string
	c, cleanup := newTestClientForComputeAPI(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.RequestURI()
		if !strings.Contains(gotPath, "expand=resources") {
			t.Errorf("missing expand=resources in %q", gotPath)
		}
		decoded, err := url.QueryUnescape(gotPath)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(decoded, "filter[]=name='Ubuntu Server:24.04.1-240905'") {
			t.Errorf("expected exact name filter in path, got %q", gotPath)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"resources":[{"id":"81000000000005","name":"Ubuntu Server:24.04.1-240905","guid":"g1"}]}`))
	}))
	defer cleanup()
	ctx := context.Background()
	tpl, err := c.FindServiceTemplate(ctx, "Ubuntu Server", "24.04.1-240905")
	if err != nil {
		t.Fatal(err)
	}
	if tpl.ID != "81000000000005" || tpl.Name != "Ubuntu Server:24.04.1-240905" || tpl.GUID != "g1" {
		t.Fatalf("tpl: %+v", tpl)
	}
	_ = gotPath
}

func TestFindServiceTemplateLIKEFilter(t *testing.T) {
	t.Parallel()
	c, cleanup := newTestClientForComputeAPI(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		decoded, _ := url.QueryUnescape(r.URL.RequestURI())
		if !strings.Contains(decoded, "filter[]=name='%Ubuntu%'") {
			t.Fatalf("unexpected filter in %q", decoded)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"resources":[{"id":"1","name":"Ubuntu Server:24.04.1-240905"}]}`))
	}))
	defer cleanup()
	tpl, err := c.FindServiceTemplate(context.Background(), "Ubuntu", "")
	if err != nil {
		t.Fatal(err)
	}
	if tpl.ID != "1" {
		t.Fatalf("id: %q", tpl.ID)
	}
}

func TestFindServiceTemplateEscapesQuote(t *testing.T) {
	t.Parallel()
	c, cleanup := newTestClientForComputeAPI(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		decoded, _ := url.QueryUnescape(r.URL.RequestURI())
		if !strings.Contains(decoded, "filter[]=name='%a''b%'") {
			t.Fatalf("expected doubled quote in LIKE pattern, got %q", decoded)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"resources":[{"id":"x","name":"x"}]}`))
	}))
	defer cleanup()
	_, err := c.FindServiceTemplate(context.Background(), "a'b", "")
	if err != nil {
		t.Fatal(err)
	}
}

func TestFindServiceTemplateNotFound(t *testing.T) {
	t.Parallel()
	c, cleanup := newTestClientForComputeAPI(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"resources":[]}`))
	}))
	defer cleanup()
	_, err := c.FindServiceTemplate(context.Background(), "Nope", "")
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound wrap, got %v", err)
	}
}

func TestFindServiceTemplateAmbiguous(t *testing.T) {
	t.Parallel()
	c, cleanup := newTestClientForComputeAPI(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"resources":[{"id":"1","name":"Prod:1"},{"id":"2","name":"Prod:2"}]}`))
	}))
	defer cleanup()
	_, err := c.FindServiceTemplate(context.Background(), "Prod", "")
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "multiple service templates matched") {
		t.Fatalf("expected intro sentence: %s", msg)
	}
	if !strings.Contains(msg, `full_name="Prod:1"`) || !strings.Contains(msg, `full_name="Prod:2"`) {
		t.Fatalf("expected listed full names: %s", msg)
	}
	if !strings.Contains(msg, "id=1") || !strings.Contains(msg, "id=2") {
		t.Fatalf("expected listed ids: %s", msg)
	}
	if !strings.Contains(msg, `name="Prod"`) {
		t.Fatalf("expected repeated lookup name: %s", msg)
	}
}
