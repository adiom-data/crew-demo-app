package auth

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	frameworkauth "github.com/adiom-data/framework/auth"
	authdb "github.com/adiom-data/crew-demo-app/internal/auth/db"
)

type dbAuthorizer struct {
	db *sql.DB
}

func (a dbAuthorizer) Authorize(ctx context.Context, external frameworkauth.ExternalIdentity) (frameworkauth.Identity, error) {
	if a.db == nil {
		return frameworkauth.Identity{}, errors.New("database is required for user authorization")
	}

	subject := strings.TrimSpace(external.Subject)
	issuer := strings.TrimSpace(external.Issuer)
	if subject == "" || issuer == "" {
		return frameworkauth.Identity{}, errors.New("external issuer and subject are required")
	}

	email, _ := external.Claims["email"].(string)
	name, _ := external.Claims["name"].(string)
	userID, err := authdb.UpsertUser(ctx, a.db, issuer, subject, email, name)
	if err != nil {
		return frameworkauth.Identity{}, err
	}

	return frameworkauth.Identity{
		Subject: userID,
		Scopes:  []string{"sample:user"},
		Attributes: map[string]string{
			"email": email,
			"name":  name,
		},
	}, nil
}
