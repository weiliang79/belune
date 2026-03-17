import { createFileRoute, useNavigate } from "@tanstack/react-router";
import {
  useService,
  useUpdateService,
  useDeleteService,
} from "@/lib/hooks/use-services";
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
import { Separator } from "@/components/ui/separator";
import { useState } from "react";

export const Route = createFileRoute(
  "/_app/projects/$projectId/services/$serviceId/settings",
)({
  component: ServiceSettingsPage,
});

function ServiceSettingsPage() {
  const { projectId, serviceId } = Route.useParams();
  const navigate = useNavigate();
  const { data: service } = useService(projectId, serviceId);
  const updateService = useUpdateService(projectId, serviceId);
  const deleteService = useDeleteService(projectId);
  const [error, setError] = useState("");

  const form = useForm({
    defaultValues: {
      name: service?.name ?? "",
      source_repo: service?.source_repo ?? "",
      source_image: service?.source_image ?? "",
      dockerfile_path: service?.dockerfile_path ?? "",
    },
    onSubmit: async ({ value }) => {
      setError("");
      try {
        await updateService.mutateAsync({
          name: value.name || undefined,
          source_repo: value.source_repo || undefined,
          source_image: value.source_image || undefined,
          dockerfile_path: value.dockerfile_path || undefined,
        });
      } catch (e) {
        setError(e instanceof Error ? e.message : "Update failed");
      }
    },
  });

  if (!service) return null;

  return (
    <div className="space-y-6">
      <Card>
        <CardHeader>
          <CardTitle>Service Settings</CardTitle>
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
              name="name"
              validators={{ onChange: z.string().min(1, "Name is required") }}
              children={(field) => (
                <div className="space-y-2">
                  <Label>Service Name</Label>
                  <Input
                    value={field.state.value}
                    onBlur={field.handleBlur}
                    onChange={(e) => field.handleChange(e.target.value)}
                  />
                </div>
              )}
            />
            {service.type === "image" && (
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
            {service.type === "git" && (
              <>
                <form.Field
                  name="source_repo"
                  children={(field) => (
                    <div className="space-y-2">
                      <Label>Repository URL</Label>
                      <Input
                        value={field.state.value}
                        onBlur={field.handleBlur}
                        onChange={(e) => field.handleChange(e.target.value)}
                      />
                    </div>
                  )}
                />
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

      <Separator />

      <Card className="border-destructive/50">
        <CardHeader>
          <CardTitle className="text-destructive">Danger Zone</CardTitle>
        </CardHeader>
        <CardContent>
          <AlertDialog>
            <AlertDialogTrigger render={<Button variant="destructive" />}>
              Delete Service
            </AlertDialogTrigger>
            <AlertDialogContent>
              <AlertDialogHeader>
                <AlertDialogTitle>Delete {service.name}?</AlertDialogTitle>
                <AlertDialogDescription>
                  This will stop the running container and permanently delete
                  this service. This action cannot be undone.
                </AlertDialogDescription>
              </AlertDialogHeader>
              <AlertDialogFooter>
                <AlertDialogCancel>Cancel</AlertDialogCancel>
                <AlertDialogAction
                  onClick={async () => {
                    await deleteService.mutateAsync(serviceId);
                    navigate({
                      to: "/projects/$projectId/services",
                      params: { projectId },
                    });
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
