import { createFileRoute } from "@tanstack/react-router";
import { useState } from "react";
import { RouteError } from "@/lib/components/route-error";
import { useAuditLogs } from "@/lib/hooks/use-audit-logs";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import { formatDate } from "@/lib/utils/format";

export const Route = createFileRoute("/_app/audit")({
  component: AuditLogPage,
  errorComponent: RouteError,
});

const PAGE_SIZE = 50;

function actionColor(action: string) {
  if (action.startsWith("delete")) return "destructive" as const;
  if (action.startsWith("create")) return "default" as const;
  if (action.startsWith("update")) return "secondary" as const;
  return "outline" as const;
}

function AuditLogPage() {
  const [offset, setOffset] = useState(0);
  const { data, isLoading } = useAuditLogs({ limit: PAGE_SIZE, offset });

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold">Audit Log</h1>
        <p className="text-muted-foreground">
          Activity log of all sensitive operations.
        </p>
      </div>

      <Card>
        <CardHeader>
          <div className="flex items-center justify-between">
            <CardTitle>Events</CardTitle>
            {data && (
              <span className="text-muted-foreground text-sm">
                {data.total} total
              </span>
            )}
          </div>
        </CardHeader>
        <CardContent>
          {isLoading ? (
            <div className="space-y-2">
              {[1, 2, 3, 4, 5].map((i) => (
                <Skeleton key={i} className="h-14 w-full" />
              ))}
            </div>
          ) : !data || data.items.length === 0 ? (
            <p className="text-muted-foreground py-8 text-center text-sm">
              {offset > 0 ? "No more entries." : "No audit log entries yet."}
            </p>
          ) : (
            <div className="space-y-2">
              {data.items.map((log) => (
                <AuditLogRow key={log.id} log={log} />
              ))}
            </div>
          )}

          <div className="mt-4 flex items-center justify-between">
            <Button
              variant="outline"
              size="sm"
              disabled={offset === 0}
              onClick={() => setOffset(Math.max(0, offset - PAGE_SIZE))}
            >
              Previous
            </Button>
            <span className="text-muted-foreground text-sm">
              {offset + 1}–{offset + (data?.items.length ?? 0)}
            </span>
            <Button
              variant="outline"
              size="sm"
              disabled={!data || data.items.length < PAGE_SIZE}
              onClick={() => setOffset(offset + PAGE_SIZE)}
            >
              Next
            </Button>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}

function AuditLogRow({
  log,
}: {
  log: {
    id: string;
    action: string;
    resource_type: string;
    resource_id: string | null;
    user_email?: string;
    ip_address: string | null;
    details: Record<string, unknown> | null;
    created_at: string;
  };
}) {
  const [expanded, setExpanded] = useState(false);

  return (
    <div
      className="cursor-pointer rounded-md border p-3 transition-colors hover:bg-muted/50"
      onClick={() => setExpanded(!expanded)}
    >
      <div className="flex flex-wrap items-center gap-x-3 gap-y-1 md:flex-nowrap">
        <span className="text-muted-foreground w-36 shrink-0 text-xs">
          {formatDate(log.created_at)}
        </span>
        <Badge variant={actionColor(log.action)} className="shrink-0">
          {log.action}
        </Badge>
        <span className="text-sm">
          {log.resource_type}
          {log.resource_id && (
            <span className="text-muted-foreground font-mono text-xs">
              {" "}
              {log.resource_id.slice(0, 8)}
            </span>
          )}
        </span>
        <span className="text-muted-foreground ml-auto shrink-0 text-xs">
          {log.user_email ?? "system"}
        </span>
        {log.ip_address && (
          <span className="text-muted-foreground shrink-0 font-mono text-xs">
            {log.ip_address}
          </span>
        )}
      </div>
      {expanded && log.details && (
        <pre className="bg-muted mt-2 overflow-auto rounded p-2 text-xs">
          {JSON.stringify(log.details, null, 2)}
        </pre>
      )}
    </div>
  );
}
