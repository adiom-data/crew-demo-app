package api

import (
	"testing"
	"time"

	samplev1 "github.com/adiom-data/crew-demo-app/gen/go/sample/v1"
	apidb "github.com/adiom-data/crew-demo-app/internal/api/db"
)

func dbPartnerFixture() apidb.Partner {
	return apidb.Partner{
		ID:            "11111111-1111-1111-1111-111111111111",
		Name:          "Acme",
		ContactEmail:  "ops@acme.com",
		Company:       "Acme Inc",
		Region:        "US-East",
		Tier:          "pro",
		Status:        "active",
		BillingStatus: "past_due",
		Notes:         "vip",
		CreatedAt:     time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC),
	}
}

func TestValidatePartnerInput(t *testing.T) {
	tests := []struct {
		name        string
		partnerName string
		email       string
		wantErr     bool
	}{
		{"valid", "Acme", "ops@acme.com", false},
		{"trims whitespace", "  Acme  ", "  ops@acme.com  ", false},
		{"missing name", "", "ops@acme.com", true},
		{"blank name", "   ", "ops@acme.com", true},
		{"missing email", "Acme", "", true},
		{"email without at", "Acme", "acme.com", true},
		{"email at start", "Acme", "@acme.com", true},
		{"email at end", "Acme", "ops@", true},
		{"email with space", "Acme", "ops @acme.com", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePartnerInput(tt.partnerName, tt.email)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validatePartnerInput(%q, %q) error = %v, wantErr %v", tt.partnerName, tt.email, err, tt.wantErr)
			}
		})
	}
}

func TestTierTextRoundTrip(t *testing.T) {
	tiers := []samplev1.Tier{
		samplev1.Tier_TIER_STARTER,
		samplev1.Tier_TIER_PRO,
		samplev1.Tier_TIER_ENTERPRISE,
	}
	for _, tier := range tiers {
		if got := textToTier(tierToText(tier)); got != tier {
			t.Errorf("round trip tier %v = %v", tier, got)
		}
	}
	if got := tierToText(samplev1.Tier_TIER_UNSPECIFIED); got != "" {
		t.Errorf("unspecified tier text = %q, want empty", got)
	}
	if got := textToTier("bogus"); got != samplev1.Tier_TIER_UNSPECIFIED {
		t.Errorf("unknown tier text = %v, want unspecified", got)
	}
}

func TestStatusTextRoundTrip(t *testing.T) {
	statuses := []samplev1.Status{
		samplev1.Status_STATUS_PENDING,
		samplev1.Status_STATUS_ACTIVE,
		samplev1.Status_STATUS_CHURNED,
	}
	for _, status := range statuses {
		if got := textToStatus(statusToText(status)); got != status {
			t.Errorf("round trip status %v = %v", status, got)
		}
	}
	if got := textToStatus("bogus"); got != samplev1.Status_STATUS_UNSPECIFIED {
		t.Errorf("unknown status text = %v, want unspecified", got)
	}
}

func TestTextToBilling(t *testing.T) {
	cases := map[string]samplev1.BillingStatus{
		"current":  samplev1.BillingStatus_BILLING_STATUS_CURRENT,
		"past_due": samplev1.BillingStatus_BILLING_STATUS_PAST_DUE,
		"trialing": samplev1.BillingStatus_BILLING_STATUS_TRIALING,
		"":         samplev1.BillingStatus_BILLING_STATUS_UNSPECIFIED,
		"bogus":    samplev1.BillingStatus_BILLING_STATUS_UNSPECIFIED,
	}
	for text, want := range cases {
		if got := textToBilling(text); got != want {
			t.Errorf("textToBilling(%q) = %v, want %v", text, got, want)
		}
	}
}

func TestToProtoPartnerMapsFields(t *testing.T) {
	p := toProtoPartner(dbPartnerFixture())
	if p.GetTier() != samplev1.Tier_TIER_PRO {
		t.Errorf("tier = %v", p.GetTier())
	}
	if p.GetStatus() != samplev1.Status_STATUS_ACTIVE {
		t.Errorf("status = %v", p.GetStatus())
	}
	if p.GetBillingStatus() != samplev1.BillingStatus_BILLING_STATUS_PAST_DUE {
		t.Errorf("billing = %v", p.GetBillingStatus())
	}
	if p.GetCreatedAt() == "" {
		t.Error("created_at should be a non-empty RFC3339 string")
	}
}
