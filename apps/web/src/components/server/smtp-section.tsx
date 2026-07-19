import { useState } from "react";
import { toast } from "sonner";
import {
  useSmtpSettings,
  useUpdateSmtpSettings,
  useTestSmtpSettings,
} from "@/lib/hooks/use-smtp-settings";
import { useAuthStore } from "@/lib/stores/auth";
import type { SmtpSettings, SmtpTLSMode } from "@/lib/api/smtp-settings";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";

const TLS_MODES: { value: SmtpTLSMode; label: string }[] = [
  { value: "starttls", label: "STARTTLS (port 587)" },
  { value: "tls", label: "TLS / SSL (port 465)" },
  { value: "none", label: "None (unencrypted)" },
];

export function SmtpSection() {
  const { data, isLoading } = useSmtpSettings();

  if (isLoading || !data) {
    return (
      <p className="text-muted-foreground text-sm">Loading SMTP settings…</p>
    );
  }
  // Remount when the loaded config changes so fields initialise from it.
  return <SmtpForm key={data.host + data.port} initial={data} />;
}

function SmtpForm({ initial }: { initial: SmtpSettings }) {
  const update = useUpdateSmtpSettings();
  const test = useTestSmtpSettings();
  const adminEmail = useAuthStore((s) => s.user?.email ?? "");

  const [host, setHost] = useState(initial.host);
  const [port, setPort] = useState(String(initial.port || 587));
  const [user, setUser] = useState(initial.user);
  const [password, setPassword] = useState("");
  const [fromEmail, setFromEmail] = useState(initial.from_email);
  const [fromName, setFromName] = useState(initial.from_name || "Belune");
  const [tlsMode, setTlsMode] = useState<SmtpTLSMode>(
    initial.tls_mode || "starttls",
  );
  const [testTo, setTestTo] = useState(adminEmail);

  const buildData = () => ({
    host: host.trim(),
    port: Number(port) || 587,
    user: user.trim(),
    from_email: fromEmail.trim(),
    from_name: fromName.trim() || "Belune",
    tls_mode: tlsMode,
    password, // blank preserves the stored secret
  });

  const handleSave = (e: React.FormEvent) => {
    e.preventDefault();
    toast.promise(update.mutateAsync(buildData()), {
      loading: "Saving SMTP settings…",
      success: () => {
        setPassword("");
        return "SMTP settings saved";
      },
      error: (err) => err.message,
    });
  };

  const handleTest = () => {
    if (!testTo.trim()) {
      toast.error("Enter a recipient for the test email");
      return;
    }
    if (!host.trim()) {
      toast.error("SMTP host is required to test");
      return;
    }
    toast.promise(test.mutateAsync({ ...buildData(), to: testTo.trim() }), {
      loading: `Sending test to ${testTo.trim()}…`,
      success: (res) => {
        if (!res.ok) throw new Error(res.error ?? "Delivery failed");
        return "Test email sent";
      },
      error: (err) => err.message,
    });
  };

  return (
    <form onSubmit={handleSave} className="space-y-4">
      <p className="text-muted-foreground text-sm">
        Outbound email for password resets, invitations, alerts, and email
        notification channels. Leave the host blank to disable email (messages
        are logged instead). Changes take effect immediately — no restart.
      </p>

      <div className="grid gap-3 sm:grid-cols-3">
        <div className="space-y-1.5 sm:col-span-2">
          <Label htmlFor="smtp-host">Host</Label>
          <Input
            id="smtp-host"
            value={host}
            onChange={(e) => setHost(e.target.value)}
            placeholder="smtp.example.com"
          />
        </div>
        <div className="space-y-1.5">
          <Label htmlFor="smtp-port">Port</Label>
          <Input
            id="smtp-port"
            inputMode="numeric"
            value={port}
            onChange={(e) => setPort(e.target.value.replace(/[^0-9]/g, ""))}
            placeholder="587"
          />
        </div>
      </div>

      <div className="grid gap-3 sm:grid-cols-2">
        <div className="space-y-1.5">
          <Label htmlFor="smtp-user">Username</Label>
          <Input
            id="smtp-user"
            value={user}
            onChange={(e) => setUser(e.target.value)}
            placeholder="apikey / user@example.com"
            autoComplete="off"
          />
        </div>
        <div className="space-y-1.5">
          <Label htmlFor="smtp-password">Password</Label>
          <Input
            id="smtp-password"
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            placeholder={
              initial.password_set ? "•••• (leave blank to keep)" : ""
            }
            autoComplete="off"
          />
        </div>
      </div>

      <div className="grid gap-3 sm:grid-cols-2">
        <div className="space-y-1.5">
          <Label htmlFor="smtp-from-email">From address</Label>
          <Input
            id="smtp-from-email"
            value={fromEmail}
            onChange={(e) => setFromEmail(e.target.value)}
            placeholder="noreply@example.com"
          />
        </div>
        <div className="space-y-1.5">
          <Label htmlFor="smtp-from-name">From name</Label>
          <Input
            id="smtp-from-name"
            value={fromName}
            onChange={(e) => setFromName(e.target.value)}
            placeholder="Belune"
          />
        </div>
      </div>

      <div className="space-y-1.5">
        <Label>Encryption</Label>
        <Select
          value={tlsMode}
          onValueChange={(v) => setTlsMode((v as SmtpTLSMode) ?? "starttls")}
        >
          <SelectTrigger>
            <SelectValue placeholder="Select mode" />
          </SelectTrigger>
          <SelectContent>
            {TLS_MODES.map((m) => (
              <SelectItem key={m.value} value={m.value}>
                {m.label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>

      {/* Send-test row */}
      <div className="border-t pt-4">
        <Label htmlFor="smtp-test-to">Send a test email</Label>
        <div className="mt-1.5 flex flex-col gap-2 sm:flex-row">
          <Input
            id="smtp-test-to"
            type="email"
            value={testTo}
            onChange={(e) => setTestTo(e.target.value)}
            placeholder="you@example.com"
            className="sm:flex-1"
          />
          <Button
            type="button"
            variant="outline"
            onClick={handleTest}
            disabled={test.isPending}
          >
            {test.isPending ? "Sending…" : "Send test"}
          </Button>
        </div>
        <p className="text-muted-foreground mt-1.5 text-xs">
          Uses the values above, so you can test before saving. A blank password
          reuses the stored one.
        </p>
      </div>

      <div className="flex justify-end">
        <Button type="submit" disabled={update.isPending}>
          {update.isPending ? "Saving…" : "Save"}
        </Button>
      </div>
    </form>
  );
}
