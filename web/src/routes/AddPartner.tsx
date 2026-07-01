import { useState, type FormEvent, type ReactNode } from "react";
import { Link, useNavigate } from "react-router-dom";
import { partnerClient } from "../api/clients";
import { Tier } from "../gen/sample/v1/partner_pb";
import { errorMessage } from "../session";
import { TIER_OPTIONS } from "../lib/format";

export function AddPartner() {
  const navigate = useNavigate();
  const [name, setName] = useState("");
  const [contactEmail, setContactEmail] = useState("");
  const [company, setCompany] = useState("");
  const [region, setRegion] = useState("");
  const [tier, setTier] = useState<Tier>(Tier.STARTER);
  const [notes, setNotes] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

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
      const res = await partnerClient.createPartner({
        name: name.trim(),
        contactEmail: contactEmail.trim(),
        company: company.trim(),
        region: region.trim(),
        tier,
        notes: notes.trim(),
      });
      navigate(`/partners/${res.partner?.id ?? ""}`);
    } catch (err) {
      setError(errorMessage(err));
      setSubmitting(false);
    }
  }

  return (
    <div className="page page-narrow">
      <div className="page-head">
        <h2 className="page-title">Add Partner</h2>
        <Link className="button" to="/dashboard">
          Cancel
        </Link>
      </div>

      <form className="form" onSubmit={onSubmit} noValidate>
        {error && <p className="error">{error}</p>}
        <Field label="Name" required>
          <input value={name} onChange={(e) => setName(e.target.value)} autoFocus />
        </Field>
        <Field label="Contact email" required>
          <input type="email" value={contactEmail} onChange={(e) => setContactEmail(e.target.value)} />
        </Field>
        <div className="form-row">
          <Field label="Company">
            <input value={company} onChange={(e) => setCompany(e.target.value)} />
          </Field>
          <Field label="Region">
            <input value={region} onChange={(e) => setRegion(e.target.value)} />
          </Field>
        </div>
        <Field label="Tier">
          <select value={tier} onChange={(e) => setTier(Number(e.target.value) as Tier)}>
            {TIER_OPTIONS.map((opt) => (
              <option key={opt.value} value={opt.value}>
                {opt.label}
              </option>
            ))}
          </select>
        </Field>
        <Field label="Notes">
          <textarea rows={3} value={notes} onChange={(e) => setNotes(e.target.value)} />
        </Field>
        <div className="form-actions">
          <button className="button primary" type="submit" disabled={submitting}>
            {submitting ? "Saving…" : "Create partner"}
          </button>
        </div>
      </form>
    </div>
  );
}

function Field({
  label,
  required,
  children,
}: {
  label: string;
  required?: boolean;
  children: ReactNode;
}) {
  return (
    <label className="field">
      <span className="field-label">
        {label}
        {required && <span className="req"> *</span>}
      </span>
      {children}
    </label>
  );
}
