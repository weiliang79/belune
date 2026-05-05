import { useEffect, useState } from "react";
import { useNavigate } from "@tanstack/react-router";
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
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { useCreateApplication } from "@/lib/hooks/use-applications";
import { useFeatures } from "@/lib/hooks/use-features";

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
  const navigate = useNavigate();
  const createApplication = useCreateApplication(projectId);
  const { data: features } = useFeatures();

  const [appName, setAppName] = useState("");
  const [appSlug, setAppSlug] = useState("");
  const [appSlugManual, setAppSlugManual] = useState(false);
  const [appType, setAppType] = useState<"image" | "git">("image");
  const [sourceImage, setSourceImage] = useState("");
  const [sourceRepo, setSourceRepo] = useState("");
  const [dockerfilePath, setDockerfilePath] = useState("Dockerfile");
  const [buildType, setBuildType] = useState("dockerfile");
  const [appError, setAppError] = useState("");

  useEffect(() => {
    if (!open) {
      setAppName("");
      setAppSlug("");
      setAppSlugManual(false);
      setAppType("image");
      setSourceImage("");
      setSourceRepo("");
      setDockerfilePath("Dockerfile");
      setBuildType("dockerfile");
      setAppError("");
    }
  }, [open]);

  const handleCreateApp = async () => {
    setAppError("");
    if (!appName.trim()) {
      setAppError("Application name is required");
      return;
    }
    if (appType === "image" && !sourceImage.trim()) {
      setAppError("Image name is required");
      return;
    }
    if (appType === "git" && !sourceRepo.trim()) {
      setAppError("Repository URL is required");
      return;
    }
    try {
      const application = await createApplication.mutateAsync({
        name: appName.trim(),
        slug: appSlug || undefined,
        type: appType,
        ...(appType === "image"
          ? { source_image: sourceImage, build_type: "image" }
          : {
              source_repo: sourceRepo,
              dockerfile_path: dockerfilePath,
              build_type: buildType,
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
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>New Application</DialogTitle>
          <DialogDescription>
            Add an application to your project.
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-4 py-2">
          {appError && (
            <div className="bg-destructive/10 text-destructive rounded-md px-3 py-2 text-sm">
              {appError}
            </div>
          )}
          <div className="space-y-2">
            <Label htmlFor="app-name">Application Name</Label>
            <Input
              id="app-name"
              value={appName}
              onChange={(e) => {
                setAppName(e.target.value);
                if (!appSlugManual) {
                  setAppSlug(slugify(e.target.value));
                }
              }}
              placeholder="my-api"
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="app-slug">Slug</Label>
            <Input
              id="app-slug"
              value={appSlug}
              onChange={(e) => {
                setAppSlug(slugify(e.target.value));
                setAppSlugManual(true);
              }}
              placeholder={appName ? slugify(appName) : "auto-generated"}
            />
            <p className="text-muted-foreground text-xs">
              Used in container naming. Auto-generated from name unless
              overridden.
            </p>
          </div>
          <div className="space-y-2">
            <Label>Source Type</Label>
            <ToggleGroup
              variant="outline"
              value={[appType]}
              onValueChange={(v) => {
                if (v.length > 0) setAppType(v[0] as "image" | "git");
              }}
              className="justify-start"
            >
              <ToggleGroupItem value="image">Docker Image</ToggleGroupItem>
              <ToggleGroupItem value="git">Git Repository</ToggleGroupItem>
            </ToggleGroup>
          </div>
          {appType === "image" ? (
            <div className="space-y-2">
              <Label htmlFor="source-image">Docker Image</Label>
              <Input
                id="source-image"
                value={sourceImage}
                onChange={(e) => setSourceImage(e.target.value)}
                placeholder="nginx:latest"
              />
            </div>
          ) : (
            <>
              <div className="space-y-2">
                <Label htmlFor="source-repo">Repository URL</Label>
                <Input
                  id="source-repo"
                  value={sourceRepo}
                  onChange={(e) => setSourceRepo(e.target.value)}
                  placeholder="https://github.com/user/repo.git"
                />
              </div>
              <div className="space-y-2">
                <Label>Build Type</Label>
                <ToggleGroup
                  variant="outline"
                  value={[buildType]}
                  onValueChange={(v) => {
                    if (v.length > 0) setBuildType(v[0]);
                  }}
                  className="justify-start"
                >
                  <ToggleGroupItem value="dockerfile">
                    Dockerfile
                  </ToggleGroupItem>
                  <ToggleGroupItem value="buildpacks">
                    Buildpacks
                  </ToggleGroupItem>
                  <ToggleGroupItem value="railpack">Railpack</ToggleGroupItem>
                </ToggleGroup>
                {buildType === "railpack" &&
                  features?.buildkit_available === false && (
                    <Alert variant="warning">
                      <TriangleAlert />
                      <AlertTitle>Warning</AlertTitle>
                      <AlertDescription>
                        BuildKit is not reachable. Railpack builds will fail.
                      </AlertDescription>
                    </Alert>
                  )}
              </div>
              {buildType === "dockerfile" && (
                <div className="space-y-2">
                  <Label htmlFor="dockerfile-path">Dockerfile Path</Label>
                  <Input
                    id="dockerfile-path"
                    value={dockerfilePath}
                    onChange={(e) => setDockerfilePath(e.target.value)}
                    placeholder="Dockerfile"
                  />
                </div>
              )}
            </>
          )}
        </div>
        <DialogFooter>
          <Button
            onClick={handleCreateApp}
            disabled={createApplication.isPending}
          >
            {createApplication.isPending && (
              <Loader2 className="mr-1 h-4 w-4 animate-spin" />
            )}
            Create Application
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
