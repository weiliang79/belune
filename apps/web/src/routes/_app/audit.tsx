import { createFileRoute } from "@tanstack/react-router";
import { useState } from "react";
import {
  ActivityIcon,
  DownloadIcon,
  ListFilterIcon,
  ScrollTextIcon,
  SearchIcon,
  UserIcon,
  UsersIcon,
} from "lucide-react";
import type { ColumnDef } from "@tanstack/react-table";
import { RouteError } from "@/lib/components/route-error";
import { useAuditLogs, useAuditActions } from "@/lib/hooks/use-audit-logs";
import { useUsers } from "@/lib/hooks/use-users";
import { auditExportUrl } from "@/lib/api/audit-logs";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { DataTable } from "@/components/ui/data-table";
import { PageHeader } from "@/components/ui/page-header";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { formatDateTime } from "@/lib/utils/format";
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

const auditColumns: ColumnDef<AuditLog>[] = [
  {
    id: "action",
    header: "Action",
    accessorFn: (l) => l.action,
    cell: ({ row: { original: log } }) => (
      <span
        className={cn(
          "inline-block shrink-0 rounded-md px-2 py-0.5 text-xs font-medium",
          actionClass(log.action),
        )}
      >
        {actionLabel(log.action)}
      </span>
    ),
  },
  {
    id: "resource",
    header: "Resource",
    accessorFn: (l) =>
      [l.resource_type, l.resource_name, l.resource_id]
        .filter(Boolean)
        .join(" "),
    cell: ({ row: { original: log } }) => {
      const resource = log.resource_name || log.resource_id?.slice(0, 8);
      return (
        <span className="text-sm">
          <span className="text-text-faint text-xs tracking-wide uppercase">
            {log.resource_type}
          </span>{" "}
          {resource && <span className="font-medium">{resource}</span>}
        </span>
      );
    },
  },
  {
    id: "actor",
    header: "Actor",
    accessorFn: (l) => l.user_email ?? "",
    meta: { className: "text-muted-foreground text-xs" },
    cell: ({ row: { original: log } }) => log.user_email ?? "system",
  },
  {
    id: "ip",
    header: "IP",
    enableGlobalFilter: false,
    meta: { className: "text-text-faint font-mono text-xs" },
    cell: ({ row: { original: log } }) => log.ip_address ?? "—",
  },
  {
    id: "time",
    header: "Time",
    enableGlobalFilter: false,
    meta: {
      headerClassName: "text-right",
      className: "text-text-faint text-right text-xs whitespace-nowrap",
    },
    cell: ({ row: { original: log } }) => formatDateTime(log.created_at),
  },
];

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
      <PageHeader
        icon={<ScrollTextIcon className="size-5" />}
        title={
          <>
            Audit Log
            {data && (
              <span className="text-muted-foreground ml-2 text-base font-normal">
                {data.total} {data.total === 1 ? "event" : "events"}
              </span>
            )}
          </>
        }
        description="Activity log of all sensitive operations."
        actions={
          <Button variant="outline" size="sm" onClick={handleExport}>
            <DownloadIcon aria-hidden="true" className="size-4" />
            Export CSV
          </Button>
        }
      />

      <Card>
        <CardContent className="space-y-4">
          <div className="flex flex-wrap items-center gap-2">
            <div className="relative min-w-0 flex-1 sm:max-w-xs">
              <SearchIcon
                aria-hidden="true"
                className="text-text-faint pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2"
              />
              <Input
                value={filters.search}
                onChange={(e) =>
                  setFilters((p) => ({ ...p, search: e.target.value }))
                }
                placeholder="Search actor, resource, action…"
                aria-label="Search audit log"
                className="pl-9"
              />
            </div>

            <Select
              value={filters.action}
              onValueChange={(v) => update({ action: v ?? "" })}
            >
              <SelectTrigger className="w-44 capitalize">
                <SelectValue placeholder="All actions" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem
                  value=""
                  icon={<ListFilterIcon />}
                  className="capitalize"
                >
                  All actions
                </SelectItem>
                {actions?.map((a) => (
                  <SelectItem
                    key={a}
                    value={a}
                    icon={<ActivityIcon />}
                    className="capitalize"
                  >
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
                <SelectValue placeholder="All Actors" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="" icon={<UsersIcon />}>
                  All Actors
                </SelectItem>
                {users?.map((u) => (
                  <SelectItem key={u.id} value={u.id} icon={<UserIcon />}>
                    {u.email}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          <DataTable
            columns={auditColumns}
            data={data?.items ?? []}
            isLoading={isLoading}
            getRowId={(log) => log.id}
            globalFilter={filters.search}
            onGlobalFilterChange={(v) =>
              setFilters((p) => ({ ...p, search: v }))
            }
            emptyMessage={
              filters.search.trim()
                ? "No entries match your search."
                : offset > 0
                  ? "No more entries."
                  : "No audit log entries found."
            }
            renderDetailPanel={({ row }) =>
              row.original.details ? (
                <pre className="bg-elev overflow-auto rounded p-2 text-xs">
                  {JSON.stringify(row.original.details, null, 2)}
                </pre>
              ) : (
                <span className="text-text-faint text-xs">
                  No additional details.
                </span>
              )
            }
            pagination={{
              mode: "manual",
              offset,
              pageSize: PAGE_SIZE,
              hasMore: (data?.items.length ?? 0) === PAGE_SIZE,
              onOffsetChange: setOffset,
            }}
          />
        </CardContent>
      </Card>
    </div>
  );
}
