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
import { useFeatures } from "@/lib/hooks/use-features";
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

  const form = useForm({
    defaultValues: {
      name: application?.name ?? "",
      source_repo: application?.source_repo ?? "",
      source_image: application?.source_image ?? "",
      dockerfile_path: application?.dockerfile_path ?? "",
      build_type_override: application?.build_type_override ?? "",
    },
    onSubmit: async ({ value }) => {
      toast.promise(
        updateApplication.mutateAsync({
          name: value.name || undefined,
          source_repo: value.source_repo || undefined,
          source_image: value.source_image || undefined,
          dockerfile_path: value.dockerfile_path || undefined,
          build_type_override: value.build_type_override || undefined,
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

  if (!application) return null;

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
