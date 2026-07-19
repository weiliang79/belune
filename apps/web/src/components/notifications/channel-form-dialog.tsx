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
      help: "Comma-separated addresses.",
    },
  ],
};

// Config keys for the optional per-channel SMTP override (email only).
const SMTP_FIELDS = [
  "smtp_host",
  "smtp_port",
  "smtp_user",
  "smtp_password",
  "smtp_from_email",
  "smtp_from_name",
  "smtp_tls_mode",
] as const;

const SMTP_TLS_MODES = [
  { value: "starttls", label: "STARTTLS (port 587)" },
  { value: "tls", label: "TLS / SSL (port 465)" },
  { value: "none", label: "None (unencrypted)" },
];

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
  // Config fields keyed by field key, prefilled from the channel's (secret-free)
  // stored config on edit; cleared when the type changes.
  const [config, setConfig] = useState<Record<string, string>>(() =>
    initialConfig(channel),
  );
  // Email only: route this channel through its own SMTP server (Option B).
  const [customSmtp, setCustomSmtp] = useState(
    () => !!(channel?.type === "email" && channel.config?.smtp),
  );
  // Optional secrets the operator explicitly cleared on edit (sent as empty so
  // the server drops the stored value rather than preserving it).
  const [clearedSecrets, setClearedSecrets] = useState<Set<string>>(
    () => new Set(),
  );

  const toggleCleared = (key: string) =>
    setClearedSecrets((prev) => {
      const next = new Set(prev);
      if (next.has(key)) next.delete(key);
      else next.add(key);
      return next;
    });

  const fields = TYPE_FIELDS[type];

  const grouped = useMemo(() => groupEvents(events ?? []), [events]);

  const toggleEvent = (t: string) =>
    setSelectedEvents((prev) =>
      prev.includes(t) ? prev.filter((e) => e !== t) : [...prev, t],
    );

  const setKey = (k: string, v: string) =>
    setConfig((prev) => ({ ...prev, [k]: v }));

  const handleTypeChange = (v: ChannelType) => {
    setType(v);
    setConfig({});
    setCustomSmtp(false);
    setClearedSecrets(new Set());
  };

  const smtpActive = type === "email" && customSmtp;

  // buildConfig returns the provider config to send, or an error. On edit, an
  // all-blank config is omitted so the stored one is preserved.
  const buildConfig = (): {
    config?: Record<string, unknown>;
    error?: string;
    omit?: boolean;
  } => {
    const anyProvided =
      fields.some((f) => (config[f.key] ?? "").trim() !== "") ||
      clearedSecrets.size > 0 ||
      (smtpActive && SMTP_FIELDS.some((k) => (config[k] ?? "").trim() !== ""));
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
      if (f.secret && editing) {
        // Explicitly cleared → send empty so the server drops the stored value.
        // Otherwise a blank secret is omitted so the stored one is preserved.
        if (clearedSecrets.has(f.key)) {
          out[f.key] = "";
          continue;
        }
        if (raw === "") continue;
      }
      if (f.required && raw === "") return { error: `${f.label} is required` };
      if (raw !== "") out[f.key] = raw;
    }

    if (smtpActive) {
      const host = (config.smtp_host ?? "").trim();
      if (!host) return { error: "A custom mail server needs a host" };
      const smtp: Record<string, unknown> = {
        host,
        port: Number(config.smtp_port) || 587,
        user: (config.smtp_user ?? "").trim(),
        from_email: (config.smtp_from_email ?? "").trim(),
        from_name: (config.smtp_from_name ?? "").trim(),
        tls_mode: config.smtp_tls_mode || "starttls",
      };
      // Include the password only when entered; blank omits it so the server
      // preserves the stored one (or stays passwordless on create).
      if ((config.smtp_password ?? "") !== "")
        smtp.password = config.smtp_password;
      out.smtp = smtp;
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
                  value={clearedSecrets.has(f.key) ? "" : (config[f.key] ?? "")}
                  disabled={clearedSecrets.has(f.key)}
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
                {/* Optional secrets can be removed on edit — a blank field alone
                    can't distinguish "keep" from "clear". */}
                {editing && f.secret && !f.required && (
                  <label className="text-muted-foreground flex items-center gap-2 text-xs">
                    <input
                      type="checkbox"
                      className="size-3.5"
                      checked={clearedSecrets.has(f.key)}
                      onChange={() => toggleCleared(f.key)}
                    />
                    Remove the stored {f.label.toLowerCase()}
                  </label>
                )}
                {f.help && (
                  <p className="text-muted-foreground text-xs">{f.help}</p>
                )}
              </div>
            ))}
            {editing && fields.some((f) => f.secret) && (
              <p className="text-muted-foreground text-xs">
                Secret fields are hidden — leave them blank to keep the stored
                value.
              </p>
            )}
          </div>

          {/* Email: optional per-channel mail server (Option B) */}
          {type === "email" && (
            <div className="space-y-3 rounded-md border p-3">
              <div className="flex items-start justify-between gap-3">
                <div className="min-w-0">
                  <p className="text-sm font-medium">
                    Use a custom mail server
                  </p>
                  <p className="text-muted-foreground text-xs">
                    Off, this channel sends through the instance SMTP (Server →
                    Email). On, it dials its own server instead.
                  </p>
                </div>
                <Switch
                  aria-label="Use a custom mail server"
                  checked={customSmtp}
                  onCheckedChange={(v: boolean) => setCustomSmtp(v)}
                  className="mt-0.5 shrink-0"
                />
              </div>

              {customSmtp && (
                <div className="space-y-3 border-t pt-3">
                  <div className="grid grid-cols-3 gap-2">
                    <div className="col-span-2 space-y-1.5">
                      <Label htmlFor="channel-smtp_host">Host</Label>
                      <Input
                        id="channel-smtp_host"
                        value={config.smtp_host ?? ""}
                        onChange={(e) => setKey("smtp_host", e.target.value)}
                        placeholder="smtp.example.com"
                      />
                    </div>
                    <div className="space-y-1.5">
                      <Label htmlFor="channel-smtp_port">Port</Label>
                      <Input
                        id="channel-smtp_port"
                        inputMode="numeric"
                        value={config.smtp_port ?? ""}
                        onChange={(e) =>
                          setKey(
                            "smtp_port",
                            e.target.value.replace(/[^0-9]/g, ""),
                          )
                        }
                        placeholder="587"
                      />
                    </div>
                  </div>

                  <div className="grid grid-cols-2 gap-2">
                    <div className="space-y-1.5">
                      <Label htmlFor="channel-smtp_user">Username</Label>
                      <Input
                        id="channel-smtp_user"
                        value={config.smtp_user ?? ""}
                        onChange={(e) => setKey("smtp_user", e.target.value)}
                        autoComplete="off"
                      />
                    </div>
                    <div className="space-y-1.5">
                      <Label htmlFor="channel-smtp_password">Password</Label>
                      <Input
                        id="channel-smtp_password"
                        type="password"
                        value={config.smtp_password ?? ""}
                        onChange={(e) =>
                          setKey("smtp_password", e.target.value)
                        }
                        placeholder={
                          editing && !!channel?.config?.smtp
                            ? "•••• (leave blank to keep)"
                            : ""
                        }
                        autoComplete="off"
                      />
                    </div>
                  </div>

                  <div className="grid grid-cols-2 gap-2">
                    <div className="space-y-1.5">
                      <Label htmlFor="channel-smtp_from_email">
                        From address
                      </Label>
                      <Input
                        id="channel-smtp_from_email"
                        value={config.smtp_from_email ?? ""}
                        onChange={(e) =>
                          setKey("smtp_from_email", e.target.value)
                        }
                        placeholder="noreply@example.com"
                      />
                    </div>
                    <div className="space-y-1.5">
                      <Label htmlFor="channel-smtp_from_name">From name</Label>
                      <Input
                        id="channel-smtp_from_name"
                        value={config.smtp_from_name ?? ""}
                        onChange={(e) =>
                          setKey("smtp_from_name", e.target.value)
                        }
                        placeholder="Belune"
                      />
                    </div>
                  </div>

                  <div className="space-y-1.5">
                    <Label>Encryption</Label>
                    <Select
                      value={config.smtp_tls_mode || "starttls"}
                      onValueChange={(v) =>
                        setKey("smtp_tls_mode", v ?? "starttls")
                      }
                    >
                      <SelectTrigger>
                        <SelectValue placeholder="Select mode" />
                      </SelectTrigger>
                      <SelectContent>
                        {SMTP_TLS_MODES.map((m) => (
                          <SelectItem key={m.value} value={m.value}>
                            {m.label}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </div>
                </div>
              )}
            </div>
          )}

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

// initialConfig maps a channel's stored (secret-free) config into the flat
// field state the form uses. Secret fields are absent from the config, so they
// stay blank and show a "leave blank to keep" placeholder.
function initialConfig(
  channel?: NotificationChannel | null,
): Record<string, string> {
  const c = channel?.config;
  if (!c) return {};
  const out: Record<string, string> = {};
  const asStr = (v: unknown): string | undefined => {
    if (typeof v === "string") return v;
    if (typeof v === "number") return String(v);
    return undefined;
  };

  for (const f of TYPE_FIELDS[channel.type]) {
    const v = c[f.key];
    if (f.key === "recipients" && Array.isArray(v)) {
      out.recipients = v.join(", ");
    } else {
      const s = asStr(v);
      if (s !== undefined) out[f.key] = s;
    }
  }

  if (channel.type === "email" && c.smtp && typeof c.smtp === "object") {
    const s = c.smtp as Record<string, unknown>;
    const pairs: [string, string][] = [
      ["smtp_host", "host"],
      ["smtp_port", "port"],
      ["smtp_user", "user"],
      ["smtp_from_email", "from_email"],
      ["smtp_from_name", "from_name"],
      ["smtp_tls_mode", "tls_mode"],
    ];
    for (const [k, key] of pairs) {
      const val = asStr(s[key]);
      if (val !== undefined) out[k] = val;
    }
  }
  return out;
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
