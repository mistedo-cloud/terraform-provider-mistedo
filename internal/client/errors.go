package client

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

// ErrNotFound indicates the requested resource does not exist (logical or HTTP 404).
var ErrNotFound = errors.New("resource not found")

// APIError represents an error response from the Mistedo API.
type APIError struct {
	StatusCode int
	Body       []byte
	Message    string
}

func (e *APIError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("API error %d: %s", e.StatusCode, e.Message)
	}
	return fmt.Sprintf("API error %d: %s", e.StatusCode, string(e.Body))
}

// UserFacing returns a concise message suitable for Terraform diagnostics (no raw body dump).
func (e *APIError) UserFacing() string {
	if e.Message != "" {
		return e.Message
	}
	if len(e.Body) > 0 && len(e.Body) < 512 {
		return string(e.Body)
	}
	msg := http.StatusText(e.StatusCode)
	if msg == "" {
		msg = "error"
	}
	if len(e.Body) == 0 && e.StatusCode >= 400 {
		return msg + " (empty response body; the API gateway often omits details for 4xx/5xx)"
	}
	return msg
}

// ParseErrorBody attempts to extract a message from JSON error body.
func ParseErrorBody(body []byte) string {
	if len(body) == 0 {
		return ""
	}
	var v struct {
		Message string `json:"message"`
		Error   string `json:"error"`
		Detail  string `json:"detail"`
		Reason  string `json:"reason"`
		Code    string `json:"code"`
		Errors  []string
	}
	if err := json.Unmarshal(body, &v); err != nil {
		return ""
	}
	var parts []string
	for _, s := range []string{v.Message, v.Detail, v.Reason, v.Error} {
		if s != "" {
			parts = append(parts, s)
			break
		}
	}
	if len(v.Errors) > 0 {
		details := strings.Join(v.Errors, "; ")
		if details != "" {
			parts = append(parts, details)
		}
	}
	if v.Code != "" {
		parts = append(parts, "code="+v.Code)
	}
	return strings.Join(parts, " | ")
}

// NewAPIError builds an APIError from an HTTP response.
func NewAPIError(resp *http.Response, body []byte) *APIError {
	return &APIError{
		StatusCode: resp.StatusCode,
		Body:       body,
		Message:    ParseErrorBody(body),
	}
}
