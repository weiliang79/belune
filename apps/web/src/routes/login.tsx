import { createFileRoute, useNavigate } from "@tanstack/react-router";
import { useForm } from "@tanstack/react-form";
import { z } from "zod";
import { login, getMe } from "@/lib/api/auth";
import { useAuthStore } from "@/lib/stores/auth";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { useState } from "react";

export const Route = createFileRoute("/login")({
  component: LoginPage,
});

function LoginPage() {
  const navigate = useNavigate();
  const setUser = useAuthStore((s) => s.setUser);
  const [error, setError] = useState("");

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
        setError(e instanceof Error ? e.message : "Login failed");
      }
    },
  });

  return (
    <div className="flex min-h-screen items-center justify-center">
      <Card className="w-full max-w-sm">
        <CardHeader>
          <CardTitle className="text-2xl">Login</CardTitle>
          <CardDescription>Sign in to your PaaS dashboard</CardDescription>
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
                onChange: z.string().min(1, "Password is required"),
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
            <form.Subscribe
              selector={(s) => s.isSubmitting}
              children={(isSubmitting) => (
                <Button
                  type="submit"
                  className="w-full"
                  disabled={isSubmitting}
                >
                  {isSubmitting ? "Signing in..." : "Sign in"}
                </Button>
              )}
            />
          </form>
        </CardContent>
      </Card>
    </div>
  );
}
