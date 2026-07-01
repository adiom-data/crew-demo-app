import { useState, type FormEvent } from "react";
import { onboardingClient } from "../api/clients";
import { errorMessage } from "../session";

// PublicOnboarding is the unauthenticated self-serve form. Submissions land as
// Pending for an admin to review.
export function PublicOnboarding() {
  const [name, setName] = useState("");
  const [contactEmail, setContactEmail] = useState("");
  const [company, setCompany] = useState("");
  const [region, setRegion] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const [done, setDone] = useState(false);

  function validate(): string | null {
    if (!name.trim()) return "Name is required.";
    const email = contactEmail.trim();
    if (!email) return "Contact email is required.";
    if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email)) return "Contact email is not valid.";
    return null;
  }

  async function onSubmit(event: FormEvent) {
    event.preventDefault();
    const validationError = validate();
    if (validationError) {
      setError(validationError);
      return;
    }
    setError(null);
    setSubmitting(true);
    try {
      await onboardingClient.submitOnboarding({
        name: name.trim(),
        contactEmail: contactEmail.trim(),
        company: company.trim(),
        region: region.trim(),
      });
      setDone(true);
    } catch (err) {
      setError(errorMessage(err));
      setSubmitting(false);
    }
  }

  return (
    <main className="shell">
      <section className="panel" aria-live="polite">
        <div className="mast">
          <p className="eyebrow">Partner onboarding</p>
          <h1>Join us</h1>
        </div>

        {done ? (
          <div className="stack">
            <p className="lead">
              Thanks, {name.trim()}! Your request has been received and is now <strong>pending</strong>{" "}
              review. Our team will be in touch at {contactEmail.trim()}.
            </p>
          </div>
        ) : (
          <form className="form" onSubmit={onSubmit} noValidate>
            {error && <p className="error">{error}</p>}
            <p className="lead">Tell us a bit about your organization to get started.</p>
            <label className="field">
              <span className="field-label">
                Your name<span className="req"> *</span>
              </span>
              <input value={name} onChange={(e) => setName(e.target.value)} autoFocus />
            </label>
            <label className="field">
              <span className="field-label">
                Contact email<span className="req"> *</span>
              </span>
              <input type="email" value={contactEmail} onChange={(e) => setContactEmail(e.target.value)} />
            </label>
            <label className="field">
              <span className="field-label">Company</span>
              <input value={company} onChange={(e) => setCompany(e.target.value)} />
            </label>
            <label className="field">
              <span className="field-label">Region</span>
              <input value={region} onChange={(e) => setRegion(e.target.value)} />
            </label>
            <div className="form-actions">
              <button className="button primary" type="submit" disabled={submitting}>
                {submitting ? "Submitting…" : "Submit request"}
              </button>
            </div>
          </form>
        )}
      </section>
    </main>
  );
}
