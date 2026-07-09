import type { Partner } from "../gen/sample/v1/partner_pb";
import { billingLabel, formatDate, statusLabel, tierLabel } from "./format";

// parseCsv parses RFC 4180-ish CSV text into rows of string cells. It handles
// quoted fields, escaped quotes (""), embedded commas/newlines, and CRLF. Fully
// blank lines are dropped.
export function parseCsv(input: string): string[][] {
  const rows: string[][] = [];
  let field = "";
  let row: string[] = [];
  let inQuotes = false;
  let i = 0;

  const endField = () => {
    row.push(field);
    field = "";
  };
  const endRow = () => {
    endField();
    rows.push(row);
    row = [];
  };

  while (i < input.length) {
    const c = input[i];
    if (inQuotes) {
      if (c === '"') {
        if (input[i + 1] === '"') {
          field += '"';
          i += 2;
          continue;
        }
        inQuotes = false;
        i += 1;
        continue;
      }
      field += c;
      i += 1;
      continue;
    }
    if (c === '"') {
      inQuotes = true;
      i += 1;
      continue;
    }
    if (c === ",") {
      endField();
      i += 1;
      continue;
    }
    if (c === "\r") {
      i += 1;
      continue;
    }
    if (c === "\n") {
      endRow();
      i += 1;
      continue;
    }
    field += c;
    i += 1;
  }
  if (field !== "" || row.length > 0) {
    endRow();
  }

  return rows.filter((r) => !(r.length === 1 && r[0].trim() === ""));
}

export interface CsvPartnerRow {
  name: string;
  contactEmail: string;
  company: string;
  region: string;
  tier: string;
}

export function rowsToCsv(rows: string[][]): string {
  return rows
    .map((row) =>
      row
        .map((cell) => {
          if (/[",\r\n]/.test(cell)) {
            return `"${cell.replaceAll('"', '""')}"`;
          }
          return cell;
        })
        .join(","),
    )
    .join("\n");
}

export function partnersToCsv(partners: Partner[]): string {
  return rowsToCsv([
    ["name", "contact_email", "company", "region", "tier", "status", "billing_status", "joined"],
    ...partners.map((partner) => [
      partner.name,
      partner.contactEmail,
      partner.company,
      partner.region,
      tierLabel(partner.tier),
      statusLabel(partner.status),
      billingLabel(partner.billingStatus),
      formatDate(partner.createdAt),
    ]),
  ]);
}

// Header aliases we accept, mapped to canonical field keys.
const HEADER_ALIASES: Record<string, keyof CsvPartnerRow> = {
  name: "name",
  contact_email: "contactEmail",
  contactemail: "contactEmail",
  email: "contactEmail",
  company: "company",
  region: "region",
  tier: "tier",
};

export interface CsvParseResult {
  rows: CsvPartnerRow[];
  error?: string;
}

// rowsToPartnerRows interprets the first row as a header and maps subsequent rows
// to partner fields. Returns an error string if required headers are missing.
export function csvToPartnerRows(input: string): CsvParseResult {
  const parsed = parseCsv(input);
  if (parsed.length === 0) {
    return { rows: [], error: "The file is empty." };
  }

  const header = parsed[0].map((h) => h.trim().toLowerCase());
  const index: Partial<Record<keyof CsvPartnerRow, number>> = {};
  header.forEach((h, i) => {
    const key = HEADER_ALIASES[h];
    if (key && index[key] === undefined) {
      index[key] = i;
    }
  });

  if (index.name === undefined || index.contactEmail === undefined) {
    return {
      rows: [],
      error: "CSV must include at least 'name' and 'contact_email' columns.",
    };
  }

  const cell = (cols: string[], i?: number) => (i === undefined ? "" : (cols[i] ?? "").trim());
  const rows = parsed.slice(1).map((cols) => ({
    name: cell(cols, index.name),
    contactEmail: cell(cols, index.contactEmail),
    company: cell(cols, index.company),
    region: cell(cols, index.region),
    tier: cell(cols, index.tier),
  }));

  return { rows };
}
