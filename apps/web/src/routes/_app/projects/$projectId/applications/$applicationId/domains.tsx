import { createFileRoute } from "@tanstack/react-router";
import {
  useDomains,
  useAddDomain,
  useRemoveDomain,
} from "@/lib/hooks/use-domains";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { useState } from "react";

export const Route = createFileRoute(
  "/_app/projects/$projectId/applications/$applicationId/domains",
)({
  component: DomainsPage,
});

function DomainsPage() {
  const { projectId, applicationId } = Route.useParams();
  const { data: domains, isLoading } = useDomains(projectId, applicationId);
  const addDomain = useAddDomain(projectId, applicationId);
  const removeDomain = useRemoveDomain(projectId, applicationId);
  const [hostname, setHostname] = useState("");
  const [sslEnabled, setSslEnabled] = useState(true);
  const [error, setError] = useState("");

  const handleAdd = async () => {
    if (!hostname.trim()) return;
    setError("");
    try {
      await addDomain.mutateAsync({
        hostname: hostname.trim(),
        ssl_enabled: sslEnabled,
      });
      setHostname("");
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to add domain");
    }
  };

  if (isLoading) {
    return <div className="text-muted-foreground">Loading domains...</div>;
  }

  return (
    <div className="space-y-4">
      <Card>
        <CardHeader>
          <CardTitle>Add Domain</CardTitle>
        </CardHeader>
        <CardContent>
          {error && (
            <div className="bg-destructive/10 text-destructive mb-3 rounded-md px-3 py-2 text-sm">
              {error}
            </div>
          )}
          <div className="flex items-end gap-3">
            <div className="flex-1 space-y-2">
              <Label htmlFor="hostname">Hostname</Label>
              <Input
                id="hostname"
                placeholder="app.example.com"
                value={hostname}
                onChange={(e) => setHostname(e.target.value)}
              />
            </div>
            <label className="flex items-center gap-1.5 pb-2 text-sm">
              <input
                type="checkbox"
                checked={sslEnabled}
                onChange={(e) => setSslEnabled(e.target.checked)}
              />
              SSL
            </label>
            <Button onClick={handleAdd} disabled={addDomain.isPending}>
              {addDomain.isPending ? "Adding..." : "Add"}
            </Button>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Domains</CardTitle>
        </CardHeader>
        <CardContent>
          {!domains || domains.length === 0 ? (
            <p className="text-muted-foreground py-6 text-center text-sm">
              No domains configured.
            </p>
          ) : (
            <div className="space-y-2">
              {domains.map((domain) => (
                <div
                  key={domain.id}
                  className="flex items-center justify-between rounded-md border p-3"
                >
                  <div className="flex items-center gap-2">
                    <span className="font-mono text-sm">{domain.hostname}</span>
                    {domain.ssl_enabled && (
                      <Badge variant="secondary">SSL</Badge>
                    )}
                    {domain.verified_at && (
                      <Badge variant="default">Verified</Badge>
                    )}
                  </div>
                  <Button
                    size="sm"
                    variant="ghost"
                    className="text-destructive"
                    onClick={() => removeDomain.mutate(domain.id)}
                    disabled={removeDomain.isPending}
                  >
                    Remove
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
