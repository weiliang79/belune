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
import { useDeleteApplication } from "@/lib/hooks/use-applications";

interface Props {
  projectId: string;
  applicationId: string;
  applicationName: string;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

export function DeleteApplicationDialog({
  projectId,
  applicationId,
  applicationName,
  open,
  onOpenChange,
}: Props) {
  const navigate = useNavigate();
  const deleteApplication = useDeleteApplication(projectId);
  const inputId = useId();
  const [confirmText, setConfirmText] = useState("");

  // Clear on *open*, not on close. This component stays mounted between
  // openings, so without a reset a user who typed the name, cancelled, then
  // reopened would find the confirmation already satisfied — one click from
  // deleting, which is exactly what typing the name is meant to prevent.
  // Resetting on open rather than close also avoids the field visibly emptying
  // during the close animation.
  useEffect(() => {
    if (open) setConfirmText("");
  }, [open]);

  // Trimmed because a trailing space from copy-paste is not a different
  // application, but otherwise exact: matching case is the deliberate act.
  const confirmed = confirmText.trim() === applicationName.trim();

  return (
    <AlertDialog open={open} onOpenChange={onOpenChange}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>Delete {applicationName}?</AlertDialogTitle>
          <AlertDialogDescription>
            This will stop the running container and permanently delete this
            application. This action cannot be undone.
          </AlertDialogDescription>
        </AlertDialogHeader>

        <div className="space-y-2">
          <Label htmlFor={inputId} className="font-normal">
            Type <span className="text-foreground font-medium">{applicationName}</span>{" "}
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
  );
}
