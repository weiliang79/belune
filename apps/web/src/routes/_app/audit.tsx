import { createFileRoute } from "@tanstack/react-router";
import { useMemo, useState } from "react";
import { DownloadIcon, SearchIcon } from "lucide-react";
import { RouteError } from "@/lib/components/route-error";
import { useAuditLogs, useAuditActions } from "@/lib/hooks/use-audit-logs";
import { useUsers } from "@/lib/hooks/use-users";
import { auditExportUrl } from "@/lib/api/audit-logs";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { formatRelativeTime } from "@/lib/utils/format";
import { actionLabel, actionClass } from "@/lib/utils/audit-format";
import { cn } from "@/lib/utils";
import type { AuditLog } from "@/lib/types";

export const Route = createFileRoute("/_app/audit")({
  component: AuditLogPage,
  errorComponent: RouteError,
});

const PAGE_SIZE = 50;

interface Filters {
  search: string;
  action: string;
  actorId: string;
}

function AuditLogPage() {
  const [offset, setOffset] = useState(0);
  const [filters, setFilters] = useState<Filters>({
    search: "",
    action: "",
    actorId: "",
  });

  const { data, isLoading } = useAuditLogs({
    limit: PAGE_SIZE,
    offset,
    action: filters.action || undefined,
    user_id: filters.actorId || undefined,
  });
  const { data: actions } = useAuditActions();
  const { data: users } = useUsers();

  const update = (f: Partial<Filters>) => {
    setFilters((prev) => ({ ...prev, ...f }));
    setOffset(0);
  };

  // Free-text search is applied client-side over the loaded page.
  const items = useMemo(() => {
    const q = filters.search.trim().toLowerCase();
    if (!q || !data) return data?.items ?? [];
    return data.items.filter((l) =>
      [l.action, l.resource_type, l.resource_name, l.resource_id, l.user_email]
        .filter(Boolean)
        .some((v) => v!.toLowerCase().includes(q)),
    );
  }, [data, filters.search]);

  const handleExport = () => {
    window.open(
      auditExportUrl({
        action: filters.action || undefined,
        user_id: filters.actorId || undefined,
      }),
      "_blank",
    );
  };

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">
            Audit Log
            {data && (
              <span className="text-muted-foreground ml-2 text-base font-normal">
                {data.total} {data.total === 1 ? "event" : "events"}
              </span>
            )}
          </h1>
          <p className="text-muted-foreground text-sm">
            Activity log of all sensitive operations.
          </p>
        </div>
        <Button variant="outline" size="sm" onClick={handleExport}>
          <DownloadIcon aria-hidden="true" className="size-4" />
          Export CSV
        </Button>
      </div>

      <div className="flex flex-wrap items-center gap-2">
        <div className="relative min-w-0 flex-1 sm:max-w-xs">
          <SearchIcon
            aria-hidden="true"
            className="text-text-faint pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2"
          />
          <Input
            value={filters.search}
            onChange={(e) => setFilters((p) => ({ ...p, search: e.target.value }))}
            placeholder="Search actor, resource, action…"
            aria-label="Search audit log"
            className="pl-9"
          />
        </div>

        <Select
          value={filters.action}
          onValueChange={(v) => update({ action: v ?? "" })}
        >
          <SelectTrigger className="w-44">
            <SelectValue placeholder="All actions" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="">All actions</SelectItem>
            {actions?.map((a) => (
              <SelectItem key={a} value={a}>
                {actionLabel(a)}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>

        <Select
          value={filters.actorId}
          onValueChange={(v) => update({ actorId: v ?? "" })}
        >
          <SelectTrigger className="w-44">
            <SelectValue placeholder="All actors" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="">All actors</SelectItem>
            {users?.map((u) => (
              <SelectItem key={u.id} value={u.id}>
                {u.email}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>Events</CardTitle>
        </CardHeader>
        <CardContent>
          {isLoading ? (
            <div className="space-y-2">
              {[1, 2, 3, 4, 5].map((i) => (
                <Skeleton key={i} className="h-12 w-full" />
              ))}
            </div>
          ) : items.length === 0 ? (
            <p className="text-muted-foreground py-8 text-center text-sm">
              {offset > 0 ? "No more entries." : "No audit log entries found."}
            </p>
          ) : (
            <div className="space-y-1.5">
              {items.map((log) => (
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

function AuditLogRow({ log }: { log: AuditLog }) {
  const [expanded, setExpanded] = useState(false);
  const resource = log.resource_name || log.resource_id?.slice(0, 8);

  return (
    <div
      className="hover:bg-card-hover cursor-pointer rounded-md border px-3 py-2.5 transition-colors"
      onClick={() => setExpanded(!expanded)}
    >
      <div className="flex flex-wrap items-center gap-x-3 gap-y-1 md:flex-nowrap">
        <span
          className={cn(
            "shrink-0 rounded-md px-2 py-0.5 text-xs font-medium",
            actionClass(log.action),
          )}
        >
          {actionLabel(log.action)}
        </span>
        <span className="text-sm">
          <span className="text-text-faint text-xs tracking-wide uppercase">
            {log.resource_type}
          </span>{" "}
          {resource && <span className="font-medium">{resource}</span>}
        </span>
        <span className="text-muted-foreground ml-auto shrink-0 text-xs">
          {log.user_email ?? "system"}
        </span>
        {log.ip_address && (
          <span className="text-text-faint shrink-0 font-mono text-xs">
            {log.ip_address}
          </span>
        )}
        <span className="text-text-faint w-24 shrink-0 text-right text-xs">
          {formatRelativeTime(log.created_at)}
        </span>
      </div>
      {expanded && log.details && (
        <pre className="bg-elev mt-2 overflow-auto rounded p-2 text-xs">
          {JSON.stringify(log.details, null, 2)}
        </pre>
      )}
    </div>
  );
}
