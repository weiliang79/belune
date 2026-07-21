import { useState } from "react";
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

const PROVIDERS: { value: BackupProvider; label: string }[] = [
  { value: "s3", label: "AWS S3" },
  { value: "r2", label: "Cloudflare R2" },
  { value: "b2", label: "Backblaze B2" },
  { value: "wasabi", label: "Wasabi" },
  { value: "minio", label: "MinIO" },
  { value: "other", label: "Other (S3-compatible)" },
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
type EndpointMode = "aws" | "r2" | "regionTemplate" | "manual";

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

  const [name, setName] = useState(destination?.name ?? "");
  const [provider, setProvider] = useState<BackupProvider>(initialProvider);
  const [endpoint, setEndpoint] = useState(destination?.endpoint ?? "");
  const [accountId, setAccountId] = useState(initialAccountId);
  const [region, setRegion] = useState(destination?.region ?? "us-east-1");
  const [bucket, setBucket] = useState(destination?.bucket ?? "");
  const [prefix, setPrefix] = useState(destination?.prefix ?? "");
  const [useSSL, setUseSSL] = useState(destination?.use_ssl ?? true);
  const [accessKey, setAccessKey] = useState("");
  const [secretKey, setSecretKey] = useState("");

  const meta = PROVIDER_META[provider];
  const resolvedEndpoint = resolveEndpoint(
    provider,
    region,
    accountId,
    endpoint,
  );

  // buildData assembles the payload shared by save + test (resolved endpoint,
  // region, and blank-preserving credentials).
  const buildData = () => ({
    name,
    provider,
    endpoint: resolvedEndpoint,
    region: provider === "r2" ? "auto" : region.trim() || "us-east-1",
    bucket,
    prefix: prefix.trim(),
    use_ssl: meta.forceSSL ? true : useSSL,
    // On edit, blank credentials preserve the stored secret.
    access_key: accessKey || undefined,
    secret_key: secretKey || undefined,
  });

  const validateProviderFields = (): boolean => {
    if (meta.mode === "r2" && !accountId.trim()) {
      toast.error("Account ID is required for Cloudflare R2");
      return false;
    }
    if (meta.mode === "manual" && !endpoint.trim()) {
      toast.error("Endpoint is required for this provider");
      return false;
    }
    return true;
  };

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!validateProviderFields()) return;
    const data = buildData();
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
  };

  const handleTest = () => {
    if (!bucket.trim()) {
      toast.error("Bucket is required to test");
      return;
    }
    if (!validateProviderFields()) return;
    test
      .mutateAsync({ ...buildData(), id: destination?.id })
      .then((res) => {
        if (res.ok) toast.success(`Connected to ${bucket}`);
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
          An S3-compatible bucket used to store database backups for this
          project.
        </DialogDescription>
      </DialogHeader>

      <form onSubmit={handleSubmit} className="space-y-3">
        <div className="space-y-1.5">
          <Label htmlFor="dest-name">Name</Label>
          <Input
            id="dest-name"
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder="AWS S3 Backup"
            required
          />
        </div>

        <div className="space-y-1.5">
          <Label>Provider</Label>
          <Select
            value={provider}
            onValueChange={(v) => setProvider((v as BackupProvider) ?? "s3")}
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

        <div className="grid grid-cols-2 gap-3">
          <div className="space-y-1.5">
            <Label htmlFor="dest-bucket">Bucket</Label>
            <Input
              id="dest-bucket"
              value={bucket}
              onChange={(e) => setBucket(e.target.value)}
              placeholder="my-backups"
              required
            />
          </div>
          {provider !== "r2" && (
            <div className="space-y-1.5">
              <Label htmlFor="dest-region">Region</Label>
              <Input
                id="dest-region"
                value={region}
                onChange={(e) => setRegion(e.target.value)}
                placeholder={meta.regionPlaceholder ?? "us-east-1"}
              />
            </div>
          )}
        </div>

        <div className="space-y-1.5">
          <Label htmlFor="dest-prefix">Prefix</Label>
          <Input
            id="dest-prefix"
            value={prefix}
            onChange={(e) => setPrefix(e.target.value)}
            placeholder="e.g. backups/prod"
          />
          <p className="text-muted-foreground text-xs">
            Optional base path inside the bucket. Each backup config can add its
            own sub-path under this.
          </p>
        </div>

        {/* R2: account id instead of a raw endpoint */}
        {meta.mode === "r2" && (
          <div className="space-y-1.5">
            <Label htmlFor="dest-account">Account ID</Label>
            <Input
              id="dest-account"
              value={accountId}
              onChange={(e) => setAccountId(e.target.value)}
              placeholder="your-cloudflare-account-id"
              required
            />
          </div>
        )}

        {/* MinIO / Other: free-form endpoint */}
        {meta.mode === "manual" && (
          <div className="space-y-1.5">
            <Label htmlFor="dest-endpoint">Endpoint</Label>
            <Input
              id="dest-endpoint"
              value={endpoint}
              onChange={(e) => setEndpoint(e.target.value)}
              placeholder="minio.example.com:9000"
              required
            />
            <p className="text-muted-foreground text-xs">
              Host (and port) only — no scheme. SSL is controlled below.
            </p>
          </div>
        )}

        {/* Derived endpoint preview for templated providers */}
        {(meta.mode === "r2" || meta.mode === "regionTemplate") &&
          resolvedEndpoint && (
            <p className="text-muted-foreground text-xs">
              Endpoint:{" "}
              <code className="font-mono">
                {(meta.forceSSL ? "https://" : "http://") + resolvedEndpoint}
              </code>
            </p>
          )}
        {meta.mode === "aws" && (
          <p className="text-muted-foreground text-xs">
            Endpoint is derived from the region (s3.&lt;region&gt;.amazonaws.com).
          </p>
        )}

        <div className="grid grid-cols-2 gap-3">
          <div className="space-y-1.5">
            <Label htmlFor="dest-access">Access key</Label>
            <Input
              id="dest-access"
              value={accessKey}
              onChange={(e) => setAccessKey(e.target.value)}
              placeholder={editing ? "•••• (unchanged)" : ""}
              autoComplete="off"
            />
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="dest-secret">Secret key</Label>
            <Input
              id="dest-secret"
              type="password"
              value={secretKey}
              onChange={(e) => setSecretKey(e.target.value)}
              placeholder={editing ? "•••• (unchanged)" : ""}
              autoComplete="off"
            />
          </div>
        </div>

        {/* SSL is only user-controllable for self-hosted/other endpoints;
            managed providers are always HTTPS. */}
        {!meta.forceSSL && (
          <label className="flex items-center gap-2 text-sm">
            <Checkbox checked={useSSL} onCheckedChange={setUseSSL} />
            Use SSL (HTTPS)
          </label>
        )}

        <DialogFooter className="sm:justify-between">
          <Button
            type="button"
            variant="outline"
            onClick={handleTest}
            disabled={test.isPending}
          >
            {test.isPending ? "Testing…" : "Test connection"}
          </Button>
          <Button type="submit" disabled={pending}>
            {pending ? "Saving…" : editing ? "Save" : "Create"}
          </Button>
        </DialogFooter>
      </form>
    </>
  );
}
