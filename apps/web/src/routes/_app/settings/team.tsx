import { useState } from "react";
import { createFileRoute } from "@tanstack/react-router";
import { toast } from "sonner";
import { useAuthStore } from "@/lib/stores/auth";
import {
  useUsers,
  useCreateUser,
  useUpdateUserRole,
  useDeleteUser,
  useResetUserPassword,
} from "@/lib/hooks/use-users";
import { SettingsNav } from "@/lib/components/settings-nav";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Badge } from "@/components/ui/badge";
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

export const Route = createFileRoute("/_app/settings/team")({
  component: TeamSettingsPage,
});

function TeamSettingsPage() {
  const currentUser = useAuthStore((s) => s.user);
  const { data: users, isLoading } = useUsers();

  const [createOpen, setCreateOpen] = useState(false);
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
        <h1 className="text-2xl font-bold">Settings</h1>
        <p className="text-muted-foreground">
          Manage your account and platform settings.
        </p>
      </div>

      <SettingsNav />

      <Card>
        <CardHeader className="flex flex-row items-center justify-between">
          <CardTitle>Team Members</CardTitle>
          <Button size="sm" onClick={() => setCreateOpen(true)}>
            Add User
          </Button>
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
                  <TableHead>Email</TableHead>
                  <TableHead>Role</TableHead>
                  <TableHead>Created</TableHead>
                  <TableHead className="text-right">Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {users?.map((user) => (
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

      <CreateUserDialog open={createOpen} onOpenChange={setCreateOpen} />

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

function UserRow({
  user,
  isSelf,
  onResetPassword,
  onDelete,
}: {
  user: { id: string; email: string; role: string; created_at?: string };
  isSelf: boolean;
  onResetPassword: () => void;
  onDelete: () => void;
}) {
  const updateRole = useUpdateUserRole();

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
      <TableCell className="font-medium">
        {user.email}
        {isSelf && (
          <Badge variant="outline" className="ml-2">
            you
          </Badge>
        )}
      </TableCell>
      <TableCell>
        {isSelf ? (
          <Badge variant={user.role === "admin" ? "default" : "secondary"}>
            {user.role}
          </Badge>
        ) : (
          <DropdownMenu>
            <DropdownMenuTrigger
              render={<Badge variant={user.role === "admin" ? "default" : "secondary"} className="cursor-pointer" />}
            >
              {user.role}
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
        {user.created_at
          ? new Date(user.created_at).toLocaleDateString()
          : "-"}
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
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [role, setRole] = useState("member");
  const createUser = useCreateUser();

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();

    if (!email || !password) {
      toast.error("Email and password are required");
      return;
    }
    if (password.length < 8) {
      toast.error("Password must be at least 8 characters");
      return;
    }

    toast.promise(
      createUser.mutateAsync({ email, password, role }).then(() => {
        setEmail("");
        setPassword("");
        setRole("member");
        onOpenChange(false);
      }),
      {
        loading: "Creating user...",
        success: "User created",
        error: (err) => err.message,
      },
    );
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Add User</DialogTitle>
          <DialogDescription>
            Create a new user account for your team.
          </DialogDescription>
        </DialogHeader>
        <form onSubmit={handleSubmit} className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="email">Email</Label>
            <Input
              id="email"
              type="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              placeholder="user@example.com"
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="password">Password</Label>
            <Input
              id="password"
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              placeholder="At least 8 characters"
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="role">Role</Label>
            <div className="flex gap-2">
              <Button
                type="button"
                variant={role === "member" ? "default" : "outline"}
                size="sm"
                onClick={() => setRole("member")}
              >
                Member
              </Button>
              <Button
                type="button"
                variant={role === "admin" ? "default" : "outline"}
                size="sm"
                onClick={() => setRole("admin")}
              >
                Admin
              </Button>
            </div>
          </div>
          <DialogFooter>
            <Button type="submit" disabled={createUser.isPending}>
              {createUser.isPending ? "Creating..." : "Create User"}
            </Button>
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
  const [password, setPassword] = useState("");
  const resetPassword = useResetUserPassword();

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();

    if (password.length < 8) {
      toast.error("Password must be at least 8 characters");
      return;
    }

    toast.promise(
      resetPassword.mutateAsync({ userId, password }).then(() => {
        setPassword("");
        onOpenChange(false);
      }),
      {
        loading: "Resetting password...",
        success: `Password reset for ${email}`,
        error: (err) => err.message,
      },
    );
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Reset Password</DialogTitle>
          <DialogDescription>
            Set a new password for {email}.
          </DialogDescription>
        </DialogHeader>
        <form onSubmit={handleSubmit} className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="new-password">New Password</Label>
            <Input
              id="new-password"
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              placeholder="At least 8 characters"
            />
          </div>
          <DialogFooter>
            <Button type="submit" disabled={resetPassword.isPending}>
              {resetPassword.isPending ? "Resetting..." : "Reset Password"}
            </Button>
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
