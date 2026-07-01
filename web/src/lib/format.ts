import { BillingStatus, Status, Tier } from "../gen/sample/v1/partner_pb";

export function tierLabel(tier: Tier): string {
  switch (tier) {
    case Tier.STARTER:
      return "Starter";
    case Tier.PRO:
      return "Pro";
    case Tier.ENTERPRISE:
      return "Enterprise";
    default:
      return "—";
  }
}

export function statusLabel(status: Status): string {
  switch (status) {
    case Status.PENDING:
      return "Pending";
    case Status.ACTIVE:
      return "Active";
    case Status.CHURNED:
      return "Churned";
    default:
      return "—";
  }
}

// statusModifier returns the CSS modifier suffix for a status badge.
export function statusModifier(status: Status): string {
  switch (status) {
    case Status.PENDING:
      return "pending";
    case Status.ACTIVE:
      return "active";
    case Status.CHURNED:
      return "churned";
    default:
      return "unknown";
  }
}

export function billingLabel(billing: BillingStatus): string {
  switch (billing) {
    case BillingStatus.CURRENT:
      return "Current";
    case BillingStatus.PAST_DUE:
      return "Past due";
    case BillingStatus.TRIALING:
      return "Trialing";
    default:
      return "—";
  }
}

// parseTier maps a free-text tier (e.g. from a CSV cell) to the Tier enum.
// Unknown/blank values map to STARTER so imports still succeed.
export function parseTier(text: string): Tier {
  switch (text.trim().toLowerCase()) {
    case "pro":
      return Tier.PRO;
    case "enterprise":
      return Tier.ENTERPRISE;
    case "starter":
    case "":
      return Tier.STARTER;
    default:
      return Tier.STARTER;
  }
}

export function formatDate(iso: string): string {
  if (!iso) {
    return "—";
  }
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) {
    return iso;
  }
  return date.toLocaleDateString(undefined, {
    year: "numeric",
    month: "short",
    day: "numeric",
  });
}

export const TIER_OPTIONS: { value: Tier; label: string }[] = [
  { value: Tier.STARTER, label: "Starter" },
  { value: Tier.PRO, label: "Pro" },
  { value: Tier.ENTERPRISE, label: "Enterprise" },
];

export const STATUS_OPTIONS: { value: Status; label: string }[] = [
  { value: Status.PENDING, label: "Pending" },
  { value: Status.ACTIVE, label: "Active" },
  { value: Status.CHURNED, label: "Churned" },
];
