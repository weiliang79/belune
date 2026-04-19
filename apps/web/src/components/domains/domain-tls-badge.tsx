import { Badge } from "@/components/ui/badge";
import type { DomainExpanded } from "@/lib/types";

type Status = "active" | "pending" | "custom" | "off" | "error";

function statusFor(domain: DomainExpanded): Status {
  const mode = domain.ssl_mode ?? "automatic";
  if (mode === "off") return "off";
  if (mode === "custom") {
    if (domain.cert_path && domain.key_path) return "custom";
    return "error";
  }
  if (mode === "automatic" || mode === "dns_challenge") {
    return domain.verified_at ? "active" : "pending";
  }
  return "error";
}

const STYLES: Record<
  Status,
  { label: string; variant: "default" | "secondary" | "outline" | "destructive"; className?: string }
> = {
  active: {
    label: "Active",
    variant: "default",
    className: "bg-emerald-600 hover:bg-emerald-600",
  },
  pending: {
    label: "Pending",
    variant: "default",
    className: "bg-amber-500 hover:bg-amber-500",
  },
  custom: { label: "Custom", variant: "secondary" },
  off: { label: "Off", variant: "outline" },
  error: { label: "Error", variant: "destructive" },
};

export function DomainTLSBadge({ domain }: { domain: DomainExpanded }) {
  const status = statusFor(domain);
  const style = STYLES[status];
  return (
    <Badge variant={style.variant} className={style.className}>
      TLS: {style.label}
    </Badge>
  );
}
