import { Link, useRouterState } from "@tanstack/react-router";
import { cn } from "@/lib/utils";
import { useAuthStore } from "@/lib/stores/auth";

export function SettingsNav() {
  const routerState = useRouterState();
  const currentPath = routerState.location.pathname;
  const user = useAuthStore((s) => s.user);
  const isAdmin = user?.role === "admin";

  const tabs = [
    { to: "/settings", label: "General", exact: true },
    ...(isAdmin
      ? [
          { to: "/settings/server", label: "Server" },
          { to: "/settings/team", label: "Team" },
        ]
      : []),
  ];

  return (
    <nav className="flex gap-1 border-b">
      {tabs.map((tab) => {
        const isActive = tab.exact
          ? currentPath === tab.to
          : currentPath.startsWith(tab.to);
        return (
          <Link
            key={tab.to}
            to={tab.to as any}
            className={cn(
              "border-b-2 px-4 py-2 text-sm font-medium transition-colors",
              isActive
                ? "border-primary text-foreground"
                : "text-muted-foreground hover:text-foreground border-transparent",
            )}
          >
            {tab.label}
          </Link>
        );
      })}
    </nav>
  );
}
