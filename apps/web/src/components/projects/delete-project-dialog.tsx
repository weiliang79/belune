import { useEffect, useId, useState } from "react";
import { useNavigate } from "@tanstack/react-router";
import { toast } from "sonner";
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
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { useDeleteProject } from "@/lib/hooks/use-projects";

interface Props {
  projectId: string;
  projectName: string;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

/**
 * Mirrors DeleteApplicationDialog — deliberately, so the two most destructive
 * actions in the product ask for the same thing in the same way. This one
 * destroys strictly more: every application in the project goes with it.
 */
export function DeleteProjectDialog({
  projectId,
  projectName,
  open,
  onOpenChange,
}: Props) {
  const navigate = useNavigate();
  const deleteProject = useDeleteProject();
  const inputId = useId();
  const [confirmText, setConfirmText] = useState("");

  // Clear on *open*, not on close: this component stays mounted between
  // openings, so without a reset a user who typed the name, cancelled and
  // reopened would find the confirmation already satisfied — one click from
  // deleting, which is what typing the name exists to prevent.
  useEffect(() => {
    if (open) setConfirmText("");
  }, [open]);

  // Trimmed because a trailing space from copy-paste is not a different
  // project, but otherwise exact: matching case is the deliberate act.
  const confirmed = confirmText.trim() === projectName.trim();

  return (
    <AlertDialog open={open} onOpenChange={onOpenChange}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>Delete {projectName}?</AlertDialogTitle>
          <AlertDialogDescription>
            This will permanently delete the project, all its applications, and
            stop all running containers. This action cannot be undone.
          </AlertDialogDescription>
        </AlertDialogHeader>

        <div className="space-y-2">
          <Label htmlFor={inputId} className="font-normal">
            Type{" "}
            <span className="text-foreground font-medium">{projectName}</span>{" "}
            to confirm.
          </Label>
          <Input
            id={inputId}
            value={confirmText}
            onChange={(e) => setConfirmText(e.target.value)}
            autoComplete="off"
            autoCorrect="off"
            spellCheck={false}
          />
        </div>

        <AlertDialogFooter>
          <AlertDialogCancel>Cancel</AlertDialogCancel>
          <AlertDialogAction
            variant="destructive-solid"
            disabled={!confirmed}
            onClick={() => {
              toast.promise(
                deleteProject.mutateAsync(projectId).then(() => {
                  navigate({ to: "/projects" });
                }),
                {
                  loading: "Deleting project...",
                  success: "Project deleted",
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
  );
}
