package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// authClient performs OAuth login flows against the OpusDNS API. Two
// workflows are supported by the helpers below:
//
//   - Single-step user-token (see auth-login.http):
//     passwordGrant() returns the user access_token directly, which can be
//     used as `Authorization: Bearer` for endpoints that accept user tokens.
//
//   - Three-step client_credentials bootstrap (see api-key-connect-test.http),
//     orchestrated by login():
//     1. POST /v1/auth/token (grant_type=password) with username/password ->
//     user access_token.
//     2. POST /v1/auth/client_credentials (Bearer user token) ->
//     api_key + client_secret.
//     3. POST /v1/auth/token (grant_type=client_credentials) with
//     client_id=org_id, client_secret -> final api access_token.
type authClient struct {
	endpoint string // e.g. https://api.opusdns.com (no trailing /, no version)
	http     *http.Client
}

// tokenResponse mirrors the /v1/auth/token success body.
type tokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}

// clientCredentialsResponse mirrors the /v1/auth/client_credentials body.
type clientCredentialsResponse struct {
	APIKey       string `json:"api_key"`
	ClientSecret string `json:"client_secret"`
}

// login executes the three-step exchange and returns the final bearer
// access_token usable as `Authorization: Bearer <token>` for subsequent API
// calls. The minted api_key/client_secret are returned for caller logging or
// future reuse if desired.
func (a *authClient) login(ctx context.Context, username, password, orgID, apiKeyName, apiKeyDescription string) (accessToken, apiKey, clientSecret string, err error) {
	// Step 1: user password grant.
	userToken, err := a.passwordGrant(ctx, username, password)
	if err != nil {
		return "", "", "", fmt.Errorf("password grant failed: %w", err)
	}

	// Step 2: mint client credentials using user token.
	creds, err := a.mintClientCredentials(ctx, userToken, apiKeyName, apiKeyDescription)
	if err != nil {
		return "", "", "", fmt.Errorf("client_credentials mint failed: %w", err)
	}

	// Step 3: exchange client credentials for an api access token.
	apiToken, err := a.clientCredentialsGrant(ctx, orgID, creds.ClientSecret)
	if err != nil {
		return "", "", "", fmt.Errorf("client_credentials grant failed: %w", err)
	}

	return apiToken, creds.APIKey, creds.ClientSecret, nil
}

func (a *authClient) passwordGrant(ctx context.Context, username, password string) (string, error) {
	form := url.Values{}
	form.Set("grant_type", "password")
	form.Set("username", username)
	form.Set("password", password)

	var out tokenResponse
	if err := a.doForm(ctx, "/v1/auth/token", form, "", &out); err != nil {
		return "", err
	}
	if out.AccessToken == "" {
		return "", fmt.Errorf("password grant returned empty access_token")
	}
	return out.AccessToken, nil
}

func (a *authClient) mintClientCredentials(ctx context.Context, userToken, name, description string) (*clientCredentialsResponse, error) {
	body := map[string]string{
		"api_key_name":        name,
		"api_key_description": description,
	}
	var out clientCredentialsResponse
	if err := a.doJSON(ctx, "/v1/auth/client_credentials", body, userToken, &out); err != nil {
		return nil, err
	}
	if out.ClientSecret == "" {
		return nil, fmt.Errorf("client_credentials response missing client_secret")
	}
	return &out, nil
}

func (a *authClient) clientCredentialsGrant(ctx context.Context, clientID, clientSecret string) (string, error) {
	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("client_id", clientID)
	form.Set("client_secret", clientSecret)

	var out tokenResponse
	if err := a.doForm(ctx, "/v1/auth/token", form, "", &out); err != nil {
		return "", err
	}
	if out.AccessToken == "" {
		return "", fmt.Errorf("client_credentials grant returned empty access_token")
	}
	return out.AccessToken, nil
}

func (a *authClient) doForm(ctx context.Context, path string, form url.Values, bearer string, out interface{}) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.endpoint+path, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	return a.execute(req, out)
}

func (a *authClient) doJSON(ctx context.Context, path string, body interface{}, bearer string, out interface{}) error {
	data, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.endpoint+path, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	return a.execute(req, out)
}

func (a *authClient) execute(req *http.Request, out interface{}) error {
	client := a.http
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close() //nolint:errcheck

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("%s %s: HTTP %d: %s", req.Method, req.URL.Path, resp.StatusCode, strings.TrimSpace(string(body)))
	}
	if out == nil || len(body) == 0 {
		return nil
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

// bearerTransport wraps an underlying http.RoundTripper, stripping the
// SDK-injected `X-Api-Key` header and substituting `Authorization: Bearer
// <token>` so requests authenticate against the OpusDNS API's OAuth2 scheme.
type bearerTransport struct {
	token string
	base  http.RoundTripper
}

func (t *bearerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Clone to avoid mutating the caller's request.
	r := req.Clone(req.Context())
	r.Header.Del("X-Api-Key")
	r.Header.Set("Authorization", "Bearer "+t.token)
	base := t.base
	if base == nil {
		base = http.DefaultTransport
	}
	return base.RoundTrip(r)
}
