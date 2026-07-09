import { useEffect, useMemo, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { partnerClient } from "../api/clients";
import { Status, type Partner } from "../gen/sample/v1/partner_pb";
import { errorMessage } from "../session";
import { STATUS_OPTIONS, formatDate, statusLabel, tierLabel } from "../lib/format";
import { partnersToCsv } from "../lib/csv";
import { StatusBadge } from "../components/StatusBadge";

interface Summary {
  total: number;
  active: number;
  pending: number;
}

type StatusFilter = "all" | Status;

export function Dashboard() {
  const navigate = useNavigate();
  const [partners, setPartners] = useState<Partner[] | null>(null);
  const [summary, setSummary] = useState<Summary>({ total: 0, active: 0, pending: 0 });
  const [statusFilter, setStatusFilter] = useState<StatusFilter>("all");
  const [error, setError] = useState<string | null>(null);

  const filteredPartners = useMemo(() => {
    if (partners === null || statusFilter === "all") {
      return partners ?? [];
    }
    return partners.filter((p) => p.status === statusFilter);
  }, [partners, statusFilter]);

  const onExportCsv = () => {
    const csv = partnersToCsv(filteredPartners);
    const blob = new Blob([csv], { type: "text/csv;charset=utf-8" });
    const url = URL.createObjectURL(blob);
    const link = document.createElement("a");
    link.href = url;
    link.download = "partners.csv";
    link.click();
    URL.revokeObjectURL(url);
  };

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
        <div className="dashboard-section">
          <div className="dashboard-toolbar">
            <label className="field filter-field">
              <span className="field-label">Status</span>
              <select
                value={statusFilter}
                onChange={(event) => {
                  const value = event.target.value;
                  setStatusFilter(value === "all" ? "all" : Number(value));
                }}
              >
                <option value="all">All statuses</option>
                {STATUS_OPTIONS.map((option) => (
                  <option key={option.value} value={option.value}>
                    {option.label}
                  </option>
                ))}
              </select>
            </label>
            <button className="button" type="button" onClick={onExportCsv} disabled={filteredPartners.length === 0}>
              Export CSV
            </button>
            <p className="muted dashboard-count">
              Showing {filteredPartners.length} of {partners.length} partners
            </p>
          </div>

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
                {partners.length > 0 && filteredPartners.length === 0 && (
                  <tr>
                    <td colSpan={6} className="empty">
                      No partners match the {statusLabel(statusFilter as Status).toLowerCase()} filter.{" "}
                      <button className="link-button" type="button" onClick={() => setStatusFilter("all")}>
                        Clear filter
                      </button>
                    </td>
                  </tr>
                )}
                {filteredPartners.map((p) => (
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
