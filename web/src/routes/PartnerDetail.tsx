import { useCallback, useEffect, useState } from "react";
import { Link, useParams, useSearchParams } from "react-router-dom";
import { billingClient, partnerClient } from "../api/clients";
import {
  Status,
  SubscriptionStatus,
  type Activity,
  type Partner,
  type SubscriptionPlan,
} from "../gen/sample/v1/partner_pb";
import { errorMessage } from "../session";
import {
  billingLabel,
  formatDate,
  planLabel,
  PLAN_OPTIONS,
  STATUS_OPTIONS,
  subscriptionLabel,
  tierLabel,
} from "../lib/format";
import { StatusBadge } from "../components/StatusBadge";

export function PartnerDetail() {
  const { id = "" } = useParams();
  const [searchParams, setSearchParams] = useSearchParams();
  const [partner, setPartner] = useState<Partner | null>(null);
  const [activities, setActivities] = useState<Activity[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const checkout = searchParams.get("checkout");

  const load = useCallback(async () => {
    try {
      const res = await partnerClient.getPartner({ id });
      setPartner(res.partner ?? null);
      setActivities(res.activities);
      return res.partner ?? null;
    } catch (err) {
      setError(errorMessage(err));
      return null;
    }
  }, [id]);

  useEffect(() => {
    void load();
  }, [load]);

  // Stripe redirects the browser back as soon as payment succeeds, which can beat
  // the checkout.session.completed webhook. Re-fetch a few times so the page
  // doesn't sit on "Not subscribed" right after a successful checkout.
  useEffect(() => {
    if (checkout !== "success") return;
    let cancelled = false;
    let attempts = 0;
    const tick = async () => {
      if (cancelled) return;
      const p = await load();
      attempts += 1;
      if (p && p.subscriptionStatus !== SubscriptionStatus.UNSPECIFIED) return;
      if (attempts < 6) setTimeout(() => void tick(), 1000);
    };
    void tick();
    return () => {
      cancelled = true;
    };
  }, [checkout, load]);

  function dismissBanner() {
    searchParams.delete("checkout");
    setSearchParams(searchParams, { replace: true });
  }

  async function subscribe(plan: SubscriptionPlan) {
    setBusy(true);
    setError(null);
    try {
      const res = await billingClient.createCheckoutSession({ partnerId: id, plan });
      window.location.href = res.checkoutUrl;
    } catch (err) {
      setError(errorMessage(err));
      setBusy(false);
    }
  }

  async function changeStatus(status: Status) {
    if (!partner || status === partner.status) return;
    setBusy(true);
    setError(null);
    try {
      await partnerClient.updatePartnerStatus({ id, status });
      await load();
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setBusy(false);
    }
  }

  if (error && !partner) {
    return (
      <div className="page page-narrow">
        <p className="error">{error}</p>
        <Link className="button" to="/dashboard">
          Back to dashboard
        </Link>
      </div>
    );
  }

  if (!partner) {
    return (
      <div className="page page-narrow">
        <p className="muted">Loading partner…</p>
      </div>
    );
  }

  return (
    <div className="page page-narrow">
      <div className="page-head">
        <div>
          <Link className="back-link" to="/dashboard">
            ← Dashboard
          </Link>
          <h2 className="page-title">{partner.name}</h2>
        </div>
        <StatusBadge status={partner.status} />
      </div>

      {error && <p className="error">{error}</p>}

      {checkout === "success" && (
        <p className="notice" onClick={dismissBanner}>
          Payment received. Activating the subscription…
        </p>
      )}
      {checkout === "cancel" && (
        <p className="notice" onClick={dismissBanner}>
          Checkout was cancelled. No charge was made.
        </p>
      )}

      <dl className="facts">
        <div>
          <dt>Company</dt>
          <dd>{partner.company || "—"}</dd>
        </div>
        <div>
          <dt>Contact</dt>
          <dd>{partner.contactEmail}</dd>
        </div>
        <div>
          <dt>Region</dt>
          <dd>{partner.region || "—"}</dd>
        </div>
        <div>
          <dt>Tier</dt>
          <dd>{tierLabel(partner.tier)}</dd>
        </div>
        <div>
          <dt>Billing</dt>
          <dd>{billingLabel(partner.billingStatus)}</dd>
        </div>
        <div>
          <dt>Subscription</dt>
          <dd>
            {partner.subscriptionStatus === SubscriptionStatus.UNSPECIFIED
              ? "Not subscribed"
              : `${planLabel(partner.subscriptionPlan)} · ${subscriptionLabel(partner.subscriptionStatus)}`}
          </dd>
        </div>
        <div>
          <dt>Joined</dt>
          <dd>{formatDate(partner.createdAt)}</dd>
        </div>
        {partner.notes && (
          <div>
            <dt>Notes</dt>
            <dd>{partner.notes}</dd>
          </div>
        )}
      </dl>

      <section className="detail-section">
        <h3 className="section-title">Subscription</h3>
        {partner.subscriptionStatus === SubscriptionStatus.ACTIVE ? (
          <p className="muted">
            Subscribed on the {planLabel(partner.subscriptionPlan).toLowerCase()} plan.
          </p>
        ) : (
          <div className="actions">
            {PLAN_OPTIONS.map((opt) => (
              <button
                key={opt.value}
                className="button primary"
                disabled={busy}
                onClick={() => subscribe(opt.value)}
              >
                {opt.label}
              </button>
            ))}
          </div>
        )}
      </section>

      <section className="detail-section">
        <h3 className="section-title">Change status</h3>
        <div className="actions">
          {STATUS_OPTIONS.map((opt) => (
            <button
              key={opt.value}
              className={`button${opt.value === partner.status ? " primary" : ""}`}
              disabled={busy || opt.value === partner.status}
              onClick={() => changeStatus(opt.value)}
            >
              {opt.label}
            </button>
          ))}
        </div>
      </section>

      <section className="detail-section">
        <h3 className="section-title">Activity</h3>
        {activities.length === 0 ? (
          <p className="muted">No activity yet.</p>
        ) : (
          <ul className="activity">
            {activities.map((a) => (
              <li key={a.id} className="activity-item">
                <span className="activity-dot" />
                <div>
                  <p className="activity-message">{a.message}</p>
                  <p className="activity-meta">
                    {a.type} · {formatDate(a.createdAt)}
                  </p>
                </div>
              </li>
            ))}
          </ul>
        )}
      </section>
    </div>
  );
}
