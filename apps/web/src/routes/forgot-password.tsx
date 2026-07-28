import { createFileRoute, Link } from "@tanstack/react-router";
import { useForm } from "@tanstack/react-form";
import { z } from "zod";
import { forgotPassword } from "@/lib/api/auth";
import { redirectIfAuthenticated } from "@/lib/utils/auth-guard";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { AuthLayout } from "@/lib/components/layout/auth-layout";
import { useState } from "react";

export const Route = createFileRoute("/forgot-password")({
  component: ForgotPasswordPage,
  // Signed-in users don't reset from here (they use the profile page); bounce
  // them to the dashboard instead of showing the logged-out reset form.
  beforeLoad: () => redirectIfAuthenticated("/projects"),
});

function ForgotPasswordPage() {
  const [submitted, setSubmitted] = useState(false);
  const [submitError, setSubmitError] = useState("");

  const form = useForm({
    defaultValues: { email: "" },
    onSubmit: async ({ value }) => {
      try {
        await forgotPassword(value.email);
        setSubmitted(true);
      } catch {
        setSubmitError("Something went wrong. Please try again.");
      }
    },
  });

  return (
    <AuthLayout
      title="Forgot password"
      description="Enter your email and we'll send you a reset link."
    >
      {submitted ? (
        <div className="space-y-4">
          <div className="bg-status-ready-soft text-status-ready ring-status-ready-line rounded-md px-3 py-2.5 text-sm ring-1 ring-inset">
            If that email is registered, a reset link has been sent. Check your
            inbox.
          </div>
          <Link
            to="/login"
            className="text-muted-foreground hover:text-foreground text-sm underline-offset-4 hover:underline"
          >
            Back to login
          </Link>
        </div>
      ) : (
        <form
          onSubmit={(e) => {
            e.preventDefault();
            e.stopPropagation();
            form.handleSubmit();
          }}
          className="space-y-4"
        >
          <form.Field
            name="email"
            validators={{ onChange: z.string().email("Valid email required") }}
            children={(field) => (
              <div className="space-y-2">
                <Label htmlFor="email">Email</Label>
                <Input
                  id="email"
                  type="email"
                  placeholder="you@example.com"
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
          {submitError && (
            <p className="text-destructive text-sm">{submitError}</p>
          )}
          <form.Subscribe
            selector={(s) => s.isSubmitting}
            children={(isSubmitting) => (
              <Button type="submit" className="w-full" disabled={isSubmitting}>
                {isSubmitting ? "Sending..." : "Send reset link"}
              </Button>
            )}
          />
          <div className="text-center">
            <Link
              to="/login"
              className="text-muted-foreground hover:text-foreground text-sm underline-offset-4 hover:underline"
            >
              Back to login
            </Link>
          </div>
        </form>
      )}
    </AuthLayout>
  );
}
