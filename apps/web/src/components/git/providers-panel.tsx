import { useState } from "react";
import { Trash2, ExternalLink, Plus, Lock, Users } from "lucide-react";
import {
  useGitProviderConfigs,
  useDeleteGitProviderConfig,
} from "@/lib/hooks/use-git-providers";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { ProviderFormDialog } from "./provider-form-dialog";
import { ProviderIcon } from "./provider-icon";

export function ProvidersPanel() {
  const { data: configs } = useGitProviderConfigs();
  const del = useDeleteGitProviderConfig();
  const [dialogOpen, setDialogOpen] = useState(false);

  return (
    <div className="space-y-6">
      <Card>
        <CardHeader className="flex flex-row items-center justify-between">
          <CardTitle>Configured Providers</CardTitle>
          <Button size="sm" onClick={() => setDialogOpen(true)}>
            <Plus className="mr-1 h-4 w-4" /> Add provider
          </Button>
        </CardHeader>
        <CardContent>
          {!configs || configs.length === 0 ? (
            <p className="text-muted-foreground text-sm">
              No providers configured yet. Add one so users can connect their
              accounts.
            </p>
          ) : (
            <div className="divide-y">
              {configs.map((c) => (
                <div
                  key={c.id}
                  className="flex items-center justify-between py-3"
                >
                  <div className="space-y-1">
                    <div className="flex flex-wrap items-center gap-2">
                      <ProviderIcon
                        provider={c.provider}
                        className="text-foreground size-4"
                      />
                      <span className="font-medium capitalize">{c.provider}</span>
                      {c.has_secret ? (
                        <Badge variant="secondary">Configured</Badge>
                      ) : (
                        <Badge variant="outline">Missing secret</Badge>
                      )}
                      {c.is_public ? (
                        <Badge variant="outline" className="gap-1">
                          <Users className="h-3 w-3" /> Public
                        </Badge>
                      ) : (
                        <Badge variant="outline" className="gap-1">
                          <Lock className="h-3 w-3" /> Private
                        </Badge>
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
                      title="Remove provider"
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

      <ProviderFormDialog open={dialogOpen} onOpenChange={setDialogOpen} />
    </div>
  );
}
