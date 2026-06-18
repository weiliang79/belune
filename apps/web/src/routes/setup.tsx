import { createFileRoute, useNavigate } from "@tanstack/react-router";
import { useForm } from "@tanstack/react-form";
import { z } from "zod";
import { setup, login, getMe } from "@/lib/api/auth";
import { useAuthStore } from "@/lib/stores/auth";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { AuthLayout } from "@/lib/components/layout/auth-layout";
import { useState } from "react";

export const Route = createFileRoute("/setup")({
  component: SetupPage,
});

function SetupPage() {
  const navigate = useNavigate();
  const setUser = useAuthStore((s) => s.setUser);
  const [error, setError] = useState("");

  const form = useForm({
    defaultValues: { email: "", password: "", confirmPassword: "", username: "" },
    onSubmit: async ({ value }) => {
      setError("");
      if (value.password !== value.confirmPassword) {
        setError("Passwords do not match");
        return;
      }
      try {
        await setup(value.email, value.password, value.username);
        await login(value.email, value.password);
        const user = await getMe();
        setUser(user);
        navigate({ to: "/dashboard" });
      } catch (e) {
        setError(e instanceof Error ? e.message : "Setup failed");
      }
    },
  });

  return (
    <AuthLayout
      title="Create admin account"
      description="Set up the first administrator to get started."
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
              name="username"
              validators={{
                onChange: z.string().min(1, "Username is required"),
              }}
              children={(field) => (
                <div className="space-y-2">
                  <Label htmlFor="username">Username</Label>
                  <Input
                    id="username"
                    type="text"
                    placeholder="admin"
                    value={field.state.value}
                    onBlur={field.handleBlur}
                    onChange={(e) => field.handleChange(e.target.value)}
                  />
                  {field.state.meta.errors.length > 0 && (
                    <p className="text-destructive text-sm">
                      {typeof field.state.meta.errors[0] === 'string' ? field.state.meta.errors[0] : field.state.meta.errors[0]?.message}
                    </p>
                  )}
                </div>
              )}
            />
            <form.Field
              name="email"
              validators={{
                onChange: z.string().email("Valid email required"),
              }}
              children={(field) => (
                <div className="space-y-2">
                  <Label htmlFor="email">Email</Label>
                  <Input
                    id="email"
                    type="email"
                    placeholder="admin@example.com"
                    value={field.state.value}
                    onBlur={field.handleBlur}
                    onChange={(e) => field.handleChange(e.target.value)}
                  />
                  {field.state.meta.errors.length > 0 && (
                    <p className="text-destructive text-sm">
                      {typeof field.state.meta.errors[0] === 'string' ? field.state.meta.errors[0] : field.state.meta.errors[0]?.message}
                    </p>
                  )}
                </div>
              )}
            />
            <form.Field
              name="password"
              validators={{
                onChange: z
                  .string()
                  .min(8, "Password must be at least 8 characters"),
              }}
              children={(field) => (
                <div className="space-y-2">
                  <Label htmlFor="password">Password</Label>
                  <Input
                    id="password"
                    type="password"
                    value={field.state.value}
                    onBlur={field.handleBlur}
                    onChange={(e) => field.handleChange(e.target.value)}
                  />
                  {field.state.meta.errors.length > 0 && (
                    <p className="text-destructive text-sm">
                      {typeof field.state.meta.errors[0] === 'string' ? field.state.meta.errors[0] : field.state.meta.errors[0]?.message}
                    </p>
                  )}
                </div>
              )}
            />
            <form.Field
              name="confirmPassword"
              validators={{
                onChange: z.string().min(1, "Please confirm your password"),
              }}
              children={(field) => (
                <div className="space-y-2">
                  <Label htmlFor="confirmPassword">Confirm Password</Label>
                  <Input
                    id="confirmPassword"
                    type="password"
                    value={field.state.value}
                    onBlur={field.handleBlur}
                    onChange={(e) => field.handleChange(e.target.value)}
                  />
                  {field.state.meta.errors.length > 0 && (
                    <p className="text-destructive text-sm">
                      {typeof field.state.meta.errors[0] === 'string' ? field.state.meta.errors[0] : field.state.meta.errors[0]?.message}
                    </p>
                  )}
                </div>
              )}
            />
            <form.Subscribe
              selector={(s) => s.isSubmitting}
              children={(isSubmitting) => (
                <Button
                  type="submit"
                  className="w-full"
                  disabled={isSubmitting}
                >
                  {isSubmitting
                    ? "Creating account..."
                    : "Create admin account"}
                </Button>
              )}
        />
      </form>
    </AuthLayout>
  );
}
