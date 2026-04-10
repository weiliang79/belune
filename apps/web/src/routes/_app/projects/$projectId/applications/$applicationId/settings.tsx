import { createFileRoute, useNavigate } from "@tanstack/react-router";
import {
  useApplication,
  useUpdateApplication,
  useDeleteApplication,
  useUpdateWebhook,
} from "@/lib/hooks/use-applications";
import { useForm } from "@tanstack/react-form";
import { z } from "zod";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from "@/components/ui/alert-dialog";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Separator } from "@/components/ui/separator";
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { useFeatures } from "@/lib/hooks/use-features";
import { useGitCredentials } from "@/lib/hooks/use-git-credentials";
import { toast } from "sonner";
import { TriangleAlert } from "lucide-react";
import { useCallback } from "react";

export const Route = createFileRoute(
  "/_app/projects/$projectId/applications/$applicationId/settings",
)({
  component: ApplicationSettingsPage,
});

function ApplicationSettingsPage() {
  const { projectId, applicationId } = Route.useParams();
  const navigate = useNavigate();
  const { data: application } = useApplication(projectId, applicationId);
  const updateApplication = useUpdateApplication(projectId, applicationId);
  const updateWebhook = useUpdateWebhook(projectId, applicationId);
  const deleteApplication = useDeleteApplication(projectId);
  const { data: features } = useFeatures();
  const { data: gitCredentials } = useGitCredentials();

  const form = useForm({
    defaultValues: {
      name: application?.name ?? "",
      source_repo: application?.source_repo ?? "",
      source_image: application?.source_image ?? "",
      dockerfile_path: application?.dockerfile_path ?? "",
      build_type_override: application?.build_type_override ?? "",
      cpu_limit: application?.cpu_limit ?? 0,
      memory_limit_mb: application ? Math.round(application.memory_limit / (1024 * 1024)) : 0,
      git_token: "",
      git_credential_id: (application as Record<string, unknown>)?.git_credential_id as string ?? "",
      health_check_path: application?.health_check_path ?? "",
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
          memory_limit: value.memory_limit_mb > 0 ? value.memory_limit_mb * 1024 * 1024 : 0,
          git_token: value.git_token || undefined,
          git_credential_id: value.git_credential_id || undefined,
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

  const webhookForm = useForm({
    defaultValues: {
      webhook_secret: application?.webhook_secret ?? "",
      auto_deploy_branch: application?.auto_deploy_branch ?? "main",
    },
    onSubmit: async ({ value }) => {
      toast.promise(
        updateWebhook.mutateAsync({
          webhook_secret: value.webhook_secret,
          auto_deploy_branch: value.auto_deploy_branch || "main",
        }),
        {
          loading: "Saving...",
          success: "Webhook settings saved",
          error: (err) => err.message,
        },
      );
    },
  });

  const generateSecret = useCallback(() => {
    const bytes = new Uint8Array(32);
    crypto.getRandomValues(bytes);
    const secret = Array.from(bytes, (b) => b.toString(16).padStart(2, "0")).join("");
    webhookForm.setFieldValue("webhook_secret", secret);
  }, [webhookForm]);

  if (!application) {
    return (
      <div className="space-y-6">
        <Card>
          <CardHeader><Skeleton className="h-5 w-48" /></CardHeader>
          <CardContent className="space-y-4">
            {[1, 2, 3].map((i) => (
              <div key={i} className="space-y-2">
                <Skeleton className="h-4 w-24" />
                <Skeleton className="h-9 w-full" />
              </div>
            ))}
          </CardContent>
        </Card>
      </div>
    );
  }

  return (
    <div className="space-y-6">
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
              <ToggleGroup variant="outline" value={[application.type]} disabled className="justify-start">
                <ToggleGroupItem value="image">Docker Image</ToggleGroupItem>
                <ToggleGroupItem value="git">Git Repository</ToggleGroupItem>
              </ToggleGroup>
            </div>
            {application.type === "git" && (
              <form.Field
                name="build_type_override"
                children={(field) => {
                  const effectiveValue = field.state.value || application.build_type;
                  return (
                    <div className="space-y-2">
                      <Label>Build Type</Label>
                      <ToggleGroup variant="outline"
                        value={[effectiveValue]}
                        onValueChange={(v) => {
                          if (v.length > 0) {
                            field.handleChange(v[0] === application.build_type ? "" : v[0]);
                          }
                        }}
                        className="justify-start"
                      >
                        <ToggleGroupItem value="dockerfile">Dockerfile</ToggleGroupItem>
                        <ToggleGroupItem value="buildpacks">Buildpacks</ToggleGroupItem>
                        <ToggleGroupItem value="railpack">Railpack</ToggleGroupItem>
                      </ToggleGroup>
                      {effectiveValue === "railpack" && features?.buildkit_available === false && (
                        <Alert variant="destructive">
                          <TriangleAlert />
                          <AlertDescription>
                            BuildKit is not reachable. Railpack builds will fail.
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
                    onChange: z.string().refine(
                      (v) => !v || v.startsWith("https://") || v.startsWith("git@"),
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
                  name="git_credential_id"
                  children={(field) => (
                    <div className="space-y-2">
                      <Label>Git Credential</Label>
                      <Select
                        value={field.state.value}
                        onValueChange={field.handleChange}
                      >
                        <SelectTrigger>
                          <SelectValue placeholder="None (use token below)" />
                        </SelectTrigger>
                        <SelectContent>
                          <SelectItem value="">None</SelectItem>
                          {gitCredentials?.map((cred) => (
                            <SelectItem key={cred.id} value={cred.id}>
                              {cred.name} ({cred.provider})
                            </SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                      <p className="text-muted-foreground text-xs">
                        Select a centralized credential or enter a token below.
                      </p>
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
                        Per-app token for private repositories. Overridden when a credential is selected above.
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
                      onChange={(e) => field.handleChange(parseFloat(e.target.value) || 0)}
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
                      onChange={(e) => field.handleChange(parseInt(e.target.value) || 0)}
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
                      HTTP path polled after deploy to confirm readiness. Leave empty to skip.
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

      {application.type === "git" && (
        <>
          <Separator />

          <Card>
            <CardHeader>
              <CardTitle>Webhook (Auto-Deploy)</CardTitle>
            </CardHeader>
            <CardContent>
              <p className="text-muted-foreground text-sm mb-4">
                Configure a webhook secret to enable push-to-deploy. Add this URL
                as a webhook in your GitHub or GitLab repository:
              </p>
              <div className="bg-muted rounded-md px-3 py-2 text-sm font-mono mb-4 break-all">
                {window.location.origin}/api/webhooks/push
              </div>
              <form
                onSubmit={(e) => {
                  e.preventDefault();
                  e.stopPropagation();
                  webhookForm.handleSubmit();
                }}
                className="space-y-4"
              >
                <webhookForm.Field
                  name="webhook_secret"
                  children={(field) => (
                    <div className="space-y-2">
                      <Label>Webhook Secret</Label>
                      <div className="flex gap-2">
                        <Input
                          value={field.state.value}
                          onBlur={field.handleBlur}
                          onChange={(e) => field.handleChange(e.target.value)}
                          placeholder="Enter or generate a secret"
                          className="font-mono"
                        />
                        <Button type="button" variant="outline" onClick={generateSecret}>
                          Generate
                        </Button>
                      </div>
                      <p className="text-muted-foreground text-xs">
                        Use this secret when configuring the webhook in your git provider.
                      </p>
                    </div>
                  )}
                />
                <webhookForm.Field
                  name="auto_deploy_branch"
                  children={(field) => (
                    <div className="space-y-2">
                      <Label>Auto-Deploy Branch</Label>
                      <Input
                        value={field.state.value}
                        onBlur={field.handleBlur}
                        onChange={(e) => field.handleChange(e.target.value)}
                        placeholder="main"
                      />
                      <p className="text-muted-foreground text-xs">
                        Only pushes to this branch will trigger a deploy.
                      </p>
                    </div>
                  )}
                />
                <webhookForm.Subscribe
                  selector={(s) => s.isSubmitting}
                  children={(isSubmitting) => (
                    <Button type="submit" disabled={isSubmitting}>
                      {isSubmitting ? "Saving..." : "Save Webhook Settings"}
                    </Button>
                  )}
                />
              </form>
            </CardContent>
          </Card>
        </>
      )}

      <Separator />

      <Card className="border-destructive/50">
        <CardHeader>
          <CardTitle className="text-destructive">Danger Zone</CardTitle>
        </CardHeader>
        <CardContent>
          <AlertDialog>
            <AlertDialogTrigger render={<Button variant="destructive" />}>
              Delete Application
            </AlertDialogTrigger>
            <AlertDialogContent>
              <AlertDialogHeader>
                <AlertDialogTitle>Delete {application.name}?</AlertDialogTitle>
                <AlertDialogDescription>
                  This will stop the running container and permanently delete
                  this application. This action cannot be undone.
                </AlertDialogDescription>
              </AlertDialogHeader>
              <AlertDialogFooter>
                <AlertDialogCancel>Cancel</AlertDialogCancel>
                <AlertDialogAction
                  onClick={() => {
                    toast.promise(
                      deleteApplication.mutateAsync(applicationId).then(() => {
                        navigate({
                          to: "/projects/$projectId",
                          params: { projectId },
                        });
                      }),
                      {
                        loading: "Deleting application...",
                        success: "Application deleted",
                        error: (err) => err.message,
                      },
                    );
                  }}
                >
                  Delete
                </AlertDialogAction>
              </AlertDialogFooter>
            </AlertDialogContent>
          </AlertDialog>
        </CardContent>
      </Card>
    </div>
  );
}
