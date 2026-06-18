import { Link } from "@tanstack/react-router";
import { Menu, PanelLeft } from "lucide-react";
import { Button } from "@/components/ui/button";
import { useSidebarStore } from "@/lib/stores/sidebar";
import { useAuthStore } from "@/lib/stores/auth";
import { useNotificationStream } from "@/lib/hooks/use-notifications";
import { initialsOf } from "@/lib/utils/initials";
import { ThemeToggle } from "./theme-toggle";
import { AccentToggle } from "./accent-toggle";
import { NotificationBell } from "./notification-bell";

export function Topbar({ onMobileMenu }: { onMobileMenu: () => void }) {
  const toggleSidebar = useSidebarStore((s) => s.toggle);
  const user = useAuthStore((s) => s.user);
  const identity = user?.username || user?.email || "User";

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
            background:
              "linear-gradient(140deg, var(--brand), var(--brand-press))",
          }}
        >
          {initialsOf(identity)}
        </Link>
      </div>
    </header>
  );
}
