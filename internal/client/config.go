package client

// Config holds provider configuration for the Mistedo API client.
// Authentication is done via Keycloak (OAuth2 resource owner password flow).
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
