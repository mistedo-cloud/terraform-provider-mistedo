package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const defaultAuthURL = "https://auth.mistedo.by/realms"
const defaultAuthRealm = "master"
const defaultAuthClientID = "cloud-console"

// DNSAPIPrefix is the path prefix for the DNS API (see OpenAPI dns.yml).
const DNSAPIPrefix = "/api/dns/v1"

// Token refresh before expiry (seconds). Request new token before the current one expires.
const tokenRefreshBeforeExpiry = 60

// Client is the Mistedo API client.
//
// Token storage: the client caches the JWT in memory for the lifetime of the Client instance.
// Terraform runs the provider in a separate process for each operation (plan, apply, etc.),
// so the token is naturally scoped to one run and reused for all API calls in that run.
// When the token expires (using expires_in from the auth response) or is missing,
// a new token is requested. No disk persistence is used by default to avoid storing
// credentials; each new provider process obtains a fresh token when needed.
type Client struct {
	cfg          *Config
	http         *http.Client
	baseURL      string
	authURL      string
	authRealm    string
	authClientID string
	token        string
	tokenExp     time.Time // when the token expires (zero = unknown, treat as expired)
	mu           sync.Mutex
	// Cached ManageIQ "Red Hat NetworkManager" provider id (same for a site).
	muManageIQ   sync.Mutex
	manageIQProv string

	// Serializes AddFirewallRule for the same security group so concurrent creates
	// (e.g. multiple mistedo_security_group_rule resources) cannot pick the same "new" rule.
	muRuleAdd sync.Mutex
	ruleAddMu map[string]*sync.Mutex
}

// New creates a new Mistedo API client with OAuth2 password-grant authentication.
func New(cfg *Config) (*Client, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}
	if strings.TrimSpace(cfg.Location) == "" {
		return nil, fmt.Errorf("location is required")
	}
	authURL := strings.TrimSuffix(cfg.AuthURL, "/")
	if authURL == "" {
		authURL = defaultAuthURL
	}
	// Token URL is {base}/realms/{realm}/protocol/openid-connect/token
	if !strings.Contains(authURL, "/realms") {
		authURL = authURL + "/realms"
	}
	realm := cfg.AuthRealm
	if realm == "" {
		realm = defaultAuthRealm
	}
	clientID := cfg.AuthClientID
	if clientID == "" {
		clientID = defaultAuthClientID
	}
	loc := strings.TrimSpace(cfg.Location)
	baseURL := fmt.Sprintf("https://api.%s.mistedo.by", loc)
	return &Client{
		cfg:          cfg,
		http:         &http.Client{Timeout: 30 * time.Second},
		baseURL:      baseURL,
		authURL:      authURL,
		authRealm:    realm,
		authClientID: clientID,
	}, nil
}

// getToken returns a valid JWT, from cache or by requesting a new one.
// Token is cached in memory and refreshed when expired (or shortly before, see tokenRefreshBeforeExpiry).
func (c *Client) getToken(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now()
	if c.token != "" && (c.tokenExp.IsZero() || now.Add(tokenRefreshBeforeExpiry*time.Second).Before(c.tokenExp)) {
		return c.token, nil
	}
	token, exp, err := c.fetchToken(ctx)
	if err != nil {
		return "", err
	}
	c.token = token
	c.tokenExp = exp
	return c.token, nil
}

// fetchToken requests a new token from the auth server. Returns token and expiry time (zero if unknown).
func (c *Client) fetchToken(ctx context.Context) (token string, expiresAt time.Time, err error) {
	tokenURL := c.authURL
	tokenURL = tokenURL + "/" + c.authRealm + "/protocol/openid-connect/token"
	form := url.Values{}
	form.Set("grant_type", "password")
	form.Set("client_id", c.authClientID)
	form.Set("username", c.cfg.Username)
	form.Set("password", c.cfg.Password)
	body := form.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, bytes.NewBufferString(body))
	if err != nil {
		return "", time.Time{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.http.Do(req)
	if err != nil {
		return "", time.Time{}, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", time.Time{}, fmt.Errorf("auth failed %d: %s", resp.StatusCode, string(raw))
	}
	var out struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"` // seconds until expiry
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", time.Time{}, err
	}
	exp := time.Time{}
	if out.ExpiresIn > 0 {
		exp = time.Now().Add(time.Duration(out.ExpiresIn) * time.Second)
	}
	return out.AccessToken, exp, nil
}

// EnsureToken fetches and caches a JWT from the auth server. Call this during provider Configure
// to validate credentials and fail fast at plan time if auth_url or credentials are wrong.
func (c *Client) EnsureToken(ctx context.Context) error {
	_, err := c.getToken(ctx)
	return err
}

// InvalidateToken clears the cached token so the next API call will request a new one.
// Call this after receiving 401 Unauthorized to force a token refresh.
func (c *Client) InvalidateToken() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.token = ""
	c.tokenExp = time.Time{}
}

// Do performs an authenticated request to the regional API (base URL https://api.<location>.mistedo.by).
// It sets account/role headers expected by Mistedo services.
func (c *Client) Do(ctx context.Context, method, path string, body []byte) (*http.Response, error) {
	return c.doOnce(ctx, method, path, body, true, false)
}

// DoManageIQ is like Do but also sets x-miq-group (required by ManageIQ-backed compute APIs such as security groups).
// Format: "<account>.<role>" — must match the configured provider account and role.
func (c *Client) DoManageIQ(ctx context.Context, method, path string, body []byte) (*http.Response, error) {
	return c.doOnce(ctx, method, path, body, true, true)
}

func (c *Client) doOnce(ctx context.Context, method, path string, body []byte, allowRefresh, manageIQ bool) (*http.Response, error) {
	token, err := c.getToken(ctx)
	if err != nil {
		return nil, err
	}
	fullURL := c.baseURL + path
	var req *http.Request
	if body != nil {
		req, err = http.NewRequestWithContext(ctx, method, fullURL, bytes.NewReader(body))
	} else {
		req, err = http.NewRequestWithContext(ctx, method, fullURL, nil)
	}
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.cfg.Account != "" {
		req.Header.Set("x-auth-account", c.cfg.Account)
	}
	if c.cfg.Role != "" {
		req.Header.Set("x-auth-role", c.cfg.Role)
	}
	if c.cfg.Account != "" && c.cfg.Role != "" {
		g := c.cfg.Account + "." + c.cfg.Role
		req.Header.Set("x-auth-group", g)
		if manageIQ {
			req.Header.Set("x-miq-group", g)
		}
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusUnauthorized && allowRefresh {
		_ = resp.Body.Close()
		c.InvalidateToken()
		return c.doOnce(ctx, method, path, body, false, manageIQ)
	}
	return resp, nil
}

// Account returns the configured account.
func (c *Client) Account() string { return c.cfg.Account }

// Location returns the configured location.
func (c *Client) Location() string { return c.cfg.Location }

// Role returns the configured API role.
func (c *Client) Role() string { return c.cfg.Role }
