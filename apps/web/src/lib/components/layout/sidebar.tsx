import { Link, useRouterState } from "@tanstack/react-router";
import { useState, type ComponentType } from "react";
import {
  Folder,
  Rocket,
  Activity,
  User,
  Server,
  Users,
  Gauge,
  Database,
  ShieldCheck,
  LogOut,
  Loader2,
  X,
} from "lucide-react";
import { useAuthStore } from "@/lib/stores/auth";
import { useSidebarStore } from "@/lib/stores/sidebar";
import { useIsMobile } from "@/lib/hooks/use-is-mobile";
import { logout } from "@/lib/api/auth";
import { BRAND } from "@/lib/brand";
import { initialsOf } from "@/lib/utils/initials";
import { cn } from "@/lib/utils";

interface NavItem {
  to: string;
  label: string;
  Icon: ComponentType<{ className?: string; "aria-hidden"?: boolean }>;
  admin?: boolean;
  exact?: boolean;
}

const HOME_NAV: NavItem[] = [
  { to: "/projects", label: "Projects", Icon: Folder },
  { to: "/deployments", label: "Deployments", Icon: Rocket },
  { to: "/requests", label: "Requests", Icon: Activity, admin: true },
];

const SETTINGS_NAV: NavItem[] = [
  { to: "/account", label: "Account", Icon: User, exact: true },
  { to: "/server", label: "Server", Icon: Server, admin: true },
  { to: "/team", label: "Team", Icon: Users, admin: true },
  { to: "/quotas", label: "Quotas", Icon: Gauge, admin: true },
  { to: "/backups", label: "Backups", Icon: Database, admin: true },
  { to: "/audit", label: "Audit Log", Icon: ShieldCheck, admin: true },
];

interface SidebarProps {
  /** Whether the off-canvas drawer is open (mobile only). */
  mobileOpen: boolean;
  /** Close the off-canvas drawer (mobile only). */
  onMobileClose: () => void;
}

export function Sidebar({ mobileOpen, onMobileClose }: SidebarProps) {
  const { isOpen } = useSidebarStore();
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

  const navLink = ({ to, label, Icon, exact }: NavItem) => {
    const active = isActive(to, exact);
    return (
      <Link
        key={to}
        to={to as never}
        onClick={() => isMobile && onMobileClose()}
        aria-label={!expanded ? label : undefined}
        aria-current={active ? "page" : undefined}
        title={!expanded ? label : undefined}
        className={cn(
          "group flex items-center gap-3 rounded-md px-3 py-2 text-sm font-medium transition-colors",
          !expanded && "justify-center px-0",
          active
            ? "bg-sidebar-accent text-sidebar-accent-foreground"
            : "text-muted-foreground hover:bg-sidebar-accent/60 hover:text-foreground",
        )}
      >
        <Icon
          aria-hidden={true}
          className={cn(
            "h-[18px] w-[18px] shrink-0 transition-colors",
            active ? "text-primary" : "text-muted-foreground group-hover:text-foreground",
          )}
        />
        {expanded ? label : <span className="sr-only">{label}</span>}
      </Link>
    );
  };

  const section = (heading: string, items: NavItem[]) => {
    const visible = items.filter((i) => !i.admin || isAdmin);
    if (visible.length === 0) return null;
    return (
      <div>
        {expanded && (
          <p className="text-text-faint mb-1 px-3 text-[10.5px] font-semibold uppercase tracking-wider">
            {heading}
          </p>
        )}
        <div className="space-y-0.5">{visible.map(navLink)}</div>
      </div>
    );
  };

  const identity = user?.username || user?.email || "User";

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
      {/* Branding / identity block — static (single-tenant, no org switcher) */}
      <div
        className={cn(
          "flex h-14 items-center gap-2.5 border-b px-4",
          !expanded && "justify-center px-0",
        )}
      >
        <div
          aria-hidden="true"
          className="grid size-8 shrink-0 place-items-center rounded-lg text-white shadow-sm"
          style={{
            background: "linear-gradient(140deg, var(--brand), var(--brand-press))",
          }}
        >
          <Rocket className="h-[18px] w-[18px]" />
        </div>
        {expanded && (
          <div className="flex min-w-0 flex-col leading-tight">
            <span className="truncate text-sm font-semibold">{BRAND.name}</span>
            <span className="text-text-faint font-mono text-[11px]">
              {BRAND.version}
            </span>
          </div>
        )}
        <button
          onClick={onMobileClose}
          aria-label="Close navigation"
          className="ml-auto hover:opacity-80 md:hidden"
        >
          <X aria-hidden="true" className="h-5 w-5" />
        </button>
      </div>

      <nav className="flex-1 space-y-4 overflow-y-auto p-2">
        {section("Home", HOME_NAV)}
        {section("Settings", SETTINGS_NAV)}
      </nav>

      {/* Footer — user identity + logout (theme/accent live in the top bar) */}
      <div className="border-t p-2">
        <div
          className={cn(
            "flex items-center gap-2.5 rounded-md px-2 py-1.5",
            !expanded && "justify-center px-0",
          )}
        >
          <div
            aria-hidden="true"
            className="bg-elev text-foreground grid size-8 shrink-0 place-items-center rounded-full text-xs font-semibold"
          >
            {initialsOf(identity)}
          </div>
          {expanded && (
            <div className="flex min-w-0 flex-col leading-tight">
              <span className="truncate text-sm font-medium">{identity}</span>
              {user?.email && (
                <span className="text-text-faint truncate text-xs">
                  {user.email}
                </span>
              )}
            </div>
          )}
          {expanded && (
            <button
              onClick={handleLogout}
              disabled={isLoggingOut}
              aria-label="Log out"
              title="Log out"
              className="text-muted-foreground hover:text-foreground ml-auto shrink-0 rounded-md p-1.5 transition-colors"
            >
              {isLoggingOut ? (
                <Loader2 aria-hidden="true" className="h-4 w-4 animate-spin" />
              ) : (
                <LogOut aria-hidden="true" className="h-4 w-4" />
              )}
            </button>
          )}
        </div>
        {!expanded && (
          <button
            onClick={handleLogout}
            disabled={isLoggingOut}
            aria-label="Log out"
            title="Log out"
            className="text-muted-foreground hover:text-foreground mt-1 flex w-full justify-center rounded-md p-1.5 transition-colors"
          >
            {isLoggingOut ? (
              <Loader2 aria-hidden="true" className="h-4 w-4 animate-spin" />
            ) : (
              <LogOut aria-hidden="true" className="h-4 w-4" />
            )}
          </button>
        )}
      </div>
    </aside>
  );
}
