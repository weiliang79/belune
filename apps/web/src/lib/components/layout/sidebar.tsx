import { Link, useRouterState } from "@tanstack/react-router";
import { useState } from "react";
import { useAuthStore } from "@/lib/stores/auth";
import { useSidebarStore } from "@/lib/stores/sidebar";
import { useIsMobile } from "@/lib/hooks/use-is-mobile";
import { logout } from "@/lib/api/auth";
import { Button } from "@/components/ui/button";
import { Loader2, X } from "lucide-react";
import { cn } from "@/lib/utils";
import { ThemeToggle } from "./theme-toggle";

interface SidebarProps {
  /** Whether the off-canvas drawer is open (mobile only). */
  mobileOpen: boolean;
  /** Close the off-canvas drawer (mobile only). */
  onMobileClose: () => void;
}

export function Sidebar({ mobileOpen, onMobileClose }: SidebarProps) {
  const { isOpen, toggle } = useSidebarStore();
  const { user, clearUser } = useAuthStore();
  const isMobile = useIsMobile();
  const routerState = useRouterState();
  const currentPath = routerState.location.pathname;
  const isAdmin = user?.role === "admin";
  const [isLoggingOut, setIsLoggingOut] = useState(false);

  // On mobile the drawer is always full-width with labels; on desktop the
  // persisted `isOpen` controls the collapsed icon rail.
  const expanded = isMobile ? true : isOpen;

  const handleLogout = async () => {
    setIsLoggingOut(true);
    try {
      await logout();
      clearUser();
      window.location.href = "/login";
    } finally {
      setIsLoggingOut(false);
    }
  };

  const isActive = (to: string, exact = false) =>
    exact ? currentPath === to : currentPath.startsWith(to);

  const navLink = (to: string, label: string, exact = false) => (
    <Link
      key={to}
      to={to as any}
      onClick={() => isMobile && onMobileClose()}
      aria-label={!expanded ? label : undefined}
      className={cn(
        "hover:bg-sidebar-accent hover:text-sidebar-accent-foreground flex items-center rounded-md px-3 py-2 text-sm font-medium transition-colors",
        isActive(to, exact) && "bg-sidebar-accent text-sidebar-accent-foreground",
      )}
    >
      {expanded ? label : <span className="sr-only">{label}</span>}
    </Link>
  );

  return (
    <aside
      className={cn(
        "bg-sidebar text-sidebar-foreground flex h-screen flex-col border-r",
        // Mobile: fixed off-canvas drawer
        "fixed inset-y-0 left-0 z-50 w-64 -translate-x-full transition-transform",
        mobileOpen && "translate-x-0",
        // Desktop: static, width controlled by the persisted collapse state
        "md:static md:z-auto md:translate-x-0 md:transition-[width]",
        isOpen ? "md:w-64" : "md:w-16",
      )}
    >
      <div className="flex h-14 items-center justify-between border-b px-4">
        <button
          onClick={toggle}
          aria-label={isOpen ? "Collapse sidebar" : "Expand sidebar"}
          aria-expanded={isOpen}
          className="hidden text-lg font-bold hover:opacity-80 md:block"
        >
          {isOpen ? "PaaS" : "P"}
        </button>
        <span className="text-lg font-bold md:hidden">PaaS</span>
        <button
          onClick={onMobileClose}
          aria-label="Close navigation"
          className="hover:opacity-80 md:hidden"
        >
          <X aria-hidden="true" className="h-5 w-5" />
        </button>
      </div>

      <nav className="flex-1 space-y-4 overflow-y-auto p-2">
        {/* Home section */}
        <div>
          {expanded && (
            <p className="text-muted-foreground mb-1 px-3 text-xs font-semibold uppercase tracking-wider">
              Home
            </p>
          )}
          <div className="space-y-1">
            {navLink("/projects", "Projects")}
            {navLink("/deployments", "Deployments")}
            {isAdmin && navLink("/requests", "Requests")}
          </div>
        </div>

        {/* Settings section */}
        <div>
          {expanded && (
            <p className="text-muted-foreground mb-1 px-3 text-xs font-semibold uppercase tracking-wider">
              Settings
            </p>
          )}
          <div className="space-y-1">
            {navLink("/account", "Account", true)}
            {isAdmin && navLink("/server", "Server")}
            {isAdmin && navLink("/team", "Team")}
            {isAdmin && navLink("/quotas", "Quotas")}
            {isAdmin && navLink("/backups", "Backups")}
            {navLink("/git-credentials", "Git Credentials")}
            {isAdmin && navLink("/audit", "Audit Log")}
          </div>
        </div>
      </nav>

      <div className="border-t p-3">
        {expanded && user && (
          <div className="text-muted-foreground mb-2 truncate text-xs">
            {user.username || user.email}
          </div>
        )}
        <ThemeToggle expanded={expanded} />
        <Button
          variant="ghost"
          size="sm"
          className="w-full justify-start"
          onClick={handleLogout}
          disabled={isLoggingOut}
          aria-label={!expanded ? "Logout" : undefined}
        >
          {isLoggingOut ? (
            <Loader2 aria-hidden="true" className="h-4 w-4 animate-spin" />
          ) : expanded ? (
            "Logout"
          ) : (
            <span aria-hidden="true">{"→"}</span>
          )}
        </Button>
      </div>
    </aside>
  );
}
