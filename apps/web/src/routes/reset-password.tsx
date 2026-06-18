import { createFileRoute, Link, useNavigate } from "@tanstack/react-router";
import { useForm } from "@tanstack/react-form";
import { z } from "zod";
import { resetPassword } from "@/lib/api/auth";
import { ApiError } from "@/lib/api/client";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { AuthLayout } from "@/lib/components/layout/auth-layout";
import { useState } from "react";

export const Route = createFileRoute("/reset-password")({
  component: ResetPasswordPage,
  validateSearch: (search: Record<string, unknown>) => ({
    token: typeof search.token === "string" ? search.token : "",
  }),
});

function ResetPasswordPage() {
  const { token } = Route.useSearch();
  const navigate = useNavigate();
  const [error, setError] = useState("");

  const form = useForm({
    defaultValues: { password: "", confirm: "" },
    onSubmit: async ({ value }) => {
      if (value.password !== value.confirm) {
        setError("Passwords do not match");
        return;
      }
      setError("");
      try {
        await resetPassword(token, value.password);
        navigate({ to: "/login" });
      } catch (e) {
        setError(e instanceof ApiError ? e.message : "Reset failed");
      }
    },
  });

  if (!token) {
    return (
      <AuthLayout title="Reset password">
        <div className="bg-destructive/10 text-destructive rounded-md px-3 py-2.5 text-sm">
          Invalid or missing reset token.{" "}
          <Link
            to="/forgot-password"
            className="font-medium underline underline-offset-4"
          >
            Request a new one.
          </Link>
        </div>
      </AuthLayout>
    );
  }

  return (
    <AuthLayout
      title="Reset password"
      description="Choose a new password for your account."
    >
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
          name="password"
          validators={{
            onChange: z.string().min(8, "At least 8 characters required"),
          }}
          children={(field) => (
            <div className="space-y-2">
              <Label htmlFor="password">New password</Label>
              <Input
                id="password"
                type="password"
                placeholder="At least 8 characters"
                value={field.state.value}
                onBlur={field.handleBlur}
                onChange={(e) => field.handleChange(e.target.value)}
              />
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
          name="confirm"
          validators={{
            onChange: z.string().min(1, "Please confirm your password"),
          }}
          children={(field) => (
            <div className="space-y-2">
              <Label htmlFor="confirm">Confirm password</Label>
              <Input
                id="confirm"
                type="password"
                value={field.state.value}
                onBlur={field.handleBlur}
                onChange={(e) => field.handleChange(e.target.value)}
              />
            </div>
          )}
        />
        <form.Subscribe
          selector={(s) => s.isSubmitting}
          children={(isSubmitting) => (
            <Button type="submit" className="w-full" disabled={isSubmitting}>
              {isSubmitting ? "Resetting..." : "Reset password"}
            </Button>
          )}
        />
      </form>
    </AuthLayout>
  );
}
