package api

import (
	"context"
	"database/sql"

	"connectrpc.com/connect"
	"github.com/adiom-data/framework/auth/tokenissuer"
	samplev1 "github.com/adiom-data/crew-demo-app/gen/go/sample/v1"
	apidb "github.com/adiom-data/crew-demo-app/internal/api/db"
)

type sampleService struct {
	db *sql.DB
}

func (s sampleService) GetSession(ctx context.Context, req *connect.Request[samplev1.GetSessionRequest]) (*connect.Response[samplev1.GetSessionResponse], error) {
	user, ok := tokenissuer.AuthValueFromContext[*samplev1.User](ctx)
	if !ok || user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, tokenissuer.ErrMissingBearerToken)
	}
	database := &samplev1.Database{Enabled: s.db != nil}
	if s.db != nil {
		if err := apidb.Ping(ctx, s.db); err != nil {
			database.Error = "database unavailable"
		}
	}
	return connect.NewResponse(&samplev1.GetSessionResponse{
		Authenticated: user.GetId() != "",
		User:          user,
		Database:      database,
	}), nil
}
