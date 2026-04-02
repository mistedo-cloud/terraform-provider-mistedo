package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDoFailsWhenAuthReturns401(t *testing.T) {
	authSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":"invalid_grant"}`))
	}))
	defer authSrv.Close()

	cfg := &Config{
		Username:  "u",
		Password:  "p",
		Account:   "a",
		Location:  "l",
		AuthURL:   authSrv.URL,
		AuthRealm: "r",
	}
	c, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	c.baseURL = "http://example.com"
	ctx := context.Background()
	_, err = c.Do(ctx, "GET", "/", nil)
	if err == nil {
		t.Fatal("expected error from Do when auth returns 401")
	}
}

func TestDoSucceedsAfterTokenFetch(t *testing.T) {
	authSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"access_token":"secret123"}`))
	}))
	defer authSrv.Close()
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer secret123" {
			t.Error("missing or wrong Authorization header")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer apiSrv.Close()

	cfg := &Config{
		Username:  "u",
		Password:  "p",
		Account:   "a",
		Location:  "l",
		AuthURL:   authSrv.URL,
		AuthRealm: "r",
	}
	c, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	c.baseURL = apiSrv.URL
	ctx := context.Background()
	resp, err := c.Do(ctx, "GET", "/", nil)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d", resp.StatusCode)
	}
}
