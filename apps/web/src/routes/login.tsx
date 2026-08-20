import { createFileRoute, useNavigate, Link } from "@tanstack/react-router";
import { useForm } from "@tanstack/react-form";
import { z } from "zod";
import { login, verifyLogin, getMe } from "@/lib/api/auth";
import { ApiError } from "@/lib/api/client";
import { useAuthStore } from "@/lib/stores/auth";
import { safeRedirectPath } from "@/lib/utils/redirect";
import { redirectIfAuthenticated } from "@/lib/utils/auth-guard";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { AuthLayout } from "@/lib/components/layout/auth-layout";
import {
  MailIcon,
  LockIcon,
  EyeIcon,
  EyeOffIcon,
  ShieldCheckIcon,
} from "lucide-react";
import { useState } from "react";

export const Route = createFileRoute("/login")({
  component: LoginPage,
  validateSearch: (search: Record<string, unknown>): { redirect?: string } => {
    const redirect = safeRedirectPath(search.redirect);
    return redirect ? { redirect } : {};
  },
  // Already signed in? Don't show a login form that would just re-authenticate —
  // send the user on, honouring a carried redirect target (validated above) or
  // the project list. The root route skips its auth check on /login, so this
  // must probe the session itself rather than read the (empty) store.
  beforeLoad: ({ search }) =>
    redirectIfAuthenticated(search.redirect ?? "/projects"),
});

/** A pending second factor: the password was accepted, and this is what is left
 *  to do. Held in state rather than a route, so a challenge is never in a URL
 *  that could be shared, bookmarked or logged. */
interface PendingChallenge {
  challenge: string;
  methods: string[];
}

function LoginPage() {
  const navigate = useNavigate();
  const { redirect } = Route.useSearch();
  const setUser = useAuthStore((s) => s.setUser);
  const [error, setError] = useState("");
  const [showPassword, setShowPassword] = useState(false);
  const [pending, setPending] = useState<PendingChallenge | null>(null);

  const finish = async () => {
    const user = await getMe();
    setUser(user);
    // Return the user to wherever they were headed (validated same-origin
    // path); default to Projects. The dead /dashboard hop is gone.
    navigate({ to: (redirect ?? "/projects") as never });
  };

  const describeError = (e: unknown) => {
    if (e instanceof ApiError && e.status === 429 && e.retryAfter) {
      const mins = Math.ceil(e.retryAfter / 60);
      return `Account temporarily locked due to repeated failed login attempts. Try again in ${mins} minute${mins === 1 ? "" : "s"}.`;
    }
    return e instanceof Error ? e.message : "Login failed";
  };

  const form = useForm({
    defaultValues: { email: "", password: "" },
    onSubmit: async ({ value }) => {
      setError("");
      try {
        const result = await login(value.email, value.password);
        // No session yet: this account needs a second factor.
        if (result.challenge) {
          setPending({ challenge: result.challenge, methods: result.methods });
          return;
        }
        await finish();
      } catch (e) {
        setError(describeError(e));
      }
    },
  });

  if (pending) {
    return (
      <SecondFactorStep
        pending={pending}
        onCancel={() => {
          setPending(null);
          setError("");
        }}
        onVerified={finish}
        describeError={describeError}
      />
    );
  }

  return (
    <AuthLayout title="Welcome back" description="Sign in to your dashboard.">
      <form
        onSubmit={(e) => {
          e.preventDefault();
          e.stopPropagation();
          form.handleSubmit();
        }}
        className="space-y-4"
      >
        {error && (
          <div className="bg-destructive/10 text-destructive rounded-md px-3 py-2 text-sm">
            {error}
          </div>
        )}
        <form.Field
          name="email"
          validators={{
            onChange: z.string().email("Valid email required"),
          }}
          children={(field) => (
            <div className="space-y-2">
              <Label htmlFor="email">Email</Label>
              <div className="relative">
                <MailIcon
                  aria-hidden="true"
                  className="text-text-faint pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2"
                />
                <Input
                  id="email"
                  type="email"
                  placeholder="admin@example.com"
                  className="pl-9"
                  value={field.state.value}
                  onBlur={field.handleBlur}
                  onChange={(e) => field.handleChange(e.target.value)}
                />
              </div>
              {field.state.meta.errors.length > 0 && (
                <p className="text-destructive text-sm">
                  {typeof field.state.meta.errors[0] === "string"
                    ? field.state.meta.errors[0]
                    : field.state.meta.errors[0]?.message}
                </p>
              )}
            </div>
          )}
        />
        <form.Field
          name="password"
          validators={{
            onChange: z.string().min(1, "Password is required"),
          }}
          children={(field) => (
            <div className="space-y-2">
              <Label htmlFor="password">Password</Label>
              <div className="relative">
                <LockIcon
                  aria-hidden="true"
                  className="text-text-faint pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2"
                />
                <Input
                  id="password"
                  type={showPassword ? "text" : "password"}
                  className="px-9"
                  value={field.state.value}
                  onBlur={field.handleBlur}
                  onChange={(e) => field.handleChange(e.target.value)}
                />
                <button
                  type="button"
                  onClick={() => setShowPassword((s) => !s)}
                  aria-label={showPassword ? "Hide password" : "Show password"}
                  className="text-text-faint hover:text-foreground absolute top-1/2 right-3 -translate-y-1/2"
                >
                  {showPassword ? (
                    <EyeOffIcon aria-hidden="true" className="size-4" />
                  ) : (
                    <EyeIcon aria-hidden="true" className="size-4" />
                  )}
                </button>
              </div>
              {field.state.meta.errors.length > 0 && (
                <p className="text-destructive text-sm">
                  {typeof field.state.meta.errors[0] === "string"
                    ? field.state.meta.errors[0]
                    : field.state.meta.errors[0]?.message}
                </p>
              )}
            </div>
          )}
        />
        <form.Subscribe
          selector={(s) => s.isSubmitting}
          children={(isSubmitting) => (
            <Button type="submit" className="w-full" disabled={isSubmitting}>
              {isSubmitting ? "Signing in..." : "Sign in"}
            </Button>
          )}
        />
        <div className="text-center">
          <Link
            to="/forgot-password"
            className="text-muted-foreground hover:text-foreground text-sm underline-offset-4 hover:underline"
          >
            Forgot password?
          </Link>
        </div>
        <p className="text-text-faint text-center text-xs">
          Single-tenant install — accounts are managed by your administrator.
        </p>
      </form>
    </AuthLayout>
  );
}

function SecondFactorStep({
  pending,
  onCancel,
  onVerified,
  describeError,
}: {
  pending: PendingChallenge;
  onCancel: () => void;
  onVerified: () => Promise<void>;
  describeError: (e: unknown) => string;
}) {
  const [code, setCode] = useState("");
  const [error, setError] = useState("");
  const [submitting, setSubmitting] = useState(false);
  // Recovery codes are just another method on the same endpoint, so switching
  // is a change of one field rather than a different flow.
  const canUseRecoveryCode = pending.methods.includes("recovery_code");
  const [useRecoveryCode, setUseRecoveryCode] = useState(false);
  const method = useRecoveryCode ? "recovery_code" : "totp";

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError("");
    setSubmitting(true);
    try {
      await verifyLogin(pending.challenge, method, code);
      await onVerified();
    } catch (err) {
      setError(describeError(err));
      setCode("");
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <AuthLayout
      title="Two-factor authentication"
      description={
        useRecoveryCode
          ? "Enter one of your recovery codes."
          : "Enter the code from your authenticator app."
      }
    >
      <form onSubmit={submit} className="space-y-4">
        {error && (
          <div className="bg-destructive/10 text-destructive rounded-md px-3 py-2 text-sm">
            {error}
          </div>
        )}
        <div className="space-y-2">
          <Label htmlFor="code">
            {useRecoveryCode ? "Recovery code" : "Verification code"}
          </Label>
          <div className="relative">
            <ShieldCheckIcon
              aria-hidden="true"
              className="text-text-faint pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2"
            />
            <Input
              id="code"
              autoFocus
              autoComplete="one-time-code"
              inputMode={useRecoveryCode ? "text" : "numeric"}
              placeholder={useRecoveryCode ? "XXXX-XXXX-XXXX-XXXX" : "123456"}
              className="pl-9"
              value={code}
              onChange={(e) => setCode(e.target.value)}
            />
          </div>
        </div>
        <Button type="submit" className="w-full" disabled={submitting || !code}>
          {submitting ? "Verifying..." : "Verify"}
        </Button>
        {canUseRecoveryCode && (
          <button
            type="button"
            onClick={() => {
              setUseRecoveryCode((s) => !s);
              setCode("");
              setError("");
            }}
            className="text-muted-foreground hover:text-foreground w-full text-center text-sm underline-offset-4 hover:underline"
          >
            {useRecoveryCode
              ? "Use your authenticator app instead"
              : "Lost your device? Use a recovery code"}
          </button>
        )}
        <button
          type="button"
          onClick={onCancel}
          className="text-text-faint hover:text-foreground w-full text-center text-xs underline-offset-4 hover:underline"
        >
          Back to sign in
        </button>
      </form>
    </AuthLayout>
  );
}
