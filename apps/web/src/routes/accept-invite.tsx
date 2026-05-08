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
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
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
      <div className="flex min-h-screen items-center justify-center">
        <Card className="w-full max-w-sm">
          <CardContent className="pt-6">
            <p className="text-destructive text-sm">
              {lookupError || "Invalid invitation link."}
            </p>
          </CardContent>
        </Card>
      </div>
    );
  }

  if (!inviteInfo) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <p className="text-muted-foreground text-sm">Verifying invitation…</p>
      </div>
    );
  }

  return (
    <div className="flex min-h-screen items-center justify-center">
      <Card className="w-full max-w-sm">
        <CardHeader>
          <CardTitle className="text-2xl">Accept invitation</CardTitle>
          <CardDescription>
            You've been invited to join as{" "}
            <Badge variant="secondary" className="ml-1">
              {inviteInfo.role}
            </Badge>
          </CardDescription>
        </CardHeader>
        <CardContent>
          <div className="mb-4 text-sm">
            <span className="text-muted-foreground">Email: </span>
            <span>{inviteInfo.email}</span>
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
        </CardContent>
      </Card>
    </div>
  );
}
