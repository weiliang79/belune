import { createFileRoute } from "@tanstack/react-router";
import { useState } from "react";
import { useDomains } from "@/lib/hooks/use-domains";
import { Button } from "@/components/ui/button";
import { DomainFormDialog } from "@/components/domains/domain-form-dialog";
import { DomainFeaturesDialog } from "@/components/domains/domain-features";
import { DomainRow } from "@/components/domains/domain-row";
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
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-lg font-semibold">Domains</h2>
          <p className="text-muted-foreground text-sm">
            Configure hostnames, TLS, and routing for this application.
          </p>
        </div>
        <Button onClick={openAdd}>Add Domain</Button>
      </div>

      {isLoading ? (
        <DomainListSkeleton />
      ) : !domains || domains.length === 0 ? (
        <DomainEmptyState onAdd={openAdd} />
      ) : (
        <div className="space-y-3">
          {domains.map((domain) => (
            <DomainRow
              key={domain.id}
              projectId={projectId}
              applicationId={applicationId}
              domain={domain}
              onEdit={() => openEdit(domain)}
              onEditFeatures={() => setFeaturesFor(domain)}
            />
          ))}
        </div>
      )}

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
