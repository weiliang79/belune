import { useMemo, useState } from "react";
import { createFileRoute } from "@tanstack/react-router";
import { useForm } from "@tanstack/react-form";
import { z } from "zod";
import type { ColumnDef } from "@tanstack/react-table";
import { RouteError } from "@/lib/components/route-error";
import { toast } from "sonner";
import { useAuthStore } from "@/lib/stores/auth";
import {
  useUsers,
  useCreateUser,
  useUpdateUserRole,
  useDeleteUser,
  useResetUserPassword,
} from "@/lib/hooks/use-users";
import {
  useInvitations,
  useInviteUser,
  useRevokeInvitation,
} from "@/lib/hooks/use-invitations";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { PageHeader } from "@/components/ui/page-header";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Badge } from "@/components/ui/badge";
import {
  ShieldIcon,
  UserIcon,
  SearchIcon,
  UsersIcon,
  MailIcon,
} from "lucide-react";
import { initialsOf } from "@/lib/utils/initials";
import { formatDateTime } from "@/lib/utils/format";
import type { User, Invitation } from "@/lib/types";
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
import { DataTable, buildActionColumnDef } from "@/components/ui/data-table";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";

export const Route = createFileRoute("/_app/team")({
  component: TeamSettingsPage,
  errorComponent: RouteError,
});

function displayName(user: {
  first_name?: string;
  last_name?: string;
  username?: string;
  email: string;
}) {
  const full = `${user.first_name ?? ""} ${user.last_name ?? ""}`.trim();
  return full || user.username || user.email.split("@")[0];
}

function TeamSettingsPage() {
  const currentUser = useAuthStore((s) => s.user);
  const { data: users, isLoading } = useUsers();

  const [search, setSearch] = useState("");
  const [createOpen, setCreateOpen] = useState(false);
  const [inviteOpen, setInviteOpen] = useState(false);
  const [resetPasswordUser, setResetPasswordUser] = useState<{
    id: string;
    email: string;
  } | null>(null);
  const [deleteUser, setDeleteUser] = useState<{
    id: string;
    email: string;
  } | null>(null);

  const selfId = currentUser?.id;
  const columns = useMemo<ColumnDef<User>[]>(
    () => [
      {
        id: "member",
        header: "Member",
        // Combined accessor so the global filter matches name and email.
        accessorFn: (u) => `${displayName(u)} ${u.email}`,
        cell: ({ row: { original: user } }) => {
          const name = displayName(user);
          return (
            <div className="flex items-center gap-2.5">
              <span
                className="grid size-8 shrink-0 place-items-center rounded-full text-xs font-semibold text-white"
                style={{
                  background:
                    "linear-gradient(140deg, var(--brand), var(--brand-press))",
                }}
                aria-hidden="true"
              >
                {initialsOf(name)}
              </span>
              <div className="min-w-0">
                <div className="flex items-center gap-1.5">
                  <span className="truncate font-medium">{name}</span>
                  {user.id === selfId && (
                    <Badge
                      variant="outline"
                      className="px-1.5 py-0 text-[10px]"
                    >
                      you
                    </Badge>
                  )}
                </div>
                <span className="text-text-faint truncate text-xs">
                  {user.email}
                </span>
              </div>
            </div>
          );
        },
      },
      {
        id: "role",
        header: "Role",
        accessorKey: "role",
        enableGlobalFilter: false,
        cell: ({ row: { original: user } }) => (
          <UserRoleCell user={user} isSelf={user.id === selfId} />
        ),
      },
      {
        id: "created_at",
        header: "Joined",
        accessorKey: "created_at",
        enableGlobalFilter: false,
        meta: { className: "text-muted-foreground text-sm" },
        cell: ({ row: { original: user } }) =>
          user.created_at ? formatDateTime(user.created_at) : "—",
      },
      {
        id: "last_active_at",
        header: "Last active",
        accessorKey: "last_active_at",
        enableGlobalFilter: false,
        meta: { className: "text-muted-foreground text-sm" },
        cell: ({ row: { original: user } }) =>
          user.last_active_at ? formatDateTime(user.last_active_at) : "—",
      },
      buildActionColumnDef({
        meta: { headerClassName: "text-right", className: "text-right" },
        cell: ({ row: { original: user } }) =>
          user.id === selfId ? null : (
            <div className="flex justify-end gap-2">
              <Button
                variant="outline"
                size="sm"
                onClick={() =>
                  setResetPasswordUser({ id: user.id, email: user.email })
                }
              >
                Reset Password
              </Button>
              <Button
                variant="destructive"
                size="sm"
                onClick={() =>
                  setDeleteUser({ id: user.id, email: user.email })
                }
              >
                Delete
              </Button>
            </div>
          ),
      }),
    ],
    [selfId],
  );

  return (
    <div className="space-y-6">
      <PageHeader
        icon={<UsersIcon className="size-5" />}
        title={
          <>
            Team Members
            {users && (
              <span className="text-muted-foreground ml-2 text-base font-normal">
                {users.length} {users.length === 1 ? "person" : "people"}
              </span>
            )}
          </>
        }
        description="Manage who has access to this server and their permissions."
      />

      <Card>
        <CardHeader className="flex flex-row items-center justify-between gap-3">
          <CardTitle className="flex items-center gap-2">
            <UsersIcon aria-hidden="true" className="size-4" />
            Active Members
          </CardTitle>
          <div className="flex items-center gap-2">
            <div className="relative hidden sm:block">
              <SearchIcon
                aria-hidden="true"
                className="text-text-faint pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2"
              />
              <Input
                value={search}
                onChange={(e) => setSearch(e.target.value)}
                placeholder="Search by name or email…"
                aria-label="Search members"
                className="w-56 pl-9"
              />
            </div>
            <Button
              size="sm"
              variant="outline"
              onClick={() => setInviteOpen(true)}
            >
              Invite by Email
            </Button>
            <Button size="sm" onClick={() => setCreateOpen(true)}>
              Add User
            </Button>
          </div>
        </CardHeader>
        <CardContent>
          <DataTable
            columns={columns}
            data={users ?? []}
            isLoading={isLoading}
            getRowId={(u) => u.id}
            enableSorting
            globalFilter={search}
            onGlobalFilterChange={setSearch}
            emptyMessage={
              search.trim()
                ? "No members match your search."
                : "No members yet."
            }
          />
        </CardContent>
      </Card>

      <PendingInvitationsCard />

      <CreateUserDialog open={createOpen} onOpenChange={setCreateOpen} />
      <InviteUserDialog open={inviteOpen} onOpenChange={setInviteOpen} />

      {resetPasswordUser && (
        <ResetPasswordDialog
          userId={resetPasswordUser.id}
          email={resetPasswordUser.email}
          open={true}
          onOpenChange={(open) => !open && setResetPasswordUser(null)}
        />
      )}

      {deleteUser && (
        <DeleteUserDialog
          userId={deleteUser.id}
          email={deleteUser.email}
          open={true}
          onOpenChange={(open) => !open && setDeleteUser(null)}
        />
      )}
    </div>
  );
}

function RoleBadge({ role }: { role: string }) {
  const isAdmin = role === "admin";
  return (
    <Badge variant={isAdmin ? "default" : "secondary"} className="gap-1">
      {isAdmin ? (
        <ShieldIcon aria-hidden="true" className="size-3" />
      ) : (
        <UserIcon aria-hidden="true" className="size-3" />
      )}
      <span className="capitalize">{role}</span>
    </Badge>
  );
}

function UserRoleCell({ user, isSelf }: { user: User; isSelf: boolean }) {
  const updateRole = useUpdateUserRole();

  const handleRoleChange = (newRole: string) => {
    if (newRole === user.role) return;
    toast.promise(updateRole.mutateAsync({ userId: user.id, role: newRole }), {
      loading: "Updating role...",
      success: `Role updated to ${newRole}`,
      error: (err) => err.message,
    });
  };

  if (isSelf) return <RoleBadge role={user.role} />;

  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        render={<button type="button" className="cursor-pointer" />}
      >
        <RoleBadge role={user.role} />
      </DropdownMenuTrigger>
      <DropdownMenuContent>
        <DropdownMenuItem onClick={() => handleRoleChange("admin")}>
          <ShieldIcon aria-hidden="true" />
          Admin
        </DropdownMenuItem>
        <DropdownMenuItem onClick={() => handleRoleChange("member")}>
          <UserIcon aria-hidden="true" />
          Member
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

function CreateUserDialog({
  open,
  onOpenChange,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const createUser = useCreateUser();

  const form = useForm({
    defaultValues: { email: "", password: "", role: "member" },
    onSubmit: async ({ value }) => {
      toast.promise(
        createUser
          .mutateAsync({
            email: value.email,
            password: value.password,
            role: value.role,
          })
          .then(() => {
            form.reset();
            onOpenChange(false);
          }),
        {
          loading: "Creating user...",
          success: "User created",
          error: (err) => err.message,
        },
      );
    },
  });

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Add User</DialogTitle>
          <DialogDescription>
            Create a new user account for your team.
          </DialogDescription>
        </DialogHeader>
        <form
          onSubmit={(e) => {
            e.preventDefault();
            e.stopPropagation();
            form.handleSubmit();
          }}
          className="space-y-4"
        >
          <form.Field
            name="email"
            validators={{
              onChange: z.string().email("Email is required"),
            }}
            children={(field) => (
              <div className="space-y-2">
                <Label htmlFor="email">Email</Label>
                <Input
                  id="email"
                  type="email"
                  value={field.state.value}
                  onBlur={field.handleBlur}
                  onChange={(e) => field.handleChange(e.target.value)}
                  placeholder="user@example.com"
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
          <form.Field
            name="password"
            validators={{
              onChange: z
                .string()
                .min(8, "Password must be at least 8 characters"),
            }}
            children={(field) => (
              <div className="space-y-2">
                <Label htmlFor="password">Password</Label>
                <Input
                  id="password"
                  type="password"
                  value={field.state.value}
                  onBlur={field.handleBlur}
                  onChange={(e) => field.handleChange(e.target.value)}
                  placeholder="At least 8 characters"
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
          <form.Field
            name="role"
            children={(field) => (
              <div className="space-y-2">
                <Label htmlFor="role">Role</Label>
                <div className="flex gap-2">
                  <Button
                    type="button"
                    variant={
                      field.state.value === "member" ? "default" : "outline"
                    }
                    size="sm"
                    onClick={() => field.handleChange("member")}
                  >
                    Member
                  </Button>
                  <Button
                    type="button"
                    variant={
                      field.state.value === "admin" ? "default" : "outline"
                    }
                    size="sm"
                    onClick={() => field.handleChange("admin")}
                  >
                    Admin
                  </Button>
                </div>
              </div>
            )}
          />
          <DialogFooter>
            <form.Subscribe
              selector={(s) => s.isSubmitting}
              children={(isSubmitting) => (
                <Button type="submit" disabled={isSubmitting}>
                  {isSubmitting ? "Creating..." : "Create User"}
                </Button>
              )}
            />
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

function ResetPasswordDialog({
  userId,
  email,
  open,
  onOpenChange,
}: {
  userId: string;
  email: string;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const resetPassword = useResetUserPassword();

  const form = useForm({
    defaultValues: { password: "" },
    onSubmit: async ({ value }) => {
      toast.promise(
        resetPassword
          .mutateAsync({ userId, password: value.password })
          .then(() => {
            form.reset();
            onOpenChange(false);
          }),
        {
          loading: "Resetting password...",
          success: `Password reset for ${email}`,
          error: (err) => err.message,
        },
      );
    },
  });

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Reset Password</DialogTitle>
          <DialogDescription>Set a new password for {email}.</DialogDescription>
        </DialogHeader>
        <form
          onSubmit={(e) => {
            e.preventDefault();
            e.stopPropagation();
            form.handleSubmit();
          }}
          className="space-y-4"
        >
          <form.Field
            name="password"
            validators={{
              onChange: z
                .string()
                .min(8, "Password must be at least 8 characters"),
            }}
            children={(field) => (
              <div className="space-y-2">
                <Label htmlFor="new-password">New Password</Label>
                <Input
                  id="new-password"
                  type="password"
                  value={field.state.value}
                  onBlur={field.handleBlur}
                  onChange={(e) => field.handleChange(e.target.value)}
                  placeholder="At least 8 characters"
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
          <DialogFooter>
            <form.Subscribe
              selector={(s) => s.isSubmitting}
              children={(isSubmitting) => (
                <Button type="submit" disabled={isSubmitting}>
                  {isSubmitting ? "Resetting..." : "Reset Password"}
                </Button>
              )}
            />
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

function DeleteUserDialog({
  userId,
  email,
  open,
  onOpenChange,
}: {
  userId: string;
  email: string;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const deleteUserMutation = useDeleteUser();

  const handleDelete = () => {
    toast.promise(
      deleteUserMutation.mutateAsync(userId).then(() => {
        onOpenChange(false);
      }),
      {
        loading: "Deleting user...",
        success: `User ${email} deleted`,
        error: (err) => err.message,
      },
    );
  };

  return (
    <AlertDialog open={open} onOpenChange={onOpenChange}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>Delete {email}?</AlertDialogTitle>
          <AlertDialogDescription>
            This will permanently remove this user account. Their projects will
            remain but will no longer have an owner.
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>Cancel</AlertDialogCancel>
          <AlertDialogAction
            onClick={handleDelete}
            className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
          >
            {deleteUserMutation.isPending ? "Deleting..." : "Delete"}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}

function PendingInvitationsCard() {
  const { data: invitations, isLoading } = useInvitations();
  const revoke = useRevokeInvitation();

  const handleRevoke = (id: string, email: string) => {
    toast.promise(revoke.mutateAsync(id), {
      loading: "Revoking invitation…",
      success: `Invitation for ${email} revoked`,
      error: (err) => err.message,
    });
  };

  const columns = useMemo<ColumnDef<Invitation>[]>(
    () => [
      {
        id: "email",
        header: "Email",
        accessorKey: "email",
        meta: { className: "font-medium" },
        cell: ({ row: { original: inv } }) => inv.email,
      },
      {
        id: "role",
        header: "Role",
        accessorKey: "role",
        cell: ({ row: { original: inv } }) => (
          <Badge variant={inv.role === "admin" ? "default" : "secondary"}>
            {inv.role}
          </Badge>
        ),
      },
      {
        id: "expires_at",
        header: "Expires",
        accessorKey: "expires_at",
        meta: { className: "text-muted-foreground text-sm" },
        cell: ({ row: { original: inv } }) => formatDateTime(inv.expires_at),
      },
      buildActionColumnDef({
        meta: { headerClassName: "text-right", className: "text-right" },
        cell: ({ row: { original: inv } }) => (
          <Button
            variant="outline"
            size="sm"
            onClick={() => handleRevoke(inv.id, inv.email)}
            disabled={revoke.isPending}
          >
            Revoke
          </Button>
        ),
      }),
    ],
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [revoke.isPending],
  );

  if (!isLoading && (!invitations || invitations.length === 0)) return null;

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <MailIcon aria-hidden="true" className="size-4" />
          Pending Invitations
        </CardTitle>
      </CardHeader>
      <CardContent>
        <DataTable
          columns={columns}
          data={invitations ?? []}
          isLoading={isLoading}
          getRowId={(inv) => inv.id}
          enableSorting
        />
      </CardContent>
    </Card>
  );
}

function InviteUserDialog({
  open,
  onOpenChange,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const inviteUser = useInviteUser();

  const form = useForm({
    defaultValues: { email: "", role: "member" },
    onSubmit: async ({ value }) => {
      toast.promise(
        inviteUser
          .mutateAsync({ email: value.email, role: value.role })
          .then(() => {
            form.reset();
            onOpenChange(false);
          }),
        {
          loading: "Sending invitation…",
          success: "Invitation sent",
          error: (err) => err.message,
        },
      );
    },
  });

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Invite User</DialogTitle>
          <DialogDescription>
            Send an email invitation with a sign-up link.
          </DialogDescription>
        </DialogHeader>
        <form
          onSubmit={(e) => {
            e.preventDefault();
            e.stopPropagation();
            form.handleSubmit();
          }}
          className="space-y-4"
        >
          <form.Field
            name="email"
            validators={{ onChange: z.string().email("Email is required") }}
            children={(field) => (
              <div className="space-y-2">
                <Label htmlFor="invite-email">Email</Label>
                <Input
                  id="invite-email"
                  type="email"
                  value={field.state.value}
                  onBlur={field.handleBlur}
                  onChange={(e) => field.handleChange(e.target.value)}
                  placeholder="user@example.com"
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
          <form.Field
            name="role"
            children={(field) => (
              <div className="space-y-2">
                <Label>Role</Label>
                <div className="flex gap-2">
                  <Button
                    type="button"
                    variant={
                      field.state.value === "member" ? "default" : "outline"
                    }
                    size="sm"
                    onClick={() => field.handleChange("member")}
                  >
                    Member
                  </Button>
                  <Button
                    type="button"
                    variant={
                      field.state.value === "admin" ? "default" : "outline"
                    }
                    size="sm"
                    onClick={() => field.handleChange("admin")}
                  >
                    Admin
                  </Button>
                </div>
              </div>
            )}
          />
          <DialogFooter>
            <form.Subscribe
              selector={(s) => s.isSubmitting}
              children={(isSubmitting) => (
                <Button type="submit" disabled={isSubmitting}>
                  {isSubmitting ? "Sending…" : "Send Invitation"}
                </Button>
              )}
            />
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
