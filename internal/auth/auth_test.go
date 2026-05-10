package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestStaticProvider(t *testing.T) {
	p := NewStaticProvider("my-jwt-token")

	token, err := p.GetToken(context.Background())
	if err != nil {
		t.Fatalf("GetToken() error = %v", err)
	}
	if token != "my-jwt-token" {
		t.Errorf("token = %q, want %q", token, "my-jwt-token")
	}
}

func TestStaticProvider_EmptyToken(t *testing.T) {
	p := NewStaticProvider("")
	_, err := p.GetToken(context.Background())
	if err == nil {
		t.Error("expected error for empty token")
	}
}

func TestStaticProvider_Metadata(t *testing.T) {
	p := NewStaticProvider("my-jwt-token")
	md, err := p.GetMetadata(context.Background())
	if err != nil {
		t.Fatalf("GetMetadata() error = %v", err)
	}
	authHeader, ok := md["authorization"]
	if !ok {
		t.Fatal("expected authorization key in metadata")
	}
	if authHeader != "Bearer my-jwt-token" {
		t.Errorf("authorization = %q, want %q", authHeader, "Bearer my-jwt-token")
	}
}

func TestOAuthProvider(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}

		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		if r.FormValue("grant_type") != "client_credentials" {
			t.Errorf("grant_type = %q, want client_credentials", r.FormValue("grant_type"))
		}
		if r.FormValue("client_id") != "my-client" {
			t.Errorf("client_id = %q", r.FormValue("client_id"))
		}
		if r.FormValue("client_secret") != "my-secret" {
			t.Errorf("client_secret = %q", r.FormValue("client_secret"))
		}

		resp := map[string]interface{}{
			"access_token": "oauth-token-123",
			"token_type":   "Bearer",
			"expires_in":   3600,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	cfg := OAuthConfig{
		TokenURL:     server.URL,
		ClientID:     "my-client",
		ClientSecret: "my-secret",
	}

	p := NewOAuthProvider(cfg)
	token, err := p.GetToken(context.Background())
	if err != nil {
		t.Fatalf("GetToken() error = %v", err)
	}
	if token != "oauth-token-123" {
		t.Errorf("token = %q, want %q", token, "oauth-token-123")
	}
}

func TestOAuthProvider_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	cfg := OAuthConfig{
		TokenURL:     server.URL,
		ClientID:     "my-client",
		ClientSecret: "my-secret",
	}

	p := NewOAuthProvider(cfg)
	_, err := p.GetToken(context.Background())
	if err == nil {
		t.Error("expected error for server error response")
	}
}

func TestOAuthProvider_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not json"))
	}))
	defer server.Close()

	cfg := OAuthConfig{
		TokenURL:     server.URL,
		ClientID:     "my-client",
		ClientSecret: "my-secret",
	}

	p := NewOAuthProvider(cfg)
	_, err := p.GetToken(context.Background())
	if err == nil {
		t.Error("expected error for invalid JSON response")
	}
}

func TestOAuthProvider_ContextCanceled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second)
	}))
	defer server.Close()

	cfg := OAuthConfig{
		TokenURL:     server.URL,
		ClientID:     "my-client",
		ClientSecret: "my-secret",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	p := NewOAuthProvider(cfg)
	_, err := p.GetToken(ctx)
	if err == nil {
		t.Error("expected error for canceled context")
	}
}

func TestOAuthProvider_Metadata(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"access_token": "oauth-token-456",
			"token_type":   "Bearer",
			"expires_in":   3600,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	cfg := OAuthConfig{
		TokenURL:     server.URL,
		ClientID:     "my-client",
		ClientSecret: "my-secret",
	}

	p := NewOAuthProvider(cfg)
	md, err := p.GetMetadata(context.Background())
	if err != nil {
		t.Fatalf("GetMetadata() error = %v", err)
	}
	if md["authorization"] != "Bearer oauth-token-456" {
		t.Errorf("authorization = %q", md["authorization"])
	}
}

func TestNewProviderFromConfig_Static(t *testing.T) {
	p, err := NewProviderFromConfig("static", "my-token", OAuthConfig{})
	if err != nil {
		t.Fatalf("NewProviderFromConfig() error = %v", err)
	}
	token, err := p.GetToken(context.Background())
	if err != nil {
		t.Fatalf("GetToken() error = %v", err)
	}
	if token != "my-token" {
		t.Errorf("token = %q", token)
	}
}

func TestNewProviderFromConfig_None(t *testing.T) {
	p, err := NewProviderFromConfig("", "", OAuthConfig{})
	if err != nil {
		t.Fatalf("NewProviderFromConfig() error = %v", err)
	}
	if p != nil {
		t.Error("expected nil provider for empty auth type")
	}
}

func TestNewProviderFromConfig_InvalidType(t *testing.T) {
	_, err := NewProviderFromConfig("kerberos", "", OAuthConfig{})
	if err == nil {
		t.Error("expected error for invalid auth type")
	}
}
