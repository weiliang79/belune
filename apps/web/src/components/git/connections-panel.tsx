import { Plug, Trash2 } from "lucide-react";
import {
  useGitIntegrations,
  useAvailableProviders,
  useStartGitConnect,
  useDeleteGitIntegration,
} from "@/lib/hooks/use-git-integrations";
import { toast } from "sonner";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
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
  Tooltip,
  TooltipContent,
  TooltipPositioner,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import { formatRelativeTime } from "@/lib/utils/format";
import { ProviderIcon } from "./provider-icon";

const PROVIDER_LABELS: Record<string, string> = {
  github: "GitHub",
  gitlab: "GitLab",
  bitbucket: "Bitbucket",
  gitea: "Gitea",
};

export function ConnectionsPanel() {
  const { data: integrations } = useGitIntegrations();
  const { data: available } = useAvailableProviders();
  const startConnect = useStartGitConnect();
  const del = useDeleteGitIntegration();

  return (
    <div className="space-y-6">
      <Card>
        <CardHeader>
          <CardTitle>Connect an account</CardTitle>
        </CardHeader>
        <CardContent>
          {!available || available.length === 0 ? (
            <p className="text-muted-foreground text-sm">
              No providers are available yet. Ask an administrator to set up a
              git provider first.
            </p>
          ) : (
            <div className="divide-y rounded-lg border">
              {available.map((p) => {
                const host = p.base_url.replace(/^https?:\/\//, "");
                return (
                  <button
                    key={`${p.provider}:${p.base_url}`}
                    type="button"
                    disabled={startConnect.isPending}
                    onClick={() =>
                      startConnect.mutate({
                        provider: p.provider,
                        baseUrl: p.base_url || undefined,
                      })
                    }
                    className="hover:bg-accent/50 flex w-full items-center gap-3 px-4 py-3 text-left transition-colors disabled:opacity-60"
                  >
                    <span className="bg-muted text-foreground grid size-8 shrink-0 place-items-center rounded-md">
                      <ProviderIcon provider={p.provider} className="size-4" />
                    </span>
                    <span className="min-w-0 flex-1">
                      <span className="block text-sm font-medium">
                        {PROVIDER_LABELS[p.provider] ?? p.provider}
                        {host ? (
                          <span className="text-muted-foreground font-normal">
                            {" · "}
                            <code className="text-xs">{host}</code>
                          </span>
                        ) : null}
                      </span>
                      <span className="text-muted-foreground text-xs">
                        Connect an account
                      </span>
                    </span>
                    <Plug className="text-muted-foreground h-4 w-4 shrink-0" />
                  </button>
                );
              })}
            </div>
          )}
        </CardContent>
      </Card>

      <Card>
        <CardHeader className="flex flex-row items-center justify-between">
          <CardTitle>Your connections</CardTitle>
          {integrations && integrations.length > 0 && (
            <Badge variant="secondary">{integrations.length}</Badge>
          )}
        </CardHeader>
        <CardContent>
          {!integrations || integrations.length === 0 ? (
            <p className="text-muted-foreground text-sm">
              No connected accounts yet.
            </p>
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full text-sm">
                <thead>
                  <tr className="text-muted-foreground border-b text-left text-xs uppercase">
                    <th className="px-2 py-2 font-medium">Provider</th>
                    <th className="px-2 py-2 font-medium">Account</th>
                    <th className="px-2 py-2 font-medium">Host</th>
                    <th className="px-2 py-2 font-medium">Connected</th>
                    <th className="px-2 py-2" />
                  </tr>
                </thead>
                <tbody className="divide-y">
                  {integrations.map((i) => (
                    <tr key={i.id}>
                      <td className="px-2 py-3">
                        <span className="flex items-center gap-2 font-medium">
                          <span className="bg-muted text-foreground grid size-6 shrink-0 place-items-center rounded">
                            <ProviderIcon provider={i.provider} className="size-3.5" />
                          </span>
                          {PROVIDER_LABELS[i.provider] ?? i.provider}
                        </span>
                      </td>
                      <td className="px-2 py-3">
                        <code className="text-xs">{i.account_login}</code>
                      </td>
                      <td className="text-muted-foreground px-2 py-3 text-xs">
                        {i.base_url ? i.base_url.replace(/^https?:\/\//, "") : "—"}
                      </td>
                      <td className="text-muted-foreground px-2 py-3 text-xs">
                        {formatRelativeTime(i.created_at)}
                      </td>
                      <td className="px-2 py-3 text-right">
                        <AlertDialog>
                          <Tooltip>
                            <TooltipTrigger
                              render={
                                <AlertDialogTrigger
                                  render={
                                    <Button
                                      variant="ghost"
                                      size="icon-sm"
                                      aria-label={`Disconnect ${i.account_login}`}
                                      className="text-destructive hover:bg-destructive/10 hover:text-destructive"
                                    />
                                  }
                                />
                              }
                            >
                              <Trash2 className="h-4 w-4" />
                            </TooltipTrigger>
                            <TooltipPositioner>
                              <TooltipContent>Disconnect</TooltipContent>
                            </TooltipPositioner>
                          </Tooltip>
                          <AlertDialogContent>
                            <AlertDialogHeader>
                              <AlertDialogTitle>
                                Disconnect {i.account_login}?
                              </AlertDialogTitle>
                              <AlertDialogDescription>
                                This removes the{" "}
                                {PROVIDER_LABELS[i.provider] ?? i.provider}{" "}
                                connection for {i.account_login}. Apps deploying
                                through it will need a different connection.
                              </AlertDialogDescription>
                            </AlertDialogHeader>
                            <AlertDialogFooter>
                              <AlertDialogCancel>Cancel</AlertDialogCancel>
                              <AlertDialogAction
                                onClick={() =>
                                  del.mutate(i.id, {
                                    onSuccess: () =>
                                      toast.success("Disconnected"),
                                  })
                                }
                                className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
                              >
                                Disconnect
                              </AlertDialogAction>
                            </AlertDialogFooter>
                          </AlertDialogContent>
                        </AlertDialog>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
