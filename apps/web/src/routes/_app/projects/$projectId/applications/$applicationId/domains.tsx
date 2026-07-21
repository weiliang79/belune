import { createFileRoute } from "@tanstack/react-router";
import { useState } from "react";
import { Globe } from "lucide-react";
import { useDomains } from "@/lib/hooks/use-domains";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { DomainFormDialog } from "@/components/domains/domain-form-dialog";
import { DomainFeaturesDialog } from "@/components/domains/domain-features";
import { DomainsTable } from "@/components/domains/domains-table";
import { DomainEmptyState } from "@/components/domains/domain-empty-state";
import { DomainListSkeleton } from "@/components/domains/domain-list-skeleton";
import type { DomainExpanded } from "@/lib/types";

export const Route = createFileRoute(
  "/_app/projects/$projectId/applications/$applicationId/domains",
)({
  component: DomainsPage,
});

function DomainsPage() {
  const { projectId, applicationId } = Route.useParams();
  const { data: domains, isLoading } = useDomains(projectId, applicationId);
  const [dialogOpen, setDialogOpen] = useState(false);
  const [editing, setEditing] = useState<DomainExpanded | undefined>(undefined);
  const [featuresFor, setFeaturesFor] = useState<DomainExpanded | undefined>(
    undefined,
  );

  const openAdd = () => {
    setEditing(undefined);
    setDialogOpen(true);
  };
  const openEdit = (domain: DomainExpanded) => {
    setEditing(domain);
    setDialogOpen(true);
  };
  const onOpenChange = (open: boolean) => {
    setDialogOpen(open);
    if (!open) setEditing(undefined);
  };

  return (
    <div className="space-y-4">
      <Card>
        <CardHeader>
          <div className="flex items-center justify-between">
            <div>
              <CardTitle className="flex items-center gap-2">
                <Globe aria-hidden="true" className="size-4" />
                Domains
              </CardTitle>
              <CardDescription>
                Configure hostnames, TLS, and routing for this application.
              </CardDescription>
            </div>
            <Button size="sm" variant="outline" onClick={openAdd}>
              Add Domain
            </Button>
          </div>
        </CardHeader>
        <CardContent>
          {isLoading ? (
            <DomainListSkeleton />
          ) : !domains || domains.length === 0 ? (
            <DomainEmptyState onAdd={openAdd} />
          ) : (
            <DomainsTable
              projectId={projectId}
              applicationId={applicationId}
              domains={domains}
              isLoading={isLoading}
              onEdit={openEdit}
              onEditFeatures={setFeaturesFor}
            />
          )}
        </CardContent>
      </Card>

      <DomainFormDialog
        projectId={projectId}
        applicationId={applicationId}
        domain={editing}
        open={dialogOpen}
        onOpenChange={onOpenChange}
      />

      <DomainFeaturesDialog
        projectId={projectId}
        applicationId={applicationId}
        domain={featuresFor}
        open={Boolean(featuresFor)}
        onOpenChange={(open) => !open && setFeaturesFor(undefined)}
      />
    </div>
  );
}
