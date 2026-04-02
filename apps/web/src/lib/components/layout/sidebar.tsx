import { Link, useRouterState } from "@tanstack/react-router";
import { useAuthStore } from "@/lib/stores/auth";
import { useSidebarStore } from "@/lib/stores/sidebar";
import { logout } from "@/lib/api/auth";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

const navItems = [
  { to: "/dashboard" as const, label: "Dashboard", icon: "/" },
  { to: "/projects" as const, label: "Projects", icon: "/" },
  { to: "/settings" as const, label: "Settings", icon: "/" },
];

export function Sidebar() {
  const { isOpen, toggle } = useSidebarStore();
  const { user, clearUser } = useAuthStore();
  const routerState = useRouterState();
  const currentPath = routerState.location.pathname;

  const handleLogout = async () => {
    await logout();
    clearUser();
    window.location.href = "/login";
  };

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

      <nav className="flex-1 space-y-1 p-2">
        {navItems.map((item) => (
          <Link
            key={item.to}
            to={item.to}
            className={cn(
              "hover:bg-sidebar-accent hover:text-sidebar-accent-foreground flex items-center rounded-md px-3 py-2 text-sm font-medium transition-colors",
              currentPath.startsWith(item.to) &&
                "bg-sidebar-accent text-sidebar-accent-foreground",
            )}
          >
            {isOpen && item.label}
          </Link>
        ))}
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
        >
          {isOpen ? "Logout" : "->"}
        </Button>
      </div>
    </aside>
  );
}
