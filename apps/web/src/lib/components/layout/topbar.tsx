import { Link } from "@tanstack/react-router";
import { Menu, PanelLeft, Bell } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { useSidebarStore } from "@/lib/stores/sidebar";
import { useAuthStore } from "@/lib/stores/auth";
import { ThemeToggle } from "./theme-toggle";
import { AccentToggle } from "./accent-toggle";

function initialsOf(name: string): string {
  const parts = name.trim().split(/[\s@._-]+/).filter(Boolean);
  if (parts.length === 0) return "?";
  if (parts.length === 1) return parts[0].slice(0, 2).toUpperCase();
  return (parts[0][0] + parts[1][0]).toUpperCase();
}

/**
 * Placeholder notification bell. The notification backend lands in a later
 * phase; for now this shows the calm empty state (no fake data, no badge).
 */
function NotificationBell() {
  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        render={<Button variant="ghost" size="icon" aria-label="Notifications" />}
      >
        <Bell aria-hidden="true" className="h-4 w-4" />
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-72">
        <div className="text-muted-foreground px-3 py-6 text-center text-sm">
          You&apos;re all caught up.
        </div>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

export function Topbar({ onMobileMenu }: { onMobileMenu: () => void }) {
  const toggleSidebar = useSidebarStore((s) => s.toggle);
  const user = useAuthStore((s) => s.user);
  const identity = user?.username || user?.email || "User";

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

      <div className="flex-1" />

      <div className="flex items-center gap-0.5">
        <NotificationBell />
        <ThemeToggle />
        <AccentToggle />
        <Link
          to="/account"
          aria-label="Account"
          title={identity}
          className="ml-1 grid size-8 place-items-center rounded-full text-xs font-semibold text-white"
          style={{
            background: "linear-gradient(140deg, var(--brand), var(--brand-press))",
          }}
        >
          {initialsOf(identity)}
        </Link>
      </div>
    </header>
  );
}
