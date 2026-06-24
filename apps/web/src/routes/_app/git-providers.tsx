import { useState } from "react";
import { createFileRoute } from "@tanstack/react-router";
import { toast } from "sonner";
import { Github, Trash2, ExternalLink } from "lucide-react";
import { RouteError } from "@/lib/components/route-error";
import {
  useGitProviderConfigs,
  useSaveGitProviderConfig,
  useDeleteGitProviderConfig,
} from "@/lib/hooks/use-git-providers";
import {
  getGitHubManifest,
  submitGitHubManifest,
  type GitProvider,
} from "@/lib/api/git-providers";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Badge } from "@/components/ui/badge";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";

export const Route = createFileRoute("/_app/git-providers")({
  component: GitProvidersPage,
  errorComponent: RouteError,
});

const OAUTH_PROVIDERS: { value: GitProvider; label: string; selfHosted: boolean }[] = [
  { value: "gitlab", label: "GitLab", selfHosted: true },
  { value: "bitbucket", label: "Bitbucket", selfHosted: false },
  { value: "gitea", label: "Gitea", selfHosted: true },
];

function GitProvidersPage() {
  const { data: configs } = useGitProviderConfigs();
  const save = useSaveGitProviderConfig();
  const del = useDeleteGitProviderConfig();

  const [org, setOrg] = useState("");
  const [oauthProvider, setOauthProvider] = useState<GitProvider>("gitlab");
  const [baseUrl, setBaseUrl] = useState("");
  const [clientId, setClientId] = useState("");
  const [clientSecret, setClientSecret] = useState("");

  const origin = window.location.origin;
  const selfHosted =
    OAUTH_PROVIDERS.find((p) => p.value === oauthProvider)?.selfHosted ?? false;
  const callbackUrl = `${origin}/api/git/providers/${oauthProvider}/oauth/callback`;

  const startGitHubApp = async () => {
    try {
      const manifest = await getGitHubManifest(org || undefined);
      submitGitHubManifest(manifest);
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "Failed to start GitHub App setup");
    }
  };

  const saveOAuth = () => {
    if (!clientId || !clientSecret) {
      toast.error("Client ID and secret are required");
      return;
    }
    toast.promise(
      save
        .mutateAsync({
          provider: oauthProvider,
          base_url: baseUrl || undefined,
          client_id: clientId,
          client_secret: clientSecret,
        })
        .then(() => {
          setClientId("");
          setClientSecret("");
        }),
      { loading: "Saving...", success: "Provider saved", error: (e) => e.message },
    );
  };

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-semibold">Git Providers</h1>
        <p className="text-muted-foreground text-sm">
          Connect a GitHub App or OAuth client so users can link their accounts
          and deploy from their repositories.
        </p>
      </div>

      {/* GitHub App */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Github className="h-5 w-5" /> GitHub App
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <p className="text-muted-foreground text-sm">
            Create a GitHub App for this instance. GitHub will guide you through
            the setup and redirect back here automatically.
          </p>
          <div className="flex items-end gap-2">
            <div className="space-y-2">
              <Label htmlFor="gh-org">Organization (optional)</Label>
              <Input
                id="gh-org"
                value={org}
                onChange={(e) => setOrg(e.target.value)}
                placeholder="my-org (leave empty for personal account)"
                className="w-72"
              />
            </div>
            <Button onClick={startGitHubApp}>
              <Github className="mr-1 h-4 w-4" /> Create GitHub App
            </Button>
          </div>
        </CardContent>
      </Card>

      {/* OAuth providers */}
      <Card>
        <CardHeader>
          <CardTitle>OAuth Provider</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <p className="text-muted-foreground text-sm">
            Register an OAuth application on the provider, then enter its client
            credentials here. Use this redirect/callback URL when registering:
          </p>
          <code className="bg-muted block rounded px-3 py-2 text-xs break-all">
            {callbackUrl}
          </code>
          <div className="grid gap-4 sm:grid-cols-2">
            <div className="space-y-2">
              <Label>Provider</Label>
              <Select
                value={oauthProvider}
                onValueChange={(v) => setOauthProvider(v as GitProvider)}
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {OAUTH_PROVIDERS.map((p) => (
                    <SelectItem key={p.value} value={p.value}>
                      {p.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            {selfHosted && (
              <div className="space-y-2">
                <Label>Base URL</Label>
                <Input
                  value={baseUrl}
                  onChange={(e) => setBaseUrl(e.target.value)}
                  placeholder="https://gitlab.example.com"
                />
              </div>
            )}
            <div className="space-y-2">
              <Label>Client ID</Label>
              <Input
                value={clientId}
                onChange={(e) => setClientId(e.target.value)}
              />
            </div>
            <div className="space-y-2">
              <Label>Client Secret</Label>
              <Input
                type="password"
                value={clientSecret}
                onChange={(e) => setClientSecret(e.target.value)}
                placeholder="Leave empty to keep existing"
              />
            </div>
          </div>
          <Button onClick={saveOAuth} disabled={save.isPending}>
            Save Provider
          </Button>
        </CardContent>
      </Card>

      {/* Configured providers */}
      <Card>
        <CardHeader>
          <CardTitle>Configured Providers</CardTitle>
        </CardHeader>
        <CardContent>
          {!configs || configs.length === 0 ? (
            <p className="text-muted-foreground text-sm">
              No providers configured yet.
            </p>
          ) : (
            <div className="divide-y">
              {configs.map((c) => (
                <div
                  key={c.id}
                  className="flex items-center justify-between py-3"
                >
                  <div className="space-y-1">
                    <div className="flex items-center gap-2">
                      <span className="font-medium capitalize">{c.provider}</span>
                      {c.has_secret ? (
                        <Badge variant="secondary">Configured</Badge>
                      ) : (
                        <Badge variant="outline">Missing secret</Badge>
                      )}
                    </div>
                    <p className="text-muted-foreground text-xs">
                      {c.base_url || "default host"}
                      {c.app_slug ? ` · ${c.app_slug}` : ""}
                      {c.client_id ? ` · ${c.client_id}` : ""}
                    </p>
                  </div>
                  <div className="flex items-center gap-2">
                    {c.app_slug && (
                      <a
                        href={`https://github.com/apps/${c.app_slug}`}
                        target="_blank"
                        rel="noreferrer"
                        className="text-muted-foreground hover:text-foreground"
                        title="Open on GitHub"
                      >
                        <ExternalLink className="h-4 w-4" />
                      </a>
                    )}
                    <Button
                      variant="ghost"
                      size="icon"
                      onClick={() => {
                        if (confirm(`Remove ${c.provider} provider config?`)) {
                          del.mutate(c.id);
                        }
                      }}
                    >
                      <Trash2 className="h-4 w-4" />
                    </Button>
                  </div>
                </div>
              ))}
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
