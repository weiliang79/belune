import { Link, useRouterState } from "@tanstack/react-router";
import { useState } from "react";
import { useAuthStore } from "@/lib/stores/auth";
import { useSidebarStore } from "@/lib/stores/sidebar";
import { logout } from "@/lib/api/auth";
import { Button } from "@/components/ui/button";
import { Loader2 } from "lucide-react";
import { cn } from "@/lib/utils";

export function Sidebar() {
  const { isOpen, toggle } = useSidebarStore();
  const { user, clearUser } = useAuthStore();
  const routerState = useRouterState();
  const currentPath = routerState.location.pathname;
  const isAdmin = user?.role === "admin";
  const [isLoggingOut, setIsLoggingOut] = useState(false);

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
      className={cn(
        "hover:bg-sidebar-accent hover:text-sidebar-accent-foreground flex items-center rounded-md px-3 py-2 text-sm font-medium transition-colors",
        isActive(to, exact) && "bg-sidebar-accent text-sidebar-accent-foreground",
      )}
    >
      {isOpen && label}
    </Link>
  );

  return (
    <aside
      className={cn(
        "bg-sidebar text-sidebar-foreground flex h-screen flex-col border-r transition-all",
        isOpen ? "w-64" : "w-16",
      )}
    >
      <div className="flex h-14 items-center border-b px-4">
        <button onClick={toggle} className="text-lg font-bold hover:opacity-80">
          {isOpen ? "PaaS" : "P"}
        </button>
      </div>

      <nav className="flex-1 space-y-4 overflow-y-auto p-2">
        {/* Home section */}
        <div>
          {isOpen && (
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
          {isOpen && (
            <p className="text-muted-foreground mb-1 px-3 text-xs font-semibold uppercase tracking-wider">
              Settings
            </p>
          )}
          <div className="space-y-1">
            {navLink("/account", "Account", true)}
            {isAdmin && navLink("/server", "Server")}
            {isAdmin && navLink("/team", "Team")}
            {navLink("/git-credentials", "Git Credentials")}
            {isAdmin && navLink("/audit", "Audit Log")}
          </div>
        </div>
      </nav>

      <div className="border-t p-3">
        {isOpen && user && (
          <div className="text-muted-foreground mb-2 truncate text-xs">
            {user.username || user.email}
          </div>
        )}
        <Button
          variant="ghost"
          size="sm"
          className="w-full justify-start"
          onClick={handleLogout}
          disabled={isLoggingOut}
        >
          {isLoggingOut ? (
            <Loader2 className="h-4 w-4 animate-spin" />
          ) : (
            isOpen ? "Logout" : "->"
          )}
        </Button>
      </div>
    </aside>
  );
}
