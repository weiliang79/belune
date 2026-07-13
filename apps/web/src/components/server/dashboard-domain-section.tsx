import { useState } from "react";
import { toast } from "sonner";
import { useQuery } from "@tanstack/react-query";
import {
  KeyRoundIcon,
  RefreshCw,
  ShieldCheckIcon,
  ShieldOffIcon,
} from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { useSettings, useUpdateSettings } from "@/lib/hooks/use-settings";
import { useCertificates } from "@/lib/hooks/use-certificates";
import { api } from "@/lib/api/client";

interface DashboardTLS {
  domain: string;
  ssl_mode: string;
  tls_status: string;
  tls_issuer?: string;
  tls_not_after: string | null;
  tls_error?: string;
}

// The same three modes an application domain offers — the dashboard is served by
// the same proxy and has no business behaving differently.
const SSL_MODES = [
  { value: "automatic", label: "Automatic (Let's Encrypt)", Icon: ShieldCheckIcon },
  { value: "custom", label: "Custom Certificate", Icon: KeyRoundIcon },
  { value: "off", label: "Off (plain HTTP)", Icon: ShieldOffIcon },
] as const;

// Mirrors the badge convention: coloured text on a low-opacity fill of the same
// colour with a matching border. Active follows the accent.
const STATUS_STYLES: Record<
  string,
  {
    label: string;
    variant: "default" | "secondary" | "outline" | "destructive" | "light";
    className?: string;
  }
> = {
  active: { label: "HTTPS active", variant: "light" },
  pending: {
    label: "Waiting for certificate",
    variant: "outline",
    className:
      "border-status-building-line bg-status-building-soft text-status-building",
  },
  expiring: {
    label: "Expiring",
    variant: "outline",
    className:
      "border-status-building-line bg-status-building-soft text-status-building",
  },
  expired: { label: "Expired", variant: "destructive" },
  failed: { label: "Failed", variant: "destructive" },
  disabled: { label: "HTTPS off", variant: "outline" },
  // Dev: Caddy issues from its own CA, so this is the finished state.
  local: { label: "Local certificate", variant: "secondary" },
  unknown: { label: "Not set", variant: "secondary" },
};

/**
 * The dashboard's own domain and TLS.
 *
 * Belune is reachable on the server's bare IP over plain HTTP out of the box —
 * that is how you get in to create the first admin. Naming a domain here gives
 * the dashboard its own route in the proxy, which is what lets Caddy obtain a
 * certificate for it: certificates are only ever issued for hostnames the proxy
 * has been told about.
 *
 * The mode is the same choice an application domain gets, because it is the same
 * proxy doing the work. Off is the one that needs care: it drops the HTTP→HTTPS
 * redirect as well, since redirecting to a port with no certificate would lock
 * the operator out of the panel.
 */
export function DashboardDomainSection() {
  const { data: settings } = useSettings();
  const updateSettings = useUpdateSettings();
  const { data: certificates, isLoading: certificatesLoading } =
    useCertificates();

  const setting = (key: string) =>
    settings?.find((s) => s.key === key)?.value?.trim() ?? "";

  const savedDomain = setting("dashboard_domain");
  // Absent means automatic — what every install did before the mode existed.
  const savedMode = setting("dashboard_ssl_mode") || "automatic";
  const savedCertID = setting("dashboard_certificate_id");

  const [draft, setDraft] = useState<{
    domain: string;
    mode: string;
    certID: string;
  } | null>(null);

  const domain = draft?.domain ?? savedDomain;
  const mode = draft?.mode ?? savedMode;
  const certID = draft?.certID ?? savedCertID;

  const patch = (next: Partial<{ domain: string; mode: string; certID: string }>) =>
    setDraft({ domain, mode, certID, ...next });

  const dirty =
    domain.trim() !== savedDomain ||
    mode !== savedMode ||
    certID !== savedCertID;

  // Custom with nothing selected would leave :443 with nothing to serve. The API
  // refuses it too; catching it here just saves a round-trip.
  const missingCert = Boolean(domain.trim()) && mode === "custom" && !certID;

  const { data: tls, refetch: refetchTLS } = useQuery({
    queryKey: ["dashboard-tls", savedDomain, savedMode, savedCertID],
    queryFn: () => api.get<DashboardTLS>("/server/dashboard-tls"),
    // A certificate normally lands within a minute of the DNS being right, so
    // poll while the operator is watching rather than making them refresh.
    refetchInterval: savedDomain ? 15000 : false,
    enabled: Boolean(savedDomain),
  });

  const save = () => {
    const nextDomain = domain.trim();
    // Clearing the domain takes the whole route away, so the mode and certificate
    // have nothing left to apply to.
    const payload = nextDomain
      ? [
          { key: "dashboard_domain", value: nextDomain },
          { key: "dashboard_ssl_mode", value: mode },
          {
            key: "dashboard_certificate_id",
            value: mode === "custom" ? certID : "",
          },
        ]
      : [
          { key: "dashboard_domain", value: "" },
          { key: "dashboard_certificate_id", value: "" },
        ];

    toast.promise(
      updateSettings.mutateAsync(payload).then(() => {
        setDraft(null);
        void refetchTLS();
      }),
      {
        loading: nextDomain ? "Applying…" : "Clearing domain…",
        success: !nextDomain
          ? "Domain cleared."
          : mode === "automatic"
            ? "Saved. A certificate is requested automatically."
            : "Saved.",
        error: (err) => err.message,
      },
    );
  };

  const clear = () => {
    setDraft({ domain: "", mode: "automatic", certID: "" });
    toast.promise(
      updateSettings
        .mutateAsync([
          { key: "dashboard_domain", value: "" },
          { key: "dashboard_certificate_id", value: "" },
        ])
        .then(() => {
          setDraft(null);
          void refetchTLS();
        }),
      {
        loading: "Clearing domain…",
        success: "Domain cleared.",
        error: (err) => err.message,
      },
    );
  };

  const style =
    STATUS_STYLES[tls?.tls_status ?? "unknown"] ?? STATUS_STYLES.unknown;

  return (
    <div className="space-y-4">
      <div className="grid max-w-2xl gap-4 sm:grid-cols-2">
        <div className="space-y-2">
          <Label htmlFor="dashboard-domain">Dashboard domain</Label>
          <Input
            id="dashboard-domain"
            value={domain}
            onChange={(e) => patch({ domain: e.target.value })}
            placeholder="belune.example.com"
          />
        </div>

        <div className="space-y-2">
          <Label htmlFor="dashboard-ssl-mode">TLS mode</Label>
          <Select
            value={mode}
            onValueChange={(v) => patch({ mode: v ?? "automatic" })}
          >
            <SelectTrigger id="dashboard-ssl-mode">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {SSL_MODES.map((m) => (
                <SelectItem key={m.value} value={m.value} icon={<m.Icon />}>
                  {m.label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
      </div>

      {mode === "custom" && (
        <div className="max-w-2xl space-y-2">
          <Label htmlFor="dashboard-certificate">Certificate</Label>
          <Select
            value={certID}
            onValueChange={(v) => patch({ certID: v ?? "" })}
            disabled={certificatesLoading}
          >
            <SelectTrigger id="dashboard-certificate" className="w-full min-w-0">
              {/* Name only: the default echoes the whole item — name and every
                  subject — into the trigger, which a wildcard or Origin CA
                  certificate overflows. Subjects stay in the dropdown. */}
              <SelectValue
                placeholder={
                  certificatesLoading ? "Loading…" : "Select a certificate"
                }
              >
                {(value: unknown) => {
                  const cert = (certificates ?? []).find((c) => c.id === value);
                  if (!cert) {
                    return certificatesLoading
                      ? "Loading…"
                      : "Select a certificate";
                  }
                  return <span className="truncate">{cert.name}</span>;
                }}
              </SelectValue>
            </SelectTrigger>
            <SelectContent>
              {(certificates ?? []).map((cert) => (
                <SelectItem key={cert.id} value={cert.id}>
                  {cert.name}
                  <span className="text-muted-foreground ml-2 font-mono text-xs">
                    {cert.subjects.join(", ")}
                  </span>
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          {(certificates ?? []).length === 0 && !certificatesLoading && (
            <p className="text-muted-foreground text-xs">
              No certificates uploaded yet — add one on the Certificates page.
            </p>
          )}
        </div>
      )}

      <div className="flex items-center gap-2">
        <Button
          onClick={save}
          disabled={updateSettings.isPending || !dirty || missingCert}
        >
          Save
        </Button>
        {savedDomain && !dirty && (
          <Button
            variant="outline"
            onClick={clear}
            disabled={updateSettings.isPending}
          >
            Clear
          </Button>
        )}
      </div>

      {savedDomain && (
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
        <p className="text-destructive bg-destructive/10 max-w-2xl rounded-md p-2 text-xs break-words">
          {tls.tls_error}
        </p>
      )}

      {savedDomain && tls?.tls_status === "pending" && !tls?.tls_error && (
        <p className="text-muted-foreground max-w-2xl text-xs">
          Waiting on Let's Encrypt. This needs an A record for{" "}
          <span className="font-mono">{savedDomain}</span> pointing at this
          server, and ports 80 and 443 reachable from the internet.
        </p>
      )}

      {mode === "off" && (
        <p className="text-muted-foreground max-w-2xl text-xs">
          The dashboard will be served over plain HTTP, and the HTTP→HTTPS
          redirect is removed with it. Your session cookie and password cross the
          network unencrypted — only reasonable behind a proxy that terminates TLS
          for you, or on a private network.
        </p>
      )}

      <p className="text-muted-foreground max-w-2xl text-xs">
        Serve the dashboard on your own hostname. Leave the domain empty to keep
        reaching Belune on the server's IP address over plain HTTP.
      </p>
    </div>
  );
}
