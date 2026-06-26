package auth

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"strings"

	"github.com/adiom-data/framework/auth/tokenissuer"
	"github.com/adiom-data/framework/gen/go/adiom/auth/v1/authv1connect"
	"github.com/adiom-data/framework/httpapp"
)

type Service struct {
	Issuer          *tokenissuer.Issuer
	Routes          []httpapp.Route
	ConnectServices []httpapp.ConnectService
}

type Config struct {
	PublicBaseURL    string
	ProxyRedirectURL string
	IssuerURL        string
	KeyID            string
	PrivateKeyBase64 string
	StateKeyBase64   string
	OIDC             OIDCConfig
	InsecureCookies  bool
}

func New(ctx context.Context, db *sql.DB, cfg Config) (*Service, error) {
	cfg.PublicBaseURL = strings.TrimRight(cfg.PublicBaseURL, "/")
	cfg.ProxyRedirectURL = strings.TrimRight(strings.TrimSpace(cfg.ProxyRedirectURL), "/")
	cfg.IssuerURL = strings.TrimRight(strings.TrimSpace(cfg.IssuerURL), "/")
	issuer, err := tokenIssuer(cfg)
	if err != nil {
		return nil, err
	}

	authorizer := dbAuthorizer{db: db}
	authHandler, err := browserAuthHandler(ctx, db, authorizer, issuer, cfg)
	if err != nil {
		return nil, fmt.Errorf("configure browser auth: %w", err)
	}
	exchangeService, err := authExchangeService(authorizer, issuer, cfg.OIDC)
	if err != nil {
		return nil, fmt.Errorf("configure auth exchange service: %w", err)
	}

	return &Service{
		Issuer: issuer,
		Routes: []httpapp.Route{
			httpapp.Handle("/auth/", http.StripPrefix("/auth", authHandler)),
		},
		ConnectServices: []httpapp.ConnectService{
			httpapp.ConnectHandler[authv1connect.AuthServiceHandler](
				authv1connect.AuthServiceName,
				authv1connect.NewAuthServiceHandler,
				exchangeService,
			),
		},
	}, nil
}
