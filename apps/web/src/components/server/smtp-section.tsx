import { useForm } from "@tanstack/react-form";
import { z } from "zod";
import { toast } from "sonner";
import {
  useSmtpSettings,
  useUpdateSmtpSettings,
  useTestSmtpSettings,
} from "@/lib/hooks/use-smtp-settings";
import { useAuthStore } from "@/lib/stores/auth";
import type { SmtpSettings, SmtpTLSMode } from "@/lib/api/smtp-settings";
import { Button } from "@/components/ui/button";
import { Field, FieldError, FieldLabel } from "@/components/ui/field";
import { fieldError } from "@/lib/utils/field-error";
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

  const form = useForm({
    defaultValues: {
      host: initial.host,
      port: String(initial.port || 587),
      user: initial.user,
      password: "",
      fromEmail: initial.from_email,
      fromName: initial.from_name || "Belune",
      tlsMode: initial.tls_mode || ("starttls" as SmtpTLSMode),
      testTo: adminEmail,
    },
    onSubmit: ({ value }) => {
      const data = {
        host: value.host.trim(),
        port: Number(value.port) || 587,
        user: value.user.trim(),
        from_email: value.fromEmail.trim(),
        from_name: value.fromName.trim() || "Belune",
        tls_mode: value.tlsMode,
        password: value.password, // blank preserves the stored secret
      };
      toast.promise(update.mutateAsync(data), {
        loading: "Saving SMTP settings…",
        success: () => {
          form.setFieldValue("password", "");
          return "SMTP settings saved";
        },
        error: (err) => err.message,
      });
    },
  });

  const handleTest = () => {
    const v = form.state.values;
    if (!v.testTo.trim()) {
      toast.error("Enter a recipient for the test email");
      return;
    }
    if (!v.host.trim()) {
      toast.error("SMTP host is required to test");
      return;
    }
    toast.promise(
      test.mutateAsync({
        host: v.host.trim(),
        port: Number(v.port) || 587,
        user: v.user.trim(),
        from_email: v.fromEmail.trim(),
        from_name: v.fromName.trim() || "Belune",
        tls_mode: v.tlsMode,
        password: v.password,
        to: v.testTo.trim(),
      }),
      {
        loading: `Sending test to ${v.testTo.trim()}…`,
        success: (res) => {
          if (!res.ok) throw new Error(res.error ?? "Delivery failed");
          return "Test email sent";
        },
        error: (err) => err.message,
      },
    );
  };

  return (
    <form
      onSubmit={(e) => {
        e.preventDefault();
        e.stopPropagation();
        form.handleSubmit();
      }}
      className="space-y-4"
    >
      <p className="text-muted-foreground text-sm">
        Outbound email for password resets, invitations, alerts, and email
        notification channels. Leave the host blank to disable email (messages
        are logged instead). Changes take effect immediately — no restart.
      </p>

      <div className="grid gap-3 sm:grid-cols-3">
        <form.Field
          name="host"
          children={(field) => (
            <div className="space-y-1.5 sm:col-span-2">
              <Label htmlFor="smtp-host">Host</Label>
              <Input
                id="smtp-host"
                value={field.state.value}
                onBlur={field.handleBlur}
                onChange={(e) => field.handleChange(e.target.value)}
                placeholder="smtp.example.com"
              />
            </div>
          )}
        />
        <form.Field
          name="port"
          validators={{
            onChange: z
              .string()
              .refine(
                (v) => v === "" || (Number(v) >= 1 && Number(v) <= 65535),
                "Port must be between 1 and 65535",
              ),
          }}
          children={(field) => {
            const error = fieldError(field.state.meta.errors);
            return (
              <Field data-invalid={!!error}>
                <FieldLabel htmlFor="smtp-port">Port</FieldLabel>
                <Input
                  id="smtp-port"
                  inputMode="numeric"
                  value={field.state.value}
                  onBlur={field.handleBlur}
                  onChange={(e) =>
                    field.handleChange(e.target.value.replace(/[^0-9]/g, ""))
                  }
                  placeholder="587"
                  aria-invalid={!!error}
                />
                {error && <FieldError>{error}</FieldError>}
              </Field>
            );
          }}
        />
      </div>

      <div className="grid gap-3 sm:grid-cols-2">
        <form.Field
          name="user"
          children={(field) => (
            <div className="space-y-1.5">
              <Label htmlFor="smtp-user">Username</Label>
              <Input
                id="smtp-user"
                value={field.state.value}
                onBlur={field.handleBlur}
                onChange={(e) => field.handleChange(e.target.value)}
                placeholder="apikey / user@example.com"
                autoComplete="off"
              />
            </div>
          )}
        />
        <form.Field
          name="password"
          children={(field) => (
            <div className="space-y-1.5">
              <Label htmlFor="smtp-password">Password</Label>
              <Input
                id="smtp-password"
                type="password"
                value={field.state.value}
                onBlur={field.handleBlur}
                onChange={(e) => field.handleChange(e.target.value)}
                placeholder={
                  initial.password_set ? "•••• (leave blank to keep)" : ""
                }
                autoComplete="off"
              />
            </div>
          )}
        />
      </div>

      <div className="grid gap-3 sm:grid-cols-2">
        <form.Field
          name="fromEmail"
          validators={{
            onChange: z.union([
              z.literal(""),
              z.string().email("Enter a valid email address"),
            ]),
          }}
          children={(field) => {
            const error = fieldError(field.state.meta.errors);
            return (
              <Field data-invalid={!!error}>
                <FieldLabel htmlFor="smtp-from-email">From address</FieldLabel>
                <Input
                  id="smtp-from-email"
                  value={field.state.value}
                  onBlur={field.handleBlur}
                  onChange={(e) => field.handleChange(e.target.value)}
                  placeholder="noreply@example.com"
                  aria-invalid={!!error}
                />
                {error && <FieldError>{error}</FieldError>}
              </Field>
            );
          }}
        />
        <form.Field
          name="fromName"
          children={(field) => (
            <div className="space-y-1.5">
              <Label htmlFor="smtp-from-name">From name</Label>
              <Input
                id="smtp-from-name"
                value={field.state.value}
                onBlur={field.handleBlur}
                onChange={(e) => field.handleChange(e.target.value)}
                placeholder="Belune"
              />
            </div>
          )}
        />
      </div>

      <form.Field
        name="tlsMode"
        children={(field) => (
          <div className="space-y-1.5">
            <Label>Encryption</Label>
            <Select
              value={field.state.value}
              onValueChange={(v) =>
                field.handleChange((v as SmtpTLSMode) ?? "starttls")
              }
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
        )}
      />

      {/* Send-test row */}
      <div className="border-t pt-4">
        <form.Field
          name="testTo"
          children={(field) => (
            <>
              <Label htmlFor="smtp-test-to">Send a test email</Label>
              <div className="mt-1.5 flex flex-col gap-2 sm:flex-row">
                <Input
                  id="smtp-test-to"
                  type="email"
                  value={field.state.value}
                  onBlur={field.handleBlur}
                  onChange={(e) => field.handleChange(e.target.value)}
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
            </>
          )}
        />
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
