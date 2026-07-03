import { useForm } from "@tanstack/react-form";
import { z } from "zod";
import { toast } from "sonner";
import { TriangleAlert } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Alert, AlertDescription } from "@/components/ui/alert";
import {
  SegmentedControl,
  SegmentedControlItem,
} from "@/components/ui/segmented-control";
import { useUpdateApplication } from "@/lib/hooks/use-applications";
import { useFeatures } from "@/lib/hooks/use-features";
import type { Application } from "@/lib/types";

interface Props {
  projectId: string;
  applicationId: string;
  application: Application;
}

export function ApplicationSettingsForm({
  projectId,
  applicationId,
  application,
}: Props) {
  const updateApplication = useUpdateApplication(projectId, applicationId);
  const { data: features } = useFeatures();

  const form = useForm({
    defaultValues: {
      name: application.name ?? "",
      source_repo: application.source_repo ?? "",
      source_image: application.source_image ?? "",
      dockerfile_path: application.dockerfile_path ?? "",
      build_type_override: application.build_type_override ?? "",
      cpu_limit: application.cpu_limit ?? 0,
      memory_limit_mb: Math.round(application.memory_limit / (1024 * 1024)),
      git_token: "",
      health_check_path: application.health_check_path ?? "",
    },
    onSubmit: async ({ value }) => {
      toast.promise(
        updateApplication.mutateAsync({
          name: value.name || undefined,
          source_repo: value.source_repo || undefined,
          source_image: value.source_image || undefined,
          dockerfile_path: value.dockerfile_path || undefined,
          build_type_override: value.build_type_override || undefined,
          cpu_limit: value.cpu_limit,
          memory_limit:
            value.memory_limit_mb > 0
              ? value.memory_limit_mb * 1024 * 1024
              : 0,
          git_token: value.git_token || undefined,
          health_check_path: value.health_check_path,
        }),
        {
          loading: "Saving...",
          success: "Settings saved",
          error: (err) => err.message,
        },
      );
    },
  });

  return (
    <Card>
      <CardHeader>
        <CardTitle>Application Settings</CardTitle>
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
          <form.Field
            name="name"
            validators={{ onChange: z.string().min(1, "Name is required") }}
            children={(field) => (
              <div className="space-y-2">
                <Label>Application Name</Label>
                <Input
                  value={field.state.value}
                  onBlur={field.handleBlur}
                  onChange={(e) => field.handleChange(e.target.value)}
                />
              </div>
            )}
          />
          <div className="space-y-2">
            <Label>Type</Label>
            <SegmentedControl
              value={application.type}
              onValueChange={() => {}}
              disabled
            >
              <SegmentedControlItem value="image">
                Docker Image
              </SegmentedControlItem>
              <SegmentedControlItem value="git">
                Git Repository
              </SegmentedControlItem>
            </SegmentedControl>
          </div>
          {application.type === "git" && (
            <form.Field
              name="build_type_override"
              children={(field) => {
                const effectiveValue =
                  field.state.value || application.build_type;
                return (
                  <div className="space-y-2">
                    <Label>Build Type</Label>
                    <SegmentedControl
                      value={effectiveValue}
                      onValueChange={(v) =>
                        field.handleChange(
                          v === application.build_type ? "" : v,
                        )
                      }
                    >
                      <SegmentedControlItem value="dockerfile">
                        Dockerfile
                      </SegmentedControlItem>
                      <SegmentedControlItem value="buildpacks">
                        Buildpacks
                      </SegmentedControlItem>
                      <SegmentedControlItem value="railpack">
                        Railpack
                      </SegmentedControlItem>
                    </SegmentedControl>
                    {effectiveValue === "railpack" &&
                      features?.buildkit_available === false && (
                        <Alert variant="destructive">
                          <TriangleAlert />
                          <AlertDescription>
                            BuildKit is not reachable. Railpack builds will
                            fail.
                          </AlertDescription>
                        </Alert>
                      )}
                  </div>
                );
              }}
            />
          )}
          {application.type === "image" && (
            <form.Field
              name="source_image"
              children={(field) => (
                <div className="space-y-2">
                  <Label>Docker Image</Label>
                  <Input
                    value={field.state.value}
                    onBlur={field.handleBlur}
                    onChange={(e) => field.handleChange(e.target.value)}
                  />
                </div>
              )}
            />
          )}
          {application.type === "git" && (
            <>
              <form.Field
                name="source_repo"
                validators={{
                  onChange: z
                    .string()
                    .refine(
                      (v) =>
                        !v || v.startsWith("https://") || v.startsWith("git@"),
                      "URL must start with https:// or git@",
                    ),
                }}
                children={(field) => (
                  <div className="space-y-2">
                    <Label>Repository URL</Label>
                    <Input
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
                name="git_token"
                children={(field) => (
                  <div className="space-y-2">
                    <Label>Private Token (PAT)</Label>
                    <Input
                      type="password"
                      value={field.state.value}
                      onBlur={field.handleBlur}
                      onChange={(e) => field.handleChange(e.target.value)}
                      placeholder="Leave empty to keep existing token"
                      className="font-mono"
                    />
                    <p className="text-muted-foreground text-xs">
                      Per-app token for private repositories. Use a repo-scoped
                      token where possible.
                    </p>
                  </div>
                )}
              />
              {(application.build_type_override ?? application.build_type) ===
                "dockerfile" && (
                <form.Field
                  name="dockerfile_path"
                  children={(field) => (
                    <div className="space-y-2">
                      <Label>Dockerfile Path</Label>
                      <Input
                        value={field.state.value}
                        onBlur={field.handleBlur}
                        onChange={(e) => field.handleChange(e.target.value)}
                      />
                    </div>
                  )}
                />
              )}
            </>
          )}
          <div className="grid grid-cols-2 gap-4">
            <form.Field
              name="cpu_limit"
              children={(field) => (
                <div className="space-y-2">
                  <Label>CPU Limit (cores)</Label>
                  <Input
                    type="number"
                    min="0"
                    step="0.1"
                    value={field.state.value}
                    onBlur={field.handleBlur}
                    onChange={(e) =>
                      field.handleChange(parseFloat(e.target.value) || 0)
                    }
                    placeholder="0 = unlimited"
                  />
                  <p className="text-muted-foreground text-xs">
                    e.g. 0.5 = half a core, 0 = unlimited
                  </p>
                </div>
              )}
            />
            <form.Field
              name="memory_limit_mb"
              children={(field) => (
                <div className="space-y-2">
                  <Label>Memory Limit (MB)</Label>
                  <Input
                    type="number"
                    min="0"
                    step="64"
                    value={field.state.value}
                    onBlur={field.handleBlur}
                    onChange={(e) =>
                      field.handleChange(parseInt(e.target.value) || 0)
                    }
                    placeholder="0 = unlimited"
                  />
                  <p className="text-muted-foreground text-xs">
                    e.g. 512 = 512 MB, 0 = unlimited
                  </p>
                </div>
              )}
            />
          </div>
          <div className="grid grid-cols-2 gap-4">
            <form.Field
              name="health_check_path"
              children={(field) => (
                <div className="space-y-2">
                  <Label>Health Check Path</Label>
                  <Input
                    value={field.state.value}
                    onBlur={field.handleBlur}
                    onChange={(e) => field.handleChange(e.target.value)}
                    placeholder="/healthz"
                    className="font-mono"
                  />
                  <p className="text-muted-foreground text-xs">
                    HTTP path polled after deploy to confirm readiness. Leave
                    empty to skip.
                  </p>
                </div>
              )}
            />
          </div>
          <form.Subscribe
            selector={(s) => s.isSubmitting}
            children={(isSubmitting) => (
              <Button type="submit" disabled={isSubmitting}>
                {isSubmitting ? "Saving..." : "Save Changes"}
              </Button>
            )}
          />
        </form>
      </CardContent>
    </Card>
  );
}
