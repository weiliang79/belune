import { Menu, PanelLeft } from "lucide-react";
import { useRouterState } from "@tanstack/react-router";
import { Button } from "@/components/ui/button";
import { useSidebarStore } from "@/lib/stores/sidebar";
import { useNotificationStream } from "@/lib/hooks/use-notifications";
import { useBreadcrumbStore } from "@/lib/stores/breadcrumb";
import {
  AppBreadcrumb,
  type BreadcrumbSegment,
} from "@/lib/components/app-breadcrumb";
import { ThemeToggle } from "./theme-toggle";
import { NotificationBell } from "./notification-bell";

// Friendly labels for top-level sections; mirrors the sidebar nav.
const SECTION_LABELS: Record<string, string> = {
  projects: "Projects",
  deployments: "Deployments",
  requests: "Requests",
  account: "Account",
  server: "Server",
  team: "Team",
  quotas: "Quotas",
  backups: "Backups",
  "git-providers": "Git Providers",
  audit: "Audit Log",
};

/**
 * Build the breadcrumb structurally from the current pathname, resolving dynamic
 * project/app/database names from the label store. Deriving from the live route
 * (rather than per-page publishes) keeps the trail correct on every navigation,
 * including the browser back button.
 */
function buildCrumbs(
  pathname: string,
  labels: Record<string, string>,
): BreadcrumbSegment[] {
  const segs = pathname.split("/").filter(Boolean);
  if (segs.length === 0) return [];

  if (segs[0] !== "projects") {
    const label =
      SECTION_LABELS[segs[0]] ??
      segs[0].charAt(0).toUpperCase() + segs[0].slice(1);
    return [{ label }];
  }

  const crumbs: BreadcrumbSegment[] = [{ label: "Projects", to: "/projects" }];
  if (segs.length === 1) return crumbs;

  if (segs[1] === "new") {
    crumbs.push({ label: "New Project" });
    return crumbs;
  }

  const projectId = segs[1];
  crumbs.push({
    label: labels[projectId] ?? "Project",
    to: `/projects/${projectId}`,
  });

  if (segs[2] === "applications" && segs[3]) {
    crumbs.push({ label: labels[segs[3]] ?? "Application" });
  } else if (segs[2] === "databases" && segs[3]) {
    crumbs.push({ label: labels[segs[3]] ?? "Database" });
  }

  return crumbs;
}

function TopbarBreadcrumb() {
  const pathname = useRouterState({ select: (s) => s.location.pathname });
  const labels = useBreadcrumbStore((s) => s.labels);

  const crumbs = buildCrumbs(pathname, labels);
  if (crumbs.length === 0) return null;
  return <AppBreadcrumb items={crumbs} />;
}

export function Topbar({ onMobileMenu }: { onMobileMenu: () => void }) {
  const toggleSidebar = useSidebarStore((s) => s.toggle);

  // Mounted once globally — keeps the bell badge and toasts live.
  useNotificationStream();

  return (
    <header className="bg-background flex h-14 shrink-0 items-center gap-2 border-b px-3 md:px-4">
      {/* Mobile: open drawer · Desktop: collapse rail */}
      <Button
        variant="ghost"
        size="icon"
        className="md:hidden"
        onClick={onMobileMenu}
        aria-label="Open navigation"
      >
        <Menu aria-hidden="true" className="h-5 w-5" />
      </Button>
      <Button
        variant="ghost"
        size="icon"
        className="hidden md:inline-flex"
        onClick={toggleSidebar}
        aria-label="Toggle sidebar"
      >
        <PanelLeft aria-hidden="true" className="h-4 w-4" />
      </Button>

      <TopbarBreadcrumb />

      <div className="flex-1" />

      <div className="flex items-center gap-0.5">
        <NotificationBell />
        <ThemeToggle />
      </div>
    </header>
  );
}
