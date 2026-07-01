import { AuthTokenManager } from "@adiom-data/framework-web/auth";
import { createSampleClient } from "./sampleClient";
import { createOnboardingClient, createPartnerClient } from "./partnerClient";

// Shared singletons. The token manager holds the in-memory app token; the sample
// and partner clients share it so a single login authorizes every admin call.
export const authTokenManager = new AuthTokenManager();
export const sampleClient = createSampleClient(authTokenManager);
export const partnerClient = createPartnerClient(authTokenManager);
export const onboardingClient = createOnboardingClient();
