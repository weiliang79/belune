import { useState } from "react";
import { useNavigate } from "@tanstack/react-router";
import { useForm } from "@tanstack/react-form";
import { z } from "zod";
import { Loader2, TriangleAlert } from "lucide-react";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Field, FieldError, FieldLabel } from "@/components/ui/field";
import { fieldError } from "@/lib/utils/field-error";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  SegmentedControl,
  SegmentedControlItem,
} from "@/components/ui/segmented-control";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { useCreateApplication } from "@/lib/hooks/use-applications";
import { useFeatures } from "@/lib/hooks/use-features";
import { IntegrationRepoPicker } from "./integration-repo-picker";

function slugify(name: string): string {
  return name
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-|-$/g, "");
}

interface Props {
  projectId: string;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onCreated?: (id: string) => void;
}

export function ApplicationFormDialog({
  projectId,
  open,
  onOpenChange,
  onCreated,
}: Props) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      {/* Cap the height and let only the body scroll (grid rows pin the header
          and footer): connected-account mode adds account + repo + branch fields,
          which otherwise grew the dialog past the viewport with no way to scroll. */}
      <DialogContent className="max-h-[85vh] grid-rows-[auto_minmax(0,1fr)_auto]">
        {/* Remounted per open, so the form resets without an effect. */}
        {open && (
          <FormBody
            projectId={projectId}
            onOpenChange={onOpenChange}
            onCreated={onCreated}
          />
        )}
      </DialogContent>
    </Dialog>
  );
}

function FormBody({ projectId, onOpenChange, onCreated }: Omit<Props, "open">) {
  const navigate = useNavigate();
  const createApplication = useCreateApplication(projectId);
  const { data: features } = useFeatures();
  const [appSlugManual, setAppSlugManual] = useState(false);
  const [appError, setAppError] = useState("");

  const form = useForm({
    defaultValues: {
      appName: "",
      appSlug: "",
      appType: "image" as "image" | "git",
      sourceImage: "",
      sourceRepo: "",
      gitSource: "connection" as "connection" | "url",
      gitIntegrationId: "",
      gitToken: "",
      dockerfilePath: "Dockerfile",
      rootDirectory: "",
      branch: "",
      buildType: "dockerfile",
    },
    onSubmit: async ({ value }) => {
      setAppError("");
      try {
        const application = await createApplication.mutateAsync({
          name: value.appName.trim(),
          slug: value.appSlug || undefined,
          type: value.appType,
          ...(value.appType === "image"
            ? { source_image: value.sourceImage, build_type: "image" }
            : {
                source_repo: value.sourceRepo,
                branch: value.branch.trim(),
                root_directory: value.rootDirectory.trim(),
                dockerfile_path: value.dockerfilePath,
                build_type: value.buildType,
                ...(value.gitSource === "connection" && value.gitIntegrationId
                  ? { git_integration_id: value.gitIntegrationId }
                  : {}),
                ...(value.gitSource === "url" && value.gitToken.trim()
                  ? { git_token: value.gitToken.trim() }
                  : {}),
              }),
        });
        onOpenChange(false);
        if (onCreated) {
          onCreated(application.id);
        } else {
          navigate({
            to: "/projects/$projectId/applications/$applicationId",
            params: { projectId, applicationId: application.id },
          });
        }
      } catch (e) {
        setAppError(
          e instanceof Error ? e.message : "Failed to create application",
        );
      }
    },
  });

  return (
    <form
      onSubmit={(e) => {
        e.preventDefault();
        e.stopPropagation();
        form.handleSubmit();
      }}
      className="contents"
    >
      <DialogHeader>
        <DialogTitle>New Application</DialogTitle>
        <DialogDescription>
          Add an application to your project.
        </DialogDescription>
      </DialogHeader>
      <div className="space-y-4 overflow-y-auto py-2">
        {appError && (
          <div className="bg-destructive/10 text-destructive rounded-md px-3 py-2 text-sm">
            {appError}
          </div>
        )}
        <form.Field
          name="appName"
          validators={{
            onChange: z.string().min(1, "Application name is required"),
          }}
          children={(field) => {
            const error = fieldError(field.state.meta.errors);
            return (
              <Field data-invalid={!!error}>
                <FieldLabel htmlFor="app-name">Application Name</FieldLabel>
                <Input
                  id="app-name"
                  value={field.state.value}
                  onBlur={field.handleBlur}
                  onChange={(e) => {
                    field.handleChange(e.target.value);
                    if (!appSlugManual) {
                      form.setFieldValue("appSlug", slugify(e.target.value));
                    }
                  }}
                  placeholder="my-api"
                  aria-invalid={!!error}
                />
                {error && <FieldError>{error}</FieldError>}
              </Field>
            );
          }}
        />
        <form.Subscribe
          selector={(s) => s.values.appName}
          children={(appName) => (
            <form.Field
              name="appSlug"
              children={(field) => (
                <div className="space-y-2">
                  <Label htmlFor="app-slug">Slug</Label>
                  <Input
                    id="app-slug"
                    value={field.state.value}
                    onBlur={field.handleBlur}
                    onChange={(e) => {
                      field.handleChange(slugify(e.target.value));
                      setAppSlugManual(true);
                    }}
                    placeholder={appName ? slugify(appName) : "auto-generated"}
                  />
                  <p className="text-muted-foreground text-xs">
                    Used in container naming. Auto-generated from name unless
                    overridden.
                  </p>
                </div>
              )}
            />
          )}
        />
        <form.Field
          name="appType"
          children={(field) => (
            <div className="space-y-2">
              <Label>Source</Label>
              <SegmentedControl
                value={field.state.value}
                onValueChange={(v) => field.handleChange(v as "image" | "git")}
              >
                <SegmentedControlItem value="image">
                  Docker Image
                </SegmentedControlItem>
                <SegmentedControlItem value="git">
                  Git Repository
                </SegmentedControlItem>
              </SegmentedControl>
            </div>
          )}
        />
        <form.Subscribe
          selector={(s) => s.values.appType}
          children={(appType) =>
            appType === "image" ? (
              <form.Field
                name="sourceImage"
                validators={{
                  onChangeListenTo: ["appType"],
                  onChange: ({ value, fieldApi }) =>
                    fieldApi.form.state.values.appType === "image" &&
                    value.trim() === ""
                      ? "Image name is required"
                      : undefined,
                }}
                children={(field) => {
                  const error = fieldError(field.state.meta.errors);
                  return (
                    <Field data-invalid={!!error}>
                      <FieldLabel htmlFor="source-image">
                        Docker Image
                      </FieldLabel>
                      <Input
                        id="source-image"
                        value={field.state.value}
                        onBlur={field.handleBlur}
                        onChange={(e) => field.handleChange(e.target.value)}
                        placeholder="nginx:latest"
                        aria-invalid={!!error}
                      />
                      {error && <FieldError>{error}</FieldError>}
                    </Field>
                  );
                }}
              />
            ) : (
              <>
                <form.Field
                  name="gitSource"
                  children={(field) => (
                    <div className="space-y-2">
                      <Label>Repository Source</Label>
                      <SegmentedControl
                        value={field.state.value}
                        onValueChange={(v) => {
                          field.handleChange(v as "connection" | "url");
                          form.setFieldValue("sourceRepo", "");
                          form.setFieldValue("gitIntegrationId", "");
                          form.setFieldValue("gitToken", "");
                          form.setFieldValue("branch", "");
                        }}
                      >
                        <SegmentedControlItem value="connection">
                          Connected Account
                        </SegmentedControlItem>
                        <SegmentedControlItem value="url">
                          Git URL
                        </SegmentedControlItem>
                      </SegmentedControl>
                    </div>
                  )}
                />
                {/* One always-mounted field regardless of gitSource, so its
                    required-check runs even when the picker (not a visible
                    Input) is what's supposed to fill it. */}
                <form.Field
                  name="sourceRepo"
                  validators={{
                    onChangeListenTo: ["appType", "gitSource"],
                    onChange: ({ value, fieldApi }) => {
                      const v = fieldApi.form.state.values;
                      if (v.appType !== "git" || value.trim() !== "")
                        return undefined;
                      return v.gitSource === "connection"
                        ? "Select a repository from a connected account"
                        : "Repository URL is required";
                    },
                  }}
                  children={(sourceRepoField) => {
                    const error = fieldError(sourceRepoField.state.meta.errors);
                    return (
                      <form.Subscribe
                        selector={(s) => s.values.gitSource}
                        children={(gitSource) =>
                          gitSource === "connection" ? (
                            <Field data-invalid={!!error}>
                              <IntegrationRepoPicker
                                onSelect={({
                                  integrationId,
                                  cloneUrl,
                                  branch,
                                }) => {
                                  form.setFieldValue(
                                    "gitIntegrationId",
                                    integrationId,
                                  );
                                  sourceRepoField.handleChange(cloneUrl);
                                  form.setFieldValue("branch", branch);
                                }}
                              />
                              {error && <FieldError>{error}</FieldError>}
                            </Field>
                          ) : (
                            <>
                              <Field data-invalid={!!error}>
                                <FieldLabel htmlFor="source-repo">
                                  Repository URL
                                </FieldLabel>
                                <Input
                                  id="source-repo"
                                  value={sourceRepoField.state.value}
                                  onBlur={sourceRepoField.handleBlur}
                                  onChange={(e) =>
                                    sourceRepoField.handleChange(e.target.value)
                                  }
                                  placeholder="https://github.com/user/repo.git"
                                  aria-invalid={!!error}
                                />
                                {error && <FieldError>{error}</FieldError>}
                              </Field>
                              <form.Field
                                name="gitToken"
                                children={(field) => (
                                  <div className="space-y-2">
                                    <Label htmlFor="git-token">
                                      Private Token (PAT)
                                    </Label>
                                    <Input
                                      id="git-token"
                                      type="password"
                                      value={field.state.value}
                                      onBlur={field.handleBlur}
                                      onChange={(e) =>
                                        field.handleChange(e.target.value)
                                      }
                                      placeholder="Leave empty for a public repository"
                                      className="font-mono"
                                    />
                                    <p className="text-muted-foreground text-xs">
                                      Per-app token for private repositories. A
                                      connected account is still recommended
                                      when available — it's scoped and registers
                                      push-to-deploy webhooks automatically,
                                      which a URL + PAT source does not.
                                    </p>
                                  </div>
                                )}
                              />
                            </>
                          )
                        }
                      />
                    );
                  }}
                />
                <form.Field
                  name="buildType"
                  children={(field) => (
                    <div className="space-y-2">
                      <Label>Build Method</Label>
                      <SegmentedControl
                        value={field.state.value}
                        onValueChange={(v) => field.handleChange(v)}
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
                      {field.state.value === "railpack" &&
                        features?.buildkit_available === false && (
                          <Alert variant="warning">
                            <TriangleAlert />
                            <AlertTitle>Warning</AlertTitle>
                            <AlertDescription>
                              BuildKit is not reachable. Railpack builds will
                              fail.
                            </AlertDescription>
                          </Alert>
                        )}
                    </div>
                  )}
                />
                {/* Connected-account mode has its own branch dropdown inside the
                    picker above (it writes this same field via onSelect), so
                    the free-text branch input is only for the public-URL path. */}
                <form.Subscribe
                  selector={(s) => s.values.gitSource}
                  children={(gitSource) =>
                    gitSource === "url" && (
                      <form.Field
                        name="branch"
                        children={(field) => (
                          <div className="space-y-2">
                            <Label htmlFor="app-branch">Branch</Label>
                            <Input
                              id="app-branch"
                              value={field.state.value}
                              onBlur={field.handleBlur}
                              onChange={(e) =>
                                field.handleChange(e.target.value)
                              }
                              placeholder="Default branch"
                            />
                            <p className="text-muted-foreground text-xs">
                              The branch to build, and the one whose pushes
                              deploy. Leave empty to track the repository's
                              default branch.
                            </p>
                          </div>
                        )}
                      />
                    )
                  }
                />
                <form.Field
                  name="rootDirectory"
                  children={(field) => (
                    <div className="space-y-2">
                      <Label htmlFor="root-directory">Root Directory</Label>
                      <Input
                        id="root-directory"
                        value={field.state.value}
                        onBlur={field.handleBlur}
                        onChange={(e) => field.handleChange(e.target.value)}
                        placeholder="e.g. apps/web — leave blank for repo root"
                        className="font-mono"
                      />
                      <p className="text-muted-foreground text-xs">
                        Build from a subdirectory of the repo (monorepo
                        support). Detection and the Dockerfile path and build
                        context all resolve from here.
                      </p>
                    </div>
                  )}
                />
                <form.Subscribe
                  selector={(s) =>
                    [s.values.buildType, s.values.rootDirectory] as const
                  }
                  children={([buildType, rootDirectory]) =>
                    buildType === "dockerfile" && (
                      <form.Field
                        name="dockerfilePath"
                        children={(field) => (
                          <div className="space-y-2">
                            <Label htmlFor="dockerfile-path">
                              Dockerfile Path
                            </Label>
                            <Input
                              id="dockerfile-path"
                              value={field.state.value}
                              onBlur={field.handleBlur}
                              onChange={(e) =>
                                field.handleChange(e.target.value)
                              }
                              placeholder="Dockerfile"
                            />
                            {rootDirectory.trim() && (
                              <p className="text-muted-foreground text-xs">
                                Relative to the Root Directory above, not the
                                repo root.
                              </p>
                            )}
                          </div>
                        )}
                      />
                    )
                  }
                />
              </>
            )
          }
        />
      </div>
      <DialogFooter>
        <Button type="submit" disabled={createApplication.isPending}>
          {createApplication.isPending && (
            <Loader2 className="mr-1 h-4 w-4 animate-spin" />
          )}
          Create Application
        </Button>
      </DialogFooter>
    </form>
  );
}
