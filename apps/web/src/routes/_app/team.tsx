import { useState } from "react";
import { createFileRoute } from "@tanstack/react-router";
import { useForm } from "@tanstack/react-form";
import { z } from "zod";
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
import { useInvitations, useInviteUser, useRevokeInvitation } from "@/lib/hooks/use-invitations";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Badge } from "@/components/ui/badge";
import { ShieldIcon, UserIcon, SearchIcon } from "lucide-react";
import { initialsOf } from "@/lib/utils/initials";
import { formatRelativeTime } from "@/lib/utils/format";
import type { User } from "@/lib/types";
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
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
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

function displayName(user: { first_name?: string; last_name?: string; username?: string; email: string }) {
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

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-semibold tracking-tight">
          Team Members
          {users && (
            <span className="text-muted-foreground ml-2 text-base font-normal">
              {users.length} {users.length === 1 ? "person" : "people"}
            </span>
          )}
        </h1>
        <p className="text-muted-foreground text-sm">
          Manage who has access to this server and their permissions.
        </p>
      </div>

      <Card>
        <CardHeader className="flex flex-row items-center justify-between gap-3">
          <CardTitle>Active members</CardTitle>
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
            <Button size="sm" variant="outline" onClick={() => setInviteOpen(true)}>
              Invite by Email
            </Button>
            <Button size="sm" onClick={() => setCreateOpen(true)}>
              Add User
            </Button>
          </div>
        </CardHeader>
        <CardContent>
          {isLoading ? (
            <div className="space-y-3">
              {[1, 2].map((i) => (
                <div
                  key={i}
                  className="bg-muted h-12 animate-pulse rounded"
                />
              ))}
            </div>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Member</TableHead>
                  <TableHead>Role</TableHead>
                  <TableHead>Joined</TableHead>
                  <TableHead>Last active</TableHead>
                  <TableHead className="text-right">Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {users
                  ?.filter((user) => {
                    const q = search.trim().toLowerCase();
                    if (!q) return true;
                    return (
                      user.email.toLowerCase().includes(q) ||
                      displayName(user).toLowerCase().includes(q)
                    );
                  })
                  .map((user) => (
                    <UserRow
                      key={user.id}
                      user={user}
                      isSelf={user.id === currentUser?.id}
                      onResetPassword={() =>
                        setResetPasswordUser({
                          id: user.id,
                          email: user.email,
                        })
                      }
                      onDelete={() =>
                        setDeleteUser({ id: user.id, email: user.email })
                      }
                    />
                  ))}
              </TableBody>
            </Table>
          )}
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

function UserRow({
  user,
  isSelf,
  onResetPassword,
  onDelete,
}: {
  user: User;
  isSelf: boolean;
  onResetPassword: () => void;
  onDelete: () => void;
}) {
  const updateRole = useUpdateUserRole();
  const name = displayName(user);

  const handleRoleChange = (newRole: string) => {
    if (newRole === user.role) return;
    toast.promise(updateRole.mutateAsync({ userId: user.id, role: newRole }), {
      loading: "Updating role...",
      success: `Role updated to ${newRole}`,
      error: (err) => err.message,
    });
  };

  return (
    <TableRow>
      <TableCell>
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
              {isSelf && (
                <Badge variant="outline" className="px-1.5 py-0 text-[10px]">
                  you
                </Badge>
              )}
            </div>
            <span className="text-text-faint truncate text-xs">
              {user.email}
            </span>
          </div>
        </div>
      </TableCell>
      <TableCell>
        {isSelf ? (
          <RoleBadge role={user.role} />
        ) : (
          <DropdownMenu>
            <DropdownMenuTrigger
              render={<button type="button" className="cursor-pointer" />}
            >
              <RoleBadge role={user.role} />
            </DropdownMenuTrigger>
            <DropdownMenuContent>
              <DropdownMenuItem onClick={() => handleRoleChange("admin")}>
                Admin
              </DropdownMenuItem>
              <DropdownMenuItem onClick={() => handleRoleChange("member")}>
                Member
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        )}
      </TableCell>
      <TableCell className="text-muted-foreground text-sm">
        {user.created_at ? formatRelativeTime(user.created_at) : "—"}
      </TableCell>
      <TableCell className="text-muted-foreground text-sm">
        {user.last_active_at ? formatRelativeTime(user.last_active_at) : "—"}
      </TableCell>
      <TableCell className="text-right">
        {!isSelf && (
          <div className="flex justify-end gap-2">
            <Button variant="outline" size="sm" onClick={onResetPassword}>
              Reset Password
            </Button>
            <Button variant="destructive" size="sm" onClick={onDelete}>
              Delete
            </Button>
          </div>
        )}
      </TableCell>
    </TableRow>
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
          <DialogDescription>
            Set a new password for {email}.
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

  if (!isLoading && (!invitations || invitations.length === 0)) return null;

  const handleRevoke = (id: string, email: string) => {
    toast.promise(revoke.mutateAsync(id), {
      loading: "Revoking invitation…",
      success: `Invitation for ${email} revoked`,
      error: (err) => err.message,
    });
  };

  return (
    <Card>
      <CardHeader>
        <CardTitle>Pending Invitations</CardTitle>
      </CardHeader>
      <CardContent>
        {isLoading ? (
          <div className="space-y-3">
            {[1, 2].map((i) => (
              <div key={i} className="bg-muted h-10 animate-pulse rounded" />
            ))}
          </div>
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Email</TableHead>
                <TableHead>Role</TableHead>
                <TableHead>Expires</TableHead>
                <TableHead className="text-right">Actions</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {invitations?.map((inv) => (
                <TableRow key={inv.id}>
                  <TableCell className="font-medium">{inv.email}</TableCell>
                  <TableCell>
                    <Badge variant={inv.role === "admin" ? "default" : "secondary"}>
                      {inv.role}
                    </Badge>
                  </TableCell>
                  <TableCell className="text-muted-foreground text-sm">
                    {new Date(inv.expires_at).toLocaleDateString()}
                  </TableCell>
                  <TableCell className="text-right">
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={() => handleRevoke(inv.id, inv.email)}
                      disabled={revoke.isPending}
                    >
                      Revoke
                    </Button>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}
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
                    variant={field.state.value === "member" ? "default" : "outline"}
                    size="sm"
                    onClick={() => field.handleChange("member")}
                  >
                    Member
                  </Button>
                  <Button
                    type="button"
                    variant={field.state.value === "admin" ? "default" : "outline"}
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
