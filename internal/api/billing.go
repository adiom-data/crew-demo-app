package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"connectrpc.com/connect"
	samplev1 "github.com/adiom-data/crew-demo-app/gen/go/sample/v1"
	apidb "github.com/adiom-data/crew-demo-app/internal/api/db"
	"github.com/stripe/stripe-go/v86"
	"github.com/stripe/stripe-go/v86/webhook"
)

// maxWebhookBody caps the bytes read from a webhook request. Stripe events are
// small; anything larger is not one.
const maxWebhookBody = 1 << 20

// StripeConfig holds the Stripe credentials and price ids. Every field is
// optional: when Stripe is not configured the billing RPC and the webhook
// degrade to Unavailable rather than failing startup, matching how the app
// tolerates an absent database.
type StripeConfig struct {
	SecretKey     string
	WebhookSecret string
	PriceMonthly  string
	PriceAnnual   string
}

func (c StripeConfig) enabled() bool {
	return c.SecretKey != "" && c.PriceMonthly != "" && c.PriceAnnual != ""
}

// billingService is the authenticated admin-facing billing API plus the raw
// Stripe webhook handler.
type billingService struct {
	db  *sql.DB
	cfg StripeConfig
	// publicBaseURL is PUBLIC_BASE_URL when set; otherwise the checkout return
	// URLs fall back to the request Origin.
	publicBaseURL string
	sc            *stripe.Client
}

func newBillingService(db *sql.DB, cfg StripeConfig, publicBaseURL string) billingService {
	s := billingService{db: db, cfg: cfg, publicBaseURL: strings.TrimRight(publicBaseURL, "/")}
	if cfg.enabled() {
		s.sc = stripe.NewClient(cfg.SecretKey)
	}
	return s
}

func errBillingUnavailable() error {
	return connect.NewError(connect.CodeUnavailable, errors.New("billing is not configured"))
}

func (s billingService) priceFor(plan samplev1.SubscriptionPlan) string {
	switch plan {
	case samplev1.SubscriptionPlan_SUBSCRIPTION_PLAN_MONTHLY:
		return s.cfg.PriceMonthly
	case samplev1.SubscriptionPlan_SUBSCRIPTION_PLAN_ANNUAL:
		return s.cfg.PriceAnnual
	default:
		return ""
	}
}

// returnBaseURL resolves the origin Stripe redirects back to. PUBLIC_BASE_URL
// wins when configured; otherwise we trust the browser's Origin, which keeps
// preview and release deployments working without extra env. The caller is an
// authenticated admin, so the redirect-target exposure is bounded.
func (s billingService) returnBaseURL(header http.Header) (string, error) {
	if s.publicBaseURL != "" {
		return s.publicBaseURL, nil
	}
	origin := strings.TrimRight(strings.TrimSpace(header.Get("Origin")), "/")
	if origin == "" {
		return "", connect.NewError(connect.CodeFailedPrecondition, errors.New("cannot determine return url: set PUBLIC_BASE_URL"))
	}
	u, err := url.Parse(origin)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return "", connect.NewError(connect.CodeInvalidArgument, errors.New("origin is not a valid http(s) url"))
	}
	return origin, nil
}

func (s billingService) CreateCheckoutSession(ctx context.Context, req *connect.Request[samplev1.CreateCheckoutSessionRequest]) (*connect.Response[samplev1.CreateCheckoutSessionResponse], error) {
	if s.db == nil {
		return nil, errDatabaseUnavailable()
	}
	if s.sc == nil {
		return nil, errBillingUnavailable()
	}
	id := strings.TrimSpace(req.Msg.GetPartnerId())
	if id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("partner id is required"))
	}
	price := s.priceFor(req.Msg.GetPlan())
	if price == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("a valid plan is required"))
	}

	partner, err := apidb.GetPartner(ctx, s.db, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("partner not found"))
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("get partner: %w", err))
	}

	base, err := s.returnBaseURL(req.Header())
	if err != nil {
		return nil, err
	}

	plan := subscriptionPlanToText(req.Msg.GetPlan())
	// partner_id rides on both the session and the subscription so that later
	// customer.subscription.* events resolve to a partner even if they arrive
	// before checkout.session.completed.
	meta := map[string]string{"partner_id": id, "plan": plan}

	sess, err := s.sc.V1CheckoutSessions.Create(ctx, &stripe.CheckoutSessionCreateParams{
		Mode: stripe.String(string(stripe.CheckoutSessionModeSubscription)),
		LineItems: []*stripe.CheckoutSessionCreateLineItemParams{{
			Price:    stripe.String(price),
			Quantity: stripe.Int64(1),
		}},
		SuccessURL:        stripe.String(base + "/partners/" + id + "?checkout=success"),
		CancelURL:         stripe.String(base + "/partners/" + id + "?checkout=cancel"),
		ClientReferenceID: stripe.String(id),
		CustomerEmail:     stripe.String(partner.ContactEmail),
		Metadata:          meta,
		SubscriptionData: &stripe.CheckoutSessionCreateSubscriptionDataParams{
			Metadata: meta,
		},
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("create checkout session: %w", err))
	}

	return connect.NewResponse(&samplev1.CreateCheckoutSessionResponse{CheckoutUrl: sess.URL}), nil
}

// webhookHandler serves POST /stripe/webhook. It is registered as a raw route
// with no auth interceptor and marked public in gateway.json (INV-4).
func (s billingService) webhookHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if s.db == nil || !s.cfg.enabled() || s.cfg.WebhookSecret == "" {
			http.Error(w, "billing not configured", http.StatusServiceUnavailable)
			return
		}

		body, err := io.ReadAll(io.LimitReader(r.Body, maxWebhookBody))
		if err != nil {
			http.Error(w, "cannot read body", http.StatusBadRequest)
			return
		}

		// The signature covers these exact bytes, so verify before decoding and
		// never re-marshal. IgnoreAPIVersionMismatch is required because
		// stripe-go pins an API version and rejects events from accounts on an
		// older release train.
		event, err := webhook.ConstructEventWithOptions(body, r.Header.Get("Stripe-Signature"), s.cfg.WebhookSecret,
			webhook.ConstructEventOptions{IgnoreAPIVersionMismatch: true})
		if err != nil {
			// Log the reason: a rejected event is otherwise indistinguishable
			// from a bad secret at the caller.
			slog.Warn("rejected stripe webhook", "err", err)
			http.Error(w, "invalid signature", http.StatusBadRequest)
			return
		}

		if err := s.handleEvent(r.Context(), event); err != nil {
			// 5xx asks Stripe to retry; applySubscription is idempotent.
			slog.Error("stripe webhook failed", "type", event.Type, "err", err)
			http.Error(w, "webhook processing failed", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
}

func (s billingService) handleEvent(ctx context.Context, event stripe.Event) error {
	switch event.Type {
	case "checkout.session.completed":
		var sess stripe.CheckoutSession
		if err := json.Unmarshal(event.Data.Raw, &sess); err != nil {
			return fmt.Errorf("decode checkout session: %w", err)
		}
		partnerID := sess.ClientReferenceID
		if partnerID == "" {
			partnerID = sess.Metadata["partner_id"]
		}
		plan := sess.Metadata["plan"]
		var customerID, subscriptionID string
		if sess.Customer != nil {
			customerID = sess.Customer.ID
		}
		if sess.Subscription != nil {
			subscriptionID = sess.Subscription.ID
		}
		return s.applySubscription(ctx, partnerID, customerID, subscriptionID, plan, "active",
			fmt.Sprintf("Subscribed (%s) via Stripe", planOrUnknown(plan)))

	case "customer.subscription.updated", "customer.subscription.deleted":
		var sub stripe.Subscription
		if err := json.Unmarshal(event.Data.Raw, &sub); err != nil {
			return fmt.Errorf("decode subscription: %w", err)
		}
		status := "canceled"
		if event.Type == "customer.subscription.updated" {
			status = subscriptionStatusToText(sub.Status)
			if status == "" {
				return nil // a state we don't surface (trialing, incomplete, …)
			}
		}
		partnerID, err := s.resolvePartner(ctx, sub)
		if err != nil || partnerID == "" {
			return err
		}
		plan := sub.Metadata["plan"]
		var customerID string
		if sub.Customer != nil {
			customerID = sub.Customer.ID
		}
		return s.applySubscription(ctx, partnerID, customerID, sub.ID, plan, status,
			"Subscription "+status+" via Stripe")
	}
	return nil // unhandled event types are acknowledged, not retried
}

// resolvePartner finds the partner a subscription belongs to, preferring the
// metadata we stamped at checkout and falling back to the stored id.
func (s billingService) resolvePartner(ctx context.Context, sub stripe.Subscription) (string, error) {
	if id := sub.Metadata["partner_id"]; id != "" {
		return id, nil
	}
	partner, err := apidb.GetPartnerBySubscriptionID(ctx, s.db, sub.ID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("resolve partner: %w", err)
	}
	return partner.ID, nil
}

// applySubscription writes subscription state and logs an activity. Stripe
// redelivers events, so an unchanged state is a no-op — otherwise every retry
// would append a duplicate activity row.
func (s billingService) applySubscription(ctx context.Context, partnerID, customerID, subscriptionID, plan, status, message string) error {
	if partnerID == "" {
		return nil
	}
	current, err := apidb.GetPartner(ctx, s.db, partnerID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil // unknown partner: acknowledge so Stripe stops retrying
	}
	if err != nil {
		return fmt.Errorf("get partner: %w", err)
	}

	// Events carry only what changed; keep what we already know otherwise.
	if customerID == "" {
		customerID = current.StripeCustomerID
	}
	if subscriptionID == "" {
		subscriptionID = current.StripeSubscriptionID
	}
	if plan == "" {
		plan = current.SubscriptionPlan
	}

	if current.StripeSubscriptionID == subscriptionID &&
		current.SubscriptionStatus == status &&
		current.SubscriptionPlan == plan {
		return nil
	}

	if _, err := apidb.SetSubscription(ctx, s.db, partnerID, customerID, subscriptionID, plan, status); err != nil {
		return fmt.Errorf("set subscription: %w", err)
	}
	if _, err := apidb.InsertActivity(ctx, s.db, partnerID, "subscription", message); err != nil {
		return fmt.Errorf("record activity: %w", err)
	}
	return nil
}

// subscriptionStatusToText maps Stripe's subscription statuses onto the three
// we surface. An empty result means "don't record this state".
func subscriptionStatusToText(s stripe.SubscriptionStatus) string {
	switch s {
	case stripe.SubscriptionStatusActive:
		return "active"
	case stripe.SubscriptionStatusPastDue, stripe.SubscriptionStatusUnpaid:
		return "past_due"
	case stripe.SubscriptionStatusCanceled, stripe.SubscriptionStatusIncompleteExpired:
		return "canceled"
	default:
		return ""
	}
}

func planOrUnknown(plan string) string {
	if plan == "" {
		return "unknown"
	}
	return plan
}
