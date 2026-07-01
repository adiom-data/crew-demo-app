package main

import (
	"log/slog"
	"os"
	"strings"

	"github.com/adiom-data/crew-demo-app/internal/api"
	"github.com/caarlos0/env/v11"
)

func main() {
	cfg, err := configFromEnv()
	if err != nil {
		slog.Error("invalid api config", "err", err)
		os.Exit(1)
	}
	if err := api.Run(cfg); err != nil {
		slog.Error("api failed", "err", err)
		os.Exit(1)
	}
}

type environment struct {
	PGHost     string `env:"PGHOST"`
	PGPort     string `env:"PGPORT" envDefault:"5432"`
	PGDatabase string `env:"PGDATABASE" envDefault:"postgres"`
	PGUser     string `env:"PGUSER" envDefault:"postgres"`
	PGPassword string `env:"PGPASSWORD"`
	PGSSLMode  string `env:"PGSSLMODE" envDefault:"disable"`

	PublicBaseURL        string   `env:"PUBLIC_BASE_URL"`
	ProxyRedirectURL     string   `env:"PROXY_REDIRECT_URL"`
	AuthIssuer           string   `env:"AUTH_ISSUER,required"`
	AuthKeyID            string   `env:"AUTH_KEY_ID" envDefault:"sample-auth-2026-06"`
	AuthPrivateKeyBase64 string   `env:"AUTH_PRIVATE_KEY_BASE64,required"`
	AuthStateKeyBase64   string   `env:"AUTH_STATE_KEY_BASE64,required"`
	AuthInsecureCookies  bool     `env:"AUTH_INSECURE_COOKIES" envDefault:"false"`
	OIDCIssuer           string   `env:"OIDC_ISSUER,required"`
	OIDCClientID         string   `env:"OIDC_CLIENT_ID,required"`
	OIDCClientSecret     string   `env:"OIDC_CLIENT_SECRET,required"`
	OIDCAllowedAudiences []string `env:"OIDC_ALLOWED_AUDIENCES" envSeparator:","`
}

func configFromEnv() (api.Config, error) {
	var e environment
	if err := env.Parse(&e); err != nil {
		return api.Config{}, err
	}

	return api.Config{
		DB: api.DBConfig{
			Host:     e.PGHost,
			Port:     e.PGPort,
			Database: e.PGDatabase,
			User:     e.PGUser,
			Password: e.PGPassword,
			SSLMode:  e.PGSSLMode,
		},
		Auth: api.AuthConfig{
			PublicBaseURL:    strings.TrimRight(e.PublicBaseURL, "/"),
			ProxyRedirectURL: strings.TrimRight(e.ProxyRedirectURL, "/"),
			IssuerURL:        e.AuthIssuer,
			KeyID:            e.AuthKeyID,
			PrivateKeyBase64: e.AuthPrivateKeyBase64,
			StateKeyBase64:   e.AuthStateKeyBase64,
			OIDC: api.OIDCConfig{
				Issuer:           e.OIDCIssuer,
				ClientID:         e.OIDCClientID,
				ClientSecret:     e.OIDCClientSecret,
				AllowedAudiences: append([]string{e.OIDCClientID}, e.OIDCAllowedAudiences...),
			},
			InsecureCookies: e.AuthInsecureCookies,
		},
	}, nil
}
