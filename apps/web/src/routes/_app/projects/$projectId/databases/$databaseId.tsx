import { createFileRoute, useNavigate } from "@tanstack/react-router";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
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
import { useDatabase, useDeleteDatabase } from "@/lib/hooks/use-databases";
import { useProject } from "@/lib/hooks/use-projects";
import { Loader2, Trash2 } from "lucide-react";
import { Skeleton } from "@/components/ui/skeleton";
import { AppBreadcrumb } from "@/lib/components/app-breadcrumb";
import { StatusBadge } from "@/lib/components/status-badge";
import { CopyButton } from "@/lib/components/copy-button";

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

      <div className="flex items-start justify-between">
        <div>
          <h2 className="text-xl font-semibold">{db.name}</h2>
          <p className="text-muted-foreground text-sm">{db.slug}</p>
          <div className="text-muted-foreground mt-1 flex items-center gap-2 text-sm">
            <Badge variant="outline">
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
