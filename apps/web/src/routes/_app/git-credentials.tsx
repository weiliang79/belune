import { createFileRoute } from "@tanstack/react-router";
import { useState } from "react";
import { useForm, useStore } from "@tanstack/react-form";
import { z } from "zod";
import { RouteError } from "@/lib/components/route-error";
import {
  useGitCredentials,
  useCreateGitCredential,
  useUpdateGitCredential,
  useDeleteGitCredential,
} from "@/lib/hooks/use-git-credentials";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import { GitBranchIcon } from "lucide-react";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
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
import type { GitCredential } from "@/lib/types";
import { formatDate } from "@/lib/utils/format";

export const Route = createFileRoute("/_app/git-credentials")({
  component: GitCredentialsPage,
  errorComponent: RouteError,
});

const PROVIDERS = [
  { value: "github", label: "GitHub" },
  { value: "gitlab", label: "GitLab" },
  { value: "bitbucket", label: "Bitbucket" },
  { value: "generic", label: "Generic" },
] as const;

function providerLabel(provider: string) {
  return PROVIDERS.find((p) => p.value === provider)?.label ?? provider;
}

function GitCredentialsPage() {
  const { data: credentials, isLoading } = useGitCredentials();
  const [dialogOpen, setDialogOpen] = useState(false);
  const [editing, setEditing] = useState<GitCredential | null>(null);

  if (isLoading) {
    return (
      <div className="space-y-6">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">Git Credentials</h1>
          <p className="text-muted-foreground">Manage centralized git credentials for your applications.</p>
        </div>
        <Card>
          <CardContent className="space-y-3 py-4">
            {[1, 2].map((i) => (
              <Skeleton key={i} className="h-16 w-full" />
            ))}
          </CardContent>
        </Card>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">Git Credentials</h1>
          <p className="text-muted-foreground">
            Manage centralized git credentials for your applications.
          </p>
        </div>
        <Dialog
          open={dialogOpen}
          onOpenChange={(open) => {
            setDialogOpen(open);
            if (!open) setEditing(null);
          }}
        >
          <DialogTrigger render={<Button />}>
            Add Credential
          </DialogTrigger>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>
                {editing ? "Edit Credential" : "Add Git Credential"}
              </DialogTitle>
            </DialogHeader>
            <CredentialForm
              credential={editing}
              onSuccess={() => {
                setDialogOpen(false);
                setEditing(null);
              }}
            />
          </DialogContent>
        </Dialog>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>Credentials</CardTitle>
        </CardHeader>
        <CardContent>
          {!credentials || credentials.length === 0 ? (
            <p className="text-muted-foreground py-6 text-center text-sm">
              No git credentials configured. Add one to use across applications.
            </p>
          ) : (
            <div className="space-y-2">
              {credentials.map((cred) => (
                <CredentialRow
                  key={cred.id}
                  credential={cred}
                  onEdit={() => {
                    setEditing(cred);
                    setDialogOpen(true);
                  }}
                />
              ))}
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  );
}

function CredentialRow({
  credential,
  onEdit,
}: {
  credential: GitCredential;
  onEdit: () => void;
}) {
  const deleteCred = useDeleteGitCredential();

  return (
    <div className="hover:bg-card-hover flex items-center justify-between gap-3 rounded-lg border p-3 transition-colors">
      <div className="flex min-w-0 items-center gap-3">
        <div className="bg-elev text-text-muted grid size-9 shrink-0 place-items-center rounded-lg">
          <GitBranchIcon aria-hidden="true" className="size-4" />
        </div>
        <div className="min-w-0">
          <div className="flex items-center gap-2">
            <span className="truncate text-sm font-medium">{credential.name}</span>
            <Badge variant="secondary">{providerLabel(credential.provider)}</Badge>
          </div>
          <div className="text-text-faint mt-0.5 flex items-center gap-2 text-xs">
            {credential.username && (
              <span className="truncate font-mono">{credential.username}</span>
            )}
            <span>Added {formatDate(credential.created_at)}</span>
          </div>
        </div>
      </div>
      <div className="flex items-center gap-2">
        <Button size="sm" variant="ghost" onClick={onEdit}>
          Edit
        </Button>
        <AlertDialog>
          <AlertDialogTrigger
            render={
              <Button size="sm" variant="ghost" className="text-destructive" />
            }
          >
            Delete
          </AlertDialogTrigger>
          <AlertDialogContent>
            <AlertDialogHeader>
              <AlertDialogTitle>Delete {credential.name}?</AlertDialogTitle>
              <AlertDialogDescription>
                Applications using this credential will lose access to private
                repositories.
              </AlertDialogDescription>
            </AlertDialogHeader>
            <AlertDialogFooter>
              <AlertDialogCancel>Cancel</AlertDialogCancel>
              <AlertDialogAction
                onClick={() => {
                  toast.promise(deleteCred.mutateAsync(credential.id), {
                    loading: "Deleting...",
                    success: "Credential deleted",
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
  );
}

function CredentialForm({
  credential,
  onSuccess,
}: {
  credential: GitCredential | null;
  onSuccess: () => void;
}) {
  const createCred = useCreateGitCredential();
  const updateCred = useUpdateGitCredential();

  const form = useForm({
    defaultValues: {
      name: credential?.name ?? "",
      provider: credential?.provider ?? "github",
      token: "",
      username: credential?.username ?? "",
    },
    onSubmit: async ({ value }) => {
      const name = value.name.trim();
      const token = value.token.trim();
      const username = value.username.trim();
      if (credential) {
        toast.promise(
          updateCred
            .mutateAsync({
              id: credential.id,
              name,
              provider: value.provider,
              token: token || undefined,
              username,
            })
            .then(onSuccess),
          {
            loading: "Updating...",
            success: "Credential updated",
            error: (err) => err.message,
          },
        );
      } else {
        toast.promise(
          createCred
            .mutateAsync({
              name,
              provider: value.provider,
              token,
              username,
            })
            .then(onSuccess),
          {
            loading: "Creating...",
            success: "Credential created",
            error: (err) => err.message,
          },
        );
      }
    },
  });

  const provider = useStore(form.store, (s) => s.values.provider);

  return (
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
        validators={{
          onChange: z.string().trim().min(1, "Name is required"),
        }}
        children={(field) => (
          <div className="space-y-2">
            <Label>Name</Label>
            <Input
              value={field.state.value}
              onBlur={field.handleBlur}
              onChange={(e) => field.handleChange(e.target.value)}
              placeholder="e.g. My GitHub Token"
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
        name="provider"
        children={(field) => (
          <div className="space-y-2">
            <Label>Provider</Label>
            <Select
              value={field.state.value}
              onValueChange={(v) => v && field.handleChange(v as typeof field.state.value)}
            >
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {PROVIDERS.map((p) => (
                  <SelectItem key={p.value} value={p.value}>
                    {p.label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
        )}
      />
      <form.Field
        name="token"
        validators={{
          onChange: credential
            ? z.string()
            : z.string().trim().min(1, "Token is required"),
        }}
        children={(field) => (
          <div className="space-y-2">
            <Label>Token / Password</Label>
            <Input
              type="password"
              value={field.state.value}
              onBlur={field.handleBlur}
              onChange={(e) => field.handleChange(e.target.value)}
              placeholder={
                credential
                  ? "Leave empty to keep current"
                  : "Personal access token"
              }
              className="font-mono"
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
      {(provider === "bitbucket" || provider === "generic") && (
        <form.Field
          name="username"
          children={(field) => (
            <div className="space-y-2">
              <Label>Username</Label>
              <Input
                value={field.state.value}
                onBlur={field.handleBlur}
                onChange={(e) => field.handleChange(e.target.value)}
                placeholder="Username for authentication"
              />
            </div>
          )}
        />
      )}
      <form.Subscribe
        selector={(s) => s.isSubmitting}
        children={(isSubmitting) => (
          <Button type="submit" disabled={isSubmitting} className="w-full">
            {isSubmitting ? "Saving..." : credential ? "Update" : "Create"}
          </Button>
        )}
      />
    </form>
  );
}
