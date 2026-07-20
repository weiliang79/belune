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
import {
  SegmentedControl,
  SegmentedControlItem,
} from "@/components/ui/segmented-control";
import { useSetHealthCheck } from "@/lib/hooks/use-applications";
import type { Application } from "@/lib/types";

/**
 * Configures how the application's health is checked. Three mechanisms:
 *
 *  - None — no check.
 *  - HTTP — the control plane probes a path once after each deploy. Simple, and
 *    needs nothing installed in the image, but only works for HTTP services.
 *  - Command — a native Docker HEALTHCHECK run inside the container, continuously.
 *    Works for anything (a database, a queue, a worker), and because it keeps
 *    running, the container's health drives the application's status: a failing
 *    check shows the app as Unhealthy, not Running.
 *
 * A change takes effect on the next deploy (or reload) — the check is part of
 * the container, so the running one keeps its old check until then.
 */
type HealthType = "none" | "http" | "command";

// A blank numeric input means "use the default"; it is sent as undefined so the
// server stores NULL and the platform default applies.
function numOrUndefined(s: string): number | undefined {
  const n = parseInt(s, 10);
  return Number.isFinite(n) && n > 0 ? n : undefined;
}

export function HealthCheckSection({
  projectId,
  applicationId,
  application,
}: {
  projectId: string;
  applicationId: string;
  application: Application;
}) {
  const setHealthCheck = useSetHealthCheck(projectId, applicationId);

  const [type, setType] = useState<HealthType>(application.health_check_type);
  const [path, setPath] = useState(application.health_check_path ?? "");
  const [expectStatus, setExpectStatus] = useState(
    application.health_check_expect_status?.toString() ?? "",
  );
  const [command, setCommand] = useState(application.health_check_command ?? "");
  const [interval, setInterval] = useState(
    application.health_check_interval_seconds?.toString() ?? "",
  );
  const [retries, setRetries] = useState(
    application.health_check_retries?.toString() ?? "",
  );
  const [startPeriod, setStartPeriod] = useState(
    application.health_check_start_period_seconds?.toString() ?? "",
  );
  const [timeout, setTimeout] = useState(
    application.health_check_timeout_seconds?.toString() ?? "",
  );

  const canSave =
    type === "none" ||
    (type === "http" && path.trim() !== "") ||
    (type === "command" && command.trim() !== "");

  const save = () => {
    const data =
      type === "none"
        ? { type: "none" as const }
        : type === "http"
          ? {
              type: "http" as const,
              path: path.trim(),
              expect_status: numOrUndefined(expectStatus),
              timeout_seconds: numOrUndefined(timeout),
            }
          : {
              type: "command" as const,
              command: command.trim(),
              interval_seconds: numOrUndefined(interval),
              retries: numOrUndefined(retries),
              start_period_seconds: numOrUndefined(startPeriod),
              timeout_seconds: numOrUndefined(timeout),
            };

    toast.promise(setHealthCheck.mutateAsync(data), {
      loading: "Saving...",
      success: "Health check saved — applies on the next deploy",
      error: (err) => err.message,
    });
  };

  return (
    <Card>
      <CardHeader>
        <CardTitle>Health Check</CardTitle>
        <CardDescription>
          How the platform decides the application is healthy. A command check
          runs continuously inside the container and marks the app Unhealthy when
          it fails; an HTTP check is probed once after each deploy. Changes apply
          on the next deploy.
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="space-y-2">
          <Label>Method</Label>
          <SegmentedControl
            value={type}
            onValueChange={(v) => setType(v as HealthType)}
          >
            <SegmentedControlItem value="none">None</SegmentedControlItem>
            <SegmentedControlItem value="http">HTTP</SegmentedControlItem>
            <SegmentedControlItem value="command">Command</SegmentedControlItem>
          </SegmentedControl>
        </div>

        {type === "http" && (
          <div className="space-y-4">
            <div className="space-y-2">
              <Label>Path</Label>
              <Input
                value={path}
                onChange={(e) => setPath(e.target.value)}
                placeholder="/healthz"
                className="font-mono"
              />
              <p className="text-muted-foreground text-xs">
                Probed on the container's port after each deploy. A non-2xx
                response (or the code below) fails the deploy.
              </p>
            </div>
            <div className="grid grid-cols-2 gap-4">
              <div className="space-y-2">
                <Label>Expected status</Label>
                <Input
                  type="number"
                  value={expectStatus}
                  onChange={(e) => setExpectStatus(e.target.value)}
                  placeholder="any 2xx"
                />
              </div>
              <div className="space-y-2">
                <Label>Timeout (seconds)</Label>
                <Input
                  type="number"
                  value={timeout}
                  onChange={(e) => setTimeout(e.target.value)}
                  placeholder="120"
                />
              </div>
            </div>
          </div>
        )}

        {type === "command" && (
          <div className="space-y-4">
            <div className="space-y-2">
              <Label>Command</Label>
              <Input
                value={command}
                onChange={(e) => setCommand(e.target.value)}
                placeholder="curl -f http://localhost:3000/health || exit 1"
                className="font-mono"
              />
              <p className="text-muted-foreground text-xs">
                Run inside the container via <code>sh -c</code>. Exit 0 = healthy.
                The tool you use (curl, wget, pg_isready…) must exist in the
                image.
              </p>
            </div>
            <div className="grid grid-cols-2 gap-4">
              <div className="space-y-2">
                <Label>Interval (seconds)</Label>
                <Input
                  type="number"
                  value={interval}
                  onChange={(e) => setInterval(e.target.value)}
                  placeholder="30"
                />
              </div>
              <div className="space-y-2">
                <Label>Timeout (seconds)</Label>
                <Input
                  type="number"
                  value={timeout}
                  onChange={(e) => setTimeout(e.target.value)}
                  placeholder="30"
                />
              </div>
              <div className="space-y-2">
                <Label>Retries</Label>
                <Input
                  type="number"
                  value={retries}
                  onChange={(e) => setRetries(e.target.value)}
                  placeholder="3"
                />
                <p className="text-muted-foreground text-xs">
                  Consecutive failures before Unhealthy.
                </p>
              </div>
              <div className="space-y-2">
                <Label>Start period (seconds)</Label>
                <Input
                  type="number"
                  value={startPeriod}
                  onChange={(e) => setStartPeriod(e.target.value)}
                  placeholder="0"
                />
                <p className="text-muted-foreground text-xs">
                  Grace window at startup where failures don't count.
                </p>
              </div>
            </div>
          </div>
        )}

        <Button onClick={save} disabled={!canSave || setHealthCheck.isPending}>
          {setHealthCheck.isPending ? "Saving..." : "Save Health Check"}
        </Button>
      </CardContent>
    </Card>
  );
}
