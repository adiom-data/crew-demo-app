import { useCallback, useEffect, useState } from "react";
import { Link, useParams } from "react-router-dom";
import { partnerClient } from "../api/clients";
import { Status, type Activity, type Partner } from "../gen/sample/v1/partner_pb";
import { errorMessage } from "../session";
import { billingLabel, formatDate, STATUS_OPTIONS, tierLabel } from "../lib/format";
import { StatusBadge } from "../components/StatusBadge";

export function PartnerDetail() {
  const { id = "" } = useParams();
  const [partner, setPartner] = useState<Partner | null>(null);
  const [activities, setActivities] = useState<Activity[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const load = useCallback(async () => {
    try {
      const res = await partnerClient.getPartner({ id });
      setPartner(res.partner ?? null);
      setActivities(res.activities);
    } catch (err) {
      setError(errorMessage(err));
    }
  }, [id]);

  useEffect(() => {
    void load();
  }, [load]);

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
