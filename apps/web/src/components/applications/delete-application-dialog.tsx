import { useDialogBody } from "@/lib/hooks/use-dialog-body";
import { useId, useState } from "react";
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
  // Remount on every open so confirmText always starts blank — a user who
  // typed the name, cancelled, then reopened would otherwise find the
  // confirmation already satisfied. See useDialogBody.
  const body = useDialogBody(open, null);

  return (
    <AlertDialog open={open} onOpenChange={onOpenChange}>
      <AlertDialogContent>
        <DeleteApplicationDialogBody
          key={body.key}
          projectId={projectId}
          applicationId={applicationId}
          applicationName={applicationName}
          onDone={() => onOpenChange(false)}
        />
      </AlertDialogContent>
    </AlertDialog>
  );
}

function DeleteApplicationDialogBody({
  projectId,
  applicationId,
  applicationName,
  onDone,
}: {
  projectId: string;
  applicationId: string;
  applicationName: string;
  onDone: () => void;
}) {
  const navigate = useNavigate();
  const deleteApplication = useDeleteApplication(projectId);
  const inputId = useId();
  const [confirmText, setConfirmText] = useState("");

  // Trimmed because a trailing space from copy-paste is not a different
  // application, but otherwise exact: matching case is the deliberate act.
  const confirmed = confirmText.trim() === applicationName.trim();

  return (
    <>
      <AlertDialogHeader>
        <AlertDialogTitle>Delete {applicationName}?</AlertDialogTitle>
        <AlertDialogDescription>
          This will stop the running container and permanently delete this
          application — its persistent volumes, file mounts, domains,
          environment variables and deployment history included. This action
          cannot be undone.
        </AlertDialogDescription>
        {/* Backups are called out separately for the same reason as in
            DeleteProjectDialog: they are the one thing that survives losing
            the server, so an operator may reasonably believe they are a
            safety net here. They are not. Unlike a database — which offers
            keep-or-destroy at deletion time and writes a tombstone so its
            backups stay restorable — an application has no such choice and
            no tombstone: cleanupVolumeBackups erases every archive
            unconditionally, local files and remote objects both. Saying so
            is the whole point; the dialog previously named only the
            container, which is what the database dialog's own impact
            endpoint exists to avoid ("instead of implying only the
            container and its data are at stake").
            Static wording, not a count: there is no deletion-impact
            endpoint for applications, and a dialog is the wrong place to
            learn a number is missing. Worded to claim nothing about what
            this application actually has, so it reads correctly for an app
            with no volumes at all. */}
        <AlertDialogDescription className="text-destructive font-medium">
          Every backup of its volumes is destroyed as well, including copies
          already uploaded to a remote destination. Restore from them will no
          longer be possible.
        </AlertDialogDescription>
      </AlertDialogHeader>

      <div className="space-y-2">
        <Label htmlFor={inputId} className="font-normal">
          Type{" "}
          <span className="text-foreground font-medium">{applicationName}</span>{" "}
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
                onDone();
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
    </>
  );
}
