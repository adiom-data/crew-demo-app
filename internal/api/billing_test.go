package api

import (
	"bytes"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"testing"

	samplev1 "github.com/adiom-data/crew-demo-app/gen/go/sample/v1"
	"github.com/stripe/stripe-go/v86"
	"github.com/stripe/stripe-go/v86/webhook"
)

const testWebhookSecret = "whsec_test_secret"

func testStripeConfig() StripeConfig {
	return StripeConfig{
		SecretKey:     "sk_test_123",
		WebhookSecret: testWebhookSecret,
		PriceMonthly:  "price_monthly",
		PriceAnnual:   "price_annual",
	}
}

// lazyDB returns a non-nil *sql.DB that never connects. sql.Open does not dial,
// so handlers that short-circuit before touching the database can be tested
// without a live Postgres.
func lazyDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("pgx", "postgres://nobody@127.0.0.1:1/none")
	if err != nil {
		t.Fatalf("open lazy db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

// signedRequest builds a webhook request carrying a valid Stripe-Signature over
// the exact bytes of body.
func signedRequest(body []byte, secret string) *http.Request {
	signed := webhook.GenerateTestSignedPayload(&webhook.UnsignedPayload{
		Payload: body,
		Secret:  secret,
	})
	r := httptest.NewRequest(http.MethodPost, "/stripe/webhook", bytes.NewReader(signed.Payload))
	r.Header.Set("Stripe-Signature", signed.Header)
	return r
}

func TestWebhookRejectsBadRequests(t *testing.T) {
	body := []byte(`{"id":"evt_1","object":"event","type":"invoice.paid","api_version":"2026-06-24.dahlia","data":{"object":{}}}`)

	tests := []struct {
		name    string
		service billingService
		request func() *http.Request
		want    int
	}{
		{
			name:    "rejects non-POST",
			service: newBillingService(lazyDB(t), testStripeConfig(), ""),
			request: func() *http.Request {
				return httptest.NewRequest(http.MethodGet, "/stripe/webhook", nil)
			},
			want: http.StatusMethodNotAllowed,
		},
		{
			name:    "unavailable when stripe is unconfigured",
			service: newBillingService(lazyDB(t), StripeConfig{}, ""),
			request: func() *http.Request { return signedRequest(body, testWebhookSecret) },
			want:    http.StatusServiceUnavailable,
		},
		{
			name:    "unavailable when database is absent",
			service: newBillingService(nil, testStripeConfig(), ""),
			request: func() *http.Request { return signedRequest(body, testWebhookSecret) },
			want:    http.StatusServiceUnavailable,
		},
		{
			name:    "rejects a missing signature",
			service: newBillingService(lazyDB(t), testStripeConfig(), ""),
			request: func() *http.Request {
				return httptest.NewRequest(http.MethodPost, "/stripe/webhook", bytes.NewReader(body))
			},
			want: http.StatusBadRequest,
		},
		{
			name:    "rejects a signature made with the wrong secret",
			service: newBillingService(lazyDB(t), testStripeConfig(), ""),
			request: func() *http.Request { return signedRequest(body, "whsec_wrong_secret") },
			want:    http.StatusBadRequest,
		},
		{
			name:    "rejects a payload tampered with after signing",
			service: newBillingService(lazyDB(t), testStripeConfig(), ""),
			request: func() *http.Request {
				// Keep the signature header, swap the bytes underneath it.
				signed := webhook.GenerateTestSignedPayload(&webhook.UnsignedPayload{
					Payload: body,
					Secret:  testWebhookSecret,
				})
				tampered := []byte(`{"id":"evt_2","object":"event","type":"invoice.paid","data":{"object":{}}}`)
				r := httptest.NewRequest(http.MethodPost, "/stripe/webhook", bytes.NewReader(tampered))
				r.Header.Set("Stripe-Signature", signed.Header)
				return r
			},
			want: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			tt.service.webhookHandler().ServeHTTP(rec, tt.request())
			if rec.Code != tt.want {
				t.Fatalf("status = %d, want %d (body %q)", rec.Code, tt.want, rec.Body.String())
			}
		})
	}
}

// A validly signed event that we don't act on must be acknowledged with 200 and
// must never reach the database.
func TestWebhookAcceptsSignedUnhandledEvent(t *testing.T) {
	body := []byte(`{"id":"evt_1","object":"event","type":"invoice.paid","api_version":"2026-06-24.dahlia","data":{"object":{}}}`)

	rec := httptest.NewRecorder()
	svc := newBillingService(lazyDB(t), testStripeConfig(), "")
	svc.webhookHandler().ServeHTTP(rec, signedRequest(body, testWebhookSecret))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %q)", rec.Code, rec.Body.String())
	}
}

// stripe-go pins stripe.APIVersion and webhook.ConstructEvent rejects events
// from an account on an older API release train. We pass IgnoreAPIVersionMismatch,
// so an old api_version must still be accepted — otherwise every real webhook
// from a not-freshly-created Stripe account would 400.
func TestWebhookIgnoresAPIVersionMismatch(t *testing.T) {
	body := []byte(`{"id":"evt_1","object":"event","type":"invoice.paid","api_version":"2020-08-27","data":{"object":{}}}`)

	rec := httptest.NewRecorder()
	svc := newBillingService(lazyDB(t), testStripeConfig(), "")
	svc.webhookHandler().ServeHTTP(rec, signedRequest(body, testWebhookSecret))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %q)", rec.Code, rec.Body.String())
	}
}

func TestStripeConfigEnabled(t *testing.T) {
	full := testStripeConfig()
	if !full.enabled() {
		t.Fatal("fully populated config should be enabled")
	}

	missingKey := full
	missingKey.SecretKey = ""
	missingMonthly := full
	missingMonthly.PriceMonthly = ""
	missingAnnual := full
	missingAnnual.PriceAnnual = ""

	for name, cfg := range map[string]StripeConfig{
		"empty":            {},
		"no secret key":    missingKey,
		"no monthly price": missingMonthly,
		"no annual price":  missingAnnual,
	} {
		if cfg.enabled() {
			t.Errorf("%s: config should not be enabled", name)
		}
	}
}

func TestPriceFor(t *testing.T) {
	svc := newBillingService(nil, testStripeConfig(), "")

	tests := []struct {
		plan samplev1.SubscriptionPlan
		want string
	}{
		{samplev1.SubscriptionPlan_SUBSCRIPTION_PLAN_MONTHLY, "price_monthly"},
		{samplev1.SubscriptionPlan_SUBSCRIPTION_PLAN_ANNUAL, "price_annual"},
		{samplev1.SubscriptionPlan_SUBSCRIPTION_PLAN_UNSPECIFIED, ""},
	}
	for _, tt := range tests {
		if got := svc.priceFor(tt.plan); got != tt.want {
			t.Errorf("priceFor(%v) = %q, want %q", tt.plan, got, tt.want)
		}
	}
}

func TestReturnBaseURL(t *testing.T) {
	header := func(origin string) http.Header {
		h := http.Header{}
		if origin != "" {
			h.Set("Origin", origin)
		}
		return h
	}

	tests := []struct {
		name          string
		publicBaseURL string
		origin        string
		want          string
		wantErr       bool
	}{
		{name: "public base url wins", publicBaseURL: "https://app.example.com", origin: "https://evil.test", want: "https://app.example.com"},
		{name: "trailing slash trimmed", publicBaseURL: "https://app.example.com/", want: "https://app.example.com"},
		{name: "falls back to origin", origin: "http://localhost:5173", want: "http://localhost:5173"},
		{name: "errors without either", wantErr: true},
		{name: "rejects a non-http scheme", origin: "javascript:alert(1)", wantErr: true},
		{name: "rejects a hostless origin", origin: "https://", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := newBillingService(nil, testStripeConfig(), tt.publicBaseURL)
			got, err := svc.returnBaseURL(header(tt.origin))
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected an error, got %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("returnBaseURL = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSubscriptionStatusToText(t *testing.T) {
	tests := []struct {
		status stripe.SubscriptionStatus
		want   string
	}{
		{stripe.SubscriptionStatusActive, "active"},
		{stripe.SubscriptionStatusPastDue, "past_due"},
		{stripe.SubscriptionStatusUnpaid, "past_due"},
		{stripe.SubscriptionStatusCanceled, "canceled"},
		{stripe.SubscriptionStatusIncompleteExpired, "canceled"},
		{stripe.SubscriptionStatusTrialing, ""},
		{stripe.SubscriptionStatusIncomplete, ""},
	}
	for _, tt := range tests {
		if got := subscriptionStatusToText(tt.status); got != tt.want {
			t.Errorf("subscriptionStatusToText(%q) = %q, want %q", tt.status, got, tt.want)
		}
	}
}

func TestSubscriptionEnumTextRoundTrip(t *testing.T) {
	plans := []samplev1.SubscriptionPlan{
		samplev1.SubscriptionPlan_SUBSCRIPTION_PLAN_UNSPECIFIED,
		samplev1.SubscriptionPlan_SUBSCRIPTION_PLAN_MONTHLY,
		samplev1.SubscriptionPlan_SUBSCRIPTION_PLAN_ANNUAL,
	}
	for _, plan := range plans {
		if got := textToSubscriptionPlan(subscriptionPlanToText(plan)); got != plan {
			t.Errorf("round trip of %v = %v", plan, got)
		}
	}

	// An empty column means "never subscribed" in both directions.
	if got := textToSubscriptionStatus(""); got != samplev1.SubscriptionStatus_SUBSCRIPTION_STATUS_UNSPECIFIED {
		t.Errorf(`textToSubscriptionStatus("") = %v, want UNSPECIFIED`, got)
	}
	if got := textToSubscriptionStatus("past_due"); got != samplev1.SubscriptionStatus_SUBSCRIPTION_STATUS_PAST_DUE {
		t.Errorf(`textToSubscriptionStatus("past_due") = %v`, got)
	}
	if got := textToSubscriptionStatus("bogus"); got != samplev1.SubscriptionStatus_SUBSCRIPTION_STATUS_UNSPECIFIED {
		t.Errorf(`textToSubscriptionStatus("bogus") = %v, want UNSPECIFIED`, got)
	}
}
