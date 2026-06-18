import { createFileRoute, useNavigate } from "@tanstack/react-router";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Separator } from "@/components/ui/separator";
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
import { toast } from "sonner";
import {
  useDatabase,
  useDeleteDatabase,
  useUpdateDatabase,
} from "@/lib/hooks/use-databases";
import { useProject } from "@/lib/hooks/use-projects";
import { Database as DatabaseIcon, Loader2, Trash2 } from "lucide-react";
import { Skeleton } from "@/components/ui/skeleton";
import { AppBreadcrumb } from "@/lib/components/app-breadcrumb";
import { StatusBadge } from "@/lib/components/status-badge";
import { CopyButton } from "@/lib/components/copy-button";
import { formatBytes } from "@/lib/utils/format";
import type { Database } from "@/lib/types";

export const Route = createFileRoute(
  "/_app/projects/$projectId/databases/$databaseId",
)({
  component: DatabaseDetailPage,
});

function DatabaseDetailPage() {
  const { projectId, databaseId } = Route.useParams();
  const navigate = useNavigate();
  const { data: db, isLoading } = useDatabase(projectId, databaseId);
  const { data: project } = useProject(projectId);
  const deleteDb = useDeleteDatabase(projectId);

  if (isLoading || !db) {
    return (
      <div className="space-y-6">
        <Skeleton className="h-4 w-64" />
        <div>
          <Skeleton className="h-7 w-48" />
          <Skeleton className="mt-1 h-4 w-24" />
        </div>
        <Card>
          <CardHeader>
            <Skeleton className="h-6 w-40" />
          </CardHeader>
          <CardContent className="space-y-3">
            {[1, 2, 3].map((i) => (
              <Skeleton key={i} className="h-5 w-full" />
            ))}
          </CardContent>
        </Card>
      </div>
    );
  }

  const handleDelete = () => {
    toast.promise(
      deleteDb.mutateAsync(databaseId).then(() => {
        navigate({
          to: "/projects/$projectId",
          params: { projectId },
        });
      }),
      {
        loading: "Deleting database...",
        success: "Database deleted",
        error: (err) => err.message,
      },
    );
  };

  return (
    <div className="space-y-6">
      <AppBreadcrumb
        items={[
          { label: "Projects", to: "/projects" },
          {
            label: project?.name ?? "Project",
            to: `/projects/${projectId}`,
          },
          { label: db.name },
        ]}
      />

      <div className="flex items-start gap-3">
        <div className="bg-elev text-text-muted grid size-11 shrink-0 place-items-center rounded-xl">
          <DatabaseIcon aria-hidden="true" className="size-5" />
        </div>
        <div className="min-w-0">
          <h1 className="truncate text-2xl font-semibold tracking-tight">
            {db.name}
          </h1>
          <p className="text-text-faint truncate font-mono text-sm">
            {db.slug}
          </p>
          <div className="mt-2 flex items-center gap-2">
            <Badge variant="outline" className="font-mono">
              {db.type}:{db.version}
            </Badge>
            <StatusBadge status={db.status} />
          </div>
        </div>
      </div>

      {db.status === "creating" && (
        <Card>
          <CardContent className="flex items-center gap-3 py-6">
            <Loader2 className="h-5 w-5 animate-spin" />
            <div>
              <p className="font-medium">Provisioning database...</p>
              <p className="text-muted-foreground text-sm">
                This usually takes a few seconds.
              </p>
            </div>
          </CardContent>
        </Card>
      )}

      {db.status === "running" && db.credentials && (
        <Card>
          <CardHeader>
            <CardTitle>Connection Details</CardTitle>
            <CardDescription>
              Use these credentials to connect from your services on the
              internal network.
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="space-y-3">
              <div className="flex items-center justify-between text-sm">
                <span className="text-muted-foreground">Host</span>
                <div className="flex items-center gap-1">
                  <code className="text-xs">{db.internal_host}</code>
                  <CopyButton value={db.internal_host} />
                </div>
              </div>
              <div className="flex items-center justify-between text-sm">
                <span className="text-muted-foreground">Port</span>
                <div className="flex items-center gap-1">
                  <code className="text-xs">{db.internal_port}</code>
                  <CopyButton value={String(db.internal_port)} />
                </div>
              </div>
              <Separator />
              {Object.entries(db.credentials).map(([key, value]) => (
                <div
                  key={key}
                  className="flex items-center justify-between text-sm"
                >
                  <span className="text-muted-foreground">{key}</span>
                  <div className="flex items-center gap-1">
                    <code className="text-xs">{value}</code>
                    <CopyButton value={value} />
                  </div>
                </div>
              ))}
            </div>

            {db.connection_string && (
              <>
                <Separator />
                <div className="space-y-2">
                  <p className="text-muted-foreground text-sm">
                    Connection String
                  </p>
                  <div className="bg-muted flex items-center justify-between rounded-md px-3 py-2">
                    <code className="text-xs break-all">
                      {db.connection_string}
                    </code>
                    <CopyButton value={db.connection_string} />
                  </div>
                </div>
              </>
            )}
          </CardContent>
        </Card>
      )}

      {db.status === "failed" && (
        <Card>
          <CardContent className="py-6">
            <p className="text-destructive text-sm">
              Database provisioning failed. You can delete this database and try
              again.
            </p>
          </CardContent>
        </Card>
      )}

      {db.status === "running" && <AdvancedCard db={db} />}

      <Card>
        <CardHeader>
          <CardTitle className="text-destructive">Danger Zone</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="flex items-center justify-between">
            <div>
              <p className="text-sm font-medium">Delete this database</p>
              <p className="text-muted-foreground text-xs">
                This will permanently delete the database, its data, and
                container.
              </p>
            </div>
            <AlertDialog>
              <AlertDialogTrigger
                render={<Button variant="destructive" size="sm" />}
              >
                <Trash2 className="mr-1 size-4" />
                Delete
              </AlertDialogTrigger>
              <AlertDialogContent>
                <AlertDialogHeader>
                  <AlertDialogTitle>Delete database?</AlertDialogTitle>
                  <AlertDialogDescription>
                    This will permanently delete &quot;{db.name}&quot; and all
                    its data. This action cannot be undone.
                  </AlertDialogDescription>
                </AlertDialogHeader>
                <AlertDialogFooter>
                  <AlertDialogCancel>Cancel</AlertDialogCancel>
                  <AlertDialogAction
                    onClick={handleDelete}
                    className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
                  >
                    {deleteDb.isPending ? (
                      <Loader2 className="mr-1 h-4 w-4 animate-spin" />
                    ) : null}
                    Delete
                  </AlertDialogAction>
                </AlertDialogFooter>
              </AlertDialogContent>
            </AlertDialog>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}

const MB = 1024 * 1024;

/**
 * Advanced settings: editable CPU/memory limits applied live, plus read-only
 * image and managed-volume info. Image upgrades are intentionally not editable
 * here — a major version bump is a separate guarded flow.
 */
function AdvancedCard({ db }: { db: Database }) {
  const update = useUpdateDatabase(db.project_id, db.id);
  const [cpu, setCpu] = useState(String(db.cpu_limit ?? 0));
  const [memMb, setMemMb] = useState(
    String(db.memory_limit ? Math.round(db.memory_limit / MB) : 0),
  );

  const handleSave = () => {
    const cpuVal = Number(cpu);
    const memVal = Number(memMb);
    if (
      Number.isNaN(cpuVal) ||
      cpuVal < 0 ||
      Number.isNaN(memVal) ||
      memVal < 0
    ) {
      toast.error("CPU and memory must be non-negative numbers");
      return;
    }
    toast.promise(
      update.mutateAsync({ cpu_limit: cpuVal, memory_limit: memVal * MB }),
      {
        loading: "Applying resource limits…",
        success: "Resource limits updated",
        error: (err) => err.message,
      },
    );
  };

  return (
    <Card>
      <CardHeader>
        <CardTitle>Advanced</CardTitle>
        <CardDescription>
          Resource limits apply live. 0 means unlimited.
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-6">
        {/* Resource limits — editable */}
        <div className="grid gap-4 sm:grid-cols-2">
          <div className="space-y-2">
            <Label htmlFor="db-cpu">CPU limit (cores)</Label>
            <Input
              id="db-cpu"
              type="number"
              min={0}
              step={0.1}
              value={cpu}
              onChange={(e) => setCpu(e.target.value)}
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="db-mem">Memory limit (MB)</Label>
            <Input
              id="db-mem"
              type="number"
              min={0}
              step={64}
              value={memMb}
              onChange={(e) => setMemMb(e.target.value)}
            />
          </div>
        </div>
        <Button size="sm" onClick={handleSave} disabled={update.isPending}>
          {update.isPending ? "Saving…" : "Save resource limits"}
        </Button>

        <Separator />

        {/* Image — read-only */}
        <div className="flex items-start justify-between gap-3">
          <div>
            <p className="text-sm font-medium">Image</p>
            <p className="text-text-faint text-xs">
              Major version upgrades are a separate guarded flow.
            </p>
          </div>
          <Badge variant="outline" className="font-mono">
            {db.type}:{db.version}
          </Badge>
        </div>

        {/* Volume — read-only */}
        <div className="flex items-start justify-between gap-3">
          <div>
            <p className="text-sm font-medium">Volume</p>
            <p className="text-text-faint text-xs">Managed automatically.</p>
          </div>
          <div className="text-right">
            {db.volume ? (
              <>
                <p className="font-mono text-xs">{db.volume.name}</p>
                <p className="text-text-faint text-xs">
                  {formatBytes(db.volume.size_bytes)}
                </p>
              </>
            ) : (
              <p className="text-text-faint text-xs">—</p>
            )}
          </div>
        </div>
      </CardContent>
    </Card>
  );
}
