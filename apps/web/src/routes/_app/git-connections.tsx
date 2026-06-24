import { useEffect } from "react";
import { createFileRoute, useSearch } from "@tanstack/react-router";
import { toast } from "sonner";
import { Plug, Trash2 } from "lucide-react";
import { RouteError } from "@/lib/components/route-error";
import {
  useGitIntegrations,
  useAvailableProviders,
  useStartGitConnect,
  useDeleteGitIntegration,
} from "@/lib/hooks/use-git-integrations";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";

export const Route = createFileRoute("/_app/git-connections")({
  component: GitConnectionsPage,
  errorComponent: RouteError,
  validateSearch: (search: Record<string, unknown>) => ({
    connected: typeof search.connected === "string" ? search.connected : undefined,
    error: typeof search.error === "string" ? search.error : undefined,
  }),
});

const PROVIDER_LABELS: Record<string, string> = {
  github: "GitHub",
  gitlab: "GitLab",
  bitbucket: "Bitbucket",
  gitea: "Gitea",
};

function GitConnectionsPage() {
  const { connected, error } = useSearch({ from: "/_app/git-connections" });
  const { data: integrations } = useGitIntegrations();
  const { data: available } = useAvailableProviders();
  const startConnect = useStartGitConnect();
  const del = useDeleteGitIntegration();

  useEffect(() => {
    if (connected) toast.success(`Connected ${PROVIDER_LABELS[connected] ?? connected}`);
    if (error) toast.error(`Failed to connect ${PROVIDER_LABELS[error] ?? error}`);
  }, [connected, error]);

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-semibold">Git Connections</h1>
        <p className="text-muted-foreground text-sm">
          Connect a git account to browse and deploy your repositories without
          managing tokens by hand.
        </p>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>Connect an account</CardTitle>
        </CardHeader>
        <CardContent>
          {!available || available.length === 0 ? (
            <p className="text-muted-foreground text-sm">
              No providers are configured yet. Ask an administrator to set up a
              git provider first.
            </p>
          ) : (
            <div className="flex flex-wrap gap-2">
              {available.map((p) => (
                <Button
                  key={`${p.provider}:${p.base_url}`}
                  variant="outline"
                  disabled={startConnect.isPending}
                  onClick={() =>
                    startConnect.mutate({
                      provider: p.provider,
                      baseUrl: p.base_url || undefined,
                    })
                  }
                >
                  <Plug className="mr-1 h-4 w-4" />
                  Connect {PROVIDER_LABELS[p.provider] ?? p.provider}
                  {p.base_url ? ` (${p.base_url.replace(/^https?:\/\//, "")})` : ""}
                </Button>
              ))}
            </div>
          )}
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Your connections</CardTitle>
        </CardHeader>
        <CardContent>
          {!integrations || integrations.length === 0 ? (
            <p className="text-muted-foreground text-sm">
              No connected accounts yet.
            </p>
          ) : (
            <div className="divide-y">
              {integrations.map((i) => (
                <div
                  key={i.id}
                  className="flex items-center justify-between py-3"
                >
                  <div className="space-y-1">
                    <div className="flex items-center gap-2">
                      <span className="font-medium">
                        {PROVIDER_LABELS[i.provider] ?? i.provider}
                      </span>
                      <Badge variant="secondary">{i.account_login}</Badge>
                    </div>
                    {i.base_url && (
                      <p className="text-muted-foreground text-xs">{i.base_url}</p>
                    )}
                  </div>
                  <Button
                    variant="ghost"
                    size="icon"
                    onClick={() => {
                      if (confirm(`Disconnect ${i.account_login}?`)) {
                        del.mutate(i.id);
                      }
                    }}
                  >
                    <Trash2 className="h-4 w-4" />
                  </Button>
                </div>
              ))}
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
