import { describe, expect, it } from "vitest";
import { BillingStatus, Status, Tier } from "../gen/sample/v1/partner_pb";
import { billingLabel, formatDate, parseTier, statusLabel, statusModifier, tierLabel } from "./format";

describe("labels", () => {
  it("labels tiers", () => {
    expect(tierLabel(Tier.STARTER)).toBe("Starter");
    expect(tierLabel(Tier.ENTERPRISE)).toBe("Enterprise");
    expect(tierLabel(Tier.UNSPECIFIED)).toBe("—");
  });

  it("labels statuses and modifiers", () => {
    expect(statusLabel(Status.PENDING)).toBe("Pending");
    expect(statusModifier(Status.CHURNED)).toBe("churned");
    expect(statusModifier(Status.UNSPECIFIED)).toBe("unknown");
  });

  it("labels billing", () => {
    expect(billingLabel(BillingStatus.PAST_DUE)).toBe("Past due");
  });
});

describe("parseTier", () => {
  it("maps text to enum", () => {
    expect(parseTier("pro")).toBe(Tier.PRO);
    expect(parseTier("ENTERPRISE")).toBe(Tier.ENTERPRISE);
    expect(parseTier("  starter ")).toBe(Tier.STARTER);
  });

  it("defaults unknown/blank to starter", () => {
    expect(parseTier("")).toBe(Tier.STARTER);
    expect(parseTier("gold")).toBe(Tier.STARTER);
  });
});

describe("formatDate", () => {
  it("returns a dash for empty input", () => {
    expect(formatDate("")).toBe("—");
  });

  it("passes through unparseable input", () => {
    expect(formatDate("not-a-date")).toBe("not-a-date");
  });

  it("formats a valid ISO date", () => {
    expect(formatDate("2026-06-01T12:00:00Z")).toContain("2026");
  });
});
