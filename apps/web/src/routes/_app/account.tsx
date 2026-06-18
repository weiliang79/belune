import { useState } from "react";
import { createFileRoute } from "@tanstack/react-router";
import { useTheme } from "next-themes";
import { MonitorIcon, MoonIcon, SunIcon } from "lucide-react";
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
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { cn } from "@/lib/utils";

export const Route = createFileRoute("/_app/account")({
  component: SettingsPage,
  errorComponent: RouteError,
});

function SettingsPage() {
  const user = useAuthStore((s) => s.user);

  return (
    <div className="mx-auto max-w-2xl space-y-6">
      <div>
        <h1 className="text-2xl font-semibold tracking-tight">Account</h1>
        <p className="text-muted-foreground text-sm">
          Manage your profile, security, and appearance.
        </p>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>Account</CardTitle>
        </CardHeader>
        <CardContent className="space-y-2 text-sm">
          <div className="flex justify-between">
            <span className="text-muted-foreground">Email</span>
            <span className="font-mono">{user?.email}</span>
          </div>
          <div className="flex justify-between">
            <span className="text-muted-foreground">Role</span>
            <span className="capitalize">{user?.role}</span>
          </div>
        </CardContent>
      </Card>

      <ProfileCard />
      <ChangePasswordCard />
      <AlertPreferencesCard />
      <AppearanceCard />
    </div>
  );
}

function ProfileCard() {
  const user = useAuthStore((s) => s.user);
  const setUser = useAuthStore((s) => s.setUser);
  const [username, setUsername] = useState(user?.username ?? "");
  const [firstName, setFirstName] = useState(user?.first_name ?? "");
  const [lastName, setLastName] = useState(user?.last_name ?? "");
  const updateProfile = useUpdateProfile();

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    toast.promise(
      updateProfile
        .mutateAsync({ username, first_name: firstName, last_name: lastName })
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
  };

  return (
    <Card>
      <CardHeader>
        <CardTitle>Profile</CardTitle>
      </CardHeader>
      <CardContent>
        <form onSubmit={handleSubmit} className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="profile-username">Username</Label>
            <Input
              id="profile-username"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              placeholder="username"
            />
          </div>
          <div className="grid grid-cols-2 gap-4">
            <div className="space-y-2">
              <Label htmlFor="first-name">First Name</Label>
              <Input
                id="first-name"
                value={firstName}
                onChange={(e) => setFirstName(e.target.value)}
                placeholder="First"
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="last-name">Last Name</Label>
              <Input
                id="last-name"
                value={lastName}
                onChange={(e) => setLastName(e.target.value)}
                placeholder="Last"
              />
            </div>
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
  const [currentPassword, setCurrentPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const changePassword = useChangeOwnPassword();

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();

    if (!currentPassword || !newPassword) {
      toast.error("All fields are required");
      return;
    }
    if (newPassword.length < 8) {
      toast.error("New password must be at least 8 characters");
      return;
    }
    if (newPassword !== confirmPassword) {
      toast.error("New passwords do not match");
      return;
    }

    toast.promise(
      changePassword
        .mutateAsync({
          current_password: currentPassword,
          new_password: newPassword,
        })
        .then(() => {
          setCurrentPassword("");
          setNewPassword("");
          setConfirmPassword("");
        }),
      {
        loading: "Updating password...",
        success: "Password updated",
        error: (err) => err.message,
      },
    );
  };

  return (
    <Card>
      <CardHeader>
        <CardTitle>Change Password</CardTitle>
      </CardHeader>
      <CardContent>
        <form onSubmit={handleSubmit} className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="current-password">Current Password</Label>
            <Input
              id="current-password"
              type="password"
              value={currentPassword}
              onChange={(e) => setCurrentPassword(e.target.value)}
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="new-password">New Password</Label>
            <Input
              id="new-password"
              type="password"
              value={newPassword}
              onChange={(e) => setNewPassword(e.target.value)}
              placeholder="At least 8 characters"
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="confirm-password">Confirm New Password</Label>
            <Input
              id="confirm-password"
              type="password"
              value={confirmPassword}
              onChange={(e) => setConfirmPassword(e.target.value)}
            />
          </div>
          <Button type="submit" disabled={changePassword.isPending}>
            {changePassword.isPending ? "Updating..." : "Update Password"}
          </Button>
        </form>
      </CardContent>
    </Card>
  );
}

function AlertPreferencesCard() {
  const { data: prefs, isLoading } = useAlertPreferences();
  const update = useUpdateAlertPreferences();

  const [local, setLocal] = useState<{
    deploy_failures: boolean;
    build_failures: boolean;
    quota_threshold: boolean;
    quota_threshold_percent: number;
  } | null>(null);

  const current = local ??
    prefs ?? {
      deploy_failures: true,
      build_failures: true,
      quota_threshold: true,
      quota_threshold_percent: 80,
    };

  const toggle = (
    key: "deploy_failures" | "build_failures" | "quota_threshold",
  ) => {
    setLocal({ ...current, [key]: !current[key] });
  };

  const handleSave = () => {
    toast.promise(
      update.mutateAsync(current).then(() => setLocal(null)),
      {
        loading: "Saving…",
        success: "Alert preferences saved",
        error: (err) => err.message,
      },
    );
  };

  return (
    <Card>
      <CardHeader>
        <CardTitle>Alert Preferences</CardTitle>
      </CardHeader>
      <CardContent>
        {isLoading ? (
          <div className="space-y-3">
            {[1, 2, 3].map((i) => (
              <div key={i} className="bg-muted h-8 animate-pulse rounded" />
            ))}
          </div>
        ) : (
          <div className="space-y-4">
            <PreferenceRow
              label="Deploy failures"
              description="Email when a deployment fails."
              checked={current.deploy_failures}
              onToggle={() => toggle("deploy_failures")}
            />
            <PreferenceRow
              label="Build failures"
              description="Email when a build fails."
              checked={current.build_failures}
              onToggle={() => toggle("build_failures")}
            />
            <PreferenceRow
              label="Quota threshold"
              description="Email when resource usage exceeds the threshold."
              checked={current.quota_threshold}
              onToggle={() => toggle("quota_threshold")}
            />
            {current.quota_threshold && (
              <div className="flex items-center gap-3 pl-1">
                <Label htmlFor="quota-pct" className="text-sm">
                  Alert at
                </Label>
                <Input
                  id="quota-pct"
                  type="number"
                  min={1}
                  max={100}
                  value={current.quota_threshold_percent}
                  onChange={(e) =>
                    setLocal({
                      ...current,
                      quota_threshold_percent: Number(e.target.value),
                    })
                  }
                  className="w-20"
                />
                <span className="text-muted-foreground text-sm">% usage</span>
              </div>
            )}
            <Button
              size="sm"
              onClick={handleSave}
              disabled={update.isPending || local === null}
            >
              {update.isPending ? "Saving…" : "Save"}
            </Button>
          </div>
        )}
      </CardContent>
    </Card>
  );
}

function SegmentedOption({
  active,
  onClick,
  children,
}: {
  active: boolean;
  onClick: () => void;
  children: React.ReactNode;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      aria-pressed={active}
      className={cn(
        "flex flex-1 items-center justify-center gap-2 rounded-md px-3 py-2 text-sm font-medium transition-colors",
        active
          ? "bg-card text-foreground ring-border-strong shadow-sm ring-1"
          : "text-muted-foreground hover:text-foreground",
      )}
    >
      {children}
    </button>
  );
}

function AppearanceCard() {
  const { theme, setTheme } = useTheme();
  const accent = useAccentStore((s) => s.accent);
  const setAccent = useAccentStore((s) => s.setAccent);

  const themes = [
    { value: "light", label: "Light", Icon: SunIcon },
    { value: "dark", label: "Dark", Icon: MoonIcon },
    { value: "system", label: "System", Icon: MonitorIcon },
  ];
  const accents: { value: Accent; label: string; swatch: string }[] = [
    { value: "violet", label: "Violet", swatch: "#7c3aed" },
    { value: "emerald", label: "Emerald", swatch: "#10b981" },
  ];

  return (
    <Card>
      <CardHeader>
        <CardTitle>Appearance</CardTitle>
      </CardHeader>
      <CardContent className="space-y-5">
        <div className="space-y-2">
          <Label>Theme</Label>
          <div className="bg-elev flex gap-1 rounded-lg p-1">
            {themes.map(({ value, label, Icon }) => (
              <SegmentedOption
                key={value}
                active={theme === value}
                onClick={() => setTheme(value)}
              >
                <Icon aria-hidden="true" className="size-4" />
                {label}
              </SegmentedOption>
            ))}
          </div>
        </div>

        <div className="space-y-2">
          <Label>Accent</Label>
          <div className="bg-elev flex gap-1 rounded-lg p-1">
            {accents.map(({ value, label, swatch }) => (
              <SegmentedOption
                key={value}
                active={accent === value}
                onClick={() => setAccent(value)}
              >
                <span
                  aria-hidden="true"
                  className="size-3.5 rounded-full ring-1 ring-black/10 ring-inset"
                  style={{ background: swatch }}
                />
                {label}
              </SegmentedOption>
            ))}
          </div>
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
