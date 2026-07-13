import { useState } from "react";
import { toast } from "sonner";
import {
  ArrowRightIcon,
  ListIcon,
  LockIcon,
  ShieldIcon,
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
import {
  useDomains,
  useUpsertRouteFeature,
  useDeleteRouteFeature,
} from "@/lib/hooks/use-domains";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import type { DomainExpanded, RouteFeature } from "@/lib/types";

const FEATURE_TYPES = [
  { value: "basic_auth", label: "Basic Auth", Icon: LockIcon },
  { value: "headers", label: "Custom Headers", Icon: ListIcon },
  { value: "ip_allowlist", label: "IP Allowlist", Icon: ShieldIcon },
  { value: "redirect", label: "Redirect", Icon: ArrowRightIcon },
] as const;

/**
 * Route features for one domain — middleware Caddy applies to the route.
 *
 * These save on their own, not with the dialog's Save Changes: each one is an
 * immediate upsert or delete against its own endpoint. That is deliberate but
 * worth knowing, and it is why the tab only exists when editing an existing
 * domain — a domain that has not been created yet has no id to attach them to.
 */
export function DomainFeatures({
  projectId,
  applicationId,
  domain,
}: {
  projectId: string;
  applicationId: string;
  domain: DomainExpanded;
}) {
  // Read the features from the query, not from the `domain` prop.
  //
  // The prop is a snapshot taken when Edit was clicked and it never updates: the
  // page holds the domain in useState, so adding a feature invalidated the query,
  // refetched the list, and left this dialog still rendering the object from
  // before — reporting "No route features configured" for a feature that had in
  // fact been created, saved, and pushed to Caddy. The mutations below already
  // invalidate this query, so reading from it makes the list live.
  const { data: domains } = useDomains(projectId, applicationId);
  const live = domains?.find((d) => d.id === domain.id) ?? domain;

  const upsertFeature = useUpsertRouteFeature(projectId, applicationId);
  const deleteFeature = useDeleteRouteFeature(projectId, applicationId);
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
        loading: "Adding feature…",
        success: "Feature added",
        error: (err) => err.message,
      },
    );
  };

  const features = live.route_features ?? [];

  return (
    <div className="min-w-0 space-y-3">
      {features.length > 0 ? (
        <div className="space-y-2">
          {features.map((feature) => (
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
                    loading: "Removing…",
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
                    loading: "Updating…",
                    success: enabled ? "Feature enabled" : "Feature disabled",
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

      <div className="space-y-2 border-t pt-3">
        <div className="space-y-1">
          <Label className="text-xs">Type</Label>
          <Select
            value={newFeatureType}
            onValueChange={(value) => setNewFeatureType(value ?? "")}
          >
            <SelectTrigger className="w-full capitalize">
              <SelectValue placeholder="Select…" />
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
        <div className="space-y-1">
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
          className="w-full"
          onClick={handleAddFeature}
          disabled={!newFeatureType || upsertFeature.isPending}
        >
          Add feature
        </Button>
      </div>
    </div>
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
    // min-w-0 the whole way down, or truncate does nothing: a flex item defaults
    // to min-width:auto and refuses to shrink below its content, so one long
    // unbreakable string (a bcrypt hash is exactly that) widens the row, the tab,
    // and the dialog with it.
    <div className="flex w-full min-w-0 items-center justify-between gap-2 rounded border p-2 text-sm">
      <div className="flex min-w-0 flex-1 items-center gap-2">
        <input
          type="checkbox"
          className="shrink-0"
          checked={feature.enabled}
          onChange={(e) => onToggle(e.target.checked)}
        />
        <Badge variant="outline" className="shrink-0">
          {feature.feature_type}
        </Badge>
        <span className="text-muted-foreground min-w-0 flex-1 truncate font-mono text-xs">
          {configStr}
        </span>
      </div>
      <Button
        size="sm"
        variant="ghost"
        className="text-destructive h-6 shrink-0 px-2 text-xs"
        onClick={onDelete}
      >
        Remove
      </Button>
    </div>
  );
}

/**
 * Route features in their own dialog, not a tab on the edit form.
 *
 * They do not belong to that form: every change here is an immediate upsert or
 * delete against its own endpoint, so sitting beside a "Save Changes" button
 * they read as though they save with it — and they do not. Their own dialog says
 * what is true.
 */
export function DomainFeaturesDialog({
  projectId,
  applicationId,
  domain,
  open,
  onOpenChange,
}: {
  projectId: string;
  applicationId: string;
  domain?: DomainExpanded;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      {/* grid-cols-[minmax(0,1fr)]: DialogContent is a grid whose implicit column
          is max-content, so one long unbreakable string (a bcrypt hash) would
          widen the column itself and no min-w-0/truncate on the children could
          shrink it. */}
      <DialogContent className="max-h-[calc(100dvh-2rem)] grid-cols-[minmax(0,1fr)] overflow-y-auto sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>Route Features</DialogTitle>
          <DialogDescription>
            Middleware Caddy applies to{" "}
            <span className="font-mono">{domain?.hostname}</span>. Changes take
            effect immediately.
          </DialogDescription>
        </DialogHeader>
        {domain && (
          <DomainFeatures
            key={domain.id}
            projectId={projectId}
            applicationId={applicationId}
            domain={domain}
          />
        )}
      </DialogContent>
    </Dialog>
  );
}
