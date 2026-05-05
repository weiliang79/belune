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
        <AlertDialogFooter>
          <AlertDialogCancel>Cancel</AlertDialogCancel>
          <AlertDialogAction
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
