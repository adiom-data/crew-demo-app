package api

import (
	"context"
	"database/sql"
	"fmt"

	"connectrpc.com/connect"
	samplev1 "github.com/adiom-data/crew-demo-app/gen/go/sample/v1"
	apidb "github.com/adiom-data/crew-demo-app/internal/api/db"
)

// agentQueryService is the read-only query surface exposed (unauthenticated) to the
// AdiomBot agent via the /mcp endpoint. It delegates to the same db helpers as
// partnerService but only surfaces side-effect-free reads.
type agentQueryService struct {
	db *sql.DB
}

func (s agentQueryService) ListPartners(ctx context.Context, _ *connect.Request[samplev1.AgentQueryServiceListPartnersRequest]) (*connect.Response[samplev1.AgentQueryServiceListPartnersResponse], error) {
	if s.db == nil {
		return nil, errDatabaseUnavailable()
	}
	partners, err := apidb.ListPartners(ctx, s.db)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("list partners: %w", err))
	}
	counts, err := apidb.CountByStatus(ctx, s.db)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("count partners: %w", err))
	}

	resp := &samplev1.AgentQueryServiceListPartnersResponse{
		Partners: make([]*samplev1.Partner, 0, len(partners)),
		Total:    int32(len(partners)),
		Active:   int32(counts["active"]),
		Pending:  int32(counts["pending"]),
	}
	for _, p := range partners {
		resp.Partners = append(resp.Partners, toProtoPartner(p))
	}
	return connect.NewResponse(resp), nil
}
