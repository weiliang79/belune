import { createFileRoute } from "@tanstack/react-router";
import { useState } from "react";
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
          <h1 className="text-2xl font-bold">Git Credentials</h1>
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
          <h1 className="text-2xl font-bold">Git Credentials</h1>
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
          <DialogTrigger asChild>
            <Button>Add Credential</Button>
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
    <div className="flex items-center justify-between rounded-md border p-3">
      <div className="flex items-center gap-3">
        <span className="text-sm font-medium">{credential.name}</span>
        <Badge variant="secondary">{providerLabel(credential.provider)}</Badge>
        {credential.username && (
          <span className="text-muted-foreground text-xs font-mono">
            {credential.username}
          </span>
        )}
        <span className="text-muted-foreground text-xs">
          {formatDate(credential.created_at)}
        </span>
      </div>
      <div className="flex items-center gap-2">
        <Button size="sm" variant="ghost" onClick={onEdit}>
          Edit
        </Button>
        <AlertDialog>
          <AlertDialogTrigger asChild>
            <Button size="sm" variant="ghost" className="text-destructive">
              Delete
            </Button>
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
  const [name, setName] = useState(credential?.name ?? "");
  const [provider, setProvider] = useState(credential?.provider ?? "github");
  const [token, setToken] = useState("");
  const [username, setUsername] = useState(credential?.username ?? "");
  const createCred = useCreateGitCredential();
  const updateCred = useUpdateGitCredential();

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!name.trim()) {
      toast.error("Name is required");
      return;
    }
    if (!credential && !token.trim()) {
      toast.error("Token is required");
      return;
    }

    if (credential) {
      toast.promise(
        updateCred
          .mutateAsync({
            id: credential.id,
            name: name.trim(),
            provider,
            token: token.trim() || undefined,
            username: username.trim(),
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
            name: name.trim(),
            provider,
            token: token.trim(),
            username: username.trim(),
          })
          .then(onSuccess),
        {
          loading: "Creating...",
          success: "Credential created",
          error: (err) => err.message,
        },
      );
    }
  };

  return (
    <form onSubmit={handleSubmit} className="space-y-4">
      <div className="space-y-2">
        <Label>Name</Label>
        <Input
          value={name}
          onChange={(e) => setName(e.target.value)}
          placeholder="e.g. My GitHub Token"
        />
      </div>
      <div className="space-y-2">
        <Label>Provider</Label>
        <Select value={provider} onValueChange={setProvider}>
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
      <div className="space-y-2">
        <Label>Token / Password</Label>
        <Input
          type="password"
          value={token}
          onChange={(e) => setToken(e.target.value)}
          placeholder={credential ? "Leave empty to keep current" : "Personal access token"}
          className="font-mono"
        />
      </div>
      {(provider === "bitbucket" || provider === "generic") && (
        <div className="space-y-2">
          <Label>Username</Label>
          <Input
            value={username}
            onChange={(e) => setUsername(e.target.value)}
            placeholder="Username for authentication"
          />
        </div>
      )}
      <Button
        type="submit"
        disabled={createCred.isPending || updateCred.isPending}
        className="w-full"
      >
        {createCred.isPending || updateCred.isPending
          ? "Saving..."
          : credential
            ? "Update"
            : "Create"}
      </Button>
    </form>
  );
}
