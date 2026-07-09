import { describe, expect, it } from "vitest";
import { create } from "@bufbuild/protobuf";
import { BillingStatus, PartnerSchema, Status, Tier } from "../gen/sample/v1/partner_pb";
import { csvToPartnerRows, parseCsv, partnersToCsv, rowsToCsv } from "./csv";

describe("parseCsv", () => {
  it("parses simple rows", () => {
    expect(parseCsv("a,b,c\n1,2,3")).toEqual([
      ["a", "b", "c"],
      ["1", "2", "3"],
    ]);
  });

  it("handles quoted fields with commas", () => {
    expect(parseCsv('name,note\n"Acme, Inc","hi, there"')).toEqual([
      ["name", "note"],
      ["Acme, Inc", "hi, there"],
    ]);
  });

  it("handles escaped quotes", () => {
    expect(parseCsv('q\n"she said ""hi"""')).toEqual([["q"], ['she said "hi"']]);
  });

  it("handles CRLF line endings", () => {
    expect(parseCsv("a,b\r\n1,2\r\n")).toEqual([
      ["a", "b"],
      ["1", "2"],
    ]);
  });

  it("drops fully blank lines but keeps short rows", () => {
    expect(parseCsv("a,b\n\n1\n")).toEqual([["a", "b"], ["1"]]);
  });

  it("returns empty for empty input", () => {
    expect(parseCsv("")).toEqual([]);
  });
});

describe("csvToPartnerRows", () => {
  it("maps header columns to partner fields", () => {
    const result = csvToPartnerRows(
      "name,contact_email,company,region,tier\nAcme,ops@acme.com,Acme Inc,US,pro",
    );
    expect(result.error).toBeUndefined();
    expect(result.rows).toEqual([
      { name: "Acme", contactEmail: "ops@acme.com", company: "Acme Inc", region: "US", tier: "pro" },
    ]);
  });

  it("accepts header aliases and reordering", () => {
    const result = csvToPartnerRows("Email,Name\nops@acme.com,Acme");
    expect(result.error).toBeUndefined();
    expect(result.rows[0]).toMatchObject({ name: "Acme", contactEmail: "ops@acme.com" });
  });

  it("errors when required headers are missing", () => {
    const result = csvToPartnerRows("company,region\nAcme Inc,US");
    expect(result.error).toBeDefined();
    expect(result.rows).toHaveLength(0);
  });

  it("errors on empty input", () => {
    expect(csvToPartnerRows("").error).toBeDefined();
  });

  it("tolerates ragged short rows", () => {
    const result = csvToPartnerRows("name,contact_email,company\nAcme,ops@acme.com");
    expect(result.rows[0]).toMatchObject({ name: "Acme", contactEmail: "ops@acme.com", company: "" });
  });
});

describe("rowsToCsv", () => {
  it("serializes simple rows", () => {
    expect(
      rowsToCsv([
        ["a", "b"],
        ["1", "2"],
      ]),
    ).toBe("a,b\n1,2");
  });

  it("quotes commas, newlines, CRs, and escaped quotes", () => {
    expect(rowsToCsv([["Acme, Inc", 'she said "hi"', "line\nbreak", "carriage\rreturn"]])).toBe(
      '"Acme, Inc","she said ""hi""","line\nbreak","carriage\rreturn"',
    );
  });
});

describe("partnersToCsv", () => {
  it("exports stable headers and formatted partner fields", () => {
    const partner = create(PartnerSchema, {
      id: "partner-1",
      name: "Acme, Inc",
      contactEmail: "ops@acme.com",
      company: "Acme Holdings",
      region: "North\nAmerica",
      tier: Tier.ENTERPRISE,
      status: Status.ACTIVE,
      billingStatus: BillingStatus.PAST_DUE,
      createdAt: "2026-02-03T12:00:00Z",
    });

    expect(partnersToCsv([partner])).toBe(
      [
        "name,contact_email,company,region,tier,status,billing_status,joined",
        '"Acme, Inc",ops@acme.com,Acme Holdings,"North\nAmerica",Enterprise,Active,Past due,"Feb 3, 2026"',
      ].join("\n"),
    );
  });

  it("exports empty strings for missing optional values", () => {
    const partner = create(PartnerSchema, {
      name: "Acme",
      contactEmail: "ops@acme.com",
      tier: Tier.STARTER,
      status: Status.PENDING,
      billingStatus: BillingStatus.TRIALING,
    });

    expect(partnersToCsv([partner]).split("\n")[1]).toBe("Acme,ops@acme.com,,,Starter,Pending,Trialing,—");
  });
});
