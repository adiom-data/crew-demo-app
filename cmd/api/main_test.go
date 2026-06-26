package main

import "testing"

func TestConfigFromEnvTrimsProxyRedirectURL(t *testing.T) {
	t.Setenv("AUTH_ISSUER", "http://sample-api/auth")
	t.Setenv("PROXY_REDIRECT_URL", "https://auth-preview.example.test/callback/")
	t.Setenv("AUTH_PRIVATE_KEY_BASE64", "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=")
	t.Setenv("AUTH_STATE_KEY_BASE64", "BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB=")
	t.Setenv("OIDC_ISSUER", "https://accounts.example.test")
	t.Setenv("OIDC_CLIENT_ID", "web-client")
	t.Setenv("OIDC_CLIENT_SECRET", "secret")

	cfg, err := configFromEnv()
	if err != nil {
		t.Fatalf("configFromEnv() error = %v", err)
	}

	if got, want := cfg.Auth.ProxyRedirectURL, "https://auth-preview.example.test/callback"; got != want {
		t.Fatalf("ProxyRedirectURL = %q, want %q", got, want)
	}
}
