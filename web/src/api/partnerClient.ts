import { createClient, type Client } from "@connectrpc/connect";
import { createConnectTransport } from "@connectrpc/connect-web";
import { createAuthInterceptor, type AuthTokenManager } from "@adiom-data/framework-web/auth";
import { OnboardingService, PartnerService } from "../gen/sample/v1/partner_pb";

// createPartnerClient builds an authenticated client for the admin management API.
export function createPartnerClient(tokenManager: AuthTokenManager): Client<typeof PartnerService> {
  return createClient(
    PartnerService,
    createConnectTransport({
      baseUrl: "",
      interceptors: [createAuthInterceptor(tokenManager)],
    }),
  );
}

// createOnboardingClient builds an unauthenticated client for the public
// self-serve onboarding API (no auth interceptor).
export function createOnboardingClient(): Client<typeof OnboardingService> {
  return createClient(
    OnboardingService,
    createConnectTransport({ baseUrl: "" }),
  );
}
