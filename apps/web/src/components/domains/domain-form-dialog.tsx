import { useEffect, useState } from "react";
import { toast } from "sonner";
import { Loader2 } from "lucide-react";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
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
import {
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
} from "@/components/ui/tabs";
import { useAddDomain, useUpdateDomain } from "@/lib/hooks/use-domains";
import type { DomainExpanded } from "@/lib/types";

const HOSTNAME_REGEX =
  /^([a-zA-Z0-9]([a-zA-Z0-9-]*[a-zA-Z0-9])?\.)+[a-zA-Z]{2,}$/;

const SSL_MODES = [
  { value: "automatic", label: "Automatic (ACME)" },
  { value: "dns_challenge", label: "DNS Challenge" },
  { value: "custom", label: "Custom Certificate" },
  { value: "off", label: "Off" },
] as const;

interface Props {
  projectId: string;
  applicationId: string;
  domain?: DomainExpanded;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

export function DomainFormDialog({
  projectId,
  applicationId,
  domain,
  open,
  onOpenChange,
}: Props) {
  const isEdit = !!domain;
  const addDomain = useAddDomain(projectId, applicationId);
  const updateDomain = useUpdateDomain(projectId, applicationId);

  const [hostname, setHostname] = useState("");
  const [sslMode, setSslMode] = useState("automatic");
  const [containerPort, setContainerPort] = useState("");
  const [forceHttps, setForceHttps] = useState(true);
  const [certPath, setCertPath] = useState("");
  const [keyPath, setKeyPath] = useState("");

  useEffect(() => {
    if (!open) return;
    setHostname(domain?.hostname ?? "");
    setSslMode(domain?.ssl_mode ?? "automatic");
    setContainerPort(domain?.container_port?.toString() ?? "");
    setForceHttps(domain?.force_https ?? true);
    setCertPath(domain?.cert_path ?? "");
    setKeyPath(domain?.key_path ?? "");
  }, [open, domain]);

  const pending = addDomain.isPending || updateDomain.isPending;

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    const trimmed = hostname.trim();
    if (!trimmed) {
      toast.error("Hostname is required");
      return;
    }
    if (!HOSTNAME_REGEX.test(trimmed)) {
      toast.error("Invalid hostname format");
      return;
    }
    if (sslMode === "custom" && (!certPath.trim() || !keyPath.trim())) {
      toast.error("Custom SSL mode requires cert and key paths");
      return;
    }

    const portNum = containerPort ? parseInt(containerPort) : null;

    if (isEdit && domain) {
      toast.promise(
        updateDomain
          .mutateAsync({
            domainId: domain.id,
            hostname: trimmed,
            ssl_mode: sslMode,
            force_https: forceHttps,
            container_port: portNum,
            cert_path: certPath || undefined,
            key_path: keyPath || undefined,
          })
          .then(() => onOpenChange(false)),
        {
          loading: "Saving...",
          success: "Domain updated",
          error: (err) => err.message,
        },
      );
    } else {
      toast.promise(
        addDomain
          .mutateAsync({
            hostname: trimmed,
            ssl_enabled: sslMode !== "off",
            ssl_mode: sslMode,
            force_https: forceHttps,
            container_port: portNum ?? undefined,
            cert_path: certPath || undefined,
            key_path: keyPath || undefined,
          })
          .then(() => onOpenChange(false)),
        {
          loading: "Adding...",
          success: "Domain added",
          error: (err) => err.message,
        },
      );
    }
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-lg">
        <form onSubmit={handleSubmit} className="space-y-4">
          <DialogHeader>
            <DialogTitle>{isEdit ? "Edit Domain" : "Add Domain"}</DialogTitle>
            <DialogDescription>
              {isEdit
                ? "Update hostname, TLS and routing settings."
                : "Configure a new domain with TLS and routing settings."}
            </DialogDescription>
          </DialogHeader>

          <Tabs defaultValue="hostname">
            <TabsList className="grid w-full grid-cols-3">
              <TabsTrigger value="hostname">Hostname</TabsTrigger>
              <TabsTrigger value="tls">TLS</TabsTrigger>
              <TabsTrigger value="routing">Routing</TabsTrigger>
            </TabsList>

            <TabsContent value="hostname" className="space-y-3 pt-3">
              <div className="space-y-2">
                <Label htmlFor="hostname">Hostname</Label>
                <Input
                  id="hostname"
                  placeholder="app.example.com"
                  value={hostname}
                  onChange={(e) => setHostname(e.target.value)}
                />
                <p className="text-muted-foreground text-xs">
                  Fully qualified domain. Must resolve to this server.
                </p>
              </div>
              <div className="space-y-2">
                <Label htmlFor="port">Container Port</Label>
                <Input
                  id="port"
                  type="number"
                  min="1"
                  max="65535"
                  placeholder="3000"
                  value={containerPort}
                  onChange={(e) => setContainerPort(e.target.value)}
                />
                <p className="text-muted-foreground text-xs">
                  Override the app's default port for this domain. Leave blank
                  to inherit.
                </p>
              </div>
            </TabsContent>

            <TabsContent value="tls" className="space-y-3 pt-3">
              <div className="space-y-2">
                <Label>SSL Mode</Label>
                <Select
                  value={sslMode}
                  onValueChange={(v) => setSslMode(v ?? "automatic")}
                >
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {SSL_MODES.map((m) => (
                      <SelectItem key={m.value} value={m.value}>
                        {m.label}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              {sslMode === "custom" && (
                <div className="space-y-3">
                  <div className="space-y-2">
                    <Label>Certificate Path</Label>
                    <Input
                      value={certPath}
                      onChange={(e) => setCertPath(e.target.value)}
                      placeholder="/path/to/cert.pem"
                      className="font-mono text-xs"
                    />
                  </div>
                  <div className="space-y-2">
                    <Label>Key Path</Label>
                    <Input
                      value={keyPath}
                      onChange={(e) => setKeyPath(e.target.value)}
                      placeholder="/path/to/key.pem"
                      className="font-mono text-xs"
                    />
                  </div>
                </div>
              )}
            </TabsContent>

            <TabsContent value="routing" className="space-y-3 pt-3">
              <label className="flex items-center gap-2 text-sm">
                <input
                  type="checkbox"
                  checked={forceHttps}
                  onChange={(e) => setForceHttps(e.target.checked)}
                />
                Force HTTPS
              </label>
              <p className="text-muted-foreground text-xs">
                Route features (custom headers, IP allowlist, redirects) are
                configured per-domain after saving — expand the domain row.
              </p>
            </TabsContent>
          </Tabs>

          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              onClick={() => onOpenChange(false)}
              disabled={pending}
            >
              Cancel
            </Button>
            <Button type="submit" disabled={pending}>
              {pending && <Loader2 className="mr-1 h-4 w-4 animate-spin" />}
              {isEdit ? "Save Changes" : "Add Domain"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
