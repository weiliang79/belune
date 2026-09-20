import { useForm } from "@tanstack/react-form";
import { toast } from "sonner";
import { z } from "zod";
import { HeartPulse } from "lucide-react";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  SegmentedControl,
  SegmentedControlItem,
} from "@/components/ui/segmented-control";
import { useSetHealthCheck } from "@/lib/hooks/use-applications";
import type { Application } from "@/lib/types";

/**
 * Configures how the application's health is checked. Three mechanisms:
 *
 *  - None — no check.
 *  - HTTP — the control plane probes a path once after each deploy. Simple, and
 *    needs nothing installed in the image, but only works for HTTP services.
 *  - Command — a native Docker HEALTHCHECK run inside the container, continuously.
 *    Works for anything (a database, a queue, a worker), and because it keeps
 *    running, the container's health drives the application's status: a failing
 *    check shows the app as Unhealthy, not Running.
 *
 * A change takes effect on the next deploy (or reload) — the check is part of
 * the container, so the running one keeps its old check until then.
 */
type HealthType = "none" | "http" | "command";

// A blank numeric input means "use the default"; it is sent as undefined so the
// server stores NULL and the platform default applies.
function numOrUndefined(s: string): number | undefined {
  const n = parseInt(s, 10);
  return Number.isFinite(n) && n > 0 ? n : undefined;
}

function fieldError(errors: unknown[]): string | undefined {
  const first = errors[0];
  if (!first) return undefined;
  return typeof first === "string"
    ? first
    : (first as { message?: string }).message;
}

const positiveNumber = z
  .string()
  .refine(
    (v) => v === "" || (/^\d+$/.test(v) && Number(v) > 0),
    "Must be a positive whole number",
  );

export function HealthCheckSection({
  projectId,
  applicationId,
  application,
}: {
  projectId: string;
  applicationId: string;
  application: Application;
}) {
  const setHealthCheck = useSetHealthCheck(projectId, applicationId);

  const form = useForm({
    defaultValues: {
      type: application.health_check_type as HealthType,
      path: application.health_check_path ?? "",
      expectStatus: application.health_check_expect_status?.toString() ?? "",
      command: application.health_check_command ?? "",
      interval: application.health_check_interval_seconds?.toString() ?? "",
      retries: application.health_check_retries?.toString() ?? "",
      startPeriod:
        application.health_check_start_period_seconds?.toString() ?? "",
      timeout: application.health_check_timeout_seconds?.toString() ?? "",
    },
    onSubmit: ({ value }) => {
      const data =
        value.type === "none"
          ? { type: "none" as const }
          : value.type === "http"
            ? {
                type: "http" as const,
                path: value.path.trim(),
                expect_status: numOrUndefined(value.expectStatus),
                timeout_seconds: numOrUndefined(value.timeout),
              }
            : {
                type: "command" as const,
                command: value.command.trim(),
                interval_seconds: numOrUndefined(value.interval),
                retries: numOrUndefined(value.retries),
                start_period_seconds: numOrUndefined(value.startPeriod),
                timeout_seconds: numOrUndefined(value.timeout),
              };

      toast.promise(setHealthCheck.mutateAsync(data), {
        loading: "Saving...",
        success: "Health check saved — applies on the next deploy",
        error: (err) => err.message,
      });
    },
  });

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <HeartPulse aria-hidden="true" className="size-4" />
          Health Check
        </CardTitle>
        <CardDescription>
          How the platform decides the application is healthy. A command check
          runs continuously inside the container and marks the app Unhealthy when
          it fails; an HTTP check is probed once after each deploy. Changes apply
          on the next deploy.
        </CardDescription>
      </CardHeader>
      <form
        onSubmit={(e) => {
          e.preventDefault();
          e.stopPropagation();
          form.handleSubmit();
        }}
      >
        <CardContent className="space-y-4">
          <form.Field
            name="type"
            children={(field) => (
              <div className="space-y-2">
                <Label>Method</Label>
                <SegmentedControl
                  value={field.state.value}
                  onValueChange={(v) => field.handleChange(v as HealthType)}
                >
                  <SegmentedControlItem value="none">None</SegmentedControlItem>
                  <SegmentedControlItem value="http">HTTP</SegmentedControlItem>
                  <SegmentedControlItem value="command">
                    Command
                  </SegmentedControlItem>
                </SegmentedControl>
              </div>
            )}
          />

          <form.Subscribe
            selector={(s) => s.values.type}
            children={(type) => (
              <>
                {type === "http" && (
                  <div className="space-y-4">
                    <form.Field
                      name="path"
                      validators={{
                        onChangeListenTo: ["type"],
                        onChange: ({ value, fieldApi }) =>
                          fieldApi.form.state.values.type === "http" &&
                          value.trim() === ""
                            ? "Path is required"
                            : undefined,
                      }}
                      children={(field) => {
                        const error = fieldError(field.state.meta.errors);
                        return (
                          <div className="space-y-2">
                            <Label>Path</Label>
                            <Input
                              value={field.state.value}
                              onBlur={field.handleBlur}
                              onChange={(e) =>
                                field.handleChange(e.target.value)
                              }
                              placeholder="/healthz"
                              className="font-mono"
                            />
                            {error ? (
                              <p className="text-destructive text-xs">
                                {error}
                              </p>
                            ) : (
                              <p className="text-muted-foreground text-xs">
                                Probed on the container's port after each
                                deploy. A non-2xx response (or the code below)
                                fails the deploy.
                              </p>
                            )}
                          </div>
                        );
                      }}
                    />
                    <div className="grid grid-cols-2 gap-4">
                      <form.Field
                        name="expectStatus"
                        children={(field) => (
                          <div className="space-y-2">
                            <Label>Expected status</Label>
                            <Input
                              type="number"
                              value={field.state.value}
                              onBlur={field.handleBlur}
                              onChange={(e) =>
                                field.handleChange(e.target.value)
                              }
                              placeholder="any 2xx"
                            />
                          </div>
                        )}
                      />
                      <form.Field
                        name="timeout"
                        validators={{ onChange: positiveNumber }}
                        children={(field) => {
                          const error = fieldError(field.state.meta.errors);
                          return (
                            <div className="space-y-2">
                              <Label>Timeout (seconds)</Label>
                              <Input
                                type="number"
                                value={field.state.value}
                                onBlur={field.handleBlur}
                                onChange={(e) =>
                                  field.handleChange(e.target.value)
                                }
                                placeholder="120"
                              />
                              {error && (
                                <p className="text-destructive text-xs">
                                  {error}
                                </p>
                              )}
                            </div>
                          );
                        }}
                      />
                    </div>
                  </div>
                )}

                {type === "command" && (
                  <div className="space-y-4">
                    <form.Field
                      name="command"
                      validators={{
                        onChangeListenTo: ["type"],
                        onChange: ({ value, fieldApi }) =>
                          fieldApi.form.state.values.type === "command" &&
                          value.trim() === ""
                            ? "Command is required"
                            : undefined,
                      }}
                      children={(field) => {
                        const error = fieldError(field.state.meta.errors);
                        return (
                          <div className="space-y-2">
                            <Label>Command</Label>
                            <Input
                              value={field.state.value}
                              onBlur={field.handleBlur}
                              onChange={(e) =>
                                field.handleChange(e.target.value)
                              }
                              placeholder="curl -f http://localhost:3000/health || exit 1"
                              className="font-mono"
                            />
                            {error ? (
                              <p className="text-destructive text-xs">
                                {error}
                              </p>
                            ) : (
                              <p className="text-muted-foreground text-xs">
                                Run inside the container via{" "}
                                <code>sh -c</code>. Exit 0 = healthy. The tool
                                you use (curl, wget, pg_isready…) must exist in
                                the image.
                              </p>
                            )}
                          </div>
                        );
                      }}
                    />
                    <div className="grid grid-cols-2 gap-4">
                      <form.Field
                        name="interval"
                        validators={{ onChange: positiveNumber }}
                        children={(field) => {
                          const error = fieldError(field.state.meta.errors);
                          return (
                            <div className="space-y-2">
                              <Label>Interval (seconds)</Label>
                              <Input
                                type="number"
                                value={field.state.value}
                                onBlur={field.handleBlur}
                                onChange={(e) =>
                                  field.handleChange(e.target.value)
                                }
                                placeholder="30"
                              />
                              {error && (
                                <p className="text-destructive text-xs">
                                  {error}
                                </p>
                              )}
                            </div>
                          );
                        }}
                      />
                      <form.Field
                        name="timeout"
                        validators={{ onChange: positiveNumber }}
                        children={(field) => {
                          const error = fieldError(field.state.meta.errors);
                          return (
                            <div className="space-y-2">
                              <Label>Timeout (seconds)</Label>
                              <Input
                                type="number"
                                value={field.state.value}
                                onBlur={field.handleBlur}
                                onChange={(e) =>
                                  field.handleChange(e.target.value)
                                }
                                placeholder="30"
                              />
                              {error && (
                                <p className="text-destructive text-xs">
                                  {error}
                                </p>
                              )}
                            </div>
                          );
                        }}
                      />
                      <form.Field
                        name="retries"
                        validators={{ onChange: positiveNumber }}
                        children={(field) => {
                          const error = fieldError(field.state.meta.errors);
                          return (
                            <div className="space-y-2">
                              <Label>Retries</Label>
                              <Input
                                type="number"
                                value={field.state.value}
                                onBlur={field.handleBlur}
                                onChange={(e) =>
                                  field.handleChange(e.target.value)
                                }
                                placeholder="3"
                              />
                              {error ? (
                                <p className="text-destructive text-xs">
                                  {error}
                                </p>
                              ) : (
                                <p className="text-muted-foreground text-xs">
                                  Consecutive failures before Unhealthy.
                                </p>
                              )}
                            </div>
                          );
                        }}
                      />
                      <form.Field
                        name="startPeriod"
                        validators={{ onChange: positiveNumber }}
                        children={(field) => {
                          const error = fieldError(field.state.meta.errors);
                          return (
                            <div className="space-y-2">
                              <Label>Start period (seconds)</Label>
                              <Input
                                type="number"
                                value={field.state.value}
                                onBlur={field.handleBlur}
                                onChange={(e) =>
                                  field.handleChange(e.target.value)
                                }
                                placeholder="0"
                              />
                              {error ? (
                                <p className="text-destructive text-xs">
                                  {error}
                                </p>
                              ) : (
                                <p className="text-muted-foreground text-xs">
                                  Grace window at startup where failures don't
                                  count.
                                </p>
                              )}
                            </div>
                          );
                        }}
                      />
                    </div>
                  </div>
                )}
              </>
            )}
          />

          <div className="flex justify-end">
            <form.Subscribe
              selector={(s) => [
                s.values.type,
                s.values.path,
                s.values.command,
              ] as const}
              children={([type, path, command]) => {
                const canSave =
                  type === "none" ||
                  (type === "http" && path.trim() !== "") ||
                  (type === "command" && command.trim() !== "");
                return (
                  <Button
                    type="submit"
                    disabled={!canSave || setHealthCheck.isPending}
                  >
                    {setHealthCheck.isPending ? "Saving..." : "Save"}
                  </Button>
                );
              }}
            />
          </div>
        </CardContent>
      </form>
    </Card>
  );
}
