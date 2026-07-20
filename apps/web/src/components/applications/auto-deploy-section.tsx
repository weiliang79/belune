import { useCallback, useState, type ReactNode } from "react";
import { toast } from "sonner";
import { Eye, RefreshCw } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Separator } from "@/components/ui/separator";
import { Switch } from "@/components/ui/switch";
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
  useDeleteDeployHook,
  useDeployHook,
  useGenerateDeployHook,
  useRevealDeployHook,
  useUpdateWebhook,
} from "@/lib/hooks/use-applications";
import { CopyButton } from "@/lib/components/copy-button";
import type { Application } from "@/lib/types";

interface Props {
  projectId: string;
  applicationId: string;
  application: Application;
}

/**
 * The single place auto-deploy is configured, for BOTH application types.
 *
 * Two independent mechanisms, each with its own switch:
 *
 *   - **Git Push Hook** — the provider calls us, the payload is HMAC-verified,
 *     the branch is filtered, and the commit is recorded. Git apps only.
 *   - **Deploy Hook** — a tokenized URL anything can POST to. No payload, no
 *     branch logic. Works for image apps too, which have no push to react to.
 *
 * They are complements, not alternatives: a git app can run both.
 */
export function AutoDeploySection({
  projectId,
  applicationId,
  application,
}: Props) {
  const isGit = application.type === "git";

  return (
    <Card>
      <CardHeader>
        <CardTitle>Auto Deploy</CardTitle>
        <CardDescription>How this app deploys itself.</CardDescription>
      </CardHeader>
      {/* Stacked, not columns: each row expands when switched on, so side-by-side
          leaves one column tall and the other nearly empty depending on state. */}
      <CardContent className="space-y-5">
        {isGit && (
          <>
            <PushWebhookRow
              projectId={projectId}
              applicationId={applicationId}
              application={application}
            />
            <Separator />
          </>
        )}
        <DeployHookRow
          projectId={projectId}
          applicationId={applicationId}
          application={application}
        />
      </CardContent>
    </Card>
  );
}

/**
 * Shared shape for both mechanisms: a title row carrying the switch, a
 * one-line explanation, and the details that only matter once it is on.
 */
function ToggleRow({
  id,
  title,
  description,
  checked,
  disabled,
  onCheckedChange,
  children,
}: {
  id: string;
  title: string;
  description: ReactNode;
  checked: boolean;
  disabled?: boolean;
  onCheckedChange: (next: boolean) => void;
  children?: ReactNode;
}) {
  return (
    <div className="space-y-3">
      <div className="flex items-start justify-between gap-4">
        <div className="min-w-0">
          <Label htmlFor={id} className="font-medium">
            {title}
          </Label>
          <p className="text-muted-foreground mt-0.5 text-sm">{description}</p>
        </div>
        <Switch
          id={id}
          checked={checked}
          disabled={disabled}
          onCheckedChange={onCheckedChange}
        />
      </div>
      {children}
    </div>
  );
}

/** A read-only URL/value row with a copy button, used for both mechanisms. */
function CopyRow({ value, children }: { value: string; children?: ReactNode }) {
  return (
    <div className="bg-muted flex items-center gap-2 rounded-md px-3 py-2">
      <code className="min-w-0 flex-1 font-mono text-sm break-all">{value}</code>
      <CopyButton value={value} />
      {children}
    </div>
  );
}

/**
 * Git-only. "On" means a webhook secret exists — that is what the push endpoint
 * requires to verify a delivery, and what makes the app eligible for matching.
 * Toggling on mints one so enabling is a single click, matching the deploy hook.
 */
function PushWebhookRow({ projectId, applicationId, application }: Props) {
  const updateWebhook = useUpdateWebhook(projectId, applicationId);
  const enabled = !!application.webhook_secret;
  const pushUrl = `${window.location.origin}/api/webhooks/push`;
  const savedBranch = application.auto_deploy_branch || "main";

  const [confirmDisable, setConfirmDisable] = useState(false);
  const [secretShown, setSecretShown] = useState(false);
  const [branch, setBranch] = useState(savedBranch);

  const generateSecret = () => {
    const bytes = new Uint8Array(32);
    crypto.getRandomValues(bytes);
    return Array.from(bytes, (b) => b.toString(16).padStart(2, "0")).join("");
  };

  const handleToggle = useCallback(
    async (next: boolean) => {
      if (!next) {
        setConfirmDisable(true);
        return;
      }
      await updateWebhook.mutateAsync({
        webhook_secret: generateSecret(),
        auto_deploy_branch: branch || "main",
      });
      setSecretShown(true);
      toast.success("Push webhook enabled — add the URL and secret to your repository");
    },
    [updateWebhook, branch],
  );

  const handleDisable = useCallback(async () => {
    // Clearing the secret is what actually disables it: the push endpoint fails
    // closed without one, and app matching requires it.
    await updateWebhook.mutateAsync({ webhook_secret: "" });
    setSecretShown(false);
    setConfirmDisable(false);
    toast.success("Push webhook disabled");
  }, [updateWebhook]);

  const handleRegenerate = useCallback(async () => {
    await updateWebhook.mutateAsync({ webhook_secret: generateSecret() });
    setSecretShown(true);
    toast.success("New secret generated — update it in your repository");
  }, [updateWebhook]);

  const saveBranch = useCallback(async () => {
    await updateWebhook.mutateAsync({ auto_deploy_branch: branch || "main" });
    toast.success("Branch saved");
  }, [updateWebhook, branch]);

  const secret = application.webhook_secret ?? "";

  return (
    <>
      <ToggleRow
        id="push-webhook-toggle"
        title="Git Push Hook"
        description={
          enabled
            ? `Deploys when you push to ${savedBranch}.`
            : "Deploy automatically when your git provider reports a push."
        }
        checked={enabled}
        disabled={updateWebhook.isPending}
        onCheckedChange={handleToggle}
      >
        {enabled && (
          <div className="space-y-3">
            <div className="space-y-1.5">
              <Label className="text-xs">Webhook URL</Label>
              <CopyRow value={pushUrl} />
            </div>

            <div className="space-y-1.5">
              <Label className="text-xs">Secret</Label>
              <div className="bg-muted flex items-center gap-2 rounded-md px-3 py-2">
                <code className="min-w-0 flex-1 font-mono text-sm break-all">
                  {secretShown ? secret : "•".repeat(32)}
                </code>
                <Button
                  variant="ghost"
                  size="sm"
                  className="h-7 shrink-0"
                  onClick={() => setSecretShown((s) => !s)}
                >
                  <Eye className="mr-1.5 h-3.5 w-3.5" />
                  {secretShown ? "Hide" : "Show"}
                </Button>
                {secretShown && <CopyButton value={secret} />}
                <Button
                  variant="outline"
                  size="sm"
                  className="shrink-0"
                  onClick={handleRegenerate}
                  disabled={updateWebhook.isPending}
                >
                  <RefreshCw className="mr-2 h-3.5 w-3.5" />
                  Regenerate
                </Button>
              </div>
              <p className="text-muted-foreground text-xs">
                Paste the same secret into your provider. It proves the delivery
                really came from them — it does not identify this application.
              </p>
            </div>

            <div className="space-y-1.5">
              <Label className="text-xs" htmlFor="auto-deploy-branch">
                Branch
              </Label>
              <div className="flex gap-2">
                <Input
                  id="auto-deploy-branch"
                  value={branch}
                  onChange={(e) => setBranch(e.target.value)}
                  placeholder="main"
                />
                {branch !== savedBranch && (
                  <Button
                    onClick={saveBranch}
                    disabled={updateWebhook.isPending}
                    className="shrink-0"
                  >
                    Save
                  </Button>
                )}
              </div>
              <p className="text-muted-foreground text-xs">
                Only pushes to this branch trigger a deploy.
              </p>
            </div>
          </div>
        )}
      </ToggleRow>

      <AlertDialog open={confirmDisable} onOpenChange={setConfirmDisable}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Disable push deploys?</AlertDialogTitle>
            <AlertDialogDescription>
              Pushes will stop deploying this app. Turning it back on issues a
              new secret, so you would need to update it in your repository.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction
              onClick={handleDisable}
              disabled={updateWebhook.isPending}
            >
              Disable
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  );
}

/** All app types: a tokenized URL that triggers a deploy when POSTed to. */
function DeployHookRow({ projectId, applicationId, application }: Props) {
  const { data: hook, isLoading } = useDeployHook(projectId, applicationId);
  const generate = useGenerateDeployHook(projectId, applicationId);
  const reveal = useRevealDeployHook(projectId, applicationId);
  const remove = useDeleteDeployHook(projectId, applicationId);

  // Held in local state, not in the query cache: the token is only in memory
  // for as long as the user is looking at it, and a page change drops it.
  const [url, setUrl] = useState<string | null>(null);
  const [confirmRegenerate, setConfirmRegenerate] = useState(false);
  const [confirmDisable, setConfirmDisable] = useState(false);

  const toUrl = (path?: string) =>
    path ? `${window.location.origin}${path}` : null;

  const handleToggle = useCallback(
    async (next: boolean) => {
      if (!next) {
        setConfirmDisable(true);
        return;
      }
      const result = await generate.mutateAsync();
      setUrl(toUrl(result.path));
      toast.success("Deploy hook created");
    },
    [generate],
  );

  const handleRegenerate = useCallback(async () => {
    const result = await generate.mutateAsync();
    setUrl(toUrl(result.path));
    setConfirmRegenerate(false);
    toast.success("Deploy hook regenerated — the previous URL no longer works");
  }, [generate]);

  const handleReveal = useCallback(async () => {
    const result = await reveal.mutateAsync();
    setUrl(toUrl(result.path));
  }, [reveal]);

  const handleDisable = useCallback(async () => {
    await remove.mutateAsync();
    setUrl(null);
    setConfirmDisable(false);
    toast.success("Deploy hook disabled");
  }, [remove]);

  // Git: the build clones the repository's DEFAULT ref — it does not read
  // auto_deploy_branch, which only filters push webhooks. Naming a branch here
  // would be a lie until branch selection exists.
  const effect =
    application.type === "git"
      ? "deploys the repository's default branch"
      : `re-pulls ${application.source_image ?? "the configured image"} and redeploys`;

  const regenerateButton = (
    <Button
      variant="outline"
      size="sm"
      onClick={() => setConfirmRegenerate(true)}
      disabled={generate.isPending}
      className="shrink-0"
    >
      <RefreshCw className="mr-2 h-3.5 w-3.5" />
      Regenerate
    </Button>
  );

  return (
    <>
      <ToggleRow
        id="deploy-hook-toggle"
        title="Deploy Hook"
        description={
          <>
            A private URL your CI can POST to — no credentials needed. Each call{" "}
            {effect}.
          </>
        }
        checked={!!hook?.enabled}
        disabled={isLoading || generate.isPending || remove.isPending}
        onCheckedChange={handleToggle}
      >
        {hook?.enabled &&
          (url ? (
            <div className="space-y-3">
              <div className="space-y-1.5">
                <Label className="text-xs">Hook URL</Label>
                <CopyRow value={url}>{regenerateButton}</CopyRow>
              </div>
              <div className="space-y-1.5">
                <Label className="text-xs">Example CI step</Label>
                <CopyRow value={`curl -X POST ${url}`} />
              </div>
            </div>
          ) : (
            <div className="flex items-center gap-2 rounded-md border px-3 py-2">
              <p className="text-muted-foreground min-w-0 flex-1 text-sm">
                The URL is hidden — anyone who has it can trigger a deploy.
              </p>
              <Button
                variant="outline"
                size="sm"
                onClick={handleReveal}
                disabled={reveal.isPending}
                className="shrink-0"
              >
                <Eye className="mr-2 h-3.5 w-3.5" />
                {reveal.isPending ? "Revealing..." : "Show URL"}
              </Button>
              {regenerateButton}
            </div>
          ))}
      </ToggleRow>

      <AlertDialog open={confirmRegenerate} onOpenChange={setConfirmRegenerate}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Regenerate deploy hook?</AlertDialogTitle>
            <AlertDialogDescription>
              The current URL stops working immediately. Any CI job still using
              it will start failing until you paste in the new one.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction
              onClick={handleRegenerate}
              disabled={generate.isPending}
            >
              Regenerate
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      <AlertDialog open={confirmDisable} onOpenChange={setConfirmDisable}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Disable deploy hook?</AlertDialogTitle>
            <AlertDialogDescription>
              The URL stops working immediately and cannot be recovered —
              turning the hook back on later issues a different one.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction
              onClick={handleDisable}
              disabled={remove.isPending}
            >
              Disable
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  );
}
