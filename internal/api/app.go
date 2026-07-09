package api

import (
	"context"
	"log/slog"

	samplev1 "github.com/adiom-data/crew-demo-app/gen/go/sample/v1"
	"github.com/adiom-data/crew-demo-app/gen/go/sample/v1/samplev1connect"
	apidb "github.com/adiom-data/crew-demo-app/internal/api/db"
	appauth "github.com/adiom-data/crew-demo-app/internal/auth"
	"github.com/adiom-data/framework/auth/tokenissuer"
	"github.com/adiom-data/framework/httpapp"
)

type Config struct {
	DB       DBConfig
	Auth     AuthConfig
	AgentMCP AgentMCPConfig
	Stripe   StripeConfig
}

// AgentMCPConfig configures the unauthenticated /mcp endpoint exposed to the AdiomBot
// agent. SelfBaseURL is the in-process address grpcmcp reflects against and proxies to
// (defaults to http://127.0.0.1:8080, the framework's listen address).
type AgentMCPConfig struct {
	SelfBaseURL string
}

type DBConfig = apidb.Config
type AuthConfig = appauth.Config
type OIDCConfig = appauth.OIDCConfig

// Run assembles and runs the API and auth services in one process.
func Run(cfg Config) error {
	runtime, err := httpapp.Init(context.Background())
	if err != nil {
		return err
	}
	defer func() {
		if err := runtime.Shutdown(context.Background()); err != nil {
			slog.Warn("framework shutdown failed", "err", err)
		}
	}()
	ctx := runtime.Context()

	db, err := apidb.Open(cfg.DB)
	if err != nil {
		slog.Warn("database disabled", "err", err)
	}
	if db != nil {
		defer db.Close()
	}

	authService, err := appauth.New(ctx, db, cfg.Auth)
	if err != nil {
		return err
	}

	authenticator := tokenissuer.NewBearerAuthenticatorFromVerifier(
		tokenissuer.NewLazyRemoteVerifier(tokenissuer.RemoteVerifierConfig{
			Issuer: cfg.Auth.IssuerURL,
		}),
		tokenissuer.RequireScopes("sample:user"),
		tokenissuer.WithAuthValue(func(_ context.Context, claims *tokenissuer.Claims) (*samplev1.User, error) {
			return &samplev1.User{
				Id:     claims.Subject,
				Email:  claims.Attributes["email"],
				Name:   claims.Attributes["name"],
				Scopes: claims.Scopes,
			}, nil
		}),
	)

	billing := newBillingService(db, cfg.Stripe, cfg.Auth.PublicBaseURL)
	if !cfg.Stripe.enabled() {
		slog.Warn("billing disabled: stripe is not configured")
	}

	services := []httpapp.ConnectService{
		httpapp.ConnectHandler[samplev1connect.SampleServiceHandler](
			samplev1connect.SampleServiceName,
			samplev1connect.NewSampleServiceHandler,
			sampleService{db: db},
			httpapp.WithInterceptors(tokenissuer.ConnectAuth(authenticator)),
		),
		// PartnerService is admin-only: same bearer auth as SampleService.
		httpapp.ConnectHandler[samplev1connect.PartnerServiceHandler](
			samplev1connect.PartnerServiceName,
			samplev1connect.NewPartnerServiceHandler,
			partnerService{db: db},
			httpapp.WithInterceptors(tokenissuer.ConnectAuth(authenticator)),
		),
		// OnboardingService is public: no auth interceptor so partners can self-submit.
		httpapp.ConnectHandler[samplev1connect.OnboardingServiceHandler](
			samplev1connect.OnboardingServiceName,
			samplev1connect.NewOnboardingServiceHandler,
			onboardingService{db: db},
		),
		// AgentQueryService is the read-only surface behind the /mcp endpoint. It is
		// unauthenticated by design (see AgentMCPConfig); keep it side-effect-free.
		httpapp.ConnectHandler[samplev1connect.AgentQueryServiceHandler](
			samplev1connect.AgentQueryServiceName,
			samplev1connect.NewAgentQueryServiceHandler,
			agentQueryService{db: db},
		),
		// BillingService is admin-only: same bearer auth as PartnerService. The
		// Stripe webhook it pairs with is a separate, public raw route below.
		httpapp.ConnectHandler[samplev1connect.BillingServiceHandler](
			samplev1connect.BillingServiceName,
			samplev1connect.NewBillingServiceHandler,
			billing,
			httpapp.WithInterceptors(tokenissuer.ConnectAuth(authenticator)),
		),
	}
	services = append(services, authService.ConnectServices...)

	selfBaseURL := cfg.AgentMCP.SelfBaseURL
	if selfBaseURL == "" {
		selfBaseURL = "http://127.0.0.1:8080"
	}
	routes := append([]httpapp.Route{}, authService.Routes...)
	routes = append(routes, httpapp.Handle("/mcp", newAgentMCPHandler(selfBaseURL)))
	routes = append(routes, httpapp.Handle("/stripe/webhook", billing.webhookHandler()))

	return runtime.NewService(
		httpapp.WithServices(services...),
		httpapp.WithServiceRoutes(routes...),
		httpapp.WithReflection(),
	).Run(ctx)
}
