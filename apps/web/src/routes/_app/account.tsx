import { createFileRoute } from "@tanstack/react-router";
import { useForm } from "@tanstack/react-form";
import { z } from "zod";
import { useTheme } from "next-themes";
import {
  BellRingIcon,
  CircleUserIcon,
  KeyRoundIcon,
  MonitorIcon,
  MoonIcon,
  PaletteIcon,
  SunIcon,
  UserIcon,
} from "lucide-react";
import { RouteError } from "@/lib/components/route-error";
import { toast } from "sonner";
import { useAuthStore } from "@/lib/stores/auth";
import { useChangeOwnPassword, useUpdateProfile } from "@/lib/hooks/use-users";
import {
  useAlertPreferences,
  useUpdateAlertPreferences,
} from "@/lib/hooks/use-alert-preferences";
import { useAccentStore, type Accent } from "@/lib/stores/accent";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { PageHeader } from "@/components/ui/page-header";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  SegmentedControl,
  SegmentedControlItem,
} from "@/components/ui/segmented-control";
import { formatDateTimeShort } from "@/lib/utils/format";
import { TwoFactorCard } from "@/components/account/two-factor-card";
import { ApiTokensCard } from "@/components/account/api-tokens-card";

export const Route = createFileRoute("/_app/account")({
  component: SettingsPage,
  errorComponent: RouteError,
});

function fieldError(errors: unknown[]): string | undefined {
  const first = errors[0];
  if (!first) return undefined;
  return typeof first === "string"
    ? first
    : (first as { message?: string }).message;
}

function SettingsPage() {
  const user = useAuthStore((s) => s.user);

  return (
    <div className="mx-auto max-w-2xl space-y-6">
      <PageHeader
        icon={<CircleUserIcon className="size-5" />}
        title="Account"
        description="Manage your profile, security, and appearance."
      />

      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <CircleUserIcon aria-hidden="true" className="size-4" />
            Account
          </CardTitle>
          <p className="text-muted-foreground text-sm">
            Your identity on this platform. Contact an admin to change your
            email.
          </p>
        </CardHeader>
        <CardContent className="space-y-2.5 text-sm">
          <div className="flex items-center justify-between">
            <span className="text-muted-foreground">Email</span>
            <span className="font-mono">{user?.email}</span>
          </div>
          <div className="flex items-center justify-between">
            <span className="text-muted-foreground">Role</span>
            <Badge variant="outline" className="uppercase">
              {user?.role}
            </Badge>
          </div>
          {user?.created_at && (
            <div className="flex items-center justify-between">
              <span className="text-muted-foreground">Member since</span>
              <span>{formatDateTimeShort(user.created_at)}</span>
            </div>
          )}
        </CardContent>
      </Card>

      <ProfileCard />
      <ChangePasswordCard />
      <TwoFactorCard />
      <ApiTokensCard />
      <AlertPreferencesCard />
      <AppearanceCard />
    </div>
  );
}

function ProfileCard() {
  const user = useAuthStore((s) => s.user);
  const setUser = useAuthStore((s) => s.setUser);
  const updateProfile = useUpdateProfile();

  const form = useForm({
    defaultValues: {
      username: user?.username ?? "",
      firstName: user?.first_name ?? "",
      lastName: user?.last_name ?? "",
    },
    onSubmit: ({ value }) => {
      toast.promise(
        updateProfile
          .mutateAsync({
            username: value.username,
            first_name: value.firstName,
            last_name: value.lastName,
          })
          .then((updated) => {
            if (user)
              setUser({
                ...user,
                username: updated.username,
                first_name: updated.first_name,
                last_name: updated.last_name,
              });
          }),
        {
          loading: "Saving profile...",
          success: "Profile updated",
          error: (err) => err.message,
        },
      );
    },
  });

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <UserIcon aria-hidden="true" className="size-4" />
          Profile
        </CardTitle>
        <p className="text-muted-foreground text-sm">
          Your public display name on this platform.
        </p>
      </CardHeader>
      <CardContent>
        <form
          onSubmit={(e) => {
            e.preventDefault();
            e.stopPropagation();
            form.handleSubmit();
          }}
          className="space-y-4"
        >
          <form.Field
            name="username"
            validators={{
              onChange: z.string().min(1, "Username is required"),
            }}
            children={(field) => {
              const error = fieldError(field.state.meta.errors);
              return (
                <div className="space-y-2">
                  <Label htmlFor="profile-username">Username</Label>
                  <Input
                    id="profile-username"
                    value={field.state.value}
                    onBlur={field.handleBlur}
                    onChange={(e) => field.handleChange(e.target.value)}
                    placeholder="username"
                  />
                  {error && <p className="text-destructive text-xs">{error}</p>}
                </div>
              );
            }}
          />
          <div className="grid grid-cols-2 gap-4">
            <form.Field
              name="firstName"
              children={(field) => (
                <div className="space-y-2">
                  <Label htmlFor="first-name">First Name</Label>
                  <Input
                    id="first-name"
                    value={field.state.value}
                    onBlur={field.handleBlur}
                    onChange={(e) => field.handleChange(e.target.value)}
                    placeholder="First"
                  />
                </div>
              )}
            />
            <form.Field
              name="lastName"
              children={(field) => (
                <div className="space-y-2">
                  <Label htmlFor="last-name">Last Name</Label>
                  <Input
                    id="last-name"
                    value={field.state.value}
                    onBlur={field.handleBlur}
                    onChange={(e) => field.handleChange(e.target.value)}
                    placeholder="Last"
                  />
                </div>
              )}
            />
          </div>
          <Button type="submit" disabled={updateProfile.isPending}>
            {updateProfile.isPending ? "Saving..." : "Save Profile"}
          </Button>
        </form>
      </CardContent>
    </Card>
  );
}

function ChangePasswordCard() {
  const changePassword = useChangeOwnPassword();

  const form = useForm({
    defaultValues: {
      currentPassword: "",
      newPassword: "",
      confirmPassword: "",
    },
    onSubmit: ({ value }) => {
      toast.promise(
        changePassword
          .mutateAsync({
            current_password: value.currentPassword,
            new_password: value.newPassword,
          })
          .then(() => form.reset()),
        {
          loading: "Updating password...",
          success: "Password updated",
          error: (err) => err.message,
        },
      );
    },
  });

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <KeyRoundIcon aria-hidden="true" className="size-4" />
          Change Password
        </CardTitle>
      </CardHeader>
      <CardContent>
        <form
          onSubmit={(e) => {
            e.preventDefault();
            e.stopPropagation();
            form.handleSubmit();
          }}
          className="space-y-4"
        >
          <form.Field
            name="currentPassword"
            validators={{
              onChange: z.string().min(1, "Current password is required"),
            }}
            children={(field) => {
              const error = fieldError(field.state.meta.errors);
              return (
                <div className="space-y-2">
                  <Label htmlFor="current-password">Current Password</Label>
                  <Input
                    id="current-password"
                    type="password"
                    value={field.state.value}
                    onBlur={field.handleBlur}
                    onChange={(e) => field.handleChange(e.target.value)}
                  />
                  {error && <p className="text-destructive text-xs">{error}</p>}
                </div>
              );
            }}
          />
          <form.Field
            name="newPassword"
            validators={{
              onChange: z
                .string()
                .min(8, "New password must be at least 8 characters"),
            }}
            children={(field) => {
              const error = fieldError(field.state.meta.errors);
              return (
                <div className="space-y-2">
                  <Label htmlFor="new-password">New Password</Label>
                  <Input
                    id="new-password"
                    type="password"
                    value={field.state.value}
                    onBlur={field.handleBlur}
                    onChange={(e) => field.handleChange(e.target.value)}
                    placeholder="At least 8 characters"
                  />
                  {error && <p className="text-destructive text-xs">{error}</p>}
                </div>
              );
            }}
          />
          <form.Field
            name="confirmPassword"
            validators={{
              onChangeListenTo: ["newPassword"],
              onChange: ({ value, fieldApi }) =>
                value !== fieldApi.form.state.values.newPassword
                  ? "New passwords do not match"
                  : undefined,
            }}
            children={(field) => {
              const error = fieldError(field.state.meta.errors);
              return (
                <div className="space-y-2">
                  <Label htmlFor="confirm-password">Confirm New Password</Label>
                  <Input
                    id="confirm-password"
                    type="password"
                    value={field.state.value}
                    onBlur={field.handleBlur}
                    onChange={(e) => field.handleChange(e.target.value)}
                  />
                  {error && <p className="text-destructive text-xs">{error}</p>}
                </div>
              );
            }}
          />
          <Button type="submit" disabled={changePassword.isPending}>
            {changePassword.isPending ? "Updating..." : "Update Password"}
          </Button>
        </form>
      </CardContent>
    </Card>
  );
}

const DEFAULT_ALERT_PREFS = {
  deploy_failures: true,
  deploy_success: true,
  build_failures: true,
  quota_threshold: true,
  quota_threshold_percent: 80,
};

function AlertPreferencesCard() {
  const { data: prefs, isLoading } = useAlertPreferences();
  const update = useUpdateAlertPreferences();

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <BellRingIcon aria-hidden="true" className="size-4" />
          Alert Preferences
        </CardTitle>
      </CardHeader>
      <CardContent>
        {isLoading ? (
          <div className="space-y-3">
            {[1, 2, 3, 4].map((i) => (
              <div key={i} className="bg-muted h-8 animate-pulse rounded" />
            ))}
          </div>
        ) : (
          // Keyed so a fresh save re-derives defaultValues from the refetched
          // prefs instead of racing useForm's own defaultValues-sync effect.
          <AlertPreferencesForm
            key={JSON.stringify(prefs)}
            prefs={prefs ?? DEFAULT_ALERT_PREFS}
            update={update}
          />
        )}
      </CardContent>
    </Card>
  );
}

function AlertPreferencesForm({
  prefs,
  update,
}: {
  prefs: typeof DEFAULT_ALERT_PREFS;
  update: ReturnType<typeof useUpdateAlertPreferences>;
}) {
  const form = useForm({
    defaultValues: prefs,
    onSubmit: ({ value }) => {
      toast.promise(update.mutateAsync(value), {
        loading: "Saving…",
        success: "Alert preferences saved",
        error: (err) => err.message,
      });
    },
  });

  return (
    <form
      onSubmit={(e) => {
        e.preventDefault();
        e.stopPropagation();
        form.handleSubmit();
      }}
      className="space-y-4"
    >
      <form.Field
        name="deploy_failures"
        children={(field) => (
          <PreferenceRow
            label="Deploy failures"
            description="Email and notify when a deployment fails."
            checked={field.state.value}
            onToggle={() => field.handleChange(!field.state.value)}
          />
        )}
      />
      <form.Field
        name="deploy_success"
        children={(field) => (
          <PreferenceRow
            label="Deploy successes"
            description="Notify when a deployment finishes successfully."
            checked={field.state.value}
            onToggle={() => field.handleChange(!field.state.value)}
          />
        )}
      />
      <form.Field
        name="build_failures"
        children={(field) => (
          <PreferenceRow
            label="Build failures"
            description="Email and notify when a build fails."
            checked={field.state.value}
            onToggle={() => field.handleChange(!field.state.value)}
          />
        )}
      />
      <form.Field
        name="quota_threshold"
        children={(field) => (
          <PreferenceRow
            label="Quota threshold"
            description="Email when resource usage exceeds the threshold."
            checked={field.state.value}
            onToggle={() => field.handleChange(!field.state.value)}
          />
        )}
      />
      <form.Subscribe
        selector={(s) => s.values.quota_threshold}
        children={(quotaThreshold) =>
          quotaThreshold && (
            <form.Field
              name="quota_threshold_percent"
              validators={{
                onChange: z
                  .number()
                  .min(1, "Must be between 1 and 100")
                  .max(100, "Must be between 1 and 100"),
              }}
              children={(field) => {
                const error = fieldError(field.state.meta.errors);
                return (
                  <div className="space-y-1">
                    <div className="flex items-center gap-3 pl-1">
                      <Label htmlFor="quota-pct" className="text-sm">
                        Alert at
                      </Label>
                      <Input
                        id="quota-pct"
                        type="number"
                        min={1}
                        max={100}
                        value={field.state.value}
                        onBlur={field.handleBlur}
                        onChange={(e) =>
                          field.handleChange(Number(e.target.value))
                        }
                        className="w-20"
                      />
                      <span className="text-muted-foreground text-sm">
                        % usage
                      </span>
                    </div>
                    {error && (
                      <p className="text-destructive pl-1 text-xs">{error}</p>
                    )}
                  </div>
                );
              }}
            />
          )
        }
      />
      <form.Subscribe
        selector={(s) => s.isDirty}
        children={(isDirty) => (
          <Button
            type="submit"
            size="sm"
            disabled={update.isPending || !isDirty}
          >
            {update.isPending ? "Saving…" : "Save"}
          </Button>
        )}
      />
    </form>
  );
}

function AppearanceCard() {
  const { theme, setTheme, resolvedTheme } = useTheme();
  const accent = useAccentStore((s) => s.accent);
  const setAccent = useAccentStore((s) => s.setAccent);

  const themes = [
    { value: "light", label: "Light", Icon: SunIcon },
    { value: "dark", label: "Dark", Icon: MoonIcon },
    { value: "system", label: "System", Icon: MonitorIcon },
  ];
  // The default accent tracks the theme (indigo in light, violet in dark), so
  // its swatch must reflect the resolved theme — not var(--brand), which would
  // read as emerald whenever emerald is the active accent. Fall back to violet
  // before next-themes has resolved (the app defaults to dark).
  const defaultSwatch = resolvedTheme === "light" ? "#4249bd" : "#7c3aed";
  const accents: { value: Accent; label: string; swatch: string }[] = [
    { value: "violet", label: "Default", swatch: defaultSwatch },
    { value: "emerald", label: "Emerald", swatch: "#10b981" },
  ];

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <PaletteIcon aria-hidden="true" className="size-4" />
          Appearance
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-5">
        <div className="space-y-2">
          <Label>Theme</Label>
          <SegmentedControl
            fullWidth
            value={theme ?? ""}
            onValueChange={setTheme}
          >
            {themes.map(({ value, label, Icon }) => (
              <SegmentedControlItem key={value} value={value}>
                <Icon aria-hidden="true" className="size-4" />
                {label}
              </SegmentedControlItem>
            ))}
          </SegmentedControl>
        </div>

        <div className="space-y-2">
          <Label>Accent</Label>
          <SegmentedControl
            fullWidth
            value={accent}
            onValueChange={(v) => setAccent(v as Accent)}
          >
            {accents.map(({ value, label, swatch }) => (
              <SegmentedControlItem key={value} value={value}>
                <span
                  aria-hidden="true"
                  className="size-3.5 rounded-full ring-1 ring-black/10 ring-inset"
                  style={{ background: swatch }}
                />
                {label}
              </SegmentedControlItem>
            ))}
          </SegmentedControl>
          <p className="text-text-faint text-xs">
            Stored on this device — applies across the dashboard.
          </p>
        </div>
      </CardContent>
    </Card>
  );
}

function PreferenceRow({
  label,
  description,
  checked,
  onToggle,
}: {
  label: string;
  description: string;
  checked: boolean;
  onToggle: () => void;
}) {
  return (
    <div className="flex items-center justify-between">
      <div>
        <p className="text-sm font-medium">{label}</p>
        <p className="text-muted-foreground text-xs">{description}</p>
      </div>
      <button
        type="button"
        role="switch"
        aria-checked={checked}
        onClick={onToggle}
        className={`focus-visible:ring-ring relative inline-flex h-5 w-9 shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors focus-visible:ring-2 focus-visible:outline-none ${
          checked ? "bg-primary" : "bg-input"
        }`}
      >
        <span
          className={`bg-background pointer-events-none inline-block h-4 w-4 rounded-full shadow-lg transition-transform ${
            checked ? "translate-x-4" : "translate-x-0"
          }`}
        />
      </button>
    </div>
  );
}
