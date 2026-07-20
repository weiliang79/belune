import { useState } from "react";
import { toast } from "sonner";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { PendingChangeBadge } from "@/lib/components/pending-change-badge";
import { useSetResources } from "@/lib/hooks/use-applications";
import type { Application } from "@/lib/types";

/**
 * CPU and memory limits, split out of the general settings form. Limits are
 * applied when the container is next created, so saving stamps the config
 * marker and the header badge points at Reload — the same as the other
 * container-shaping fields.
 */
export function ResourcesSection({
  projectId,
  applicationId,
  application,
}: {
  projectId: string;
  applicationId: string;
  application: Application;
}) {
  const setResources = useSetResources(projectId, applicationId);

  const [cpu, setCpu] = useState(application.cpu_limit?.toString() ?? "0");
  const [memoryMb, setMemoryMb] = useState(
    Math.round(application.memory_limit / (1024 * 1024)).toString(),
  );

  const save = () => {
    const cpuLimit = parseFloat(cpu) || 0;
    const memMb = parseInt(memoryMb, 10) || 0;
    if (cpuLimit < 0 || memMb < 0) {
      toast.error("Limits cannot be negative");
      return;
    }
    toast.promise(
      setResources.mutateAsync({
        cpu_limit: cpuLimit,
        memory_limit: memMb > 0 ? memMb * 1024 * 1024 : 0,
      }),
      {
        loading: "Saving...",
        success: "Resource limits saved",
        error: (err) => err.message,
      },
    );
  };

  return (
    <Card>
      <CardHeader>
        <CardTitle>Resources</CardTitle>
        <CardDescription>
          CPU and memory ceilings for the container. Leave a field at 0 for no
          limit. Applied when the container is next recreated.
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="grid grid-cols-2 gap-4">
          <div className="space-y-2">
            <Label>CPU Limit (cores)</Label>
            <Input
              type="number"
              min="0"
              step="0.1"
              value={cpu}
              onChange={(e) => setCpu(e.target.value)}
              placeholder="0 = unlimited"
            />
            <p className="text-muted-foreground text-xs">
              e.g. 0.5 = half a core, 0 = unlimited
            </p>
          </div>
          <div className="space-y-2">
            <Label>Memory Limit (MB)</Label>
            <Input
              type="number"
              min="0"
              step="64"
              value={memoryMb}
              onChange={(e) => setMemoryMb(e.target.value)}
              placeholder="0 = unlimited"
            />
            <p className="text-muted-foreground text-xs">
              e.g. 512 = 512 MB, 0 = unlimited
            </p>
          </div>
        </div>
        <div className="flex items-center justify-end gap-3">
          <PendingChangeBadge app={application} className="mr-auto" />
          <Button onClick={save} disabled={setResources.isPending}>
            {setResources.isPending ? "Saving..." : "Save"}
          </Button>
        </div>
      </CardContent>
    </Card>
  );
}
