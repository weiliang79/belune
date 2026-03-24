import { createFileRoute } from "@tanstack/react-router";
import {
  useDomains,
  useAddDomain,
  useRemoveDomain,
} from "@/lib/hooks/use-domains";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
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
import { Skeleton } from "@/components/ui/skeleton";
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

  const handleAdd = () => {
    const trimmed = hostname.trim();
    if (!trimmed) return;
    if (!/^([a-zA-Z0-9]([a-zA-Z0-9-]*[a-zA-Z0-9])?\.)+[a-zA-Z]{2,}$/.test(trimmed)) {
      toast.error("Invalid hostname format");
      return;
    }
    toast.promise(
      addDomain.mutateAsync({
        hostname: trimmed,
        ssl_enabled: sslEnabled,
      }).then(() => {
        setHostname("");
      }),
      {
        loading: "Adding domain...",
        success: "Domain added",
        error: (err) => err.message,
      },
    );
  };

  if (isLoading) {
    return (
      <div className="space-y-4">
        <Card>
          <CardHeader><Skeleton className="h-6 w-32" /></CardHeader>
          <CardContent><Skeleton className="h-10 w-full" /></CardContent>
        </Card>
        <Card>
          <CardHeader><Skeleton className="h-6 w-24" /></CardHeader>
          <CardContent className="space-y-2">
            {[1, 2].map((i) => (
              <Skeleton key={i} className="h-12 w-full" />
            ))}
          </CardContent>
        </Card>
      </div>
    );
  }

  return (
    <div className="space-y-4">
      <Card>
        <CardHeader>
          <CardTitle>Add Domain</CardTitle>
        </CardHeader>
        <CardContent>
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
                  <AlertDialog>
                    <AlertDialogTrigger
                      render={
                        <Button
                          size="sm"
                          variant="ghost"
                          className="text-destructive"
                        />
                      }
                    >
                      Remove
                    </AlertDialogTrigger>
                    <AlertDialogContent>
                      <AlertDialogHeader>
                        <AlertDialogTitle>
                          Remove {domain.hostname}?
                        </AlertDialogTitle>
                        <AlertDialogDescription>
                          This will remove the domain from this application.
                        </AlertDialogDescription>
                      </AlertDialogHeader>
                      <AlertDialogFooter>
                        <AlertDialogCancel>Cancel</AlertDialogCancel>
                        <AlertDialogAction
                          onClick={() => {
                            toast.promise(
                              removeDomain.mutateAsync(domain.id),
                              {
                                loading: "Removing domain...",
                                success: "Domain removed",
                                error: (err) => err.message,
                              },
                            );
                          }}
                          className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
                        >
                          Remove
                        </AlertDialogAction>
                      </AlertDialogFooter>
                    </AlertDialogContent>
                  </AlertDialog>
                </div>
              ))}
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
