import { useMemo, useState } from "react";
import { toast } from "sonner";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  useCreateNotificationChannel,
  useUpdateNotificationChannel,
  useNotificationEvents,
  useTestNotificationChannelParams,
} from "@/lib/hooks/use-notification-channels";
import {
  CHANNEL_TYPES,
  TYPE_ICON,
} from "@/components/notifications/channel-types";
import type {
  ChannelType,
  NotificationChannel,
  NotificationEvent,
  SaveNotificationChannel,
} from "@/lib/api/notification-channels";

interface Props {
  channel?: NotificationChannel | null;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

interface FieldDef {
  key: string;
  label: string;
  placeholder?: string;
  help?: string;
  secret?: boolean;
  required?: boolean;
}

// Per-provider connection fields, with inline setup help. Every value is stored
// keyring-encrypted server-side; because reads are masked, editing re-enters the
// whole config (or leaves it blank to keep the current one).
const TYPE_FIELDS: Record<ChannelType, FieldDef[]> = {
  discord: [
    {
      key: "webhook_url",
      label: "Webhook URL",
      secret: true,
      required: true,
      help: "Discord: Server Settings → Integrations → Webhooks → New Webhook → Copy URL.",
    },
  ],
  slack: [
    {
      key: "webhook_url",
      label: "Webhook URL",
      secret: true,
      required: true,
      help: "Slack: create an app → Incoming Webhooks → Add New Webhook to Workspace.",
    },
  ],
  telegram: [
    {
      key: "bot_token",
      label: "Bot token",
      secret: true,
      required: true,
      help: "Create a bot with @BotFather to get the token.",
    },
    {
      key: "chat_id",
      label: "Chat ID",
      required: true,
      help: "Message the bot, then read the chat id from @userinfobot or the getUpdates API.",
    },
  ],
  ntfy: [
    {
      key: "topic",
      label: "Topic",
      required: true,
      placeholder: "belune-alerts",
    },
    {
      key: "server_url",
      label: "Server URL",
      placeholder: "https://ntfy.sh",
      help: "Leave blank for the public ntfy.sh, or point at your self-hosted instance.",
    },
    {
      key: "access_token",
      label: "Access token",
      secret: true,
      help: "Optional — only for protected topics.",
    },
  ],
  gotify: [
    {
      key: "server_url",
      label: "Server URL",
      required: true,
      placeholder: "https://gotify.example.com",
    },
    {
      key: "app_token",
      label: "App token",
      secret: true,
      required: true,
      help: "Gotify: Apps → Create Application → copy the token.",
    },
  ],
  webhook: [
    {
      key: "url",
      label: "URL",
      required: true,
      placeholder: "https://example.com/hooks/belune",
    },
    {
      key: "secret",
      label: "Signing secret",
      secret: true,
      help: "Optional. When set, requests carry an X-Belune-Signature HMAC-SHA256 header.",
    },
  ],
  email: [
    {
      key: "recipients",
      label: "Recipients",
      required: true,
      placeholder: "ops@example.com, oncall@example.com",
      help: "Comma-separated addresses. Uses the same SMTP settings as the rest of the app.",
    },
  ],
};

export function ChannelFormDialog({ channel, open, onOpenChange }: Props) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      {/* Flex column with a capped height: header and footer stay put, only the
          fields between them scroll. */}
      <DialogContent className="flex max-h-[calc(100dvh-4rem)] flex-col sm:max-w-lg">
        {/* Remount per open/target so fields initialise from props without an effect. */}
        {open && (
          <ChannelForm
            key={channel?.id ?? "new"}
            channel={channel}
            onDone={() => onOpenChange(false)}
          />
        )}
      </DialogContent>
    </Dialog>
  );
}

function ChannelForm({
  channel,
  onDone,
}: {
  channel?: NotificationChannel | null;
  onDone: () => void;
}) {
  const editing = !!channel;
  const create = useCreateNotificationChannel();
  const update = useUpdateNotificationChannel();
  const test = useTestNotificationChannelParams();
  const { data: events } = useNotificationEvents();

  const [name, setName] = useState(channel?.name ?? "");
  const [type, setType] = useState<ChannelType>(channel?.type ?? "discord");
  const [selectedEvents, setSelectedEvents] = useState<string[]>(
    channel?.events ?? [],
  );
  // Config fields keyed by field key; cleared when the type changes.
  const [config, setConfig] = useState<Record<string, string>>({});

  const fields = TYPE_FIELDS[type];

  const grouped = useMemo(() => groupEvents(events ?? []), [events]);

  const toggleEvent = (t: string) =>
    setSelectedEvents((prev) =>
      prev.includes(t) ? prev.filter((e) => e !== t) : [...prev, t],
    );

  const handleTypeChange = (v: ChannelType) => {
    setType(v);
    setConfig({});
  };

  // buildConfig returns the provider config to send, or an error. On edit, an
  // all-blank config is omitted so the stored one is preserved.
  const buildConfig = (): {
    config?: Record<string, unknown>;
    error?: string;
    omit?: boolean;
  } => {
    const anyProvided = fields.some((f) => (config[f.key] ?? "").trim() !== "");
    if (editing && !anyProvided) return { omit: true };

    const out: Record<string, unknown> = {};
    for (const f of fields) {
      const raw = (config[f.key] ?? "").trim();
      if (f.key === "recipients") {
        const list = raw
          .split(",")
          .map((s) => s.trim())
          .filter(Boolean);
        if (f.required && list.length === 0)
          return { error: "At least one recipient is required" };
        out.recipients = list;
        continue;
      }
      if (f.required && raw === "") return { error: `${f.label} is required` };
      if (raw !== "") out[f.key] = raw;
    }
    return { config: out };
  };

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!name.trim()) {
      toast.error("Name is required");
      return;
    }
    const built = buildConfig();
    if (built.error) {
      toast.error(built.error);
      return;
    }
    const data: SaveNotificationChannel = {
      name: name.trim(),
      type,
      events: selectedEvents,
      enabled: channel?.enabled ?? true,
      ...(built.omit ? {} : { config: built.config }),
    };

    const action =
      editing && channel
        ? update.mutateAsync({ id: channel.id, data })
        : create.mutateAsync(data);
    toast.promise(action, {
      loading: editing ? "Saving channel…" : "Creating channel…",
      success: () => {
        onDone();
        return editing ? "Channel saved" : "Channel created";
      },
      error: (err) => err.message,
    });
  };

  // handleTest delivers a sample event through the current form values. On edit
  // with the config left blank, the backend falls back to the stored config.
  const handleTest = () => {
    const built = buildConfig();
    if (built.error) {
      toast.error(built.error);
      return;
    }
    toast.promise(
      test.mutateAsync({
        id: channel?.id,
        type,
        config: built.omit ? undefined : built.config,
      }),
      {
        loading: "Sending test…",
        success: (res) => {
          if (!res.ok) throw new Error(res.error ?? "Delivery failed");
          return "Test notification sent";
        },
        error: (err) => err.message,
      },
    );
  };

  const pending = create.isPending || update.isPending;

  return (
    <>
      <DialogHeader className="shrink-0">
        <DialogTitle>{editing ? "Edit Channel" : "Add Channel"}</DialogTitle>
        <DialogDescription>
          Route events to an external service. Failures are retried and surfaced
          on the channel row.
        </DialogDescription>
      </DialogHeader>

      <form
        onSubmit={handleSubmit}
        className="flex min-h-0 flex-1 flex-col gap-4"
      >
        <div className="min-h-0 flex-1 space-y-4 overflow-y-auto pr-1">
          <div className="space-y-1.5">
            <Label htmlFor="channel-name">Name</Label>
            <Input
              id="channel-name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="Ops Discord"
              required
            />
          </div>

          <div className="space-y-1.5">
            <Label>Type</Label>
            <Select
              value={type}
              onValueChange={(v) =>
                handleTypeChange((v as ChannelType) ?? "discord")
              }
              // Type is immutable once saved — the stored config is provider-specific.
              disabled={editing}
            >
              <SelectTrigger>
                <SelectValue placeholder="Select type" />
              </SelectTrigger>
              <SelectContent>
                {CHANNEL_TYPES.map((t) => {
                  const Icon = TYPE_ICON[t.value];
                  return (
                    <SelectItem key={t.value} value={t.value} icon={<Icon />}>
                      {t.label}
                    </SelectItem>
                  );
                })}
              </SelectContent>
            </Select>
          </div>

          {/* Type-specific connection fields */}
          <div className="space-y-3">
            {fields.map((f) => (
              <div key={f.key} className="space-y-1.5">
                <Label htmlFor={`channel-${f.key}`}>
                  {f.label}
                  {!f.required && (
                    <span className="text-muted-foreground ml-1 text-xs font-normal">
                      (optional)
                    </span>
                  )}
                </Label>
                <Input
                  id={`channel-${f.key}`}
                  type={f.secret ? "password" : "text"}
                  value={config[f.key] ?? ""}
                  onChange={(e) =>
                    setConfig((prev) => ({ ...prev, [f.key]: e.target.value }))
                  }
                  placeholder={
                    editing && f.secret
                      ? "•••• (leave blank to keep)"
                      : f.placeholder
                  }
                  autoComplete="off"
                />
                {f.help && (
                  <p className="text-muted-foreground text-xs">{f.help}</p>
                )}
              </div>
            ))}
            {editing && (
              <p className="text-muted-foreground text-xs">
                Leave the fields above blank to keep the current configuration.
              </p>
            )}
          </div>

          {/* Event subscriptions */}
          <div className="space-y-2">
            <Label>Events</Label>
            {grouped.length === 0 ? (
              <p className="text-muted-foreground text-xs">Loading events…</p>
            ) : (
              <div className="space-y-3">
                {grouped.map(({ group, items }) => (
                  <div key={group} className="space-y-1.5">
                    <p className="text-text-faint text-[10.5px] font-semibold tracking-wider uppercase">
                      {group}
                    </p>
                    <div className="divide-border/60 divide-y rounded-md border">
                      {items.map((ev) => (
                        <div
                          key={ev.type}
                          className="flex items-start justify-between gap-3 px-3 py-2"
                        >
                          <div className="min-w-0">
                            <p className="text-sm">{ev.label}</p>
                            <p className="text-muted-foreground text-xs">
                              {ev.description}
                            </p>
                          </div>
                          <Switch
                            aria-label={ev.label}
                            checked={selectedEvents.includes(ev.type)}
                            onCheckedChange={() => toggleEvent(ev.type)}
                            className="mt-0.5 shrink-0"
                          />
                        </div>
                      ))}
                    </div>
                  </div>
                ))}
              </div>
            )}
          </div>
        </div>

        <DialogFooter className="shrink-0 sm:justify-between">
          <Button
            type="button"
            variant="outline"
            onClick={handleTest}
            disabled={test.isPending}
          >
            {test.isPending ? "Testing…" : "Send test"}
          </Button>
          <div className="flex flex-col-reverse gap-2 sm:flex-row">
            <Button type="button" variant="outline" onClick={onDone}>
              Cancel
            </Button>
            <Button type="submit" disabled={pending}>
              {pending ? "Saving…" : editing ? "Save" : "Create"}
            </Button>
          </div>
        </DialogFooter>
      </form>
    </>
  );
}

function groupEvents(events: NotificationEvent[]) {
  const order: string[] = [];
  const map = new Map<string, NotificationEvent[]>();
  for (const ev of events) {
    if (!map.has(ev.group)) {
      map.set(ev.group, []);
      order.push(ev.group);
    }
    map.get(ev.group)!.push(ev);
  }
  return order.map((group) => ({ group, items: map.get(group)! }));
}
