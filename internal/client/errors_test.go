package client

import (
	"net/http"
	"strings"
	"testing"
)

func TestParseErrorBody(t *testing.T) {
	if ParseErrorBody(nil) != "" || ParseErrorBody([]byte{}) != "" {
		t.Fatal("ParseErrorBody must not parse empty body")
	}
	tests := []struct {
		body    string
		wantMsg string
	}{
		{`{"message":"not found"}`, "not found"},
		{`{"error":"bad request"}`, "bad request"},
		{`{"message":"m"}`, "m"},
		{`{"detail":"validation failed"}`, "validation failed"},
		{`{"reason":"denied"}`, "denied"},
		{`{"message":"Zone creation failed","status":422,"code":"END010","errors":["Zone name already exists"]}`, "Zone creation failed | Zone name already exists | code=END010"},
		{`{"errors":["first","second"]}`, "first; second"},
		{`{}`, ""},
		{`invalid`, ""},
	}
	for _, tt := range tests {
		got := ParseErrorBody([]byte(tt.body))
		if got != tt.wantMsg {
			t.Errorf("ParseErrorBody(%q) = %q, want %q", tt.body, got, tt.wantMsg)
		}
	}
}

func TestAPIErrorError(t *testing.T) {
	e := &APIError{StatusCode: 404, Message: "not found"}
	s := e.Error()
	if s != "API error 404: not found" {
		t.Errorf("Error() = %q", s)
	}
	e2 := &APIError{StatusCode: 500, Body: []byte("internal")}
	s2 := e2.Error()
	if s2 != "API error 500: internal" {
		t.Errorf("Error() = %q", s2)
	}
}

func TestNewAPIError(t *testing.T) {
	resp := &http.Response{StatusCode: 400}
	body := []byte(`{"message":"bad"}`)
	e := NewAPIError(resp, body)
	if e.StatusCode != 400 {
		t.Errorf("StatusCode = %d", e.StatusCode)
	}
	if e.Message != "bad" {
		t.Errorf("Message = %q", e.Message)
	}
}

func TestAPIErrorUserFacing(t *testing.T) {
	e := &APIError{StatusCode: 404, Message: "zone missing"}
	if e.UserFacing() != "zone missing" {
		t.Fatal(e.UserFacing())
	}
	e2 := &APIError{StatusCode: 500, Body: []byte("short")}
	if e2.UserFacing() != "short" {
		t.Fatal(e2.UserFacing())
	}
	e3 := &APIError{StatusCode: 503, Body: []byte{}}
	if e3.UserFacing() == "" {
		t.Fatal("expected status text")
	}
	e4 := &APIError{StatusCode: 500, Body: []byte{}}
	uf := e4.UserFacing()
	if !strings.Contains(uf, "empty response body") {
		t.Fatalf("expected hint for empty 5xx body, got %q", uf)
	}
}
