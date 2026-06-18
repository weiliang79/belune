import { createFileRoute, useNavigate } from "@tanstack/react-router";
import { useForm } from "@tanstack/react-form";
import { z } from "zod";
import { getInvitationByToken, acceptInvitation } from "@/lib/api/invitations";
import { ApiError } from "@/lib/api/client";
import { useAuthStore } from "@/lib/stores/auth";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Badge } from "@/components/ui/badge";
import { AuthLayout } from "@/lib/components/layout/auth-layout";
import { useEffect, useState } from "react";

export const Route = createFileRoute("/accept-invite")({
  component: AcceptInvitePage,
  validateSearch: (search: Record<string, unknown>) => ({
    token: typeof search.token === "string" ? search.token : "",
  }),
});

function AcceptInvitePage() {
  const { token } = Route.useSearch();
  const navigate = useNavigate();
  const setUser = useAuthStore((s) => s.setUser);

  const [inviteInfo, setInviteInfo] = useState<{
    email: string;
    role: string;
  } | null>(null);
  const [lookupError, setLookupError] = useState("");
  const [submitError, setSubmitError] = useState("");

  useEffect(() => {
    if (!token) return;
    getInvitationByToken(token)
      .then(setInviteInfo)
      .catch((e) => {
        setLookupError(
          e instanceof ApiError ? e.message : "Invalid or expired invitation",
        );
      });
  }, [token]);

  const form = useForm({
    defaultValues: { password: "", username: "", first_name: "", last_name: "" },
    onSubmit: async ({ value }) => {
      setSubmitError("");
      try {
        const result = await acceptInvitation({
          token,
          password: value.password,
          username: value.username || undefined,
          first_name: value.first_name || undefined,
          last_name: value.last_name || undefined,
        });
        setUser(result.user);
        navigate({ to: "/dashboard" });
      } catch (e) {
        setSubmitError(
          e instanceof ApiError ? e.message : "Failed to accept invitation",
        );
      }
    },
  });

  if (!token || lookupError) {
    return (
      <AuthLayout title="Invitation">
        <div className="bg-destructive/10 text-destructive rounded-md px-3 py-2.5 text-sm">
          {lookupError || "Invalid invitation link."}
        </div>
      </AuthLayout>
    );
  }

  if (!inviteInfo) {
    return (
      <AuthLayout title="Accept invitation">
        <p className="text-muted-foreground text-sm">Verifying invitation…</p>
      </AuthLayout>
    );
  }

  return (
    <AuthLayout title="Accept invitation">
      <div className="bg-card mb-6 flex items-center justify-between gap-2 rounded-lg border px-3 py-2.5">
        <div className="min-w-0">
          <p className="text-text-faint text-xs">Joining as</p>
          <p className="truncate font-mono text-sm">{inviteInfo.email}</p>
        </div>
        <Badge variant="secondary" className="capitalize">
          {inviteInfo.role}
        </Badge>
      </div>
      <form
        onSubmit={(e) => {
          e.preventDefault();
          e.stopPropagation();
          form.handleSubmit();
        }}
        className="space-y-4"
      >
            {submitError && (
              <div className="bg-destructive/10 text-destructive rounded-md px-3 py-2 text-sm">
                {submitError}
              </div>
            )}
            <form.Field
              name="password"
              validators={{
                onChange: z.string().min(8, "At least 8 characters required"),
              }}
              children={(field) => (
                <div className="space-y-2">
                  <Label htmlFor="password">Password</Label>
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
              name="username"
              children={(field) => (
                <div className="space-y-2">
                  <Label htmlFor="username">Username (optional)</Label>
                  <Input
                    id="username"
                    value={field.state.value}
                    onBlur={field.handleBlur}
                    onChange={(e) => field.handleChange(e.target.value)}
                    placeholder="username"
                  />
                </div>
              )}
            />
            <div className="grid grid-cols-2 gap-3">
              <form.Field
                name="first_name"
                children={(field) => (
                  <div className="space-y-2">
                    <Label htmlFor="first_name">First name</Label>
                    <Input
                      id="first_name"
                      value={field.state.value}
                      onBlur={field.handleBlur}
                      onChange={(e) => field.handleChange(e.target.value)}
                      placeholder="First"
                    />
                  </div>
                )}
              />
              <form.Field
                name="last_name"
                children={(field) => (
                  <div className="space-y-2">
                    <Label htmlFor="last_name">Last name</Label>
                    <Input
                      id="last_name"
                      value={field.state.value}
                      onBlur={field.handleBlur}
                      onChange={(e) => field.handleChange(e.target.value)}
                      placeholder="Last"
                    />
                  </div>
                )}
              />
            </div>
            <form.Subscribe
              selector={(s) => s.isSubmitting}
              children={(isSubmitting) => (
                <Button type="submit" className="w-full" disabled={isSubmitting}>
                  {isSubmitting ? "Creating account…" : "Create account"}
                </Button>
              )}
        />
      </form>
    </AuthLayout>
  );
}
