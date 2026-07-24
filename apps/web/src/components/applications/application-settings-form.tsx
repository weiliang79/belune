import { useState } from "react";
import { useForm } from "@tanstack/react-form";
import { z } from "zod";
import { toast } from "sonner";
import { TriangleAlert, Settings2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Alert, AlertDescription } from "@/components/ui/alert";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import {
  SegmentedControl,
  SegmentedControlItem,
} from "@/components/ui/segmented-control";
import { IntegrationRepoPicker } from "@/components/applications/integration-repo-picker";
import {
  useUpdateApplication,
  useChangeApplicationSource,
} from "@/lib/hooks/use-applications";
import { useFeatures } from "@/lib/hooks/use-features";
import type { Application } from "@/lib/types";

interface Props {
  projectId: string;
  applicationId: string;
  application: Application;
}

type FormValues = {
  name: string;
  source_repo: string;
  source_image: string;
  dockerfile_path: string;
  build_type_override: string;
  git_token: string;
  branch: string;
};

// Derive the "owner/repo" a connected-account picker keys on from the stored
// clone URL (https://host/owner/repo.git). Returns "" for a blank/unparseable
// URL so the picker falls back to an empty selection.
function repoFullNameFromCloneUrl(cloneUrl: string | null): string {
  if (!cloneUrl) return "";
  try {
    return new URL(cloneUrl).pathname.replace(/^\/+/, "").replace(/\.git$/, "");
  } catch {
    return "";
  }
}

export function ApplicationSettingsForm({
  projectId,
  applicationId,
  application,
}: Props) {
  const updateApplication = useUpdateApplication(projectId, applicationId);
  const changeSource = useChangeApplicationSource(projectId, applicationId);
  const { data: features } = useFeatures();

  // The type is edited here rather than in a separate section: switching it is
  // a save like any other from the user's point of view. What makes it not a
  // plain save is the endpoint — PUT cannot change type, and folding that
  // capability into it would re-open the incoherent-source hole the change-
  // source endpoint was built to close. So Save routes by whether the type
  // moved: unchanged goes to the ordinary update; changed goes through a
  // confirmation to the dedicated endpoint, which swaps every source column as
  // one unit and drops the credentials that no longer apply.
  const [selectedType, setSelectedType] = useState(application.type);
  const isSwitching = selectedType !== application.type;

  // Captured on submit and applied only once the user confirms — the switch
  // deletes secrets and needs a deploy, which is too much to do on one click.
  const [pending, setPending] = useState<FormValues | null>(null);

  // Where the git repository comes from — a connected account or a plain URL —
  // mirroring the create dialog. Defaults to whichever the app already uses; an
  // app with an integration is on the connection path.
  const [gitSource, setGitSource] = useState<"connection" | "url">(
    application.git_integration_id ? "connection" : "url",
  );
  const [gitIntegrationId, setGitIntegrationId] = useState(
    application.git_integration_id ?? "",
  );

  // A switch to git has no stored build_type to fall back on, so the control
  // starts from railpack (what the create flow defaults to). A same-type edit
  // keeps override semantics against the stored base.
  const gitBuildBase = isSwitching ? "railpack" : application.build_type;

  const form = useForm({
    defaultValues: {
      name: application.name ?? "",
      source_repo: application.source_repo ?? "",
      source_image: application.source_image ?? "",
      dockerfile_path: application.dockerfile_path ?? "",
      build_type_override: application.build_type_override ?? "",
      git_token: "",
      branch: application.branch ?? "",
    } as FormValues,
    onSubmit: async ({ value }) => {
      if (isSwitching) {
        // Guard the required field here, since the type control can flip to a
        // type whose source input the user has not filled in yet.
        if (selectedType === "git" && !value.source_repo.trim()) {
          toast.error("A repository URL is required to switch to git");
          return;
        }
        if (selectedType === "image" && !value.source_image.trim()) {
          toast.error("An image is required to switch to a prebuilt image");
          return;
        }
        setPending(value);
        return;
      }

      // Unchanged type: the ordinary update. Only the fields belonging to this
      // type are sent — the server rejects a mix, and the other type's inputs
      // are not rendered, so echoing a stale value back would produce a
      // rejection the user has no field to fix.
      const isGit = selectedType === "git";
      toast.promise(
        updateApplication.mutateAsync({
          name: value.name || undefined,
          source_repo: isGit ? value.source_repo || undefined : undefined,
          source_image: isGit ? undefined : value.source_image || undefined,
          dockerfile_path: isGit ? value.dockerfile_path || undefined : undefined,
          // Sent even when blank: blank means "the repository's default ref",
          // which must be able to clear a previously set branch.
          branch: value.branch,
          build_type_override: isGit
            ? value.build_type_override || undefined
            : undefined,
          // A token only applies to the public-URL path; a connected account
          // carries its own credentials.
          git_token:
            isGit && gitSource === "url"
              ? value.git_token || undefined
              : undefined,
          // Set the integration on the connection path, and clear it ("" =
          // clear) when the app is edited onto a plain URL. Omitted for image
          // apps so it is preserved.
          git_integration_id: isGit
            ? gitSource === "connection"
              ? gitIntegrationId
              : ""
            : undefined,
        }),
        {
          loading: "Saving...",
          success: "Settings saved",
          error: (err) => err.message,
        },
      );
    },
  });

  const confirmSwitch = () => {
    if (!pending) return;
    const v = pending;
    const data =
      selectedType === "image"
        ? { type: "image" as const, source_image: v.source_image.trim() }
        : {
            type: "git" as const,
            source_repo: v.source_repo.trim(),
            branch: v.branch.trim(),
            build_type: v.build_type_override || "railpack",
            dockerfile_path: v.dockerfile_path || undefined,
            git_token:
              gitSource === "url" ? v.git_token || undefined : undefined,
            git_integration_id:
              gitSource === "connection" ? gitIntegrationId : undefined,
          };
    toast.promise(changeSource.mutateAsync(data), {
      loading: "Changing source...",
      success: () => {
        setPending(null);
        return "Source changed — deploy to apply it";
      },
      error: (err) => err.message,
    });
  };

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <Settings2 aria-hidden="true" className="size-4" />
          Application Settings
        </CardTitle>
        <CardDescription>
          Build and runtime configuration for this application. Saving does not
          touch the running container — the badge beside the application name
          says what is needed to apply the change: the resource limits need only
          a reload, while the image, branch, or builder settings need a deploy.
        </CardDescription>
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
            <Label>Source</Label>
            <SegmentedControl
              value={selectedType}
              onValueChange={(v) => setSelectedType(v as "git" | "image")}
            >
              <SegmentedControlItem value="image">
                Docker Image
              </SegmentedControlItem>
              <SegmentedControlItem value="git">
                Git Repository
              </SegmentedControlItem>
            </SegmentedControl>
            {isSwitching && (
              <Alert>
                <TriangleAlert />
                <AlertDescription>
                  {selectedType === "image"
                    ? "Switching to an image removes the repository, branch, build settings, git credentials and push webhook secret. Domains, volumes, mounts, environment variables and the deploy hook are kept."
                    : "Switching to git removes the image reference and builds from source instead. Domains, volumes, mounts, environment variables and the deploy hook are kept."}{" "}
                  You will confirm before anything changes, and the running
                  container keeps serving until you deploy.
                </AlertDescription>
              </Alert>
            )}
          </div>
          {selectedType === "git" && (
            <form.Field
              name="build_type_override"
              children={(field) => {
                const effectiveValue = field.state.value || gitBuildBase;
                return (
                  <div className="space-y-2">
                    <Label>Build Method</Label>
                    <SegmentedControl
                      value={effectiveValue}
                      onValueChange={(v) =>
                        field.handleChange(v === gitBuildBase ? "" : v)
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
          {selectedType === "image" && (
            <form.Field
              name="source_image"
              children={(field) => (
                <div className="space-y-2">
                  <Label>Docker Image</Label>
                  <Input
                    value={field.state.value}
                    onBlur={field.handleBlur}
                    onChange={(e) => field.handleChange(e.target.value)}
                    placeholder="nginx:1.27"
                  />
                </div>
              )}
            />
          )}
          {selectedType === "git" && (
            <>
              <div className="space-y-2">
                <Label>Repository Source</Label>
                <SegmentedControl
                  value={gitSource}
                  onValueChange={(v) =>
                    setGitSource(v as "connection" | "url")
                  }
                >
                  <SegmentedControlItem value="connection">
                    Connected Account
                  </SegmentedControlItem>
                  <SegmentedControlItem value="url">
                    Public URL
                  </SegmentedControlItem>
                </SegmentedControl>
              </div>
              {gitSource === "connection" ? (
                <>
                  <form.Subscribe
                    selector={(s) => s.values.source_repo}
                    children={(repo) =>
                      repo ? (
                        <p className="text-muted-foreground text-xs">
                          Current repository:{" "}
                          <span className="font-mono">{repo}</span>. Pick one
                          below to change it.
                        </p>
                      ) : null
                    }
                  />
                  <IntegrationRepoPicker
                    initialIntegrationId={application.git_integration_id ?? ""}
                    initialRepoFullName={repoFullNameFromCloneUrl(
                      application.source_repo,
                    )}
                    initialBranch={application.branch ?? ""}
                    onSelect={({ integrationId, cloneUrl, branch }) => {
                      setGitIntegrationId(integrationId);
                      form.setFieldValue("source_repo", cloneUrl);
                      form.setFieldValue("branch", branch);
                    }}
                  />
                </>
              ) : (
                <form.Field
                  name="source_repo"
                  validators={{
                    onChange: z
                      .string()
                      .refine(
                        (v) =>
                          !v ||
                          v.startsWith("https://") ||
                          v.startsWith("git@"),
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
              )}
              {/* Connected-account mode has its own branch dropdown inside the
                  picker above (it writes this same form field via onSelect), so
                  the free-text branch input is only for the public-URL path. */}
              {gitSource === "url" && (
                <form.Field
                  name="branch"
                  children={(field) => (
                    <div className="space-y-2">
                      <Label>Branch</Label>
                      <Input
                        value={field.state.value}
                        onBlur={field.handleBlur}
                        onChange={(e) => field.handleChange(e.target.value)}
                        placeholder="Default branch"
                      />
                      <p className="text-muted-foreground text-xs">
                        The branch to build, and the one whose pushes deploy.
                        Leave empty to track the repository's default branch.
                      </p>
                    </div>
                  )}
                />
              )}
              {gitSource === "url" && (
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
                        placeholder={
                          isSwitching
                            ? "Leave empty for a public repository"
                            : "Leave empty to keep existing token"
                        }
                        className="font-mono"
                      />
                      <p className="text-muted-foreground text-xs">
                        Per-app token for private repositories. Use a repo-scoped
                        token where possible.
                      </p>
                    </div>
                  )}
                />
              )}
              <form.Subscribe
                selector={(s) => s.values.build_type_override}
                children={(override) =>
                  (override || gitBuildBase) === "dockerfile" ? (
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
                  ) : null
                }
              />
            </>
          )}
          <div className="flex justify-end">
            <form.Subscribe
              selector={(s) => s.isSubmitting}
              children={(isSubmitting) => (
                <Button type="submit" disabled={isSubmitting}>
                  {isSubmitting
                    ? "Saving..."
                    : isSwitching
                      ? "Save & change source"
                      : "Save"}
                </Button>
              )}
            />
          </div>
        </form>
      </CardContent>

      <AlertDialog
        open={pending !== null}
        onOpenChange={(open) => {
          if (!open) setPending(null);
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>
              {selectedType === "image"
                ? "Switch to a prebuilt image?"
                : "Switch to git?"}
            </AlertDialogTitle>
            <AlertDialogDescription>
              {selectedType === "image"
                ? "The repository, branch, build settings, git credentials and push webhook secret are removed. Domains, volumes and their data, file mounts, environment variables, the deploy hook, and deployment history are kept."
                : "The image reference is removed and the application will be built from source. Domains, volumes and their data, file mounts, environment variables, the deploy hook, and deployment history are kept."}{" "}
              The running container keeps serving until you deploy.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction onClick={confirmSwitch}>
              Change source
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </Card>
  );
}
