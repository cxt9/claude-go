package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/cxt9/claude-go/internal/vault"
)

const (
	// OAuth endpoints (placeholder - would use actual Claude.ai OAuth endpoints)
	authorizationEndpoint = "https://claude.ai/oauth/authorize"
	tokenEndpoint         = "https://claude.ai/oauth/token"
	clientID              = "claude-code-go"
	redirectURI           = "http://localhost:9876/callback"
)

// Provider represents an authentication provider
type Provider string

const (
	ProviderClaudeAI Provider = "claudeai"
	ProviderConsole  Provider = "console"
	ProviderBedrock  Provider = "bedrock"
	ProviderVertex   Provider = "vertex"
)

// Authenticator handles OAuth and API key authentication
type Authenticator struct {
	vault *vault.Vault
}

// NewAuthenticator creates a new authenticator
func NewAuthenticator(v *vault.Vault) *Authenticator {
	return &Authenticator{vault: v}
}

// OAuthFlowData contains the data needed to complete an OAuth flow
type OAuthFlowData struct {
	AuthURL      string
	State        string
	CodeVerifier string
}

// StartOAuthFlow initiates the OAuth flow and returns the authorization URL and flow data
func (a *Authenticator) StartOAuthFlow(ctx context.Context) (*OAuthFlowData, error) {
	// Generate state for CSRF protection
	state, err := generateRandomString(32)
	if err != nil {
		return nil, fmt.Errorf("failed to generate state: %w", err)
	}

	// Generate PKCE code verifier (43-128 chars, URL-safe)
	codeVerifier, err := generateRandomString(64)
	if err != nil {
		return nil, fmt.Errorf("failed to generate code verifier: %w", err)
	}

	// Generate S256 code challenge: BASE64URL(SHA256(code_verifier))
	codeChallenge := generateS256Challenge(codeVerifier)

	// Build authorization URL
	params := url.Values{
		"client_id":             {clientID},
		"redirect_uri":          {redirectURI},
		"response_type":         {"code"},
		"state":                 {state},
		"code_challenge":        {codeChallenge},
		"code_challenge_method": {"S256"},
		"scope":                 {"claude:read claude:write"},
	}

	authURL := fmt.Sprintf("%s?%s", authorizationEndpoint, params.Encode())

	return &OAuthFlowData{
		AuthURL:      authURL,
		State:        state,
		CodeVerifier: codeVerifier,
	}, nil
}

// generateS256Challenge creates a PKCE S256 code challenge from a verifier
func generateS256Challenge(verifier string) string {
	hash := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(hash[:])
}

// CompleteOAuthFlow exchanges the authorization code for tokens
func (a *Authenticator) CompleteOAuthFlow(ctx context.Context, code string, codeVerifier string) error {
	// Exchange code for tokens
	tokens, err := a.exchangeCodeForTokens(ctx, code, codeVerifier)
	if err != nil {
		return fmt.Errorf("token exchange failed: %w", err)
	}

	// Store tokens in vault
	oauthData := vault.OAuthData{
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		TokenType:    tokens.TokenType,
		ExpiresAt:    time.Now().Add(time.Duration(tokens.ExpiresIn) * time.Second),
		Scope:        tokens.Scope,
	}

	data, err := json.Marshal(oauthData)
	if err != nil {
		return fmt.Errorf("failed to serialize tokens: %w", err)
	}

	entry := &vault.Entry{
		ID:       "auth/claudeai",
		Type:     vault.CredentialOAuth,
		Provider: string(ProviderClaudeAI),
		Data:     data,
	}

	if err := a.vault.SetEntry(entry); err != nil {
		return fmt.Errorf("failed to store tokens: %w", err)
	}

	return nil
}

// SetAPIKey stores an API key in the vault
func (a *Authenticator) SetAPIKey(provider Provider, apiKey string) error {
	apiKeyData := vault.APIKeyData{
		APIKey: apiKey,
	}

	data, err := json.Marshal(apiKeyData)
	if err != nil {
		return fmt.Errorf("failed to serialize API key: %w", err)
	}

	entry := &vault.Entry{
		ID:       fmt.Sprintf("auth/%s", provider),
		Type:     vault.CredentialAPIKey,
		Provider: string(provider),
		Data:     data,
	}

	if err := a.vault.SetEntry(entry); err != nil {
		return fmt.Errorf("failed to store API key: %w", err)
	}

	return nil
}

// GetCredential retrieves credentials for the given provider
func (a *Authenticator) GetCredential(provider Provider) (string, error) {
	entry, err := a.vault.GetEntry(fmt.Sprintf("auth/%s", provider))
	if err != nil {
		return "", err
	}

	switch entry.Type {
	case vault.CredentialOAuth:
		var oauthData vault.OAuthData
		if err := json.Unmarshal(entry.Data, &oauthData); err != nil {
			return "", fmt.Errorf("failed to parse OAuth data: %w", err)
		}

		// Check if token needs refresh
		if time.Now().After(oauthData.ExpiresAt.Add(-5 * time.Minute)) {
			if err := a.refreshToken(provider, oauthData.RefreshToken); err != nil {
				return "", fmt.Errorf("token refresh failed: %w", err)
			}
			// Re-read the updated entry
			entry, _ = a.vault.GetEntry(fmt.Sprintf("auth/%s", provider))
			json.Unmarshal(entry.Data, &oauthData)
		}

		return oauthData.AccessToken, nil

	case vault.CredentialAPIKey:
		var apiKeyData vault.APIKeyData
		if err := json.Unmarshal(entry.Data, &apiKeyData); err != nil {
			return "", fmt.Errorf("failed to parse API key data: %w", err)
		}
		return apiKeyData.APIKey, nil

	default:
		return "", fmt.Errorf("unknown credential type: %s", entry.Type)
	}
}

// HasCredential checks if credentials exist for the given provider
func (a *Authenticator) HasCredential(provider Provider) bool {
	_, err := a.vault.GetEntry(fmt.Sprintf("auth/%s", provider))
	return err == nil
}

// ListProviders returns all configured authentication providers
func (a *Authenticator) ListProviders() ([]Provider, error) {
	entries, err := a.vault.ListEntries()
	if err != nil {
		return nil, err
	}

	var providers []Provider
	for _, entry := range entries {
		if entry.Type == vault.CredentialOAuth || entry.Type == vault.CredentialAPIKey {
			providers = append(providers, Provider(entry.Provider))
		}
	}

	return providers, nil
}

// TokenResponse represents an OAuth token response
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	Scope        string `json:"scope"`
}

func (a *Authenticator) exchangeCodeForTokens(ctx context.Context, code string, codeVerifier string) (*TokenResponse, error) {
	// Build token request with PKCE code_verifier
	data := url.Values{
		"grant_type":    {"authorization_code"},
		"client_id":     {clientID},
		"code":          {code},
		"redirect_uri":  {redirectURI},
		"code_verifier": {codeVerifier},
	}

	resp, err := http.PostForm(tokenEndpoint, data)
	if err != nil {
		return nil, fmt.Errorf("token request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token endpoint returned status %d", resp.StatusCode)
	}

	var tokens TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokens); err != nil {
		return nil, fmt.Errorf("failed to parse token response: %w", err)
	}

	return &tokens, nil
}

func (a *Authenticator) refreshToken(provider Provider, refreshToken string) error {
	// This would make an actual HTTP request to refresh the token
	// For now, this is a placeholder
	return nil
}

// StartCallbackServer starts a local HTTP server to receive OAuth callback
func StartCallbackServer(ctx context.Context) (chan string, error) {
	codeChan := make(chan string, 1)

	server := &http.Server{Addr: ":9876"}

	http.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		if code != "" {
			codeChan <- code
			fmt.Fprintf(w, "<html><body><h1>Authentication successful!</h1><p>You can close this window.</p></body></html>")
		} else {
			errMsg := r.URL.Query().Get("error")
			fmt.Fprintf(w, "<html><body><h1>Authentication failed</h1><p>%s</p></body></html>", errMsg)
		}

		// Shutdown server after handling callback
		go func() {
			time.Sleep(time.Second)
			server.Shutdown(ctx)
		}()
	})

	go server.ListenAndServe()

	return codeChan, nil
}

func generateRandomString(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(bytes)[:length], nil
}
