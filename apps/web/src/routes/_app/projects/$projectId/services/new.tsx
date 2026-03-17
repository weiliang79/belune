import { createFileRoute, useNavigate } from "@tanstack/react-router";
import { useForm } from "@tanstack/react-form";
import { z } from "zod";
import { useCreateService } from "@/lib/hooks/use-services";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Card, CardContent } from "@/components/ui/card";
import { useState } from "react";

export const Route = createFileRoute("/_app/projects/$projectId/services/new")({
  component: NewServicePage,
});

function NewServicePage() {
  const { projectId } = Route.useParams();
  const navigate = useNavigate();
  const createService = useCreateService(projectId);
  const [error, setError] = useState("");
  const [serviceType, setServiceType] = useState<"image" | "git">("image");

  const form = useForm({
    defaultValues: {
      name: "",
      source_image: "",
      source_repo: "",
      dockerfile_path: "Dockerfile",
      build_type: "dockerfile" as string,
    },
    onSubmit: async ({ value }) => {
      setError("");
      try {
        const service = await createService.mutateAsync({
          name: value.name,
          type: serviceType,
          ...(serviceType === "image"
            ? { source_image: value.source_image }
            : {
                source_repo: value.source_repo,
                dockerfile_path: value.dockerfile_path,
                build_type: value.build_type,
              }),
        });
        navigate({
          to: "/projects/$projectId/services/$serviceId",
          params: { projectId, serviceId: service.id },
        });
      } catch (e) {
        setError(e instanceof Error ? e.message : "Failed to create service");
      }
    },
  });

  return (
    <div className="mx-auto max-w-lg space-y-6">
      <div>
        <h1 className="text-2xl font-bold">Deploy New Service</h1>
        <p className="text-muted-foreground">Add a service to your project.</p>
      </div>

      <Card>
        <CardContent className="pt-6">
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
              validators={{
                onChange: z.string().min(1, "Service name is required"),
              }}
              children={(field) => (
                <div className="space-y-2">
                  <Label htmlFor="name">Service Name</Label>
                  <Input
                    id="name"
                    placeholder="my-api"
                    value={field.state.value}
                    onBlur={field.handleBlur}
                    onChange={(e) => field.handleChange(e.target.value)}
                  />
                  {field.state.meta.errors.length > 0 && (
                    <p className="text-destructive text-sm">
                      {field.state.meta.errors[0]?.toString()}
                    </p>
                  )}
                </div>
              )}
            />

            <div className="space-y-2">
              <Label>Source Type</Label>
              <div className="flex gap-2">
                <Button
                  type="button"
                  variant={serviceType === "image" ? "default" : "outline"}
                  size="sm"
                  onClick={() => setServiceType("image")}
                >
                  Docker Image
                </Button>
                <Button
                  type="button"
                  variant={serviceType === "git" ? "default" : "outline"}
                  size="sm"
                  onClick={() => setServiceType("git")}
                >
                  Git Repository
                </Button>
              </div>
            </div>

            {serviceType === "image" ? (
              <form.Field
                name="source_image"
                validators={{
                  onChange: z.string().min(1, "Image name is required"),
                }}
                children={(field) => (
                  <div className="space-y-2">
                    <Label htmlFor="source_image">Docker Image</Label>
                    <Input
                      id="source_image"
                      placeholder="nginx:latest"
                      value={field.state.value}
                      onBlur={field.handleBlur}
                      onChange={(e) => field.handleChange(e.target.value)}
                    />
                    {field.state.meta.errors.length > 0 && (
                      <p className="text-destructive text-sm">
                        {field.state.meta.errors[0]?.toString()}
                      </p>
                    )}
                  </div>
                )}
              />
            ) : (
              <>
                <form.Field
                  name="source_repo"
                  validators={{
                    onChange: z.string().url("Must be a valid URL"),
                  }}
                  children={(field) => (
                    <div className="space-y-2">
                      <Label htmlFor="source_repo">Repository URL</Label>
                      <Input
                        id="source_repo"
                        placeholder="https://github.com/user/repo.git"
                        value={field.state.value}
                        onBlur={field.handleBlur}
                        onChange={(e) => field.handleChange(e.target.value)}
                      />
                      {field.state.meta.errors.length > 0 && (
                        <p className="text-destructive text-sm">
                          {field.state.meta.errors[0]?.toString()}
                        </p>
                      )}
                    </div>
                  )}
                />
                <form.Field
                  name="dockerfile_path"
                  children={(field) => (
                    <div className="space-y-2">
                      <Label htmlFor="dockerfile_path">Dockerfile Path</Label>
                      <Input
                        id="dockerfile_path"
                        placeholder="Dockerfile"
                        value={field.state.value}
                        onBlur={field.handleBlur}
                        onChange={(e) => field.handleChange(e.target.value)}
                      />
                    </div>
                  )}
                />
                <div className="space-y-2">
                  <Label>Build Type</Label>
                  <div className="flex flex-wrap gap-2">
                    {["dockerfile", "buildpacks", "nixpacks"].map((bt) => (
                      <Button
                        key={bt}
                        type="button"
                        variant={
                          form.getFieldValue("build_type") === bt
                            ? "default"
                            : "outline"
                        }
                        size="sm"
                        onClick={() => form.setFieldValue("build_type", bt)}
                      >
                        {bt}
                      </Button>
                    ))}
                  </div>
                </div>
              </>
            )}

            <div className="flex gap-3">
              <Button
                type="button"
                variant="outline"
                onClick={() =>
                  navigate({
                    to: "/projects/$projectId/services",
                    params: { projectId },
                  })
                }
              >
                Cancel
              </Button>
              <form.Subscribe
                selector={(s) => s.isSubmitting}
                children={(isSubmitting) => (
                  <Button type="submit" disabled={isSubmitting}>
                    {isSubmitting ? "Creating..." : "Create Service"}
                  </Button>
                )}
              />
            </div>
          </form>
        </CardContent>
      </Card>
    </div>
  );
}
