import { useState } from "react";
import { toast } from "sonner";
import { useQuery } from "@tanstack/react-query";
import { RefreshCw } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { useSettings, useUpdateSettings } from "@/lib/hooks/use-settings";
import { api } from "@/lib/api/client";

interface DashboardTLS {
  domain: string;
  tls_status: string;
  tls_issuer?: string;
  tls_not_after: string | null;
  tls_error?: string;
}

const STATUS_STYLES: Record<
  string,
  {
    label: string;
    variant: "default" | "secondary" | "outline" | "destructive";
    className?: string;
  }
> = {
  active: {
    label: "HTTPS active",
    variant: "default",
    className: "bg-emerald-600 hover:bg-emerald-600",
  },
  pending: {
    label: "Waiting for certificate",
    variant: "default",
    className: "bg-amber-500 hover:bg-amber-500",
  },
  expiring: {
    label: "Expiring",
    variant: "default",
    className: "bg-amber-500 hover:bg-amber-500",
  },
  expired: { label: "Expired", variant: "destructive" },
  failed: { label: "Failed", variant: "destructive" },
  unknown: { label: "Not set", variant: "secondary" },
};

/**
 * The dashboard's own domain.
 *
 * Belune is reachable on the server's bare IP over plain HTTP out of the box —
 * that is how you get in to create the first admin. Naming a domain here gives
 * the dashboard its own route in the proxy, which is what lets Caddy obtain a
 * Let's Encrypt certificate for it: certificates are only ever issued for
 * hostnames the proxy has been told about.
 */
export function DashboardDomainSection() {
  const { data: settings } = useSettings();
  const updateSettings = useUpdateSettings();

  const saved =
    settings?.find((s) => s.key === "dashboard_domain")?.value?.trim() ?? "";
  const [draft, setDraft] = useState<string | null>(null);
  const value = draft ?? saved;
  const dirty = value.trim() !== saved;

  const { data: tls, refetch: refetchTLS } = useQuery({
    queryKey: ["dashboard-tls", saved],
    queryFn: () => api.get<DashboardTLS>("/server/dashboard-tls"),
    // A certificate normally lands within a minute of the DNS being right, so
    // poll while the operator is watching rather than making them refresh.
    refetchInterval: saved ? 15000 : false,
    enabled: Boolean(saved),
  });

  const save = (next: string) => {
    toast.promise(
      updateSettings
        .mutateAsync([{ key: "dashboard_domain", value: next }])
        .then(() => {
          setDraft(null);
          void refetchTLS();
        }),
      {
        loading: next ? "Publishing domain…" : "Clearing domain…",
        success: next
          ? "Domain saved. A certificate is requested automatically."
          : "Domain cleared.",
        error: (err) => err.message,
      },
    );
  };

  const style = STATUS_STYLES[tls?.tls_status ?? "unknown"] ?? STATUS_STYLES.unknown;

  return (
    <div className="space-y-3">
      <Label htmlFor="dashboard-domain">Dashboard domain</Label>
      <div className="flex max-w-md items-center gap-2">
        <Input
          id="dashboard-domain"
          value={value}
          onChange={(e) => setDraft(e.target.value)}
          placeholder="belune.example.com"
        />
        <Button
          onClick={() => save(value.trim())}
          disabled={updateSettings.isPending || !dirty}
        >
          Save
        </Button>
        {saved && !dirty && (
          <Button
            variant="outline"
            onClick={() => save("")}
            disabled={updateSettings.isPending}
          >
            Clear
          </Button>
        )}
      </div>

      {saved && (
        <div className="flex items-center gap-2 text-sm">
          <Badge variant={style.variant} className={style.className}>
            {style.label}
          </Badge>
          {tls?.tls_issuer && (
            <span className="text-muted-foreground text-xs">
              Issued by {tls.tls_issuer}
            </span>
          )}
          <Button
            size="icon-sm"
            variant="ghost"
            aria-label="Recheck certificate"
            onClick={() => void refetchTLS()}
          >
            <RefreshCw className="h-3.5 w-3.5" />
          </Button>
        </div>
      )}

      {tls?.tls_error && (
        <p className="text-destructive bg-destructive/10 max-w-md rounded-md p-2 text-xs break-words">
          {tls.tls_error}
        </p>
      )}

      {saved && tls?.tls_status === "pending" && !tls?.tls_error && (
        <p className="text-muted-foreground max-w-md text-xs">
          Waiting on Let's Encrypt. This needs an A record for{" "}
          <span className="font-mono">{saved}</span> pointing at this server, and
          ports 80 and 443 reachable from the internet.
        </p>
      )}

      <p className="text-muted-foreground max-w-md text-xs">
        Serve the dashboard on your own hostname over HTTPS. Leave it empty to
        keep reaching Belune on the server's IP address over plain HTTP.
      </p>
    </div>
  );
}
