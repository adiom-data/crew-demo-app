import type { Status } from "../gen/sample/v1/partner_pb";
import { statusLabel, statusModifier } from "../lib/format";

export function StatusBadge({ status }: { status: Status }) {
  return <span className={`badge badge-${statusModifier(status)}`}>{statusLabel(status)}</span>;
}
