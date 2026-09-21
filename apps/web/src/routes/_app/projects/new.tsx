import { createFileRoute, useNavigate } from "@tanstack/react-router";
import { useForm } from "@tanstack/react-form";
import { z } from "zod";
import { toast } from "sonner";
import { PlusIcon } from "lucide-react";
import { useCreateProject } from "@/lib/hooks/use-projects";
import { Button } from "@/components/ui/button";
import { Field, FieldError, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { Card, CardContent } from "@/components/ui/card";
import { PageHeader } from "@/components/ui/page-header";

export const Route = createFileRoute("/_app/projects/new")({
  component: NewProjectPage,
});

function slugify(text: string) {
  return text
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-|-$/g, "");
}

function NewProjectPage() {
  const navigate = useNavigate();
  const createProject = useCreateProject();

  const form = useForm({
    defaultValues: { name: "", slug: "" },
    onSubmit: async ({ value }) => {
      toast.promise(
        createProject
          .mutateAsync({
            name: value.name,
            slug: value.slug,
          })
          .then((project) => {
            navigate({
              to: "/projects/$projectId",
              params: { projectId: project.id },
            });
          }),
        {
          loading: "Creating project...",
          success: "Project created",
          error: (err) => err.message,
        },
      );
    },
  });

  return (
    <div className="mx-auto max-w-lg space-y-6">
      <PageHeader
        icon={<PlusIcon className="size-5" />}
        title="New Project"
        description="Create a new project to organize your services."
      />

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
            <form.Field
              name="name"
              validators={{ onChange: z.string().min(1, "Name is required") }}
              children={(field) => {
                const first = field.state.meta.errors[0];
                const error = !first
                  ? undefined
                  : typeof first === "string"
                    ? first
                    : first?.message;
                return (
                  <Field data-invalid={!!error}>
                    <FieldLabel htmlFor="name">Project Name</FieldLabel>
                    <Input
                      id="name"
                      placeholder="My App"
                      value={field.state.value}
                      onBlur={field.handleBlur}
                      onChange={(e) => {
                        field.handleChange(e.target.value);
                        const slugField = form.getFieldValue("slug");
                        if (
                          !slugField ||
                          slugField === slugify(field.state.value)
                        ) {
                          form.setFieldValue("slug", slugify(e.target.value));
                        }
                      }}
                      aria-invalid={!!error}
                    />
                    {error && <FieldError>{error}</FieldError>}
                  </Field>
                );
              }}
            />
            <form.Field
              name="slug"
              validators={{
                onChange: z
                  .string()
                  .min(1, "Slug is required")
                  .regex(
                    /^[a-z0-9-]+$/,
                    "Slug must be lowercase letters, numbers, and hyphens",
                  ),
              }}
              children={(field) => {
                const first = field.state.meta.errors[0];
                const error = !first
                  ? undefined
                  : typeof first === "string"
                    ? first
                    : first?.message;
                return (
                  <Field data-invalid={!!error}>
                    <FieldLabel htmlFor="slug">Slug</FieldLabel>
                    <Input
                      id="slug"
                      placeholder="my-app"
                      value={field.state.value}
                      onBlur={field.handleBlur}
                      onChange={(e) => field.handleChange(e.target.value)}
                      aria-invalid={!!error}
                    />
                    {error && <FieldError>{error}</FieldError>}
                  </Field>
                );
              }}
            />
            <div className="flex gap-3">
              <Button
                type="button"
                variant="outline"
                onClick={() => navigate({ to: "/projects" })}
              >
                Cancel
              </Button>
              <form.Subscribe
                selector={(s) => s.isSubmitting}
                children={(isSubmitting) => (
                  <Button type="submit" disabled={isSubmitting}>
                    {isSubmitting ? "Creating..." : "Create Project"}
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
