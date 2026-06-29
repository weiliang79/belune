import { useEffect } from "react";
import { createFileRoute, useNavigate } from "@tanstack/react-router";
import { toast } from "sonner";
import { RouteError } from "@/lib/components/route-error";
import { useAuthStore } from "@/lib/stores/auth";
import {
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
} from "@/components/ui/tabs";
import { ConnectionsPanel } from "@/components/git/connections-panel";
import { ProvidersPanel } from "@/components/git/providers-panel";

type GitTab = "connections" | "providers";

export const Route = createFileRoute("/_app/git")({
  component: GitPage,
  errorComponent: RouteError,
  validateSearch: (search: Record<string, unknown>) => ({
    tab: search.tab === "providers" ? ("providers" as const) : undefined,
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

function GitPage() {
  const { tab, connected, error } = Route.useSearch();
  const navigate = useNavigate({ from: "/git" });
  const isAdmin = useAuthStore((s) => s.user?.role === "admin");

  // Non-admins never get the Providers tab even if the URL asks for it.
  const activeTab: GitTab = isAdmin && tab === "providers" ? "providers" : "connections";

  // Surface OAuth-connect / GitHub-App-creation outcomes from the callback redirect.
  useEffect(() => {
    if (!connected && !error) return;
    if (connected === "github" && tab === "providers") {
      toast.success("GitHub App created");
    } else if (connected) {
      toast.success(`Connected ${PROVIDER_LABELS[connected] ?? connected}`);
    } else if (error) {
      toast.error(`Failed to connect ${PROVIDER_LABELS[error] ?? error}`);
    }
    // Clear the one-shot params so the toast doesn't refire on refresh.
    navigate({ search: (s) => ({ ...s, connected: undefined, error: undefined }), replace: true });
  }, [connected, error, tab, navigate]);

  const setTab = (next: GitTab) =>
    navigate({ search: (s) => ({ ...s, tab: next === "providers" ? "providers" : undefined }) });

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-semibold">Git</h1>
        <p className="text-muted-foreground text-sm">
          Connect your git accounts to browse and deploy repositories
          {isAdmin ? ", and register the provider apps that power them." : "."}
        </p>
      </div>

      {isAdmin ? (
        <Tabs value={activeTab} onValueChange={(v) => setTab(v as GitTab)}>
          <TabsList>
            <TabsTrigger value="connections">Connections</TabsTrigger>
            <TabsTrigger value="providers">Providers</TabsTrigger>
          </TabsList>
          <TabsContent value="connections" className="pt-4">
            <ConnectionsPanel />
          </TabsContent>
          <TabsContent value="providers" className="pt-4">
            <ProvidersPanel />
          </TabsContent>
        </Tabs>
      ) : (
        <ConnectionsPanel />
      )}
    </div>
  );
}
