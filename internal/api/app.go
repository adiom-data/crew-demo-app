package api

import (
	"context"
	"log/slog"

	"github.com/adiom-data/framework/auth/tokenissuer"
	"github.com/adiom-data/framework/httpapp"
	samplev1 "github.com/adiom-data/crew-demo-app/gen/go/sample/v1"
	"github.com/adiom-data/crew-demo-app/gen/go/sample/v1/samplev1connect"
	apidb "github.com/adiom-data/crew-demo-app/internal/api/db"
	appauth "github.com/adiom-data/crew-demo-app/internal/auth"
)

type Config struct {
	DB   DBConfig
	Auth AuthConfig
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
	}
	services = append(services, authService.ConnectServices...)

	return runtime.NewService(
		httpapp.WithServices(services...),
		httpapp.WithServiceRoutes(authService.Routes...),
		httpapp.WithReflection(),
	).Run(ctx)
}
