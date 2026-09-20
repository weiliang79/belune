import { useForm } from "@tanstack/react-form";
import { toast } from "sonner";
import { CloudIcon } from "lucide-react";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
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
  useCreateBackupDestination,
  useUpdateBackupDestination,
  useTestBackupDestinationParams,
} from "@/lib/hooks/use-backup-destinations";
import type { BackupDestination, BackupProvider } from "@/lib/types";

interface Props {
  projectId: string;
  destination?: BackupDestination | null;
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

const PROVIDERS: { value: BackupProvider; label: string }[] = [
  { value: "s3", label: "AWS S3" },
  { value: "r2", label: "Cloudflare R2" },
  { value: "b2", label: "Backblaze B2" },
  { value: "wasabi", label: "Wasabi" },
  { value: "minio", label: "MinIO" },
  { value: "other", label: "Other (S3-compatible)" },
  { value: "local", label: "Local (no upload)" },
];

export function DestinationFormDialog({
  projectId,
  destination,
  open,
  onOpenChange,
}: Props) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        {/* Remount per open/target so fields initialise from props without an effect. */}
        {open && (
          <DestinationForm
            key={destination?.id ?? "new"}
            projectId={projectId}
            destination={destination}
            onDone={() => onOpenChange(false)}
          />
        )}
      </DialogContent>
    </Dialog>
  );
}

// Per-provider endpoint behaviour. The stored endpoint is always host-only
// (no scheme — minio derives the scheme from use_ssl):
//   - aws:            empty endpoint (backend derives s3.<region>.amazonaws.com)
//   - r2:             <account-id>.r2.cloudflarestorage.com, region "auto"
//   - regionTemplate: host derived from region (Wasabi/B2)
//   - manual:         user-supplied endpoint (MinIO / other S3-compatible)
type EndpointMode = "aws" | "r2" | "regionTemplate" | "manual" | "local";

const PROVIDER_META: Record<
  BackupProvider,
  {
    label: string;
    mode: EndpointMode;
    template?: (region: string) => string;
    forceSSL: boolean;
    regionPlaceholder?: string;
  }
> = {
  s3: { label: "AWS S3", mode: "aws", forceSSL: true, regionPlaceholder: "us-east-1" },
  r2: { label: "Cloudflare R2", mode: "r2", forceSSL: true },
  b2: {
    label: "Backblaze B2",
    mode: "regionTemplate",
    template: (r) => `s3.${r}.backblazeb2.com`,
    forceSSL: true,
    regionPlaceholder: "us-west-004",
  },
  wasabi: {
    label: "Wasabi",
    mode: "regionTemplate",
    template: (r) => `s3.${r}.wasabisys.com`,
    forceSSL: true,
    regionPlaceholder: "us-east-1",
  },
  minio: { label: "MinIO", mode: "manual", forceSSL: false },
  other: { label: "Other (S3-compatible)", mode: "manual", forceSSL: false },
  // No transport at all: the worker keeps the staged archive on-host instead
  // of uploading it. Every S3-ish field (endpoint/region/bucket/prefix/
  // credentials/SSL) is irrelevant and hidden below.
  local: { label: "Local (no upload)", mode: "local", forceSSL: false },
};

// resolveEndpoint computes the host-only endpoint sent to the backend.
function resolveEndpoint(
  provider: BackupProvider,
  region: string,
  accountId: string,
  manualEndpoint: string,
): string {
  const meta = PROVIDER_META[provider];
  switch (meta.mode) {
    case "aws":
    case "local":
      return "";
    case "r2":
      return accountId.trim()
        ? `${accountId.trim()}.r2.cloudflarestorage.com`
        : "";
    case "regionTemplate":
      return meta.template!(region.trim() || "us-east-1");
    case "manual":
      return manualEndpoint.trim();
  }
}

function DestinationForm({
  projectId,
  destination,
  onDone,
}: {
  projectId: string;
  destination?: BackupDestination | null;
  onDone: () => void;
}) {
  const editing = !!destination;
  const create = useCreateBackupDestination(projectId);
  const update = useUpdateBackupDestination(projectId);
  const test = useTestBackupDestinationParams(projectId);

  const initialProvider = destination?.provider ?? "s3";
  // Recover the R2 account id from a stored endpoint so editing shows it.
  const initialAccountId =
    initialProvider === "r2"
      ? (destination?.endpoint.match(
          /^(.+)\.r2\.cloudflarestorage\.com$/,
        )?.[1] ?? "")
      : "";

  const form = useForm({
    defaultValues: {
      name: destination?.name ?? "",
      provider: initialProvider,
      endpoint: destination?.endpoint ?? "",
      accountId: initialAccountId,
      region: destination?.region ?? "us-east-1",
      bucket: destination?.bucket ?? "",
      prefix: destination?.prefix ?? "",
      useSSL: destination?.use_ssl ?? true,
      accessKey: "",
      secretKey: "",
    },
    onSubmit: ({ value }) => {
      const data = buildData(value);
      const action =
        editing && destination
          ? update.mutateAsync({ destId: destination.id, data })
          : create.mutateAsync(data);
      toast.promise(action, {
        loading: editing ? "Saving destination…" : "Creating destination…",
        success: () => {
          onDone();
          return editing ? "Destination saved" : "Destination created";
        },
        error: (err) => err.message,
      });
    },
  });

  // buildData assembles the payload shared by save + test (resolved endpoint,
  // region, and blank-preserving credentials). Local sends just name+provider
  // — every other field is meaningless for it, so the backend zeroes them out
  // regardless of what's sent, but keep the payload honest anyway.
  const buildData = (value: typeof form.state.values) => {
    const meta = PROVIDER_META[value.provider];
    if (value.provider === "local") return { name: value.name, provider: value.provider };
    const resolvedEndpoint = resolveEndpoint(
      value.provider,
      value.region,
      value.accountId,
      value.endpoint,
    );
    return {
      name: value.name,
      provider: value.provider,
      endpoint: resolvedEndpoint,
      region: value.provider === "r2" ? "auto" : value.region.trim() || "us-east-1",
      bucket: value.bucket,
      prefix: value.prefix.trim(),
      use_ssl: meta.forceSSL ? true : value.useSSL,
      // On edit, blank credentials preserve the stored secret.
      access_key: value.accessKey || undefined,
      secret_key: value.secretKey || undefined,
    };
  };

  const handleTest = () => {
    const value = form.state.values;
    const meta = PROVIDER_META[value.provider];
    if (value.provider !== "local" && !value.bucket.trim()) {
      toast.error("Bucket is required to test");
      return;
    }
    if (meta.mode === "r2" && !value.accountId.trim()) {
      toast.error("Account ID is required for Cloudflare R2");
      return;
    }
    if (meta.mode === "manual" && !value.endpoint.trim()) {
      toast.error("Endpoint is required for this provider");
      return;
    }
    test
      .mutateAsync({ ...buildData(value), id: destination?.id })
      .then((res) => {
        if (res.ok) toast.success(`Connected to ${value.bucket}`);
        else toast.error(res.error ?? "Connection failed");
      })
      .catch((err) => toast.error(err.message));
  };

  const pending = create.isPending || update.isPending;

  return (
    <>
      <DialogHeader>
        <DialogTitle>
          {editing ? "Edit Destination" : "Add Destination"}
        </DialogTitle>
        <DialogDescription>
          Where this project's database and volume backups are stored — an
          S3-compatible bucket, or kept on this host only.
        </DialogDescription>
      </DialogHeader>

      <form
        onSubmit={(e) => {
          e.preventDefault();
          e.stopPropagation();
          form.handleSubmit();
        }}
        className="space-y-3"
      >
        <form.Field
          name="name"
          validators={{
            onChange: ({ value }) =>
              value.trim() === "" ? "Name is required" : undefined,
          }}
          children={(field) => {
            const error = fieldError(field.state.meta.errors);
            return (
              <div className="space-y-1.5">
                <Label htmlFor="dest-name">Name</Label>
                <Input
                  id="dest-name"
                  value={field.state.value}
                  onBlur={field.handleBlur}
                  onChange={(e) => field.handleChange(e.target.value)}
                  placeholder="AWS S3 Backup"
                />
                {error && <p className="text-destructive text-xs">{error}</p>}
              </div>
            );
          }}
        />

        <form.Field
          name="provider"
          children={(field) => (
            <div className="space-y-1.5">
              <Label>Provider</Label>
              <Select
                value={field.state.value}
                onValueChange={(v) =>
                  field.handleChange((v as BackupProvider) ?? "s3")
                }
              >
                <SelectTrigger className="capitalize">
                  <SelectValue placeholder="Select provider" />
                </SelectTrigger>
                <SelectContent>
                  {PROVIDERS.map((p) => (
                    <SelectItem
                      key={p.value}
                      value={p.value}
                      icon={<CloudIcon />}
                      className="capitalize"
                    >
                      {p.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          )}
        />

        <form.Subscribe
          selector={
            (s) =>
              [
                s.values.provider,
                s.values.region,
                s.values.accountId,
                s.values.endpoint,
              ] as const
          }
          children={([provider, region, accountId, endpoint]) => {
            const meta = PROVIDER_META[provider];
            const resolvedEndpoint = resolveEndpoint(
              provider,
              region,
              accountId,
              endpoint,
            );
            return (
              <>
                {provider === "local" && (
                  <p className="text-muted-foreground text-xs">
                    Backups stay on this host's disk — nothing is uploaded.
                    They don't survive losing the host, so use an off-host
                    destination too for real disaster recovery.
                  </p>
                )}

                {provider !== "local" && (
                  <div className="grid grid-cols-2 gap-3">
                    <form.Field
                      name="bucket"
                      validators={{
                        onChangeListenTo: ["provider"],
                        onChange: ({ value, fieldApi }) =>
                          fieldApi.form.state.values.provider !== "local" &&
                          value.trim() === ""
                            ? "Bucket is required"
                            : undefined,
                      }}
                      children={(field) => {
                        const error = fieldError(field.state.meta.errors);
                        return (
                          <div className="space-y-1.5">
                            <Label htmlFor="dest-bucket">Bucket</Label>
                            <Input
                              id="dest-bucket"
                              value={field.state.value}
                              onBlur={field.handleBlur}
                              onChange={(e) =>
                                field.handleChange(e.target.value)
                              }
                              placeholder="my-backups"
                            />
                            {error && (
                              <p className="text-destructive text-xs">
                                {error}
                              </p>
                            )}
                          </div>
                        );
                      }}
                    />
                    {provider !== "r2" && (
                      <form.Field
                        name="region"
                        children={(field) => (
                          <div className="space-y-1.5">
                            <Label htmlFor="dest-region">Region</Label>
                            <Input
                              id="dest-region"
                              value={field.state.value}
                              onChange={(e) =>
                                field.handleChange(e.target.value)
                              }
                              placeholder={meta.regionPlaceholder ?? "us-east-1"}
                            />
                          </div>
                        )}
                      />
                    )}
                  </div>
                )}

                {provider !== "local" && (
                  <form.Field
                    name="prefix"
                    children={(field) => (
                      <div className="space-y-1.5">
                        <Label htmlFor="dest-prefix">Prefix</Label>
                        <Input
                          id="dest-prefix"
                          value={field.state.value}
                          onChange={(e) => field.handleChange(e.target.value)}
                          placeholder="e.g. backups/prod"
                        />
                        <p className="text-muted-foreground text-xs">
                          Optional base path inside the bucket. Each backup
                          config can add its own sub-path under this.
                        </p>
                      </div>
                    )}
                  />
                )}

                {/* R2: account id instead of a raw endpoint */}
                {meta.mode === "r2" && (
                  <form.Field
                    name="accountId"
                    validators={{
                      onChangeListenTo: ["provider"],
                      onChange: ({ value, fieldApi }) =>
                        PROVIDER_META[fieldApi.form.state.values.provider]
                          .mode === "r2" && value.trim() === ""
                          ? "Account ID is required for Cloudflare R2"
                          : undefined,
                    }}
                    children={(field) => {
                      const error = fieldError(field.state.meta.errors);
                      return (
                        <div className="space-y-1.5">
                          <Label htmlFor="dest-account">Account ID</Label>
                          <Input
                            id="dest-account"
                            value={field.state.value}
                            onBlur={field.handleBlur}
                            onChange={(e) =>
                              field.handleChange(e.target.value)
                            }
                            placeholder="your-cloudflare-account-id"
                          />
                          {error && (
                            <p className="text-destructive text-xs">
                              {error}
                            </p>
                          )}
                        </div>
                      );
                    }}
                  />
                )}

                {/* MinIO / Other: free-form endpoint */}
                {meta.mode === "manual" && (
                  <form.Field
                    name="endpoint"
                    validators={{
                      onChangeListenTo: ["provider"],
                      onChange: ({ value, fieldApi }) =>
                        PROVIDER_META[fieldApi.form.state.values.provider]
                          .mode === "manual" && value.trim() === ""
                          ? "Endpoint is required for this provider"
                          : undefined,
                    }}
                    children={(field) => {
                      const error = fieldError(field.state.meta.errors);
                      return (
                        <div className="space-y-1.5">
                          <Label htmlFor="dest-endpoint">Endpoint</Label>
                          <Input
                            id="dest-endpoint"
                            value={field.state.value}
                            onBlur={field.handleBlur}
                            onChange={(e) =>
                              field.handleChange(e.target.value)
                            }
                            placeholder="minio.example.com:9000"
                          />
                          {error ? (
                            <p className="text-destructive text-xs">
                              {error}
                            </p>
                          ) : (
                            <p className="text-muted-foreground text-xs">
                              Host (and port) only — no scheme. SSL is
                              controlled below.
                            </p>
                          )}
                        </div>
                      );
                    }}
                  />
                )}

                {/* Derived endpoint preview for templated providers */}
                {(meta.mode === "r2" || meta.mode === "regionTemplate") &&
                  resolvedEndpoint && (
                    <p className="text-muted-foreground text-xs">
                      Endpoint:{" "}
                      <code className="font-mono">
                        {(meta.forceSSL ? "https://" : "http://") +
                          resolvedEndpoint}
                      </code>
                    </p>
                  )}
                {meta.mode === "aws" && (
                  <p className="text-muted-foreground text-xs">
                    Endpoint is derived from the region
                    (s3.&lt;region&gt;.amazonaws.com).
                  </p>
                )}

                {provider !== "local" && (
                  <div className="grid grid-cols-2 gap-3">
                    <form.Field
                      name="accessKey"
                      children={(field) => (
                        <div className="space-y-1.5">
                          <Label htmlFor="dest-access">Access key</Label>
                          <Input
                            id="dest-access"
                            value={field.state.value}
                            onChange={(e) =>
                              field.handleChange(e.target.value)
                            }
                            placeholder={editing ? "•••• (unchanged)" : ""}
                            autoComplete="off"
                          />
                        </div>
                      )}
                    />
                    <form.Field
                      name="secretKey"
                      children={(field) => (
                        <div className="space-y-1.5">
                          <Label htmlFor="dest-secret">Secret key</Label>
                          <Input
                            id="dest-secret"
                            type="password"
                            value={field.state.value}
                            onChange={(e) =>
                              field.handleChange(e.target.value)
                            }
                            placeholder={editing ? "•••• (unchanged)" : ""}
                            autoComplete="off"
                          />
                        </div>
                      )}
                    />
                  </div>
                )}

                {/* SSL is only user-controllable for self-hosted/other endpoints;
                    managed providers are always HTTPS; local has no transport at all. */}
                {provider !== "local" && !meta.forceSSL && (
                  <form.Field
                    name="useSSL"
                    children={(field) => (
                      <Label className="flex items-center gap-2 text-sm font-normal">
                        <Checkbox
                          checked={field.state.value}
                          onCheckedChange={(v) =>
                            field.handleChange(v === true)
                          }
                        />
                        Use SSL (HTTPS)
                      </Label>
                    )}
                  />
                )}

                <DialogFooter className="sm:justify-between">
                  {provider === "local" ? (
                    <span />
                  ) : (
                    <Button
                      type="button"
                      variant="outline"
                      onClick={handleTest}
                      disabled={test.isPending}
                    >
                      {test.isPending ? "Testing…" : "Test connection"}
                    </Button>
                  )}
                  <Button type="submit" disabled={pending}>
                    {pending ? "Saving…" : editing ? "Save" : "Create"}
                  </Button>
                </DialogFooter>
              </>
            );
          }}
        />
      </form>
    </>
  );
}
