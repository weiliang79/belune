import { useForm } from "@tanstack/react-form";
import { toast } from "sonner";
import { z } from "zod";
import { Cpu } from "lucide-react";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Field, FieldError, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { PendingChangeBadge } from "@/lib/components/pending-change-badge";
import { useSetResources } from "@/lib/hooks/use-applications";
import type { Application } from "@/lib/types";

function fieldError(errors: unknown[]): string | undefined {
  const first = errors[0];
  if (!first) return undefined;
  return typeof first === "string"
    ? first
    : (first as { message?: string }).message;
}

const nonNegative = z
  .string()
  .refine((v) => v === "" || Number(v) >= 0, "Cannot be negative");

/**
 * CPU and memory limits, split out of the general settings form. Limits are
 * applied when the container is next created, so saving stamps the config
 * marker and the header badge points at Reload — the same as the other
 * container-shaping fields.
 */
export function ResourcesSection({
  projectId,
  applicationId,
  application,
}: {
  projectId: string;
  applicationId: string;
  application: Application;
}) {
  const setResources = useSetResources(projectId, applicationId);

  const form = useForm({
    defaultValues: {
      cpu: application.cpu_limit?.toString() ?? "0",
      memoryMb: Math.round(application.memory_limit / (1024 * 1024)).toString(),
    },
    onSubmit: ({ value }) => {
      const cpuLimit = parseFloat(value.cpu) || 0;
      const memMb = parseInt(value.memoryMb, 10) || 0;
      toast.promise(
        setResources.mutateAsync({
          cpu_limit: cpuLimit,
          memory_limit: memMb > 0 ? memMb * 1024 * 1024 : 0,
        }),
        {
          loading: "Saving...",
          success: "Resource limits saved",
          error: (err) => err.message,
        },
      );
    },
  });

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <Cpu aria-hidden="true" className="size-4" />
          Resources
        </CardTitle>
        <CardDescription>
          CPU and memory ceilings for the container. Leave a field at 0 for no
          limit. Applied when the container is next recreated.
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
          <div className="grid grid-cols-2 gap-4">
            <form.Field
              name="cpu"
              validators={{ onChange: nonNegative }}
              children={(field) => {
                const error = fieldError(field.state.meta.errors);
                return (
                  <Field data-invalid={!!error}>
                    <FieldLabel htmlFor="cpu-limit">
                      CPU Limit (cores)
                    </FieldLabel>
                    <Input
                      id="cpu-limit"
                      type="number"
                      min="0"
                      step="0.1"
                      value={field.state.value}
                      onBlur={field.handleBlur}
                      onChange={(e) => field.handleChange(e.target.value)}
                      placeholder="0 = unlimited"
                      aria-invalid={!!error}
                    />
                    {error ? (
                      <FieldError>{error}</FieldError>
                    ) : (
                      <p className="text-muted-foreground text-xs">
                        e.g. 0.5 = half a core, 0 = unlimited
                      </p>
                    )}
                  </Field>
                );
              }}
            />
            <form.Field
              name="memoryMb"
              validators={{ onChange: nonNegative }}
              children={(field) => {
                const error = fieldError(field.state.meta.errors);
                return (
                  <Field data-invalid={!!error}>
                    <FieldLabel htmlFor="memory-limit">
                      Memory Limit (MB)
                    </FieldLabel>
                    <Input
                      id="memory-limit"
                      type="number"
                      min="0"
                      step="64"
                      value={field.state.value}
                      onBlur={field.handleBlur}
                      onChange={(e) => field.handleChange(e.target.value)}
                      placeholder="0 = unlimited"
                      aria-invalid={!!error}
                    />
                    {error ? (
                      <FieldError>{error}</FieldError>
                    ) : (
                      <p className="text-muted-foreground text-xs">
                        e.g. 512 = 512 MB, 0 = unlimited
                      </p>
                    )}
                  </Field>
                );
              }}
            />
          </div>
          <div className="flex items-center justify-end gap-3">
            <PendingChangeBadge app={application} className="mr-auto" />
            <Button type="submit" disabled={setResources.isPending}>
              {setResources.isPending ? "Saving..." : "Save"}
            </Button>
          </div>
        </CardContent>
      </form>
    </Card>
  );
}
