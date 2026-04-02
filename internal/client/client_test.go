package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewClient(t *testing.T) {
	cfg := &Config{
		Username: "user",
		Password: "pass",
		Account:  "acc",
		Location: "loc",
	}
	c, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if c == nil {
		t.Fatal("client is nil")
	}
	if c.Account() != "acc" {
		t.Errorf("Account() = %q, want acc", c.Account())
	}
	if c.Location() != "loc" {
		t.Errorf("Location() = %q, want loc", c.Location())
	}
	if c.authRealm != defaultAuthRealm {
		t.Errorf("authRealm = %q, want %q", c.authRealm, defaultAuthRealm)
	}
	if c.authClientID != defaultAuthClientID {
		t.Errorf("authClientID = %q, want %q", c.authClientID, defaultAuthClientID)
	}
}

func TestClientDoWithMockAuthAndAPI(t *testing.T) {
	tokenCalled := false
	apiCalled := false
	authSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenCalled = true
		if r.Method != http.MethodPost {
			t.Errorf("auth: method %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"access_token": "mock-token"})
	}))
	defer authSrv.Close()
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiCalled = true
		if auth := r.Header.Get("Authorization"); auth != "Bearer mock-token" {
			t.Errorf("Authorization header = %q", auth)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":"net-1"}`))
	}))
	defer apiSrv.Close()

	cfg := &Config{
		Username:  "u",
		Password:  "p",
		Account:   "a",
		Location:  "l",
		AuthURL:   authSrv.URL,
		AuthRealm: "test",
	}
	c, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	c.baseURL = apiSrv.URL

	ctx := context.Background()
	resp, err := c.Do(ctx, "GET", "/networks", nil)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer resp.Body.Close()
	if !tokenCalled {
		t.Error("auth server was not called")
	}
	if !apiCalled {
		t.Error("API server was not called")
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d", resp.StatusCode)
	}
}

func TestNewClientWithAuthRealmAndClientID(t *testing.T) {
	cfg := &Config{
		Username:     "u",
		Password:     "p",
		Account:      "a",
		Location:     "l",
		AuthURL:      "https://auth.example.com/realms",
		AuthRealm:    "myrealm",
		AuthClientID: "my-client",
	}
	c, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if c.authRealm != "myrealm" {
		t.Errorf("authRealm = %q, want myrealm", c.authRealm)
	}
	if c.authClientID != "my-client" {
		t.Errorf("authClientID = %q, want my-client", c.authClientID)
	}
}
