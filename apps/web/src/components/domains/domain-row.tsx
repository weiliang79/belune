import { useState } from "react";
import { toast } from "sonner";
import {
  ArrowRightIcon,
  CopyIcon,
  ListIcon,
  LockIcon,
  MoreHorizontal,
  PencilIcon,
  ShieldIcon,
  Trash2Icon,
} from "lucide-react";
import {
  Card,
  CardContent,
  CardHeader,
} from "@/components/ui/card";
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
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import {
  useRemoveDomain,
  useUpsertRouteFeature,
  useDeleteRouteFeature,
} from "@/lib/hooks/use-domains";
import type { DomainExpanded, RouteFeature } from "@/lib/types";
import { DomainTLSBadge } from "./domain-tls-badge";

const FEATURE_TYPES = [
  { value: "basic_auth", label: "Basic Auth", Icon: LockIcon },
  { value: "headers", label: "Custom Headers", Icon: ListIcon },
  { value: "ip_allowlist", label: "IP Allowlist", Icon: ShieldIcon },
  { value: "redirect", label: "Redirect", Icon: ArrowRightIcon },
] as const;

interface Props {
  projectId: string;
  applicationId: string;
  domain: DomainExpanded;
  onEdit: () => void;
}

export function DomainRow({ projectId, applicationId, domain, onEdit }: Props) {
  const removeDomain = useRemoveDomain(projectId, applicationId);
  const upsertFeature = useUpsertRouteFeature(projectId, applicationId);
  const deleteFeature = useDeleteRouteFeature(projectId, applicationId);
  const [expanded, setExpanded] = useState(false);
  const [newFeatureType, setNewFeatureType] = useState("");
  const [newFeatureConfig, setNewFeatureConfig] = useState("{}");

  const handleCopyHostname = () => {
    navigator.clipboard
      .writeText(domain.hostname)
      .then(() => toast.success("Hostname copied"))
      .catch(() => toast.error("Failed to copy"));
  };

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

  const featureCount = domain.features?.length ?? 0;

  return (
    <Card>
      <CardHeader>
        <div className="flex items-center justify-between">
          <button
            type="button"
            className="flex cursor-pointer items-center gap-2 text-left"
            onClick={() => setExpanded(!expanded)}
          >
            <span className="font-mono text-sm">{domain.hostname}</span>
            <DomainTLSBadge domain={domain} />
            {domain.force_https && <Badge variant="outline">HTTPS</Badge>}
            {domain.container_port && (
              <Badge variant="outline">:{domain.container_port}</Badge>
            )}
            {featureCount > 0 && (
              <Badge variant="default">
                {featureCount} feature{featureCount > 1 ? "s" : ""}
              </Badge>
            )}
          </button>
          <div className="flex items-center gap-1">
            <AlertDialog>
              <DropdownMenu>
                <DropdownMenuTrigger
                  render={
                    <Button size="icon" variant="ghost" aria-label="Actions" />
                  }
                >
                  <MoreHorizontal className="h-4 w-4" />
                </DropdownMenuTrigger>
                <DropdownMenuContent align="end">
                  <DropdownMenuItem onClick={onEdit}>
                    <PencilIcon aria-hidden="true" />
                    Edit
                  </DropdownMenuItem>
                  <DropdownMenuItem onClick={handleCopyHostname}>
                    <CopyIcon aria-hidden="true" />
                    Copy hostname
                  </DropdownMenuItem>
                  <DropdownMenuSeparator />
                  <AlertDialogTrigger
                    render={
                      <DropdownMenuItem variant="destructive" />
                    }
                    nativeButton={false}
                  >
                    <Trash2Icon aria-hidden="true" />
                    Delete
                  </AlertDialogTrigger>
                </DropdownMenuContent>
              </DropdownMenu>
              <AlertDialogContent>
                <AlertDialogHeader>
                  <AlertDialogTitle>Delete {domain.hostname}?</AlertDialogTitle>
                  <AlertDialogDescription>
                    This will delete the domain and all its route features.
                  </AlertDialogDescription>
                </AlertDialogHeader>
                <AlertDialogFooter>
                  <AlertDialogCancel>Cancel</AlertDialogCancel>
                  <AlertDialogAction
                    onClick={() => {
                      toast.promise(removeDomain.mutateAsync(domain.id), {
                        loading: "Deleting domain...",
                        success: "Domain deleted",
                        error: (err) => err.message,
                      });
                    }}
                    className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
                  >
                    Delete
                  </AlertDialogAction>
                </AlertDialogFooter>
              </AlertDialogContent>
            </AlertDialog>
          </div>
        </div>
      </CardHeader>

      {expanded && (
        <CardContent className="space-y-4">
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
                  <SelectTrigger className="w-40 capitalize">
                    <SelectValue placeholder="Select..." />
                  </SelectTrigger>
                  <SelectContent>
                    {FEATURE_TYPES.map((ft) => (
                      <SelectItem
                        key={ft.value}
                        value={ft.value}
                        icon={<ft.Icon />}
                        className="capitalize"
                      >
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
  const configStr = JSON.stringify(feature.config);
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
          {configStr.slice(0, 60)}
          {configStr.length > 60 ? "..." : ""}
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
