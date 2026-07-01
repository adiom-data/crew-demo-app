import { useState, type ChangeEvent } from "react";
import { Link } from "react-router-dom";
import { partnerClient } from "../api/clients";
import type { RowError } from "../gen/sample/v1/partner_pb";
import { errorMessage } from "../session";
import { csvToPartnerRows, type CsvPartnerRow } from "../lib/csv";
import { parseTier } from "../lib/format";

interface ImportResult {
  imported: number;
  errors: RowError[];
}

export function BulkImport() {
  const [rows, setRows] = useState<CsvPartnerRow[] | null>(null);
  const [fileName, setFileName] = useState("");
  const [parseError, setParseError] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [result, setResult] = useState<ImportResult | null>(null);
  const [submitting, setSubmitting] = useState(false);

  async function onFile(event: ChangeEvent<HTMLInputElement>) {
    const file = event.target.files?.[0];
    setResult(null);
    setError(null);
    setParseError(null);
    setRows(null);
    if (!file) return;
    setFileName(file.name);
    const text = await file.text();
    const parsed = csvToPartnerRows(text);
    if (parsed.error) {
      setParseError(parsed.error);
      return;
    }
    if (parsed.rows.length === 0) {
      setParseError("No data rows found below the header.");
      return;
    }
    setRows(parsed.rows);
  }

  async function onImport() {
    if (!rows) return;
    setSubmitting(true);
    setError(null);
    try {
      const res = await partnerClient.bulkImportPartners({
        rows: rows.map((r) => ({
          name: r.name,
          contactEmail: r.contactEmail,
          company: r.company,
          region: r.region,
          tier: parseTier(r.tier),
        })),
      });
      setResult({ imported: res.imported, errors: res.errors });
      setRows(null);
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div className="page page-narrow">
      <div className="page-head">
        <h2 className="page-title">Bulk Import</h2>
        <Link className="button" to="/dashboard">
          Done
        </Link>
      </div>

      <p className="muted">
        Upload a CSV with columns <code>name, contact_email, company, region, tier</code>. The first
        row must be a header. Rows that fail validation are reported below; valid rows are imported.
      </p>

      <div className="uploader">
        <label className="button">
          Choose CSV
          <input type="file" accept=".csv,text/csv" onChange={onFile} hidden />
        </label>
        {fileName && <span className="muted">{fileName}</span>}
      </div>

      {parseError && <p className="error">{parseError}</p>}
      {error && <p className="error">{error}</p>}

      {rows && (
        <div className="stack">
          <p className="lead">
            {rows.length} row{rows.length === 1 ? "" : "s"} ready to import.
          </p>
          <div className="form-actions">
            <button className="button primary" onClick={onImport} disabled={submitting}>
              {submitting ? "Importing…" : `Import ${rows.length}`}
            </button>
          </div>
        </div>
      )}

      {result && (
        <div className="stack">
          <p className="lead">
            Imported <strong>{result.imported}</strong> partner{result.imported === 1 ? "" : "s"}.
          </p>
          {result.errors.length > 0 ? (
            <div className="report">
              <p className="report-title">{result.errors.length} row(s) skipped:</p>
              <ul className="report-list">
                {result.errors.map((e) => (
                  <li key={e.row}>
                    Row {e.row}: {e.message}
                  </li>
                ))}
              </ul>
            </div>
          ) : (
            <p className="muted">All rows imported cleanly.</p>
          )}
          <div className="form-actions">
            <Link className="button" to="/dashboard">
              View dashboard
            </Link>
          </div>
        </div>
      )}
    </div>
  );
}
