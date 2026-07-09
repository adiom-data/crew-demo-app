import { createClient, type Client } from "@connectrpc/connect";
import { createConnectTransport } from "@connectrpc/connect-web";
import { createAuthInterceptor, type AuthTokenManager } from "@adiom-data/framework-web/auth";
import { BillingService } from "../gen/sample/v1/billing_pb";

// createBillingClient builds an authenticated client for the admin billing API.
export function createBillingClient(tokenManager: AuthTokenManager): Client<typeof BillingService> {
  return createClient(
    BillingService,
    createConnectTransport({
      baseUrl: "",
      interceptors: [createAuthInterceptor(tokenManager)],
    }),
  );
}
