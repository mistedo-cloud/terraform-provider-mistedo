package client

// Config holds provider configuration for the Mistedo API client.
// Authentication uses OAuth2 resource owner password flow against the configured auth server.
type Config struct {
	Username     string
	Password     string
	Account      string
	Location     string
	Role         string
	AuthURL      string
	AuthRealm    string
	AuthClientID string
}
