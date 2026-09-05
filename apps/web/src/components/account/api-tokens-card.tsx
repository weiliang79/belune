import { useState } from "react";
import { toast } from "sonner";
import { KeyIcon, Trash2Icon } from "lucide-react";
import { Button } from "@/components/ui/button";
import { CopyButton } from "@/lib/components/copy-button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
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
  AlertDialogTrigger,
} from "@/components/ui/alert-dialog";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Badge } from "@/components/ui/badge";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Tooltip,
  TooltipContent,
  TooltipPositioner,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import {
  useCreateToken,
  useDeleteToken,
  useTokens,
} from "@/lib/hooks/use-tokens";
import { formatDateTimeShort, formatRelativeTime } from "@/lib/utils/format";
import type { TokenScope, ApiToken } from "@/lib/types";

// Mirrors the backend's validTokenExpiryDays exactly — the API rejects
// anything else, so offering it here would just be a confusing round trip.
const EXPIRY_OPTIONS = [
  { value: "1", label: "1 day" },
  { value: "7", label: "7 days" },
  { value: "14", label: "14 days" },
  { value: "30", label: "30 days" },
  { value: "60", label: "60 days" },
  { value: "90", label: "90 days" },
  { value: "never", label: "No expiry" },
];

// Mirrors middleware.scopeGrants: write also covers read and deploy, and
// read also covers metrics — deploy and metrics are narrower carve-outs, not
// rungs on a ladder. Order here is display order, all checked by default so
// an unchanged submission keeps today's full-access behavior.
const SCOPE_OPTIONS: {
  value: TokenScope;
  label: string;
  description: string;
}[] = [
  {
    value: "read",
    label: "Read",
    description: "View projects, applications, and their data.",
  },
  {
    value: "write",
    label: "Write",
    description: "Everything Read can do, plus create, update, and configure.",
  },
  {
    value: "deploy",
    label: "Deploy",
    description:
      "Trigger deploys, restarts, and other runtime actions — without general write access.",
  },
  {
    value: "metrics",
    label: "Metrics",
    description:
      "Read metrics only — narrower than Read, for a monitoring scraper.",
  },
];

export function ApiTokensCard() {
  const { data: tokens, isLoading } = useTokens();
  // Held here, not inside CreateTokenDialog: the dialog closes on success, and
  // the plaintext is shown exactly once — unmounting the dialog must not lose it.
  const [issued, setIssued] = useState<string | null>(null);

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <KeyIcon aria-hidden="true" className="size-4" />
          Personal Access Tokens
        </CardTitle>
        <CardDescription>
          Tokens authenticate as you, scoped to whatever you choose when
          creating one — for all projects you have access to.
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        {isLoading ? (
          <div className="space-y-3">
            {[1, 2].map((i) => (
              <div key={i} className="bg-muted h-12 animate-pulse rounded" />
            ))}
          </div>
        ) : tokens && tokens.length > 0 ? (
          <div className="divide-border divide-y">
            {tokens.map((token) => (
              <TokenRow key={token.id} token={token} />
            ))}
          </div>
        ) : (
          <p className="text-muted-foreground text-sm">
            No tokens yet — create one to script against this instance.
          </p>
        )}

        <CreateTokenDialog onIssued={setIssued} />
      </CardContent>

      <Dialog
        open={issued !== null}
        onOpenChange={(next) => !next && setIssued(null)}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Your new token</DialogTitle>
            <DialogDescription>
              This is the only time it is shown. Copy it now — closing this
              dialog does not revoke it, but there is no way to see it again.
            </DialogDescription>
          </DialogHeader>
          {issued && (
            <div className="bg-muted flex items-center gap-2 rounded-md px-3 py-2">
              <code className="min-w-0 flex-1 font-mono text-sm break-all">
                {issued}
              </code>
              <CopyButton value={issued} />
            </div>
          )}
          <DialogFooter>
            <Button onClick={() => setIssued(null)}>Done</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </Card>
  );
}

function TokenRow({ token }: { token: ApiToken }) {
  const deleteToken = useDeleteToken();

  const handleDelete = () => {
    toast.promise(deleteToken.mutateAsync(token.id), {
      loading: "Revoking token...",
      success: "Token revoked",
      error: (err) => err.message,
    });
  };

  return (
    <div className="flex items-center justify-between gap-4 py-3 first:pt-0 last:pb-0">
      <div className="min-w-0 space-y-1">
        <div className="flex flex-wrap items-center gap-2">
          <span className="truncate text-sm font-medium">{token.name}</span>
          {token.scopes.map((scope) => (
            <Badge
              key={scope}
              variant="secondary"
              className="text-xs capitalize"
            >
              {scope}
            </Badge>
          ))}
          {!token.expires_at && (
            <Badge variant="outline" className="text-xs">
              Never expires
            </Badge>
          )}
        </div>
        <p className="text-muted-foreground text-xs">
          {token.expires_at
            ? `Expires ${formatDateTimeShort(token.expires_at)}`
            : "No expiration"}
          {" · "}
          {token.last_used_at
            ? `Last used ${formatRelativeTime(token.last_used_at)}`
            : "Never used"}
        </p>
      </div>

      <AlertDialog>
        <Tooltip>
          <TooltipTrigger
            render={
              <AlertDialogTrigger
                render={
                  <Button
                    variant="ghost"
                    size="icon-sm"
                    aria-label={`Revoke ${token.name}`}
                    className="text-destructive hover:bg-destructive/10 hover:text-destructive shrink-0"
                  />
                }
              />
            }
          >
            <Trash2Icon aria-hidden="true" className="size-4" />
          </TooltipTrigger>
          <TooltipPositioner>
            <TooltipContent>Revoke</TooltipContent>
          </TooltipPositioner>
        </Tooltip>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Revoke {token.name}?</AlertDialogTitle>
            <AlertDialogDescription>
              Anything using this token stops working immediately. This cannot
              be undone — create a new token first if something still depends on
              this one.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction
              variant="destructive-solid"
              onClick={handleDelete}
            >
              Revoke
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}

const ALL_SCOPES = SCOPE_OPTIONS.map((o) => o.value);

function CreateTokenDialog({
  onIssued,
}: {
  onIssued: (token: string) => void;
}) {
  const [open, setOpen] = useState(false);
  const [name, setName] = useState("");
  const [expiry, setExpiry] = useState("30");
  // Every scope checked by default — an unchanged submission keeps the
  // full-access behavior tokens have always had; unchecking narrows it.
  const [scopes, setScopes] = useState<Set<TokenScope>>(
    () => new Set(ALL_SCOPES),
  );
  const createToken = useCreateToken();

  const close = () => {
    setOpen(false);
    setName("");
    setExpiry("30");
    setScopes(new Set(ALL_SCOPES));
  };

  const toggleScope = (scope: TokenScope, checked: boolean) => {
    setScopes((prev) => {
      const next = new Set(prev);
      if (checked) next.add(scope);
      else next.delete(scope);
      return next;
    });
  };

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      const result = await createToken.mutateAsync({
        name,
        scopes: Array.from(scopes),
        expiresInDays: expiry === "never" ? undefined : Number(expiry),
      });
      onIssued(result.token);
      close();
    } catch (err) {
      toast.error(
        err instanceof Error ? err.message : "Could not create token",
      );
    }
  };

  return (
    <>
      <Button onClick={() => setOpen(true)}>Create token</Button>

      <Dialog
        open={open}
        onOpenChange={(next) => (next ? setOpen(true) : close())}
      >
        <DialogContent>
          <form onSubmit={submit}>
            <DialogHeader>
              <DialogTitle>Create a personal access token</DialogTitle>
              <DialogDescription>
                Name it after what will use it — you'll want to tell tokens
                apart later, not just when this one is unused.
              </DialogDescription>
            </DialogHeader>
            <div className="space-y-4 py-4">
              <div className="space-y-2">
                <Label htmlFor="token-name">Name</Label>
                <Input
                  id="token-name"
                  autoFocus
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  placeholder="e.g. CI deploy"
                />
              </div>
              <div className="space-y-2">
                <Label>Expiration</Label>
                <Select value={expiry} onValueChange={(v) => v && setExpiry(v)}>
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {EXPIRY_OPTIONS.map((o) => (
                      <SelectItem key={o.value} value={o.value}>
                        {o.label}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              <div className="space-y-2">
                <Label>Scope</Label>
                <div className="space-y-2.5">
                  {SCOPE_OPTIONS.map((o) => (
                    <label
                      key={o.value}
                      className="flex items-start gap-2.5 text-sm"
                    >
                      <Checkbox
                        className="mt-0.5"
                        checked={scopes.has(o.value)}
                        onCheckedChange={(checked) =>
                          toggleScope(o.value, checked === true)
                        }
                      />
                      <span>
                        <span className="font-medium">{o.label}</span>
                        <span className="text-muted-foreground block text-xs">
                          {o.description}
                        </span>
                      </span>
                    </label>
                  ))}
                </div>
                {scopes.size === 0 && (
                  <p className="text-destructive text-xs">
                    Select at least one scope.
                  </p>
                )}
              </div>
            </div>
            <DialogFooter>
              <Button type="button" variant="outline" onClick={close}>
                Cancel
              </Button>
              <Button
                type="submit"
                disabled={
                  createToken.isPending || !name.trim() || scopes.size === 0
                }
              >
                {createToken.isPending ? "Creating..." : "Create"}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>
    </>
  );
}
