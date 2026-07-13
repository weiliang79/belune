import { toast } from "sonner";
import {
  CopyIcon,
  MoreHorizontal,
  PencilIcon,
  Trash2Icon,
} from "lucide-react";
import { Card, CardHeader } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from "@/components/ui/alert-dialog";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { useRemoveDomain } from "@/lib/hooks/use-domains";
import type { DomainExpanded } from "@/lib/types";
import { DomainTLSBadge } from "./domain-tls-badge";

interface Props {
  projectId: string;
  applicationId: string;
  domain: DomainExpanded;
  onEdit: () => void;
}

export function DomainRow({ projectId, applicationId, domain, onEdit }: Props) {
  const removeDomain = useRemoveDomain(projectId, applicationId);

  const handleCopyHostname = () => {
    navigator.clipboard
      .writeText(domain.hostname)
      .then(() => toast.success("Hostname copied"))
      .catch(() => toast.error("Failed to copy"));
  };

  const featureCount = domain.features?.length ?? 0;

  return (
    <Card>
      <CardHeader>
        <div className="flex items-center justify-between">
          {/* Route features moved into the edit dialog, so there is nothing left
              to expand — the row is just a summary now. */}
          <div className="flex min-w-0 items-center gap-2">
            <span className="font-mono text-sm">{domain.hostname}</span>
            <DomainTLSBadge
              domain={domain}
              projectId={projectId}
              applicationId={applicationId}
            />
            {domain.force_https && <Badge variant="outline">HTTPS</Badge>}
            {domain.container_port && (
              <Badge variant="outline">:{domain.container_port}</Badge>
            )}
            {featureCount > 0 && (
              <Badge variant="default">
                {featureCount} feature{featureCount > 1 ? "s" : ""}
              </Badge>
            )}
          </div>
          <div className="flex items-center gap-1">
            <AlertDialog>
              <DropdownMenu>
                <DropdownMenuTrigger
                  render={
                    <Button size="icon" variant="ghost" aria-label="Actions" />
                  }
                >
                  <MoreHorizontal className="h-4 w-4" />
                </DropdownMenuTrigger>
                <DropdownMenuContent align="end">
                  <DropdownMenuItem onClick={onEdit}>
                    <PencilIcon aria-hidden="true" />
                    Edit
                  </DropdownMenuItem>
                  <DropdownMenuItem onClick={handleCopyHostname}>
                    <CopyIcon aria-hidden="true" />
                    Copy hostname
                  </DropdownMenuItem>
                  <DropdownMenuSeparator />
                  <AlertDialogTrigger
                    render={
                      <DropdownMenuItem variant="destructive" />
                    }
                    nativeButton={false}
                  >
                    <Trash2Icon aria-hidden="true" />
                    Delete
                  </AlertDialogTrigger>
                </DropdownMenuContent>
              </DropdownMenu>
              <AlertDialogContent>
                <AlertDialogHeader>
                  <AlertDialogTitle>Delete {domain.hostname}?</AlertDialogTitle>
                  <AlertDialogDescription>
                    This will delete the domain and all its route features.
                  </AlertDialogDescription>
                </AlertDialogHeader>
                <AlertDialogFooter>
                  <AlertDialogCancel>Cancel</AlertDialogCancel>
                  <AlertDialogAction
                    onClick={() => {
                      toast.promise(removeDomain.mutateAsync(domain.id), {
                        loading: "Deleting domain...",
                        success: "Domain deleted",
                        error: (err) => err.message,
                      });
                    }}
                    className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
                  >
                    Delete
                  </AlertDialogAction>
                </AlertDialogFooter>
              </AlertDialogContent>
            </AlertDialog>
          </div>
        </div>
      </CardHeader>

    </Card>
  );
}

