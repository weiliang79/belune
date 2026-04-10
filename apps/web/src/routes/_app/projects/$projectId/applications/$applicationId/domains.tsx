import { createFileRoute } from "@tanstack/react-router";
import {
  useDomains,
  useAddDomain,
  useUpdateDomain,
  useRemoveDomain,
  useUpsertRouteFeature,
  useDeleteRouteFeature,
} from "@/lib/hooks/use-domains";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from "@/components/ui/alert-dialog";
import { Skeleton } from "@/components/ui/skeleton";
import { useState } from "react";
import { Loader2 } from "lucide-react";
import type { DomainExpanded, RouteFeature } from "@/lib/types";

export const Route = createFileRoute(
  "/_app/projects/$projectId/applications/$applicationId/domains",
)({
  component: DomainsPage,
});

const SSL_MODES = [
  { value: "automatic", label: "Automatic (ACME)" },
  { value: "dns_challenge", label: "DNS Challenge" },
  { value: "custom", label: "Custom Certificate" },
  { value: "off", label: "Off" },
] as const;

const FEATURE_TYPES = [
  { value: "basic_auth", label: "Basic Auth" },
  { value: "headers", label: "Custom Headers" },
  { value: "ip_allowlist", label: "IP Allowlist" },
  { value: "redirect", label: "Redirect" },
] as const;

function DomainsPage() {
  const { projectId, applicationId } = Route.useParams();
  const { data: domains, isLoading } = useDomains(projectId, applicationId);
  const addDomain = useAddDomain(projectId, applicationId);
  const [hostname, setHostname] = useState("");
  const [sslMode, setSslMode] = useState("automatic");
  const [containerPort, setContainerPort] = useState("");
  const [forceHttps, setForceHttps] = useState(true);

  const handleAdd = () => {
    const trimmed = hostname.trim();
    if (!trimmed) return;
    if (
      !/^([a-zA-Z0-9]([a-zA-Z0-9-]*[a-zA-Z0-9])?\.)+[a-zA-Z]{2,}$/.test(trimmed)
    ) {
      toast.error("Invalid hostname format");
      return;
    }
    toast.promise(
      addDomain
        .mutateAsync({
          hostname: trimmed,
          ssl_enabled: sslMode !== "off",
          ssl_mode: sslMode,
          force_https: forceHttps,
          container_port: containerPort ? parseInt(containerPort) : undefined,
        })
        .then(() => {
          setHostname("");
          setContainerPort("");
        }),
      {
        loading: "Adding domain...",
        success: "Domain added",
        error: (err) => err.message,
      },
    );
  };

  if (isLoading) {
    return (
      <div className="space-y-4">
        <Card>
          <CardHeader>
            <Skeleton className="h-6 w-32" />
          </CardHeader>
          <CardContent>
            <Skeleton className="h-10 w-full" />
          </CardContent>
        </Card>
      </div>
    );
  }

  return (
    <div className="space-y-4">
      <Card>
        <CardHeader>
          <CardTitle>Add Domain</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="grid grid-cols-2 gap-3">
            <div className="space-y-2">
              <Label htmlFor="hostname">Hostname</Label>
              <Input
                id="hostname"
                placeholder="app.example.com"
                value={hostname}
                onChange={(e) => setHostname(e.target.value)}
              />
            </div>
            <div className="space-y-2">
              <Label>Container Port</Label>
              <Input
                type="number"
                min="1"
                max="65535"
                placeholder="3000"
                value={containerPort}
                onChange={(e) => setContainerPort(e.target.value)}
              />
            </div>
          </div>
          <div className="flex items-end gap-3">
            <div className="space-y-2">
              <Label>SSL Mode</Label>
              <Select
                value={sslMode}
                onValueChange={(value) => setSslMode(value ?? "automatic")}
              >
                <SelectTrigger className="w-48">
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
            <label className="flex items-center gap-1.5 pb-2 text-sm">
              <input
                type="checkbox"
                checked={forceHttps}
                onChange={(e) => setForceHttps(e.target.checked)}
              />
              Force HTTPS
            </label>
            <Button onClick={handleAdd} disabled={addDomain.isPending}>
              {addDomain.isPending && (
                <Loader2 className="mr-1 h-4 w-4 animate-spin" />
              )}
              {addDomain.isPending ? "Adding..." : "Add"}
            </Button>
          </div>
        </CardContent>
      </Card>

      {domains && domains.length > 0 && (
        <div className="space-y-4">
          {domains.map((domain) => (
            <DomainCard
              key={domain.id}
              domain={domain}
              projectId={projectId}
              applicationId={applicationId}
            />
          ))}
        </div>
      )}

      {(!domains || domains.length === 0) && (
        <Card>
          <CardContent className="text-muted-foreground py-8 text-center text-sm">
            No domains configured.
          </CardContent>
        </Card>
      )}
    </div>
  );
}

function DomainCard({
  domain,
  projectId,
  applicationId,
}: {
  domain: DomainExpanded;
  projectId: string;
  applicationId: string;
}) {
  const removeDomain = useRemoveDomain(projectId, applicationId);
  const updateDomain = useUpdateDomain(projectId, applicationId);
  const upsertFeature = useUpsertRouteFeature(projectId, applicationId);
  const deleteFeature = useDeleteRouteFeature(projectId, applicationId);
  const [expanded, setExpanded] = useState(false);
  const [editing, setEditing] = useState(false);
  const [editPort, setEditPort] = useState(
    domain.container_port?.toString() ?? "",
  );
  const [editSslMode, setEditSslMode] = useState(
    domain.ssl_mode ?? "automatic",
  );
  const [editForceHttps, setEditForceHttps] = useState(domain.force_https);
  const [editCertPath, setEditCertPath] = useState(domain.cert_path ?? "");
  const [editKeyPath, setEditKeyPath] = useState(domain.key_path ?? "");

  const handleSave = () => {
    toast.promise(
      updateDomain
        .mutateAsync({
          domainId: domain.id,
          container_port: editPort ? parseInt(editPort) : null,
          ssl_mode: editSslMode,
          force_https: editForceHttps,
          cert_path: editCertPath || undefined,
          key_path: editKeyPath || undefined,
        })
        .then(() => setEditing(false)),
      {
        loading: "Saving...",
        success: "Domain updated",
        error: (err) => err.message,
      },
    );
  };

  // Feature management
  const [newFeatureType, setNewFeatureType] = useState("");
  const [newFeatureConfig, setNewFeatureConfig] = useState("{}");

  const handleAddFeature = () => {
    if (!newFeatureType) return;
    let config: Record<string, unknown>;
    try {
      config = JSON.parse(newFeatureConfig);
    } catch {
      toast.error("Invalid JSON config");
      return;
    }
    toast.promise(
      upsertFeature
        .mutateAsync({
          domainId: domain.id,
          feature_type: newFeatureType,
          config,
          enabled: true,
        })
        .then(() => {
          setNewFeatureType("");
          setNewFeatureConfig("{}");
        }),
      {
        loading: "Adding feature...",
        success: "Feature added",
        error: (err) => err.message,
      },
    );
  };

  return (
    <Card>
      <CardHeader>
        <div className="flex items-center justify-between">
          <div
            className="flex cursor-pointer items-center gap-2"
            onClick={() => setExpanded(!expanded)}
          >
            <span className="font-mono text-sm">{domain.hostname}</span>
            <Badge variant="secondary">{domain.ssl_mode ?? "automatic"}</Badge>
            {domain.force_https && <Badge variant="outline">HTTPS</Badge>}
            {domain.container_port && (
              <Badge variant="outline">:{domain.container_port}</Badge>
            )}
            {domain.features && domain.features.length > 0 && (
              <Badge variant="default">
                {domain.features.length} feature
                {domain.features.length > 1 ? "s" : ""}
              </Badge>
            )}
          </div>
          <div className="flex items-center gap-2">
            <Button
              size="sm"
              variant="ghost"
              onClick={() => {
                setEditing(!editing);
                setExpanded(true);
              }}
            >
              {editing ? "Cancel" : "Edit"}
            </Button>
            <AlertDialog>
              <AlertDialogTrigger>
                <Button size="sm" variant="ghost" className="text-destructive">
                  Remove
                </Button>
              </AlertDialogTrigger>
              <AlertDialogContent>
                <AlertDialogHeader>
                  <AlertDialogTitle>Remove {domain.hostname}?</AlertDialogTitle>
                  <AlertDialogDescription>
                    This will remove the domain and all its route features.
                  </AlertDialogDescription>
                </AlertDialogHeader>
                <AlertDialogFooter>
                  <AlertDialogCancel>Cancel</AlertDialogCancel>
                  <AlertDialogAction
                    onClick={() => {
                      toast.promise(removeDomain.mutateAsync(domain.id), {
                        loading: "Removing domain...",
                        success: "Domain removed",
                        error: (err) => err.message,
                      });
                    }}
                    className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
                  >
                    Remove
                  </AlertDialogAction>
                </AlertDialogFooter>
              </AlertDialogContent>
            </AlertDialog>
          </div>
        </div>
      </CardHeader>

      {expanded && (
        <CardContent className="space-y-4">
          {editing && (
            <div className="space-y-3 rounded-md border p-3">
              <div className="grid grid-cols-2 gap-3">
                <div className="space-y-2">
                  <Label>Container Port</Label>
                  <Input
                    type="number"
                    min="1"
                    max="65535"
                    placeholder="3000"
                    value={editPort}
                    onChange={(e) => setEditPort(e.target.value)}
                  />
                </div>
                <div className="space-y-2">
                  <Label>SSL Mode</Label>
                  <Select
                    value={editSslMode}
                    onValueChange={(value) =>
                      setEditSslMode(value ?? "automatic")
                    }
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
              </div>
              {editSslMode === "custom" && (
                <div className="grid grid-cols-2 gap-3">
                  <div className="space-y-2">
                    <Label>Certificate Path</Label>
                    <Input
                      value={editCertPath}
                      onChange={(e) => setEditCertPath(e.target.value)}
                      placeholder="/path/to/cert.pem"
                      className="font-mono text-xs"
                    />
                  </div>
                  <div className="space-y-2">
                    <Label>Key Path</Label>
                    <Input
                      value={editKeyPath}
                      onChange={(e) => setEditKeyPath(e.target.value)}
                      placeholder="/path/to/key.pem"
                      className="font-mono text-xs"
                    />
                  </div>
                </div>
              )}
              <div className="flex items-center gap-3">
                <label className="flex items-center gap-1.5 text-sm">
                  <input
                    type="checkbox"
                    checked={editForceHttps}
                    onChange={(e) => setEditForceHttps(e.target.checked)}
                  />
                  Force HTTPS
                </label>
                <Button
                  size="sm"
                  onClick={handleSave}
                  disabled={updateDomain.isPending}
                >
                  {updateDomain.isPending ? "Saving..." : "Save"}
                </Button>
              </div>
            </div>
          )}

          {/* Route Features */}
          <div>
            <h4 className="mb-2 text-sm font-medium">Route Features</h4>
            {domain.features && domain.features.length > 0 ? (
              <div className="space-y-2">
                {domain.features.map((feature) => (
                  <FeatureRow
                    key={feature.id}
                    feature={feature}
                    onDelete={() => {
                      toast.promise(
                        deleteFeature.mutateAsync({
                          domainId: domain.id,
                          featureId: feature.id,
                        }),
                        {
                          loading: "Removing...",
                          success: "Feature removed",
                          error: (err) => err.message,
                        },
                      );
                    }}
                    onToggle={(enabled) => {
                      toast.promise(
                        upsertFeature.mutateAsync({
                          domainId: domain.id,
                          feature_type: feature.feature_type,
                          config: feature.config as Record<string, unknown>,
                          enabled,
                        }),
                        {
                          loading: "Updating...",
                          success: enabled
                            ? "Feature enabled"
                            : "Feature disabled",
                          error: (err) => err.message,
                        },
                      );
                    }}
                  />
                ))}
              </div>
            ) : (
              <p className="text-muted-foreground text-xs">
                No route features configured.
              </p>
            )}

            <div className="mt-3 flex items-end gap-2">
              <div className="space-y-1">
                <Label className="text-xs">Type</Label>
                <Select
                  value={newFeatureType}
                  onValueChange={(value) => setNewFeatureType(value ?? "")}
                >
                  <SelectTrigger className="w-40">
                    <SelectValue placeholder="Select..." />
                  </SelectTrigger>
                  <SelectContent>
                    {FEATURE_TYPES.map((ft) => (
                      <SelectItem key={ft.value} value={ft.value}>
                        {ft.label}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              <div className="flex-1 space-y-1">
                <Label className="text-xs">Config (JSON)</Label>
                <Input
                  value={newFeatureConfig}
                  onChange={(e) => setNewFeatureConfig(e.target.value)}
                  placeholder='{"key": "value"}'
                  className="font-mono text-xs"
                />
              </div>
              <Button
                size="sm"
                variant="outline"
                onClick={handleAddFeature}
                disabled={!newFeatureType || upsertFeature.isPending}
              >
                Add
              </Button>
            </div>
          </div>
        </CardContent>
      )}
    </Card>
  );
}

function FeatureRow({
  feature,
  onDelete,
  onToggle,
}: {
  feature: RouteFeature;
  onDelete: () => void;
  onToggle: (enabled: boolean) => void;
}) {
  return (
    <div className="flex items-center justify-between rounded border p-2 text-sm">
      <div className="flex items-center gap-2">
        <input
          type="checkbox"
          checked={feature.enabled}
          onChange={(e) => onToggle(e.target.checked)}
        />
        <Badge variant="outline">{feature.feature_type}</Badge>
        <span className="text-muted-foreground font-mono text-xs">
          {JSON.stringify(feature.config).slice(0, 60)}
          {JSON.stringify(feature.config).length > 60 ? "..." : ""}
        </span>
      </div>
      <Button
        size="sm"
        variant="ghost"
        className="text-destructive h-6 px-2 text-xs"
        onClick={onDelete}
      >
        Remove
      </Button>
    </div>
  );
}
