package api

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"connectrpc.com/connect"
	samplev1 "github.com/adiom-data/crew-demo-app/gen/go/sample/v1"
	apidb "github.com/adiom-data/crew-demo-app/internal/api/db"
)

// partnerService is the authenticated admin-facing management API.
type partnerService struct {
	db *sql.DB
}

// onboardingService is the public, unauthenticated self-serve API.
type onboardingService struct {
	db *sql.DB
}

func (s partnerService) ListPartners(ctx context.Context, _ *connect.Request[samplev1.ListPartnersRequest]) (*connect.Response[samplev1.ListPartnersResponse], error) {
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

	resp := &samplev1.ListPartnersResponse{
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

func (s partnerService) GetPartner(ctx context.Context, req *connect.Request[samplev1.GetPartnerRequest]) (*connect.Response[samplev1.GetPartnerResponse], error) {
	if s.db == nil {
		return nil, errDatabaseUnavailable()
	}
	id := strings.TrimSpace(req.Msg.GetId())
	if id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("partner id is required"))
	}

	partner, err := apidb.GetPartner(ctx, s.db, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("partner not found"))
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("get partner: %w", err))
	}
	activities, err := apidb.GetActivities(ctx, s.db, id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("get activities: %w", err))
	}

	resp := &samplev1.GetPartnerResponse{
		Partner:    toProtoPartner(partner),
		Activities: make([]*samplev1.Activity, 0, len(activities)),
	}
	for _, a := range activities {
		resp.Activities = append(resp.Activities, toProtoActivity(a))
	}
	return connect.NewResponse(resp), nil
}

func (s partnerService) CreatePartner(ctx context.Context, req *connect.Request[samplev1.CreatePartnerRequest]) (*connect.Response[samplev1.CreatePartnerResponse], error) {
	if s.db == nil {
		return nil, errDatabaseUnavailable()
	}
	msg := req.Msg
	if err := validatePartnerInput(msg.GetName(), msg.GetContactEmail()); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	partner, err := apidb.CreatePartner(ctx, s.db, apidb.Partner{
		Name:         strings.TrimSpace(msg.GetName()),
		ContactEmail: strings.TrimSpace(msg.GetContactEmail()),
		Company:      strings.TrimSpace(msg.GetCompany()),
		Region:       strings.TrimSpace(msg.GetRegion()),
		Tier:         tierToText(msg.GetTier()),
		Status:       statusToText(samplev1.Status_STATUS_ACTIVE),
		Notes:        strings.TrimSpace(msg.GetNotes()),
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("create partner: %w", err))
	}
	if _, err := apidb.InsertActivity(ctx, s.db, partner.ID, "created", "Partner created"); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("record activity: %w", err))
	}
	return connect.NewResponse(&samplev1.CreatePartnerResponse{Partner: toProtoPartner(partner)}), nil
}

func (s partnerService) UpdatePartnerStatus(ctx context.Context, req *connect.Request[samplev1.UpdatePartnerStatusRequest]) (*connect.Response[samplev1.UpdatePartnerStatusResponse], error) {
	if s.db == nil {
		return nil, errDatabaseUnavailable()
	}
	id := strings.TrimSpace(req.Msg.GetId())
	if id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("partner id is required"))
	}
	status := req.Msg.GetStatus()
	if status == samplev1.Status_STATUS_UNSPECIFIED {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("a valid status is required"))
	}

	partner, err := apidb.UpdatePartnerStatus(ctx, s.db, id, statusToText(status))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("partner not found"))
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("update status: %w", err))
	}
	if _, err := apidb.InsertActivity(ctx, s.db, id, "status_changed", "Status changed to "+statusLabel(status)); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("record activity: %w", err))
	}
	return connect.NewResponse(&samplev1.UpdatePartnerStatusResponse{Partner: toProtoPartner(partner)}), nil
}

func (s partnerService) BulkImportPartners(ctx context.Context, req *connect.Request[samplev1.BulkImportPartnersRequest]) (*connect.Response[samplev1.BulkImportPartnersResponse], error) {
	if s.db == nil {
		return nil, errDatabaseUnavailable()
	}
	resp := &samplev1.BulkImportPartnersResponse{}
	for i, row := range req.Msg.GetRows() {
		if err := validatePartnerInput(row.GetName(), row.GetContactEmail()); err != nil {
			resp.Errors = append(resp.Errors, &samplev1.RowError{Row: int32(i + 1), Message: err.Error()})
			continue
		}
		_, err := apidb.CreatePartner(ctx, s.db, apidb.Partner{
			Name:         strings.TrimSpace(row.GetName()),
			ContactEmail: strings.TrimSpace(row.GetContactEmail()),
			Company:      strings.TrimSpace(row.GetCompany()),
			Region:       strings.TrimSpace(row.GetRegion()),
			Tier:         tierToText(row.GetTier()),
			Status:       statusToText(samplev1.Status_STATUS_ACTIVE),
		})
		if err != nil {
			resp.Errors = append(resp.Errors, &samplev1.RowError{Row: int32(i + 1), Message: "failed to save row"})
			continue
		}
		resp.Imported++
	}
	return connect.NewResponse(resp), nil
}

func (s onboardingService) SubmitOnboarding(ctx context.Context, req *connect.Request[samplev1.SubmitOnboardingRequest]) (*connect.Response[samplev1.SubmitOnboardingResponse], error) {
	if s.db == nil {
		return nil, errDatabaseUnavailable()
	}
	msg := req.Msg
	if err := validatePartnerInput(msg.GetName(), msg.GetContactEmail()); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	// Self-serve submissions land as Pending for an admin to review.
	partner, err := apidb.CreatePartner(ctx, s.db, apidb.Partner{
		Name:         strings.TrimSpace(msg.GetName()),
		ContactEmail: strings.TrimSpace(msg.GetContactEmail()),
		Company:      strings.TrimSpace(msg.GetCompany()),
		Region:       strings.TrimSpace(msg.GetRegion()),
		Status:       statusToText(samplev1.Status_STATUS_PENDING),
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("submit onboarding: %w", err))
	}
	if _, err := apidb.InsertActivity(ctx, s.db, partner.ID, "submitted", "Submitted via public onboarding form"); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("record activity: %w", err))
	}
	return connect.NewResponse(&samplev1.SubmitOnboardingResponse{PartnerId: partner.ID}), nil
}

// --- helpers ---

func errDatabaseUnavailable() error {
	return connect.NewError(connect.CodeUnavailable, errors.New("database is not configured"))
}

func validatePartnerInput(name, email string) error {
	if strings.TrimSpace(name) == "" {
		return errors.New("name is required")
	}
	return validateEmail(email)
}

func validateEmail(email string) error {
	email = strings.TrimSpace(email)
	if email == "" {
		return errors.New("contact email is required")
	}
	at := strings.IndexByte(email, '@')
	if at <= 0 || at == len(email)-1 || strings.ContainsAny(email, " \t") {
		return errors.New("contact email is not valid")
	}
	return nil
}

func toProtoPartner(p apidb.Partner) *samplev1.Partner {
	return &samplev1.Partner{
		Id:            p.ID,
		Name:          p.Name,
		ContactEmail:  p.ContactEmail,
		Company:       p.Company,
		Region:        p.Region,
		Tier:          textToTier(p.Tier),
		Status:        textToStatus(p.Status),
		BillingStatus: textToBilling(p.BillingStatus),
		Notes:         p.Notes,
		CreatedAt:     p.CreatedAt.UTC().Format(time.RFC3339),
	}
}

func toProtoActivity(a apidb.Activity) *samplev1.Activity {
	return &samplev1.Activity{
		Id:        a.ID,
		PartnerId: a.PartnerID,
		Type:      a.Type,
		Message:   a.Message,
		CreatedAt: a.CreatedAt.UTC().Format(time.RFC3339),
	}
}

func tierToText(t samplev1.Tier) string {
	switch t {
	case samplev1.Tier_TIER_STARTER:
		return "starter"
	case samplev1.Tier_TIER_PRO:
		return "pro"
	case samplev1.Tier_TIER_ENTERPRISE:
		return "enterprise"
	default:
		return ""
	}
}

func textToTier(s string) samplev1.Tier {
	switch s {
	case "starter":
		return samplev1.Tier_TIER_STARTER
	case "pro":
		return samplev1.Tier_TIER_PRO
	case "enterprise":
		return samplev1.Tier_TIER_ENTERPRISE
	default:
		return samplev1.Tier_TIER_UNSPECIFIED
	}
}

func statusToText(s samplev1.Status) string {
	switch s {
	case samplev1.Status_STATUS_PENDING:
		return "pending"
	case samplev1.Status_STATUS_ACTIVE:
		return "active"
	case samplev1.Status_STATUS_CHURNED:
		return "churned"
	default:
		return ""
	}
}

func textToStatus(s string) samplev1.Status {
	switch s {
	case "pending":
		return samplev1.Status_STATUS_PENDING
	case "active":
		return samplev1.Status_STATUS_ACTIVE
	case "churned":
		return samplev1.Status_STATUS_CHURNED
	default:
		return samplev1.Status_STATUS_UNSPECIFIED
	}
}

func textToBilling(s string) samplev1.BillingStatus {
	switch s {
	case "current":
		return samplev1.BillingStatus_BILLING_STATUS_CURRENT
	case "past_due":
		return samplev1.BillingStatus_BILLING_STATUS_PAST_DUE
	case "trialing":
		return samplev1.BillingStatus_BILLING_STATUS_TRIALING
	default:
		return samplev1.BillingStatus_BILLING_STATUS_UNSPECIFIED
	}
}

func statusLabel(s samplev1.Status) string {
	switch s {
	case samplev1.Status_STATUS_PENDING:
		return "Pending"
	case samplev1.Status_STATUS_ACTIVE:
		return "Active"
	case samplev1.Status_STATUS_CHURNED:
		return "Churned"
	default:
		return "Unknown"
	}
}
