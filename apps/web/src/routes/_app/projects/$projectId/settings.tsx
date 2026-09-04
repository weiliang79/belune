import { createFileRoute } from "@tanstack/react-router";
import {
  useProject,
  useUpdateProject,
  useTransferProject,
  useUpdateProjectSharing,
} from "@/lib/hooks/use-projects";
import { useUsers } from "@/lib/hooks/use-users";
import { useAuthStore } from "@/lib/stores/auth";
import { useForm } from "@tanstack/react-form";
import { z } from "zod";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  InfoIcon,
  PencilIcon,
  Trash2,
  TriangleAlert,
  UserIcon,
  Users2Icon,
} from "lucide-react";
import { DeleteProjectDialog } from "@/components/projects/delete-project-dialog";
import { Separator } from "@/components/ui/separator";
import { formatDateTimeShort } from "@/lib/utils/format";
import { useState } from "react";

export const Route = createFileRoute("/_app/projects/$projectId/settings")({
  component: ProjectSettings,
});

function ProjectSettings() {
  const { projectId } = Route.useParams();
  const { data: project } = useProject(projectId);
  const updateProject = useUpdateProject(projectId);
  const [deleteOpen, setDeleteOpen] = useState(false);
  const currentUser = useAuthStore((s) => s.user);
  const canDelete =
    currentUser?.role === "admin" || currentUser?.id === project?.user_id;

  const form = useForm({
    defaultValues: {
      name: project?.name ?? "",
    },
    onSubmit: async ({ value }) => {
      toast.promise(updateProject.mutateAsync(value), {
        loading: "Saving...",
        success: "Project updated",
        error: (err) => err.message,
      });
    },
  });

  if (!project) return null;

  return (
    <div className="space-y-6">
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <InfoIcon aria-hidden="true" className="size-4" />
            Project Details
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-2 text-sm">
          <div className="flex justify-between">
            <span className="text-muted-foreground">ID</span>
            <span className="font-mono text-xs">{project.id}</span>
          </div>
          <div className="flex justify-between">
            <span className="text-muted-foreground">Slug</span>
            <span className="font-mono text-xs">{project.slug}</span>
          </div>
          <div className="flex justify-between">
            <span className="text-muted-foreground">Created</span>
            <span>{formatDateTimeShort(project.created_at)}</span>
          </div>
          <div className="flex justify-between">
            <span className="text-muted-foreground">Updated</span>
            <span>{formatDateTimeShort(project.updated_at)}</span>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <PencilIcon aria-hidden="true" className="size-4" />
            Edit Project
          </CardTitle>
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
                  <Label htmlFor="edit-name">Name</Label>
                  <Input
                    id="edit-name"
                    value={field.state.value}
                    onBlur={field.handleBlur}
                    onChange={(e) => field.handleChange(e.target.value)}
                  />
                </div>
              )}
            />
            <div className="flex justify-end">
              <form.Subscribe
                selector={(s) => s.isSubmitting}
                children={(isSubmitting) => (
                  <Button type="submit" disabled={isSubmitting}>
                    {isSubmitting ? "Saving..." : "Save"}
                  </Button>
                )}
              />
            </div>
          </form>
        </CardContent>
      </Card>

      <SharingCard
        projectId={projectId}
        ownerId={project.user_id}
        shared={project.shared}
      />

      <TransferOwnerCard
        projectId={projectId}
        currentOwnerId={project.user_id}
      />

      {canDelete && (
        <>
          <Separator />

          {/* ring-, not border-: Card draws its edge with `ring-1` (a box-shadow)
              and Tailwind's preflight zeroes border-width, so the previous
              `border-destructive/50` set a colour on a 0px border and rendered
              nothing. Matches the application Danger Zone. */}
          <Card className="bg-status-error-soft ring-status-error-line">
            <CardHeader>
              <CardTitle className="text-destructive flex items-center gap-2">
                <TriangleAlert aria-hidden="true" className="size-4" />
                Danger Zone
              </CardTitle>
            </CardHeader>
            <CardContent>
              <div className="space-y-6">
                <div className="flex flex-wrap items-center justify-between gap-3">
                  <div className="min-w-0">
                    <div className="flex items-center gap-2">
                      <Trash2
                        aria-hidden="true"
                        className="text-destructive size-4"
                      />
                      <p className="text-sm font-medium">Delete Project</p>
                    </div>
                    <p className="text-muted-foreground mt-1 text-xs">
                      Permanently deletes the project and everything in it —
                      every application and database, their containers and
                      volumes, and every backup taken of those databases. This
                      cannot be undone.
                    </p>
                  </div>
                  <Button
                    size="sm"
                    variant="destructive-solid"
                    onClick={() => setDeleteOpen(true)}
                    // "Delete" alone is ambiguous out of context, and this is the
                    // most destructive control in the product.
                    aria-label="Delete project"
                  >
                    Delete
                  </Button>
                </div>
              </div>
            </CardContent>
          </Card>

          <DeleteProjectDialog
            projectId={projectId}
            projectName={project.name}
            open={deleteOpen}
            onOpenChange={setDeleteOpen}
          />
        </>
      )}
    </div>
  );
}

function SharingCard({
  projectId,
  ownerId,
  shared,
}: {
  projectId: string;
  ownerId: string;
  shared: boolean;
}) {
  const currentUser = useAuthStore((s) => s.user);
  const isAdmin = currentUser?.role === "admin";
  const isOwner = currentUser?.id === ownerId;
  const updateSharing = useUpdateProjectSharing(projectId);

  // Not admin-only: a Member who owns the project must be able to share it,
  // or the role is hollow. Hidden (rather than disabled) for anyone who is
  // neither — the API would 403 them anyway.
  if (!isAdmin && !isOwner) return null;

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <Users2Icon aria-hidden="true" className="size-4" />
          Sharing
        </CardTitle>
      </CardHeader>
      <CardContent>
        <div className="flex items-center justify-between gap-3">
          <div className="min-w-0">
            <p className="text-sm font-medium">Share with every Member</p>
            <p className="text-muted-foreground mt-1 text-xs">
              When on, every Member in this install can view and work in this
              project — deploy, create databases, edit env vars. Deleting an
              application, database, or domain, transferring the project, and
              unsharing it stay owner-only.
            </p>
          </div>
          <Switch
            checked={shared}
            onCheckedChange={(next) => {
              toast.promise(updateSharing.mutateAsync(next), {
                loading: next ? "Sharing project..." : "Unsharing project...",
                success: next ? "Project shared" : "Project unshared",
                error: (err) => err.message,
              });
            }}
            disabled={updateSharing.isPending}
            aria-label="Share project with every Member"
          />
        </div>
      </CardContent>
    </Card>
  );
}

function TransferOwnerCard({
  projectId,
  currentOwnerId,
}: {
  projectId: string;
  currentOwnerId: string;
}) {
  const currentUser = useAuthStore((s) => s.user);
  const isAdmin = currentUser?.role === "admin";
  // Gated on isAdmin: the card returns null below for everyone else, but the
  // hook still runs, and /api/users is admin-only.
  const { data: users } = useUsers({ enabled: isAdmin });
  const transferProject = useTransferProject(projectId);
  const [selectedUserId, setSelectedUserId] = useState(currentOwnerId);

  if (!isAdmin) return null;

  const currentOwner = users?.find((u) => u.id === currentOwnerId);

  const handleTransfer = () => {
    if (selectedUserId === currentOwnerId) return;
    toast.promise(transferProject.mutateAsync(selectedUserId), {
      loading: "Transferring ownership...",
      success: "Project ownership transferred",
      error: (err) => err.message,
    });
  };

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <UserIcon aria-hidden="true" className="size-4" />
          Project Owner
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="space-y-2">
          <Label>Owner</Label>
          <Select
            value={selectedUserId}
            onValueChange={(v) => setSelectedUserId(v ?? "")}
          >
            <SelectTrigger>
              <SelectValue placeholder="Select owner" />
            </SelectTrigger>
            <SelectContent>
              {users?.map((user) => (
                <SelectItem key={user.id} value={user.id} icon={<UserIcon />}>
                  {user.email}
                  {user.id === currentOwnerId ? " (current)" : ""}
                </SelectItem>
              )) ?? (
                <SelectItem value={currentOwnerId} icon={<UserIcon />}>
                  {currentOwner?.email ?? currentOwnerId}
                </SelectItem>
              )}
            </SelectContent>
          </Select>
        </div>
        <div className="flex justify-end">
          <Button
            onClick={handleTransfer}
            disabled={
              selectedUserId === currentOwnerId || transferProject.isPending
            }
            // The visible label is just "Transfer"; the card title supplies the
            // context visually but not to a screen reader reaching the button.
            aria-label="Transfer project ownership"
          >
            {transferProject.isPending ? "Transferring..." : "Transfer"}
          </Button>
        </div>
      </CardContent>
    </Card>
  );
}
