import { Link } from "@tanstack/react-router";
import { DatabaseIcon, PlusIcon, SettingsIcon } from "lucide-react";
import { Button, buttonVariants } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import type { Project } from "@/lib/types";

interface Props {
  project?: Project;
  onAddApplication: () => void;
  onAddDatabase: () => void;
}

export function ProjectHeader({ project, onAddApplication, onAddDatabase }: Props) {
  return (
    <div className="flex flex-wrap items-start justify-between gap-3">
      <div className="min-w-0">
        <h1 className="truncate text-2xl font-semibold tracking-tight">
          {project?.name ?? "Project"}
        </h1>
        {project && (
          <p className="text-text-faint mt-0.5 truncate font-mono text-sm">
            {project.slug}
          </p>
        )}
      </div>
      <div className="flex items-center gap-2">
        {project && (
          <Link
            to="/projects/$projectId/settings"
            params={{ projectId: project.id }}
            className={cn(buttonVariants({ variant: "outline", size: "icon" }))}
            aria-label="Project settings"
            title="Project settings"
          >
            <SettingsIcon aria-hidden="true" className="size-4" />
          </Link>
        )}
        <Button variant="outline" onClick={onAddDatabase}>
          <DatabaseIcon aria-hidden="true" className="size-4" />
          New Database
        </Button>
        <Button onClick={onAddApplication}>
          <PlusIcon aria-hidden="true" className="size-4" />
          New Application
        </Button>
      </div>
    </div>
  );
}
