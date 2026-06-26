package auth

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"time"

	frameworkauth "github.com/adiom-data/framework/auth"
	"github.com/adiom-data/framework/auth/authservice"
	"github.com/adiom-data/framework/auth/browserauth"
	"github.com/adiom-data/framework/auth/credential"
	"github.com/adiom-data/framework/auth/tokenissuer"
	"github.com/adiom-data/framework/httpapp/jwtauth"
	"golang.org/x/oauth2"
)

type OIDCConfig struct {
	Issuer           string
	ClientID         string
	ClientSecret     string
	AllowedAudiences []string
}

func browserAuthHandler(ctx context.Context, db *sql.DB, authorizer frameworkauth.Authorizer, issuer *tokenissuer.Issuer, cfg Config) (http.Handler, error) {
	if db == nil {
		return nil, errors.New("database is required for browser auth sessions")
	}
	stateKeys, err := browserauth.CookieStateKeysFromSeedBase64(cfg.StateKeyBase64)
	if err != nil {
		return nil, err
	}
	redirectURL := ""
	var redirectURLResolver browserauth.RedirectURLResolver
	if cfg.PublicBaseURL != "" {
		redirectURL = cfg.PublicBaseURL + "/auth/callback"
	} else {
		redirectURLResolver = browserauth.PublicRedirectURL("/auth/callback")
	}

	browserAuth, err := browserauth.New(ctx, browserauth.Config{
		Issuer:              cfg.OIDC.Issuer,
		ClientID:            cfg.OIDC.ClientID,
		ClientSecret:        cfg.OIDC.ClientSecret,
		RedirectURL:         redirectURL,
		ProxyRedirectURL:    cfg.ProxyRedirectURL,
		RedirectURLResolver: redirectURLResolver,
		// These options assume Google browser auth. Other OIDC providers may
		// use different parameters or rely on provider-side client config.
		AuthCodeOptions: []oauth2.AuthCodeOption{
			oauth2.AccessTypeOffline,
			oauth2.SetAuthURLParam("prompt", "consent"),
		},
		StateStore: browserauth.CookieStateStore{
			Path:     "/auth",
			Insecure: cfg.InsecureCookies,
			Keys:     stateKeys,
		},
	})
	if err != nil {
		return nil, err
	}
	return browserAuth.Handler(browserauth.HandlerConfig{
		BasePath:            "/auth",
		Store:               browserauth.SQLSessionStore{DB: db},
		Cookie:              browserauth.SessionCookie{Path: "/auth", Insecure: cfg.InsecureCookies},
		Authorizer:          authorizer,
		Issuer:              issuer,
		Refresher:           browserAuth,
		SuccessRedirect:     "/",
		LogoutRedirect:      "/",
		InvalidStateHandler: browserauth.RedirectInvalidState("/auth/login"),
	}), nil
}

func authExchangeService(authorizer frameworkauth.Authorizer, issuer *tokenissuer.Issuer, cfg OIDCConfig) (*authservice.Service, error) {
	verifier, err := jwtauth.NewVerifier(jwtauth.Config{
		Issuer:           cfg.Issuer,
		AllowedAudiences: cfg.AllowedAudiences,
	})
	if err != nil {
		return nil, err
	}
	return authservice.New(credential.OIDCJWTExchanger{Verifier: verifier}, authorizer, issuer), nil
}

func tokenIssuer(cfg Config) (*tokenissuer.Issuer, error) {
	return tokenissuer.NewFromBase64(tokenissuer.Config{
		Issuer:      cfg.IssuerURL,
		ActiveKeyID: cfg.KeyID,
		TTL:         10 * time.Minute,
	}, cfg.KeyID, cfg.PrivateKeyBase64)
}
