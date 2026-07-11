import { useForm } from "@tanstack/react-form";
import { z } from "zod";
import { toast } from "sonner";
import {
  GlobeIcon,
  KeyRoundIcon,
  Loader2,
  ShieldCheckIcon,
  ShieldOffIcon,
} from "lucide-react";
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
import { useCertificates } from "@/lib/hooks/use-certificates";
import type { DomainExpanded } from "@/lib/types";

const HOSTNAME_REGEX =
  /^([a-zA-Z0-9]([a-zA-Z0-9-]*[a-zA-Z0-9])?\.)+[a-zA-Z]{2,}$/;

const SSL_MODES = [
  { value: "automatic", label: "Automatic (ACME)", Icon: ShieldCheckIcon },
  { value: "dns_challenge", label: "DNS Challenge", Icon: GlobeIcon },
  { value: "custom", label: "Custom Certificate", Icon: KeyRoundIcon },
  { value: "off", label: "Off", Icon: ShieldOffIcon },
] as const;

interface Props {
  projectId: string;
  applicationId: string;
  domain?: DomainExpanded;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

function fieldError(errors: unknown[]): string | undefined {
  const first = errors[0];
  if (!first) return undefined;
  return typeof first === "string"
    ? first
    : (first as { message?: string }).message;
}

export function DomainFormDialog({
  projectId,
  applicationId,
  domain,
  open,
  onOpenChange,
}: Props) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-lg">
        {/* Keyed so the form re-initializes from the current domain each open. */}
        <DomainForm
          key={`${domain?.id ?? "new"}-${open}`}
          projectId={projectId}
          applicationId={applicationId}
          domain={domain}
          onClose={() => onOpenChange(false)}
        />
      </DialogContent>
    </Dialog>
  );
}

function DomainForm({
  projectId,
  applicationId,
  domain,
  onClose,
}: {
  projectId: string;
  applicationId: string;
  domain?: DomainExpanded;
  onClose: () => void;
}) {
  const isEdit = !!domain;
  const addDomain = useAddDomain(projectId, applicationId);
  const updateDomain = useUpdateDomain(projectId, applicationId);
  const pending = addDomain.isPending || updateDomain.isPending;

  // Only admins can manage certificates, so a non-admin editing a domain sees an
  // empty picker rather than an error; the form explains where they come from.
  const { data: certificateList, isLoading: certificatesLoading } =
    useCertificates();
  const certificates = certificateList ?? [];

  const form = useForm({
    defaultValues: {
      hostname: domain?.hostname ?? "",
      ssl_mode: domain?.ssl_mode ?? "automatic",
      container_port: domain?.container_port?.toString() ?? "",
      force_https: domain?.force_https ?? true,
      certificate_id: domain?.certificate_id ?? "",
    },
    onSubmit: ({ value }) => {
      const trimmed = value.hostname.trim();
      const portNum = value.container_port
        ? parseInt(value.container_port, 10)
        : null;

      const action =
        isEdit && domain
          ? updateDomain.mutateAsync({
              domainId: domain.id,
              hostname: trimmed,
              ssl_mode: value.ssl_mode,
              force_https: value.force_https,
              container_port: portNum,
              certificate_id: value.certificate_id || undefined,
            })
          : addDomain.mutateAsync({
              hostname: trimmed,
              ssl_enabled: value.ssl_mode !== "off",
              ssl_mode: value.ssl_mode,
              force_https: value.force_https,
              container_port: portNum ?? undefined,
              certificate_id: value.certificate_id || undefined,
            });

      toast.promise(action.then(() => onClose()), {
        loading: isEdit ? "Saving..." : "Adding...",
        success: isEdit ? "Domain updated" : "Domain added",
        error: (err) => err.message,
      });
    },
  });

  return (
    <form
      onSubmit={(e) => {
        e.preventDefault();
        e.stopPropagation();
        form.handleSubmit();
      }}
      className="space-y-4"
    >
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
          <form.Field
            name="hostname"
            validators={{
              onChange: z
                .string()
                .min(1, "Hostname is required")
                .regex(HOSTNAME_REGEX, "Invalid hostname format"),
            }}
            children={(field) => {
              const error = fieldError(field.state.meta.errors);
              return (
                <div className="space-y-2">
                  <Label htmlFor="hostname">Hostname</Label>
                  <Input
                    id="hostname"
                    placeholder="app.example.com"
                    value={field.state.value}
                    onBlur={field.handleBlur}
                    onChange={(e) => field.handleChange(e.target.value)}
                  />
                  {error ? (
                    <p className="text-destructive text-xs">{error}</p>
                  ) : (
                    <p className="text-muted-foreground text-xs">
                      Fully qualified domain. Must resolve to this server.
                    </p>
                  )}
                </div>
              );
            }}
          />
          <form.Field
            name="container_port"
            validators={{
              onChange: z
                .string()
                .refine(
                  (v) => {
                    if (!v) return true;
                    const n = Number(v);
                    return Number.isInteger(n) && n >= 1 && n <= 65535;
                  },
                  "Port must be between 1 and 65535",
                ),
            }}
            children={(field) => {
              const error = fieldError(field.state.meta.errors);
              return (
                <div className="space-y-2">
                  <Label htmlFor="port">Container Port</Label>
                  <Input
                    id="port"
                    type="number"
                    min="1"
                    max="65535"
                    placeholder="3000"
                    value={field.state.value}
                    onBlur={field.handleBlur}
                    onChange={(e) => field.handleChange(e.target.value)}
                  />
                  {error ? (
                    <p className="text-destructive text-xs">{error}</p>
                  ) : (
                    <p className="text-muted-foreground text-xs">
                      Override the app's default port for this domain. Leave
                      blank to inherit.
                    </p>
                  )}
                </div>
              );
            }}
          />
        </TabsContent>

        <TabsContent value="tls" className="space-y-3 pt-3">
          <form.Field
            name="ssl_mode"
            children={(field) => (
              <div className="space-y-2">
                <Label>SSL Mode</Label>
                <Select
                  value={field.state.value}
                  onValueChange={(v) => field.handleChange(v ?? "automatic")}
                >
                  <SelectTrigger className="capitalize">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {SSL_MODES.map((m) => (
                      <SelectItem
                        key={m.value}
                        value={m.value}
                        icon={<m.Icon />}
                        className="capitalize"
                      >
                        {m.label}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
            )}
          />
          <form.Subscribe
            selector={(s) => s.values.ssl_mode}
            children={(sslMode) =>
              sslMode === "custom" ? (
                <form.Field
                  name="certificate_id"
                  validators={{
                    onChangeListenTo: ["ssl_mode"],
                    onChange: ({ value, fieldApi }) =>
                      fieldApi.form.getFieldValue("ssl_mode") === "custom" &&
                      !value
                        ? "Select a certificate"
                        : undefined,
                  }}
                  children={(field) => {
                    const error = fieldError(field.state.meta.errors);
                    return (
                      <div className="space-y-2">
                        <Label>Certificate</Label>
                        <Select
                          value={field.state.value}
                          onValueChange={(v) => field.handleChange(v ?? "")}
                          disabled={certificatesLoading}
                        >
                          <SelectTrigger>
                            <SelectValue
                              placeholder={
                                certificatesLoading
                                  ? "Loading…"
                                  : "Select a certificate"
                              }
                            />
                          </SelectTrigger>
                          <SelectContent>
                            {certificates.map((cert) => (
                              <SelectItem key={cert.id} value={cert.id}>
                                {cert.name}
                                <span className="text-muted-foreground ml-2 font-mono text-xs">
                                  {cert.subjects.join(", ")}
                                </span>
                              </SelectItem>
                            ))}
                          </SelectContent>
                        </Select>
                        {!certificatesLoading && certificates.length === 0 && (
                          <p className="text-muted-foreground text-xs">
                            No certificates uploaded yet. An admin can add one
                            under Settings → Certificates.
                          </p>
                        )}
                        {error && (
                          <p className="text-destructive text-xs">{error}</p>
                        )}
                      </div>
                    );
                  }}
                />
              ) : null
            }
          />
        </TabsContent>

        <TabsContent value="routing" className="space-y-3 pt-3">
          <form.Field
            name="force_https"
            children={(field) => (
              <label className="flex items-center gap-2 text-sm">
                <input
                  type="checkbox"
                  checked={field.state.value}
                  onChange={(e) => field.handleChange(e.target.checked)}
                />
                Force HTTPS
              </label>
            )}
          />
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
          onClick={onClose}
          disabled={pending}
        >
          Cancel
        </Button>
        <form.Subscribe
          selector={(s) => s.canSubmit}
          children={(canSubmit) => (
            <Button type="submit" disabled={pending || !canSubmit}>
              {pending && <Loader2 className="mr-1 h-4 w-4 animate-spin" />}
              {isEdit ? "Save Changes" : "Add Domain"}
            </Button>
          )}
        />
      </DialogFooter>
    </form>
  );
}
