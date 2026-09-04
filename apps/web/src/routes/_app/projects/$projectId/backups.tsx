import { createFileRoute, Link } from "@tanstack/react-router";
import { Loader2, Cloud, Archive, Database, HardDrive } from "lucide-react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { CardDescription } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import { DestinationsPanel } from "@/components/backups/destinations-panel";
import { OrphanedBackupsPanel } from "@/components/backups/orphaned-backups-panel";
import { useProjectBackups } from "@/lib/hooks/use-project-backups";
import { useProject } from "@/lib/hooks/use-projects";
import { useAuthStore } from "@/lib/stores/auth";
import { formatBytes, formatDateTimeShort } from "@/lib/utils/format";
import { cn } from "@/lib/utils";
import type { ProjectBackupActivity } from "@/lib/types";

export const Route = createFileRoute("/_app/projects/$projectId/backups")({
  component: ProjectBackupsPage,
});

function statusTone(status: ProjectBackupActivity["status"]): string {
  switch (status) {
    case "succeeded":
      return "text-status-ready";
    case "failed":
      return "text-status-error";
    default:
      return "text-status-building";
  }
}

function ProjectBackupsPage() {
  const { projectId } = Route.useParams();
  const { data: activity, isLoading } = useProjectBackups(projectId);
  const { data: project } = useProject(projectId);
  const currentUser = useAuthStore((s) => s.user);
  const canDelete =
    currentUser?.role === "admin" || currentUser?.id === project?.user_id;

  return (
    <div className="space-y-6">
      <DestinationsPanel projectId={projectId} />

      <OrphanedBackupsPanel projectId={projectId} canDelete={canDelete} />

      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Archive aria-hidden="true" className="size-4" />
            Recent Backup Activity
          </CardTitle>
          <CardDescription>
            The latest backups across all databases and volumes in this project.
          </CardDescription>
        </CardHeader>
        <CardContent>
          {isLoading ? (
            <Skeleton className="h-16 w-full" />
          ) : !activity || activity.length === 0 ? (
            <p className="text-text-faint py-4 text-center text-sm">
              No backups yet. Configure scheduled backups from a database&apos;s
              Backups tab or an application&apos;s Mounts tab.
            </p>
          ) : (
            <ul className="divide-border divide-y">
              {activity.map((b) => (
                <li
                  key={b.id}
                  className="flex items-center justify-between gap-3 py-3"
                >
                  <div className="min-w-0">
                    <div className="flex items-center gap-2">
                      {b.kind === "volume" ? (
                        <HardDrive
                          className="text-text-faint h-3.5 w-3.5 shrink-0"
                          aria-label="Application volume"
                        />
                      ) : (
                        <Database
                          className="text-text-faint h-3.5 w-3.5 shrink-0"
                          aria-label="Database"
                        />
                      )}
                      {b.kind === "volume" ? (
                        <>
                          <Link
                            to="/projects/$projectId/applications/$applicationId/mounts"
                            params={{ projectId, applicationId: b.resource_id }}
                            className="hover:text-foreground truncate text-sm font-medium"
                          >
                            {b.resource_name}
                          </Link>
                          {b.app_name && (
                            <Badge variant="secondary">{b.app_name}</Badge>
                          )}
                        </>
                      ) : (
                        <Link
                          to="/projects/$projectId/databases/$databaseId"
                          params={{ projectId, databaseId: b.resource_id }}
                          className="hover:text-foreground truncate text-sm font-medium"
                        >
                          {b.resource_name}
                        </Link>
                      )}
                      <span
                        className={cn(
                          "text-xs font-medium capitalize",
                          statusTone(b.status),
                        )}
                      >
                        {b.status === "running" && (
                          <Loader2 className="mr-1 inline h-3 w-3 animate-spin" />
                        )}
                        {b.status}
                      </span>
                      {b.has_remote && (
                        <Cloud
                          className="text-text-faint h-3.5 w-3.5"
                          aria-label="Uploaded to destination"
                        />
                      )}
                      {b.status === "succeeded" && (
                        <span className="text-text-faint text-xs">
                          {formatBytes(b.size_bytes)}
                        </span>
                      )}
                    </div>
                    <p className="text-text-faint text-xs">
                      {formatDateTimeShort(b.started_at)}
                    </p>
                    {b.error && (
                      <p className="text-status-error mt-0.5 truncate text-xs">
                        {b.error}
                      </p>
                    )}
                  </div>
                </li>
              ))}
            </ul>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
