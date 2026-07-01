import type { SessionState } from "../session";

// Login renders the public sign-in card. The actual OIDC flow is owned by the
// backend BFF at /auth/login.
export function Login({ state, onRetry }: { state: SessionState; onRetry: () => void }) {
  return (
    <main className="shell">
      <section className="panel" aria-live="polite">
        <div className="mast">
          <p className="eyebrow">Adiom · On-board</p>
          <h1>Partner Portal</h1>
        </div>

        {state.status === "loading" && <p className="muted">Checking session…</p>}

        {(state.status === "anonymous" || state.status === "authenticated") && (
          <div className="stack">
            <p className="lead">
              Sign in to manage partners, review onboarding submissions, and see your dashboard.
            </p>
            <a className="button primary" href="/auth/login">
              Sign in
            </a>
            <p className="muted">
              A partner instead? <a href="/onboard">Submit an onboarding request →</a>
            </p>
          </div>
        )}

        {state.status === "error" && (
          <div className="stack">
            <p className="error">{state.message}</p>
            <button className="button" onClick={onRetry}>
              Try again
            </button>
          </div>
        )}
      </section>
    </main>
  );
}
