package auth

import (
	"errors"
	"os"
	"strings"
)

const (
	// Environment variables for authentication
	EnvAPIKey   = "CLOUDFLARE_API_KEY"
	EnvAPIEmail = "CLOUDFLARE_EMAIL"
	EnvAPIToken = "CLOUDFLARE_API_TOKEN"
)

// Errors related to authentication
var (
	ErrNoCredentials = errors.New("no Cloudflare API credentials found")
	ErrMissingEmail  = errors.New("API key authentication requires an email, set CLOUDFLARE_API_EMAIL environment variable")
)

// AuthType represents the type of authentication being used
type AuthType int

const (
	AuthTypeUnknown AuthType = iota
	AuthTypeAPIKey
	AuthTypeAPIToken
)

// CredentialInfo contains authentication information
type CredentialInfo struct {
	Type  AuthType
	Email string // Used only for API Key auth
	Key   string // Either API Key or API Token
}

// GetCredentials extracts authentication information from the environment
func GetCredentials() (*CredentialInfo, error) {
	// First check for API Token (preferred method)
	if token := os.Getenv(EnvAPIToken); token != "" {
		return &CredentialInfo{
			Type: AuthTypeAPIToken,
			Key:  token,
		}, nil
	}

	// Then check for API Key
	if key := os.Getenv(EnvAPIKey); key != "" {
		email := os.Getenv(EnvAPIEmail)
		if email == "" {
			return nil, ErrMissingEmail
		}
		return &CredentialInfo{
			Type:  AuthTypeAPIKey,
			Email: email,
			Key:   key,
		}, nil
	}

	return nil, ErrNoCredentials
}

// CheckTokenScope validates if a token error might be due to insufficient permissions
func CheckTokenScope(errorMsg string) string {
	if strings.Contains(strings.ToLower(errorMsg), "token not authorized") {
		return `Your API token may not have the correct permissions. For:
- Cache purging: Use the "Zone.Cache Purge" permission
- KV namespace operations: Use "Account.Workers KV Storage" permission
- KV values operations: Use "Account.Workers KV Storage:Edit" permission`
	}
	return ""
}
