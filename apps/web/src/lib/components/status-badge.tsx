import { StatusPill } from "@/components/ui/status-pill";

/**
 * Thin wrapper kept for existing call sites. The design-system StatusPill
 * owns the status→tone mapping and animated dots; this just forwards.
 */
export function StatusBadge({ status }: { status: string }) {
  return <StatusPill status={status} />;
}
