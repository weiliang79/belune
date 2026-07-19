import { useMemo, useState } from "react";
import { createFileRoute } from "@tanstack/react-router";
import { toast } from "sonner";
import type { ColumnDef } from "@tanstack/react-table";
import { BellRing, Pencil, Trash2, SendHorizontal } from "lucide-react";
import {
  TYPE_ICON,
  TYPE_LABEL,
} from "@/components/notifications/channel-types";
import { useAuthStore } from "@/lib/stores/auth";
import { RouteError } from "@/lib/components/route-error";
import {
  useNotificationChannels,
  useSetNotificationChannelEnabled,
  useDeleteNotificationChannel,
  useTestNotificationChannel,
} from "@/lib/hooks/use-notification-channels";
import type { NotificationChannel } from "@/lib/api/notification-channels";
import { ChannelFormDialog } from "@/components/notifications/channel-form-dialog";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { PageHeader } from "@/components/ui/page-header";
import { Switch } from "@/components/ui/switch";
import { DataTable, buildActionColumnDef } from "@/components/ui/data-table";
import {
  Tooltip,
  TooltipContent,
  TooltipPositioner,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";

export const Route = createFileRoute("/_app/notifications")({
  component: NotificationsPage,
  errorComponent: RouteError,
});

// The API guards these endpoints with RequireRole("admin"): channels are a
// global, instance-wide concern. Match that in the UI rather than landing a
// non-admin on a page whose every request 403s.
function NotificationsPage() {
  const isAdmin = useAuthStore((s) => s.user?.role === "admin");

  if (!isAdmin) {
    return (
      <div className="space-y-6">
        <PageHeader
          icon={<BellRing className="size-5" />}
          title="Notifications"
          description="Send platform events to Discord, Telegram, Slack, email and more."
        />
        <p className="text-muted-foreground text-sm">
          Notification channels are restricted to administrators.
        </p>
      </div>
    );
  }

  return <NotificationsContent />;
}

function NotificationsContent() {
  const { data: channels, isLoading } = useNotificationChannels();
  const setEnabled = useSetNotificationChannelEnabled();
  const test = useTestNotificationChannel();

  const [formOpen, setFormOpen] = useState(false);
  const [editTarget, setEditTarget] = useState<NotificationChannel | null>(
    null,
  );
  const [deleteTarget, setDeleteTarget] = useState<NotificationChannel | null>(
    null,
  );

  const openCreate = () => {
    setEditTarget(null);
    setFormOpen(true);
  };
  const openEdit = (channel: NotificationChannel) => {
    setEditTarget(channel);
    setFormOpen(true);
  };

  const handleToggle = (channel: NotificationChannel, enabled: boolean) => {
    toast.promise(setEnabled.mutateAsync({ id: channel.id, enabled }), {
      loading: enabled ? "Enabling…" : "Disabling…",
      success: enabled ? "Channel enabled" : "Channel disabled",
      error: (err) => err.message,
    });
  };

  const handleTest = (channel: NotificationChannel) => {
    toast.promise(test.mutateAsync(channel.id), {
      loading: `Sending test to ${channel.name}…`,
      success: (res) => {
        if (!res.ok) throw new Error(res.error ?? "Delivery failed");
        return "Test notification sent";
      },
      error: (err) => err.message,
    });
  };

  const columns = useMemo<ColumnDef<NotificationChannel>[]>(
    () => [
      {
        accessorKey: "type",
        header: "Type",
        cell: ({ row }) => {
          const Icon = TYPE_ICON[row.original.type] ?? BellRing;
          return (
            <div className="flex items-center gap-2">
              <Icon className="text-muted-foreground size-4 shrink-0" />
              <span>{TYPE_LABEL[row.original.type]}</span>
            </div>
          );
        },
      },
      {
        accessorKey: "name",
        header: "Name",
        cell: ({ row }) => (
          <span className="font-medium">{row.original.name}</span>
        ),
      },
      {
        accessorKey: "events",
        header: "Events",
        cell: ({ row }) => {
          const n = row.original.events.length;
          return n === 0 ? (
            <span className="text-muted-foreground">None</span>
          ) : (
            <span>
              {n} {n === 1 ? "event" : "events"}
            </span>
          );
        },
      },
      {
        id: "activity",
        header: "Last activity",
        cell: ({ row }) => <ActivityCell channel={row.original} />,
      },
      {
        id: "enabled",
        header: "Enabled",
        cell: ({ row }) => (
          <Switch
            aria-label={`${row.original.enabled ? "Disable" : "Enable"} ${row.original.name}`}
            checked={row.original.enabled}
            disabled={setEnabled.isPending}
            onCheckedChange={(checked: boolean) =>
              handleToggle(row.original, checked)
            }
          />
        ),
      },
      buildActionColumnDef<NotificationChannel>({
        meta: { headerClassName: "text-right", className: "text-right" },
        cell: ({ row: { original: channel } }) => (
          <div className="flex justify-end gap-1">
            <IconButton
              label="Send test"
              onClick={() => handleTest(channel)}
              disabled={test.isPending}
            >
              <SendHorizontal className="h-4 w-4" />
            </IconButton>
            <IconButton label="Edit" onClick={() => openEdit(channel)}>
              <Pencil className="h-4 w-4" />
            </IconButton>
            <IconButton
              label="Delete"
              destructive
              onClick={() => setDeleteTarget(channel)}
            >
              <Trash2 className="h-4 w-4" />
            </IconButton>
          </div>
        ),
      }),
    ],
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [setEnabled.isPending, test.isPending],
  );

  return (
    <div className="space-y-6">
      <PageHeader
        icon={<BellRing className="size-5" />}
        title={
          <>
            Notifications
            {channels && channels.length > 0 && (
              <span className="text-muted-foreground ml-2 text-base font-normal">
                {channels.length}{" "}
                {channels.length === 1 ? "channel" : "channels"}
              </span>
            )}
          </>
        }
        description="Route platform events — deployments, backups, TLS — to Discord, Telegram, Slack, ntfy, Gotify, a webhook or email. The in-app bell is unaffected."
      />

      <Card>
        <CardHeader className="flex flex-row items-center justify-between">
          <CardTitle>Channels</CardTitle>
          <Button size="sm" onClick={openCreate}>
            Add Channel
          </Button>
        </CardHeader>
        <CardContent>
          <DataTable
            columns={columns}
            data={channels ?? []}
            isLoading={isLoading}
            getRowId={(c) => c.id}
            emptyMessage='No channels yet. Click "Add Channel" to route events to a provider.'
          />
        </CardContent>
      </Card>

      <ChannelFormDialog
        channel={editTarget}
        open={formOpen}
        onOpenChange={(open) => {
          setFormOpen(open);
          if (!open) setEditTarget(null);
        }}
      />

      {deleteTarget && (
        <DeleteChannelDialog
          channel={deleteTarget}
          open={true}
          onOpenChange={(open) => !open && setDeleteTarget(null)}
        />
      )}
    </div>
  );
}

// ActivityCell shows the most recent delivery outcome: a failure (destructive,
// with the full error on hover) takes precedence over the last success.
function ActivityCell({ channel }: { channel: NotificationChannel }) {
  if (channel.last_error) {
    return (
      <Tooltip>
        <TooltipTrigger
          render={<span className="text-destructive cursor-help text-sm" />}
        >
          Failed
        </TooltipTrigger>
        <TooltipPositioner>
          <TooltipContent className="max-w-xs break-words">
            {channel.last_error}
          </TooltipContent>
        </TooltipPositioner>
      </Tooltip>
    );
  }
  if (channel.last_sent_at) {
    const when = new Date(channel.last_sent_at).toLocaleString();
    return (
      <span className="text-muted-foreground text-sm">
        {channel.last_event
          ? `Sent “${channel.last_event}” · ${when}`
          : `Sent ${when}`}
      </span>
    );
  }
  return <span className="text-muted-foreground text-sm">—</span>;
}

function IconButton({
  label,
  onClick,
  disabled,
  destructive,
  children,
}: {
  label: string;
  onClick: () => void;
  disabled?: boolean;
  destructive?: boolean;
  children: React.ReactNode;
}) {
  return (
    <Tooltip>
      <TooltipTrigger
        render={
          <Button
            variant="ghost"
            size="icon-sm"
            aria-label={label}
            disabled={disabled}
            className={
              destructive
                ? "text-destructive hover:bg-destructive/10 hover:text-destructive"
                : undefined
            }
            onClick={onClick}
          />
        }
      >
        {children}
      </TooltipTrigger>
      <TooltipPositioner>
        <TooltipContent>{label}</TooltipContent>
      </TooltipPositioner>
    </Tooltip>
  );
}

function DeleteChannelDialog({
  channel,
  open,
  onOpenChange,
}: {
  channel: NotificationChannel;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const del = useDeleteNotificationChannel();

  return (
    <AlertDialog open={open} onOpenChange={onOpenChange}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>Delete {channel.name}?</AlertDialogTitle>
          <AlertDialogDescription>
            This channel will stop receiving events. Deleting it cannot be
            undone; you would need to re-enter its configuration to recreate it.
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>Cancel</AlertDialogCancel>
          <AlertDialogAction
            disabled={del.isPending}
            onClick={async () => {
              await del.mutateAsync(channel.id);
              onOpenChange(false);
            }}
          >
            Delete
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}
