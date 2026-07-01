import react from "@vitejs/plugin-react";
import { defineConfig } from "vite";

// API_PROXY_TARGET lets local dev forward backend calls to a running API.
// In production the gateway serves the SPA and proxies these paths itself.
const apiTarget = process.env.API_PROXY_TARGET || "http://localhost:8080";
const proxy = {
  "/auth": { target: apiTarget, changeOrigin: false },
  "/sample.v1.SampleService": { target: apiTarget, changeOrigin: false },
  "/sample.v1.PartnerService": { target: apiTarget, changeOrigin: false },
  "/sample.v1.OnboardingService": { target: apiTarget, changeOrigin: false },
  "/adiom.auth.v1.AuthService": { target: apiTarget, changeOrigin: false },
};

export default defineConfig({
  plugins: [react()],
  server: { proxy },
  build: {
    outDir: "dist",
    emptyOutDir: true,
  },
});

