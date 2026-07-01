import { useEffect, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { partnerClient } from "../api/clients";
import type { Partner } from "../gen/sample/v1/partner_pb";
import { errorMessage } from "../session";
import { formatDate, tierLabel } from "../lib/format";
import { StatusBadge } from "../components/StatusBadge";

interface Summary {
  total: number;
  active: number;
  pending: number;
}

export function Dashboard() {
  const navigate = useNavigate();
  const [partners, setPartners] = useState<Partner[] | null>(null);
  const [summary, setSummary] = useState<Summary>({ total: 0, active: 0, pending: 0 });
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const res = await partnerClient.listPartners({});
        if (cancelled) return;
        setPartners(res.partners);
        setSummary({ total: res.total, active: res.active, pending: res.pending });
      } catch (err) {
        if (!cancelled) setError(errorMessage(err));
      }
    })();
    return () => {
      cancelled = true;
    };
  }, []);

  return (
    <div className="page">
      <div className="page-head">
        <div>
          <h2 className="page-title">Dashboard</h2>
          <p className="muted">Partner onboarding &amp; account health.</p>
        </div>
        <Link className="button primary" to="/partners/new">
          Add Partner
        </Link>
      </div>

      <div className="cards">
        <SummaryCard label="Total partners" value={summary.total} />
        <SummaryCard label="Active" value={summary.active} tone="active" />
        <SummaryCard label="Pending" value={summary.pending} tone="pending" />
      </div>

      {error && <p className="error">{error}</p>}

      {!error && partners === null && <p className="muted">Loading partners…</p>}

      {!error && partners !== null && (
        <div className="table-wrap">
          <table className="table">
            <thead>
              <tr>
                <th>Name</th>
                <th>Company</th>
                <th>Region</th>
                <th>Tier</th>
                <th>Status</th>
                <th>Joined</th>
              </tr>
            </thead>
            <tbody>
              {partners.length === 0 && (
                <tr>
                  <td colSpan={6} className="empty">
                    No partners yet. <Link to="/partners/new">Add one</Link> or run the seed script.
                  </td>
                </tr>
              )}
              {partners.map((p) => (
                <tr key={p.id} className="row-link" onClick={() => navigate(`/partners/${p.id}`)}>
                  <td className="cell-strong">{p.name}</td>
                  <td>{p.company || "—"}</td>
                  <td>{p.region || "—"}</td>
                  <td>{tierLabel(p.tier)}</td>
                  <td>
                    <StatusBadge status={p.status} />
                  </td>
                  <td>{formatDate(p.createdAt)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}

function SummaryCard({ label, value, tone }: { label: string; value: number; tone?: string }) {
  return (
    <div className={`card${tone ? ` card-${tone}` : ""}`}>
      <span className="card-value">{value}</span>
      <span className="card-label">{label}</span>
    </div>
  );
}
