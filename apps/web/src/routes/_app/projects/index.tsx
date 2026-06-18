import { useMemo, useState } from "react";
import { createFileRoute, Link } from "@tanstack/react-router";
import { FolderIcon, PlusIcon, RocketIcon, SearchIcon } from "lucide-react";
import { useProjects } from "@/lib/hooks/use-projects";
import { useApplications } from "@/lib/hooks/use-applications";
import { useDatabases } from "@/lib/hooks/use-databases";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { buttonVariants } from "@/components/ui/button";
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
import { formatDate } from "@/lib/utils/format";
import { cn } from "@/lib/utils";
import type { Project } from "@/lib/types";

export const Route = createFileRoute("/_app/projects/")({
  component: ProjectsPage,
});

type SortKey = "recent" | "name";

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

  if (total === 0) {
    return (
      <span className="text-text-faint text-xs font-medium">No services</span>
    );
  }
  const status = running === total ? "running" : running === 0 ? "error" : "building";
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
        <CardContent>
          <p className="text-text-faint text-xs">
            Created {formatDate(project.created_at)}
          </p>
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
          background: "linear-gradient(140deg, var(--brand), var(--brand-press))",
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

  const filtered = useMemo(() => {
    if (!projects) return [];
    const q = query.trim().toLowerCase();
    const matches = q
      ? projects.filter(
          (p) =>
            p.name.toLowerCase().includes(q) || p.slug.toLowerCase().includes(q),
        )
      : projects.slice();
    matches.sort((a, b) =>
      sort === "name"
        ? a.name.localeCompare(b.name)
        : new Date(b.created_at).getTime() - new Date(a.created_at).getTime(),
    );
    return matches;
  }, [projects, query, sort]);

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
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">Projects</h1>
          <p className="text-muted-foreground text-sm">
            {projects.length} {projects.length === 1 ? "project" : "projects"}
          </p>
        </div>
        <Link to="/projects/new" className={buttonVariants()}>
          <PlusIcon aria-hidden="true" className="size-4" />
          New Project
        </Link>
      </div>

      <div className="flex flex-wrap items-center gap-2">
        <div className="relative min-w-0 flex-1 sm:max-w-xs">
          <SearchIcon
            aria-hidden="true"
            className="text-text-faint pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2"
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
          <SelectTrigger className="w-36" aria-label="Sort projects">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="recent">Most recent</SelectItem>
            <SelectItem value="name">Name</SelectItem>
          </SelectContent>
        </Select>
      </div>

      {filtered.length === 0 ? (
        <div className="text-muted-foreground rounded-xl border border-dashed py-16 text-center text-sm">
          No projects match{" "}
          <span className="text-foreground font-mono">{query}</span>.
        </div>
      ) : (
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {filtered.map((project) => (
            <ProjectCard key={project.id} project={project} />
          ))}
        </div>
      )}
    </div>
  );
}
