import { useState } from "react";
import { useNavigate } from "@tanstack/react-router";
import { Bell, CheckCheck } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import {
  useNotifications,
  useUnreadCount,
  useMarkNotificationRead,
  useMarkAllNotificationsRead,
} from "@/lib/hooks/use-notifications";
import { formatDateTimeShort } from "@/lib/utils/format";
import { cn } from "@/lib/utils";
import type { Notification } from "@/lib/types";

function NotificationRow({
  notification,
  onSelect,
}: {
  notification: Notification;
  onSelect: (n: Notification) => void;
}) {
  return (
    <button
      type="button"
      onClick={() => onSelect(notification)}
      className="hover:bg-card-hover flex w-full gap-2.5 px-3 py-2.5 text-left transition-colors"
    >
      <span className="flex w-2 shrink-0 justify-center pt-1.5">
        {!notification.read && (
          <span
            aria-hidden="true"
            className="bg-brand size-2 rounded-full"
            title="Unread"
          />
        )}
      </span>
      <span className="min-w-0 flex-1">
        <span className="flex items-baseline justify-between gap-2">
          <span
            className={cn(
              "truncate text-sm",
              notification.read ? "text-foreground" : "font-medium",
            )}
          >
            {notification.title}
          </span>
          <span className="text-text-faint shrink-0 font-mono text-[11px]">
            {formatDateTimeShort(notification.created_at)}
          </span>
        </span>
        {notification.body && (
          <span className="text-muted-foreground mt-0.5 line-clamp-2 block text-xs">
            {notification.body}
          </span>
        )}
      </span>
    </button>
  );
}

export function NotificationBell() {
  const [open, setOpen] = useState(false);
  const navigate = useNavigate();
  const { data: unreadData } = useUnreadCount();
  const { data, isLoading } = useNotifications();
  const markRead = useMarkNotificationRead();
  const markAll = useMarkAllNotificationsRead();

  const unread = unreadData?.unread ?? 0;
  const items = data?.items ?? [];

  const handleSelect = (n: Notification) => {
    if (!n.read) markRead.mutate(n.id);
    setOpen(false);
    // `link` is a backend-resolved internal pathname (e.g.
    // "/projects/<id>/applications/<id>/deployments"). TanStack Router matches
    // a concrete pathname against the $param route tree (verified), so it can
    // be passed straight to navigate. Guard against empty/external values.
    if (n.link?.startsWith("/")) {
      navigate({ to: n.link });
    }
  };

  return (
    <DropdownMenu open={open} onOpenChange={setOpen}>
      <DropdownMenuTrigger
        render={
          <Button
            variant="ghost"
            size="icon"
            aria-label={
              unread > 0 ? `Notifications (${unread} unread)` : "Notifications"
            }
            className="relative"
          />
        }
      >
        <Bell aria-hidden="true" className="h-4 w-4" />
        {unread > 0 && (
          <span className="bg-brand text-brand-fg absolute -top-0.5 -right-0.5 grid min-w-4 place-items-center rounded-full px-1 text-[10px] leading-4 font-semibold">
            {unread > 9 ? "9+" : unread}
          </span>
        )}
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-80 p-0">
        <div className="flex items-center justify-between border-b px-3 py-2">
          <span className="text-sm font-semibold">Notifications</span>
          <Button
            variant="ghost"
            size="sm"
            className="h-7 gap-1.5 text-xs"
            disabled={unread === 0 || markAll.isPending}
            onClick={() => markAll.mutate()}
          >
            <CheckCheck aria-hidden="true" className="size-3.5" />
            Mark all read
          </Button>
        </div>

        <div className="max-h-96 divide-y overflow-y-auto">
          {isLoading ? (
            <p className="text-muted-foreground px-3 py-6 text-center text-sm">
              Loading…
            </p>
          ) : items.length === 0 ? (
            <p className="text-muted-foreground px-3 py-8 text-center text-sm">
              You&apos;re all caught up.
            </p>
          ) : (
            items.map((n) => (
              <NotificationRow
                key={n.id}
                notification={n}
                onSelect={handleSelect}
              />
            ))
          )}
        </div>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
