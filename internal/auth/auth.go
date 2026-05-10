package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type Provider interface {
	GetToken(ctx context.Context) (string, error)
	GetMetadata(ctx context.Context) (map[string]string, error)
}

type OAuthConfig struct {
	TokenURL     string
	ClientID     string
	ClientSecret string
	Scopes       []string
}

type staticProvider struct {
	token string
}

func NewStaticProvider(token string) Provider {
	return &staticProvider{token: token}
}

func (p *staticProvider) GetToken(ctx context.Context) (string, error) {
	if p.token == "" {
		return "", fmt.Errorf("static token is empty")
	}
	return p.token, nil
}

func (p *staticProvider) GetMetadata(ctx context.Context) (map[string]string, error) {
	token, err := p.GetToken(ctx)
	if err != nil {
		return nil, err
	}
	return map[string]string{
		"authorization": "Bearer " + token,
	}, nil
}

type oauthProvider struct {
	cfg    OAuthConfig
	client *http.Client
}

func NewOAuthProvider(cfg OAuthConfig) Provider {
	return &oauthProvider{
		cfg:    cfg,
		client: &http.Client{},
	}
}

func (p *oauthProvider) GetToken(ctx context.Context) (string, error) {
	data := url.Values{
		"grant_type":    {"client_credentials"},
		"client_id":     {p.cfg.ClientID},
		"client_secret": {p.cfg.ClientSecret},
	}
	if len(p.cfg.Scopes) > 0 {
		data.Set("scope", strings.Join(p.cfg.Scopes, " "))
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.cfg.TokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return "", fmt.Errorf("creating token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := p.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("requesting token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("token endpoint returned %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", fmt.Errorf("decoding token response: %w", err)
	}

	if tokenResp.AccessToken == "" {
		return "", fmt.Errorf("empty access token in response")
	}

	return tokenResp.AccessToken, nil
}

func (p *oauthProvider) GetMetadata(ctx context.Context) (map[string]string, error) {
	token, err := p.GetToken(ctx)
	if err != nil {
		return nil, err
	}
	return map[string]string{
		"authorization": "Bearer " + token,
	}, nil
}

func NewProviderFromConfig(authType, token string, oauthCfg OAuthConfig) (Provider, error) {
	switch authType {
	case "":
		return nil, nil
	case "static":
		return NewStaticProvider(token), nil
	case "oauth":
		return NewOAuthProvider(oauthCfg), nil
	default:
		return nil, fmt.Errorf("unsupported auth type: %q", authType)
	}
}
