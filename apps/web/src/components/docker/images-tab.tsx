import { useMemo, useState } from "react";
import type { ColumnDef } from "@tanstack/react-table";
import { Link } from "@tanstack/react-router";
import { DataTable, DataTableSearch } from "@/components/ui/data-table";
import { Badge } from "@/components/ui/badge";
import { useDockerImages } from "@/lib/hooks/use-docker";
import { formatDateTimeShort } from "@/lib/utils/format";
import type { DockerImage } from "@/lib/types";
import { ManagedBadge } from "./shared";
import { shortId, sizeLabel } from "./utils";

export function DockerImagesTab({ enabled }: { enabled: boolean }) {
  const { data, isPending } = useDockerImages(enabled);
  const [search, setSearch] = useState("");

  const columns = useMemo<ColumnDef<DockerImage>[]>(
    () => [
      {
        id: "tags",
        accessorFn: (row) => `${(row.repo_tags ?? []).join(" ")} ${row.id}`,
        header: "Repository tags",
        cell: ({ row }) => {
          const tags = row.original.repo_tags ?? [];
          return (
            <div className="flex flex-col gap-0.5">
              {tags.length > 0 ? (
                tags.map((t) => (
                  <span key={t} className="font-mono text-xs break-all">
                    {t}
                  </span>
                ))
              ) : (
                <span className="text-text-faint text-xs">&lt;untagged&gt;</span>
              )}
              <span className="text-text-faint font-mono text-xs">
                {shortId(row.original.id)}
              </span>
            </div>
          );
        },
      },
      {
        accessorKey: "size",
        header: "Size",
        cell: ({ row }) => (
          <span className="tabular-nums">{sizeLabel(row.original.size)}</span>
        ),
      },
      {
        id: "owner",
        header: "Owner",
        cell: ({ row }) => {
          const { owner, managed } = row.original;
          if (owner) {
            return (
              <Link
                to="/projects/$projectId/applications/$applicationId"
                params={{ projectId: owner.project_id, applicationId: owner.id }}
                className="text-primary text-sm hover:underline"
              >
                {owner.name}
              </Link>
            );
          }
          return <ManagedBadge managed={managed} />;
        },
      },
      {
        id: "flags",
        header: "",
        cell: ({ row }) =>
          row.original.dangling ? (
            <Badge variant="secondary" className="font-normal">
              Dangling
            </Badge>
          ) : null,
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
    <div className="space-y-3">
      <DataTableSearch
        value={search}
        onChange={setSearch}
        placeholder="Search by tag…"
        className="max-w-xs"
      />
      <DataTable
        columns={columns}
        data={data ?? []}
        isLoading={isPending}
        getRowId={(img) => img.id}
        enableSorting
        globalFilter={search}
        onGlobalFilterChange={setSearch}
        pagination={{ mode: "client", pageSize: 10 }}
        emptyMessage={
          search.trim() ? "No images match your search." : "No images on this host."
        }
      />
    </div>
  );
}
