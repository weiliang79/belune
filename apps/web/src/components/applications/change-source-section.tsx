import { useState } from "react";
import { toast } from "sonner";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { useChangeApplicationSource } from "@/lib/hooks/use-applications";
import type { Application } from "@/lib/types";

/**
 * Switches an application between building from git and running a prebuilt
 * image.
 *
 * The alternative used to be deleting and recreating, which is not equivalent:
 * deleting removes the persistent data volumes and cascades domains, env vars,
 * mounts, and deployment history, and re-adding the domains re-issues
 * certificates against Let's Encrypt's duplicate limit. Everything that belongs
 * to the application rather than to its source survives a switch.
 *
 * Deliberately a dialog rather than an editable field: type, build_type and the
 * source column have to change together, and a form that let them drift is how
 * incoherent rows were created before.
 */
export function ChangeSourceSection({
  projectId,
  applicationId,
  application,
}: {
  projectId: string;
  applicationId: string;
  application: Application;
}) {
  const [open, setOpen] = useState(false);
  const changeSource = useChangeApplicationSource(projectId, applicationId);

  const target = application.type === "git" ? "image" : "git";

  const [sourceImage, setSourceImage] = useState("");
  const [sourceRepo, setSourceRepo] = useState("");
  const [branch, setBranch] = useState("");
  const [buildType, setBuildType] = useState("railpack");
  const [gitToken, setGitToken] = useState("");

  const canSubmit =
    target === "image" ? sourceImage.trim() !== "" : sourceRepo.trim() !== "";

  const submit = () => {
    const data =
      target === "image"
        ? { type: "image" as const, source_image: sourceImage.trim() }
        : {
            type: "git" as const,
            source_repo: sourceRepo.trim(),
            branch: branch.trim(),
            build_type: buildType,
            git_token: gitToken || undefined,
          };

    toast.promise(changeSource.mutateAsync(data), {
      loading: "Changing source...",
      success: () => {
        setOpen(false);
        setSourceImage("");
        setSourceRepo("");
        setBranch("");
        setGitToken("");
        return "Source changed — deploy to apply it";
      },
      error: (err) => err.message,
    });
  };

  return (
    <Card>
      <CardHeader>
        <CardTitle>Source</CardTitle>
        <CardDescription>
          This application{" "}
          {application.type === "git"
            ? "builds from a git repository"
            : "runs a prebuilt image"}
          . Switching keeps its domains and certificates, volumes and their
          data, file mounts, environment variables, deploy hook, and deployment
          history — the container keeps serving until you deploy.
        </CardDescription>
      </CardHeader>
      <CardContent>
        <Button variant="outline" onClick={() => setOpen(true)}>
          {target === "image" ? "Switch to a prebuilt image" : "Switch to git"}
        </Button>
      </CardContent>

      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>
              {target === "image" ? "Switch to a prebuilt image" : "Switch to git"}
            </DialogTitle>
            <DialogDescription>
              {target === "image"
                ? "The repository, branch, build settings, git credentials and push webhook secret are removed. Everything else is kept."
                : "The image reference is removed and the application will be built from source instead. Everything else is kept."}
            </DialogDescription>
          </DialogHeader>

          {target === "image" ? (
            <div className="space-y-2">
              <Label>Image</Label>
              <Input
                value={sourceImage}
                onChange={(e) => setSourceImage(e.target.value)}
                placeholder="nginx:1.27"
              />
            </div>
          ) : (
            <div className="space-y-4">
              <div className="space-y-2">
                <Label>Repository URL</Label>
                <Input
                  value={sourceRepo}
                  onChange={(e) => setSourceRepo(e.target.value)}
                  placeholder="https://github.com/you/repo"
                />
              </div>
              <div className="space-y-2">
                <Label>Branch</Label>
                <Input
                  value={branch}
                  onChange={(e) => setBranch(e.target.value)}
                  placeholder="Leave blank for the repository default"
                />
              </div>
              <div className="space-y-2">
                <Label>Build method</Label>
                <Select
                  value={buildType}
                  onValueChange={(v) => setBuildType(v as string)}
                >
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="railpack">
                      Railpack (detects the stack)
                    </SelectItem>
                    <SelectItem value="dockerfile">Dockerfile</SelectItem>
                    <SelectItem value="buildpacks">Buildpacks</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <div className="space-y-2">
                <Label>Access token (private repositories only)</Label>
                <Input
                  type="password"
                  value={gitToken}
                  onChange={(e) => setGitToken(e.target.value)}
                  placeholder="Leave blank for a public repository"
                />
              </div>
            </div>
          )}

          <DialogFooter>
            <Button variant="outline" onClick={() => setOpen(false)}>
              Cancel
            </Button>
            <Button
              onClick={submit}
              disabled={!canSubmit || changeSource.isPending}
            >
              {changeSource.isPending ? "Changing..." : "Change source"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </Card>
  );
}
