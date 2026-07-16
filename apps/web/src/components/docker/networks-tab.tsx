import { useMemo, useState } from "react";
import type { ColumnDef } from "@tanstack/react-table";
import { DataTable, DataTableSearch } from "@/components/ui/data-table";
import { Card, CardContent } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { useDockerNetworks } from "@/lib/hooks/use-docker";
import { formatDateTimeShort } from "@/lib/utils/format";
import type { DockerNetwork } from "@/lib/types";
import { ManagedBadge } from "./shared";
import { shortId } from "./utils";

export function DockerNetworksTab({ enabled }: { enabled: boolean }) {
  const { data, isPending } = useDockerNetworks(enabled);
  const [search, setSearch] = useState("");

  const columns = useMemo<ColumnDef<DockerNetwork>[]>(
    () => [
      {
        accessorKey: "name",
        header: "Name",
        // Greedy width so Name fills the remaining space (network names are
        // short, unlike the long image/volume names that widen the other tabs).
        meta: { className: "w-full", headerClassName: "w-full" },
        cell: ({ row }) => (
          <span className="font-medium">{row.original.name}</span>
        ),
      },
      {
        accessorKey: "driver",
        header: "Driver",
        cell: ({ row }) => (
          <span className="text-sm">{row.original.driver}</span>
        ),
      },
      {
        accessorKey: "scope",
        header: "Scope",
        cell: ({ row }) => (
          <span className="text-muted-foreground text-sm">
            {row.original.scope}
          </span>
        ),
      },
      {
        id: "containers",
        header: "Attached",
        cell: ({ row }) => {
          const n = row.original.containers?.length ?? 0;
          if (n === 0) return <span className="text-text-faint">—</span>;
          return (
            <span className="text-sm">
              {n} container{n === 1 ? "" : "s"}
            </span>
          );
        },
      },
      {
        id: "owner",
        header: "Owner",
        cell: ({ row }) => (
          <div className="flex items-center gap-2">
            {row.original.internal && (
              <Badge variant="secondary" className="font-normal">
                Internal
              </Badge>
            )}
            <ManagedBadge managed={row.original.managed} />
          </div>
        ),
      },
      {
        accessorKey: "created_at",
        header: "Created",
        cell: ({ row }) =>
          row.original.created_at ? (
            <span className="text-muted-foreground text-sm">
              {formatDateTimeShort(row.original.created_at)}
            </span>
          ) : (
            <span className="text-text-faint">—</span>
          ),
      },
    ],
    [],
  );

  return (
    <Card>
      <CardContent className="space-y-3">
        <DataTableSearch
          value={search}
          onChange={setSearch}
          placeholder="Search by name or driver…"
          className="max-w-xs"
        />
        <DataTable
          columns={columns}
          data={data ?? []}
          isLoading={isPending}
          getRowId={(n) => n.id}
          enableSorting
          globalFilter={search}
          onGlobalFilterChange={setSearch}
          pagination={{ mode: "client", pageSize: 10 }}
          renderDetailPanel={({ row }) => (
            <NetworkContainers containers={row.original.containers} />
          )}
          emptyMessage={
            search.trim()
              ? "No networks match your search."
              : "No networks on this host."
          }
        />
      </CardContent>
    </Card>
  );
}

function NetworkContainers({
  containers,
}: {
  containers: DockerNetwork["containers"];
}) {
  const list = containers ?? [];
  if (list.length === 0) {
    return (
      <p className="text-muted-foreground py-1 text-sm">
        No containers attached to this network.
      </p>
    );
  }
  return (
    <div className="space-y-1.5 py-1">
      <p className="text-text-faint text-xs font-medium tracking-wider uppercase">
        Connected containers
      </p>
      {list.map((c) => (
        <div
          key={c.id}
          className="flex flex-wrap items-center gap-x-3 gap-y-0.5 text-sm"
        >
          <span className="font-medium">{c.name || "—"}</span>
          <span className="text-text-faint font-mono text-xs">
            {shortId(c.id)}
          </span>
          {c.ipv4_address && (
            <span className="text-muted-foreground font-mono text-xs">
              {c.ipv4_address}
            </span>
          )}
        </div>
      ))}
    </div>
  );
}
