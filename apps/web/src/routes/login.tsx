import { createFileRoute, useNavigate, Link } from "@tanstack/react-router";
import { useForm } from "@tanstack/react-form";
import { z } from "zod";
import { login, getMe } from "@/lib/api/auth";
import { ApiError } from "@/lib/api/client";
import { useAuthStore } from "@/lib/stores/auth";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { AuthLayout } from "@/lib/components/layout/auth-layout";
import { MailIcon, LockIcon, EyeIcon, EyeOffIcon } from "lucide-react";
import { useState } from "react";

export const Route = createFileRoute("/login")({
  component: LoginPage,
});

function LoginPage() {
  const navigate = useNavigate();
  const setUser = useAuthStore((s) => s.setUser);
  const [error, setError] = useState("");
  const [showPassword, setShowPassword] = useState(false);

  const form = useForm({
    defaultValues: { email: "", password: "" },
    onSubmit: async ({ value }) => {
      setError("");
      try {
        await login(value.email, value.password);
        const user = await getMe();
        setUser(user);
        navigate({ to: "/dashboard" });
      } catch (e) {
        if (e instanceof ApiError && e.status === 429 && e.retryAfter) {
          const mins = Math.ceil(e.retryAfter / 60);
          setError(
            `Account temporarily locked due to repeated failed login attempts. Try again in ${mins} minute${mins === 1 ? "" : "s"}.`,
          );
          return;
        }
        setError(e instanceof Error ? e.message : "Login failed");
      }
    },
  });

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
