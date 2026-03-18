import { createFileRoute } from "@tanstack/react-router";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { SettingsNav } from "@/lib/components/settings-nav";
import { useMetrics, useTriggerCleanup } from "@/lib/hooks/use-metrics";
import { toast } from "sonner";

export const Route = createFileRoute("/_app/settings/server")({
  component: ServerSettingsPage,
});

function ServerSettingsPage() {
  const { data: metrics, isLoading } = useMetrics();
  const cleanup = useTriggerCleanup();

  const handleCleanup = () => {
    cleanup.mutate(undefined, {
      onSuccess: () => toast.success("Cleanup task queued"),
      onError: () => toast.error("Failed to trigger cleanup"),
    });
  };

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold">Settings</h1>
        <p className="text-muted-foreground">
          Manage your account and platform settings.
        </p>
      </div>

      <SettingsNav />

      {isLoading ? (
        <div className="grid grid-cols-2 gap-4 md:grid-cols-4">
          {Array.from({ length: 4 }).map((_, i) => (
            <Card key={i}>
              <CardHeader className="pb-2">
                <div className="bg-muted h-4 w-20 animate-pulse rounded" />
              </CardHeader>
              <CardContent>
                <div className="bg-muted h-8 w-12 animate-pulse rounded" />
              </CardContent>
            </Card>
          ))}
        </div>
      ) : metrics ? (
        <>
          <div className="grid grid-cols-2 gap-4 md:grid-cols-4">
            <StatCard title="Projects" value={metrics.projects} />
            <StatCard title="Services" value={metrics.services} />
            <StatCard title="Databases" value={metrics.databases} />
            <StatCard title="Deployments" value={metrics.deployments} />
          </div>

          <Card>
            <CardHeader>
              <CardTitle>Containers</CardTitle>
            </CardHeader>
            <CardContent>
              <div className="flex items-center gap-4">
                <div className="flex items-center gap-2">
                  <Badge variant="default">{metrics.containers.running}</Badge>
                  <span className="text-muted-foreground text-sm">Running</span>
                </div>
                <div className="flex items-center gap-2">
                  <Badge variant="secondary">{metrics.containers.stopped}</Badge>
                  <span className="text-muted-foreground text-sm">Stopped</span>
                </div>
                <div className="flex items-center gap-2">
                  <Badge variant="outline">{metrics.containers.total}</Badge>
                  <span className="text-muted-foreground text-sm">Total</span>
                </div>
              </div>
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle>Maintenance</CardTitle>
            </CardHeader>
            <CardContent>
              <div className="flex items-center justify-between">
                <div>
                  <p className="text-sm font-medium">Cleanup old deployments</p>
                  <p className="text-muted-foreground text-sm">
                    Remove old deployment records, images, and dangling volumes.
                    Keeps the 3 most recent deployments per service.
                  </p>
                </div>
                <Button
                  variant="outline"
                  onClick={handleCleanup}
                  disabled={cleanup.isPending}
                >
                  {cleanup.isPending ? "Running..." : "Run Cleanup"}
                </Button>
              </div>
            </CardContent>
          </Card>
        </>
      ) : null}
    </div>
  );
}

function StatCard({ title, value }: { title: string; value: number }) {
  return (
    <Card>
      <CardHeader className="pb-2">
        <p className="text-muted-foreground text-sm font-medium">{title}</p>
      </CardHeader>
      <CardContent>
        <p className="text-3xl font-bold">{value}</p>
      </CardContent>
    </Card>
  );
}
