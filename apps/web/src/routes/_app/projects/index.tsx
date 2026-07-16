import { useState } from "react";
import { createFileRoute, Link } from "@tanstack/react-router";
import {
  ArrowDownAZIcon,
  ClockIcon,
  FolderIcon,
  PlusIcon,
  RocketIcon,
  SearchIcon,
} from "lucide-react";
import type { ColumnDef, SortingState } from "@tanstack/react-table";
import { useProjects } from "@/lib/hooks/use-projects";
import { useApplications } from "@/lib/hooks/use-applications";
import { useDatabases } from "@/lib/hooks/use-databases";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { buttonVariants } from "@/components/ui/button";
import { PageHeader } from "@/components/ui/page-header";
import { Input } from "@/components/ui/input";
import { StatusPill } from "@/components/ui/status-pill";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { DataTable } from "@/components/ui/data-table";
import { formatDateTimeShort, formatRelativeTime } from "@/lib/utils/format";
import { cn } from "@/lib/utils";
import { OperatorHealthStrip } from "@/lib/components/stats/operator-health-strip";
import type { Project } from "@/lib/types";

export const Route = createFileRoute("/_app/projects/")({
  component: ProjectsPage,
});

type SortKey = "recent" | "name";

// Accessors only — the cards are rendered via DataTable's customView, so these
// columns exist purely to drive the global filter (name + slug) and sorting.
const projectColumns: ColumnDef<Project>[] = [
  {
    id: "name",
    header: "Name",
    accessorFn: (p) => `${p.name} ${p.slug}`,
  },
  {
    id: "created_at",
    header: "Created",
    enableGlobalFilter: false,
    accessorFn: (p) => new Date(p.created_at).getTime(),
  },
];

const SERVICE_ERROR_STATES = new Set([
  "failed",
  "error",
  "crashed",
  "unhealthy",
  "exited",
]);

/** Compact running/total badge for a project's services (apps + databases). */
function ProjectServicesBadge({ projectId }: { projectId: string }) {
  const { data: applications } = useApplications(projectId);
  const { data: databases } = useDatabases(projectId);
  if (!applications || !databases) {
    return <Skeleton className="h-5 w-16 rounded-full" />;
  }
  const services = [...applications, ...databases];
  const total = services.length;
  const running = services.filter((s) => s.status === "running").length;
  const errored = services.filter((s) =>
    SERVICE_ERROR_STATES.has(s.status.toLowerCase()),
  ).length;

  if (total === 0) {
    return (
      <span className="text-text-faint text-xs font-medium">No services</span>
    );
  }
  // All stopped is a deliberate state, not a failure — only go red when a
  // service is actually errored; otherwise stopped reads neutral.
  const status =
    running === total
      ? "running"
      : errored > 0
        ? "error"
        : running === 0
          ? "stopped"
          : "building";
  return (
    <StatusPill
      status={status}
      label={`${running}/${total} up`}
      className="font-mono"
    />
  );
}

function ProjectCard({ project }: { project: Project }) {
  return (
    <Link
      to="/projects/$projectId"
      params={{ projectId: project.id }}
      className="group focus-visible:ring-ring rounded-xl outline-none focus-visible:ring-2 focus-visible:ring-offset-2"
    >
      <Card className="hover:bg-card-hover hover:border-border-strong h-full transition-colors">
        <CardHeader className="pb-3">
          <div className="flex items-start justify-between gap-2">
            <div className="flex min-w-0 items-center gap-2.5">
              <div className="bg-elev text-text-muted grid size-9 shrink-0 place-items-center rounded-lg">
                <FolderIcon aria-hidden="true" className="size-4.5" />
              </div>
              <div className="flex min-w-0 flex-col">
                <CardTitle className="group-hover:text-primary truncate text-sm font-semibold transition-colors">
                  {project.name}
                </CardTitle>
                <span className="text-text-faint truncate font-mono text-xs">
                  {project.slug}
                </span>
              </div>
            </div>
            <ProjectServicesBadge projectId={project.id} />
          </div>
        </CardHeader>
        <CardContent className="flex items-center justify-between gap-2">
          <p className="text-text-faint text-xs">
            Created {formatDateTimeShort(project.created_at)}
          </p>
          {project.last_deployed_at && (
            <p className="text-text-faint flex items-center gap-1 text-xs">
              <RocketIcon aria-hidden="true" className="size-3" />
              Deployed {formatRelativeTime(project.last_deployed_at)}
            </p>
          )}
        </CardContent>
      </Card>
    </Link>
  );
}

/** Zero-state: welcome hero + how-it-works steps + first-project CTA. */
function ProjectsOnboarding() {
  const steps = [
    {
      title: "Create a project",
      body: "A project groups related applications and databases under one roof.",
    },
    {
      title: "Add an application",
      body: "Deploy from a Git repo or a Docker image. Builds and TLS are automatic.",
    },
    {
      title: "Ship & observe",
      body: "Watch deploys stream live, then track logs, metrics, and requests.",
    },
  ];
  return (
    <div className="mx-auto max-w-2xl py-10 text-center">
      <div
        aria-hidden="true"
        className="mx-auto grid size-14 place-items-center rounded-2xl text-white shadow-sm"
        style={{
          background:
            "linear-gradient(140deg, var(--brand), var(--brand-press))",
        }}
      >
        <RocketIcon className="size-7" />
      </div>
      <h1 className="mt-5 text-2xl font-semibold tracking-tight">
        Welcome to your homelab
      </h1>
      <p className="text-muted-foreground mx-auto mt-2 max-w-md text-sm">
        Self-host your apps and databases with automatic builds, TLS, and live
        observability. Start by creating your first project.
      </p>

      <div className="mt-8 grid gap-3 text-left sm:grid-cols-3">
        {steps.map((step, i) => (
          <Card key={step.title}>
            <CardContent className="pt-5">
              <div className="bg-brand-soft text-brand mb-3 grid size-7 place-items-center rounded-full font-mono text-xs font-semibold">
                {i + 1}
              </div>
              <p className="text-sm font-medium">{step.title}</p>
              <p className="text-muted-foreground mt-1 text-xs leading-relaxed">
                {step.body}
              </p>
            </CardContent>
          </Card>
        ))}
      </div>

      <Link
        to="/projects/new"
        className={cn(buttonVariants({ size: "lg" }), "mt-8")}
      >
        <PlusIcon aria-hidden="true" className="size-4" />
        Create your first project
      </Link>
    </div>
  );
}

function ProjectsGridSkeleton() {
  return (
    <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
      {[1, 2, 3, 4, 5, 6].map((i) => (
        <Card key={i}>
          <CardHeader className="pb-3">
            <div className="flex items-center gap-2.5">
              <Skeleton className="size-9 rounded-lg" />
              <div className="flex flex-col gap-1.5">
                <Skeleton className="h-4 w-28" />
                <Skeleton className="h-3 w-20" />
              </div>
            </div>
          </CardHeader>
          <CardContent>
            <Skeleton className="h-3 w-24" />
          </CardContent>
        </Card>
      ))}
    </div>
  );
}

function ProjectsPage() {
  const { data: projects, isLoading } = useProjects();
  const [query, setQuery] = useState("");
  const [sort, setSort] = useState<SortKey>("recent");

  const sorting: SortingState =
    sort === "name"
      ? [{ id: "name", desc: false }]
      : [{ id: "created_at", desc: true }];

  if (isLoading) {
    return (
      <div className="space-y-6">
        <Skeleton className="h-8 w-40" />
        <ProjectsGridSkeleton />
      </div>
    );
  }

  if (!projects || projects.length === 0) {
    return <ProjectsOnboarding />;
  }

  return (
    <div className="space-y-6">
      <PageHeader
        icon={<FolderIcon className="size-5" />}
        title="Projects"
        description={`${projects.length} ${
          projects.length === 1 ? "project" : "projects"
        }`}
        actions={
          <Link to="/projects/new" className={buttonVariants()}>
            <PlusIcon aria-hidden="true" className="size-4" />
            New Project
          </Link>
        }
      />

      <OperatorHealthStrip />

      <div className="flex flex-wrap items-center gap-2">
        <div className="relative min-w-0 flex-1 sm:max-w-xs">
          <SearchIcon
            aria-hidden="true"
            className="text-text-faint pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2"
          />
          <Input
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder="Search projects…"
            aria-label="Search projects"
            className="pl-9"
          />
        </div>
        <Select value={sort} onValueChange={(v) => setSort(v as SortKey)}>
          <SelectTrigger className="w-52 capitalize" aria-label="Sort projects">
            <span className="text-text-faint mr-1">Sort:</span>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="recent" icon={<ClockIcon />} className="capitalize">
              Most recent
            </SelectItem>
            <SelectItem value="name" icon={<ArrowDownAZIcon />} className="capitalize">
              Name
            </SelectItem>
          </SelectContent>
        </Select>
      </div>

      <DataTable
        columns={projectColumns}
        data={projects}
        getRowId={(p) => p.id}
        enableSorting
        sorting={sorting}
        globalFilter={query}
        onGlobalFilterChange={setQuery}
        customView={{
          item: ({ row }) => <ProjectCard project={row.original} />,
          wrapperProps: {
            className: "grid gap-4 sm:grid-cols-2 lg:grid-cols-3",
          },
          empty: (
            <div className="text-muted-foreground rounded-xl border border-dashed py-16 text-center text-sm">
              No projects match{" "}
              <span className="text-foreground font-mono">{query}</span>.
            </div>
          ),
        }}
      />
    </div>
  );
}
