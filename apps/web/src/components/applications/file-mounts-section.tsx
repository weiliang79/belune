import { useState } from "react";
import { toast } from "sonner";
import { EyeIcon, FileTextIcon, PencilIcon, Trash2Icon } from "lucide-react";
import {
  useFileMounts,
  useCreateFileMount,
  useUpdateFileMount,
  useDeleteFileMount,
  useRevealFileMount,
} from "@/lib/hooks/use-file-mounts";
import type { FileMount } from "@/lib/types";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { IconAction } from "@/components/ui/icon-action";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
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

interface Props {
  projectId: string;
  applicationId: string;
}

export function FileMountsSection({ projectId, applicationId }: Props) {
  const { data: mounts, isLoading } = useFileMounts(projectId, applicationId);
  const createMount = useCreateFileMount(projectId, applicationId);
  const updateMount = useUpdateFileMount(projectId, applicationId);
  const deleteMount = useDeleteFileMount(projectId, applicationId);
  const revealMount = useRevealFileMount(projectId, applicationId);

  const [dialogOpen, setDialogOpen] = useState(false);
  const [editing, setEditing] = useState<FileMount | null>(null);
  const [mountPath, setMountPath] = useState("");
  const [content, setContent] = useState("");
  const [isSecret, setIsSecret] = useState(false);
  const [fileMode, setFileMode] = useState("");
  // For a secret mount, content starts hidden; the user reveals it to edit in
  // place. `revealed` also disambiguates "empty because hidden" (keep stored)
  // from "explicitly cleared to empty" (replace) on save.
  const [revealed, setRevealed] = useState(false);

  const [removeTarget, setRemoveTarget] = useState<FileMount | null>(null);

  const openAdd = () => {
    setEditing(null);
    setMountPath("");
    setContent("");
    setIsSecret(false);
    setFileMode("");
    setRevealed(true); // new mount: content is directly editable
    setDialogOpen(true);
  };

  const openEdit = (fm: FileMount) => {
    setEditing(fm);
    setMountPath(fm.mount_path);
    setContent(fm.content ?? ""); // empty for secrets (masked)
    setIsSecret(fm.is_secret);
    setFileMode(fm.file_mode);
    // Non-secret content is already loaded and editable; secret content stays
    // hidden until the user reveals it.
    setRevealed(!fm.is_secret);
    setDialogOpen(true);
  };

  const reveal = () => {
    if (!editing) return;
    toast.promise(
      revealMount.mutateAsync(editing.id).then((res) => {
        setContent(res.content);
        setRevealed(true);
      }),
      { loading: "Revealing...", success: "Content revealed", error: (err) => err.message },
    );
  };

  const submit = () => {
    // A hidden (not-yet-revealed) secret means "keep the stored value" — omit
    // content so the backend preserves it. Once revealed, always send content
    // (even if the user cleared it to make an empty file).
    const keepSecret = !!editing && editing.is_secret && !revealed && content === "";

    const promise = editing
      ? updateMount.mutateAsync({
          fileMountId: editing.id,
          is_secret: isSecret,
          file_mode: fileMode || undefined,
          ...(keepSecret ? {} : { content }),
        })
      : createMount.mutateAsync({
          mount_path: mountPath,
          content,
          is_secret: isSecret,
          file_mode: fileMode || undefined,
        });

    toast.promise(
      promise.then(() => setDialogOpen(false)),
      {
        loading: editing ? "Saving..." : "Creating file mount...",
        success: editing
          ? "File mount saved — reload the application to apply it"
          : "File mount created — reload the application to mount it",
        error: (err) => err.message,
      },
    );
  };

  const submitRemove = () => {
    if (!removeTarget) return;
    toast.promise(
      deleteMount.mutateAsync(removeTarget.id).then(() => setRemoveTarget(null)),
      {
        loading: "Removing...",
        success: "File mount removed",
        error: (err) => err.message,
      },
    );
  };

  const pending = createMount.isPending || updateMount.isPending;

  return (
    <Card>
      <CardHeader>
        <CardTitle>File Mounts</CardTitle>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="flex items-start justify-between gap-4">
          <p className="text-muted-foreground text-sm">
            Mount config files into the container at a fixed path — content you
            provide here is written read-only on each deploy. Use it for config
            files an app reads at startup.
          </p>
          <Button size="sm" className="shrink-0" onClick={openAdd}>
            Add File
          </Button>
        </div>

        {isLoading ? (
          <div className="space-y-3">
            {[1, 2].map((i) => (
              <Skeleton key={i} className="h-14 w-full" />
            ))}
          </div>
        ) : !mounts || mounts.length === 0 ? (
          <div className="text-muted-foreground flex flex-col items-center gap-2 rounded-lg border border-dashed p-8 text-center text-sm">
            <FileTextIcon aria-hidden="true" className="size-6" />
            <p>No file mounts. Add one to inject a config file into this app.</p>
          </div>
        ) : (
          <div className="space-y-3">
            {mounts.map((fm) => (
              <div
                key={fm.id}
                className="flex items-center justify-between gap-3 rounded-lg border p-4"
              >
                <div className="flex min-w-0 items-center gap-3">
                  <div className="bg-elev text-text-muted grid size-9 shrink-0 place-items-center rounded-lg">
                    <FileTextIcon aria-hidden="true" className="size-4" />
                  </div>
                  <div className="min-w-0">
                    <div className="flex items-center gap-2">
                      <span className="truncate font-mono text-sm">
                        {fm.mount_path}
                      </span>
                      {fm.is_secret && (
                        <Badge variant="secondary" className="shrink-0">
                          Secret
                        </Badge>
                      )}
                    </div>
                    <div className="text-text-faint text-xs">
                      mode {fm.file_mode}
                    </div>
                  </div>
                </div>
                <div className="flex shrink-0 items-center justify-end gap-1">
                  <IconAction label="Edit" onClick={() => openEdit(fm)}>
                    <PencilIcon aria-hidden="true" className="size-4" />
                  </IconAction>
                  <IconAction
                    label="Remove"
                    destructive
                    onClick={() => setRemoveTarget(fm)}
                  >
                    <Trash2Icon aria-hidden="true" className="size-4" />
                  </IconAction>
                </div>
              </div>
            ))}
          </div>
        )}
      </CardContent>

      {/* Add / edit dialog */}
      <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>
              {editing ? "Edit File Mount" : "Add File Mount"}
            </DialogTitle>
            <DialogDescription>
              The file is written into the container read-only on the next
              deploy.
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-4 py-2">
            <div className="space-y-1.5">
              <Label htmlFor="fm-path">Mount path</Label>
              <Input
                id="fm-path"
                placeholder="/etc/app/config.yaml"
                className="font-mono"
                value={mountPath}
                disabled={!!editing}
                onChange={(e) => setMountPath(e.target.value)}
              />
              <p className="text-text-faint text-xs">
                Absolute file path inside the container, e.g.{" "}
                <code>/etc/nginx/nginx.conf</code>. The parent directory must
                exist in the image.
              </p>
            </div>
            <div className="space-y-1.5">
              <div className="flex items-center justify-between">
                <Label htmlFor="fm-content">Content</Label>
                {editing && editing.is_secret && !revealed && (
                  <Button
                    type="button"
                    size="sm"
                    variant="ghost"
                    className="h-7 gap-1.5 text-xs"
                    onClick={reveal}
                    disabled={revealMount.isPending}
                  >
                    <EyeIcon aria-hidden="true" className="size-3.5" />
                    {revealMount.isPending
                      ? "Revealing..."
                      : "Reveal current content"}
                  </Button>
                )}
              </div>
              <Textarea
                id="fm-content"
                className="min-h-40 font-mono text-sm"
                placeholder={
                  editing && editing.is_secret && !revealed
                    ? "•••••••• hidden — reveal to edit, or type to replace"
                    : "file contents..."
                }
                value={content}
                onChange={(e) => setContent(e.target.value)}
              />
            </div>
            <div className="flex items-center justify-between gap-4">
              <label className="flex items-center gap-2 text-sm">
                <Checkbox
                  checked={isSecret}
                  onCheckedChange={setIsSecret}
                />
                Secret (mask content in the UI)
              </label>
              <div className="flex items-center gap-2">
                <Label htmlFor="fm-mode" className="text-sm">
                  Mode
                </Label>
                <Input
                  id="fm-mode"
                  className="w-20 font-mono"
                  placeholder="0644"
                  value={fileMode}
                  onChange={(e) => setFileMode(e.target.value)}
                />
              </div>
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setDialogOpen(false)}>
              Cancel
            </Button>
            <Button
              onClick={submit}
              disabled={pending || (!editing && !mountPath.trim())}
            >
              {pending
                ? editing
                  ? "Saving..."
                  : "Creating..."
                : editing
                  ? "Save"
                  : "Add File"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Remove dialog */}
      <AlertDialog
        open={removeTarget !== null}
        onOpenChange={(open) => {
          if (!open) setRemoveTarget(null);
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Remove file mount?</AlertDialogTitle>
            <AlertDialogDescription>
              <span className="font-mono">{removeTarget?.mount_path}</span> will
              no longer be mounted once the application is reloaded.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction
              onClick={submitRemove}
              disabled={deleteMount.isPending}
            >
              Remove
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </Card>
  );
}
