import { useMemo } from "react";
import { toast } from "sonner";
import type { ColumnDef } from "@tanstack/react-table";
import {
  CopyIcon,
  ExternalLinkIcon,
  MoreHorizontal,
  PencilIcon,
  SlidersHorizontalIcon,
  Trash2Icon,
} from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { DataTable, buildActionColumnDef } from "@/components/ui/data-table";
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
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { useRemoveDomain } from "@/lib/hooks/use-domains";
import type { DomainExpanded, RouteFeature } from "@/lib/types";
import { DomainTLSBadge } from "./domain-tls-badge";

const FEATURE_LABELS: Record<string, string> = {
  basic_auth: "Basic Auth",
  headers: "Custom Headers",
  ip_allowlist: "IP Allowlist",
  redirect: "Redirect",
  rate_limit: "Rate Limit",
};

/**
 * The domains of one application, as a table.
 *
 * A list read fine for one domain and badly for three. An app's domains are
 * siblings — admin / merchant / customer — and the questions you ask of them are
 * comparisons: which one has basic auth, which one is still pending a
 * certificate, did one drift onto a different port. In a list the badges sit at
 * whatever horizontal position the hostname's length leaves them, so the odd one
 * out is invisible. Columns are what make it obvious.
 *
 * Deliberately lean: no search, no pagination, no sorting. You do not search
 * three rows, and the chrome would cost more than it returns.
 */
export function DomainsTable({
  projectId,
  applicationId,
  domains,
  isLoading,
  onEdit,
  onEditFeatures,
}: {
  projectId: string;
  applicationId: string;
  domains: DomainExpanded[];
  isLoading: boolean;
  onEdit: (domain: DomainExpanded) => void;
  onEditFeatures: (domain: DomainExpanded) => void;
}) {
  const columns = useMemo<ColumnDef<DomainExpanded>[]>(
    () => [
      {
        accessorKey: "hostname",
        header: "Domain",
        cell: ({ row }) => <DomainLink domain={row.original} />,
      },
      {
        accessorKey: "tls_status",
        header: "TLS",
        cell: ({ row }) => (
          <DomainTLSBadge
            domain={row.original}
            projectId={projectId}
            applicationId={applicationId}
          />
        ),
      },
      {
        accessorKey: "force_https",
        header: "HTTPS",
        cell: ({ row }) =>
          row.original.force_https ? (
            <Badge variant="outline">Forced</Badge>
          ) : (
            <span className="text-muted-foreground">—</span>
          ),
      },
      {
        accessorKey: "container_port",
        header: "Port",
        cell: ({ row }) =>
          row.original.container_port ? (
            <span className="font-mono text-xs">
              {row.original.container_port}
            </span>
          ) : (
            <span className="text-muted-foreground">Default</span>
          ),
      },
      {
        accessorKey: "route_features",
        header: "Features",
        // The names, not a count. "1 feature" tells you something is configured
        // but not what — and "what" is the entire question when you are checking
        // whether the admin subdomain is the one behind basic auth.
        cell: ({ row }) => <FeatureCell features={row.original.route_features} />,
      },
      buildActionColumnDef<DomainExpanded>({
        meta: { headerClassName: "text-right", className: "text-right" },
        cell: ({ row: { original: domain } }) => (
          <DomainActions
            projectId={projectId}
            applicationId={applicationId}
            domain={domain}
            onEdit={() => onEdit(domain)}
            onEditFeatures={() => onEditFeatures(domain)}
          />
        ),
      }),
    ],
    [projectId, applicationId, onEdit, onEditFeatures],
  );

  return (
    <DataTable
      columns={columns}
      data={domains}
      isLoading={isLoading}
      getRowId={(d) => d.id}
      emptyMessage="No domains configured yet."
    />
  );
}

// The scheme follows the domain's own TLS mode rather than assuming https. An
// ssl_mode=off domain has no HTTPS listener for that name, so linking to https
// would hand the user a connection error on a domain that is working exactly as
// configured. (The header's "Open URL" button is blunter and always says https.)
function DomainLink({ domain }: { domain: DomainExpanded }) {
  const scheme = domain.ssl_mode === "off" ? "http" : "https";
  return (
    <a
      href={`${scheme}://${domain.hostname}`}
      target="_blank"
      rel="noopener noreferrer"
      className="group inline-flex items-center gap-1.5 font-mono text-sm hover:underline"
    >
      {domain.hostname}
      <ExternalLinkIcon
        aria-hidden="true"
        className="text-muted-foreground size-3 opacity-0 transition-opacity group-hover:opacity-100"
      />
    </a>
  );
}

function FeatureCell({ features }: { features?: RouteFeature[] }) {
  const list = features ?? [];
  if (list.length === 0) {
    return <span className="text-muted-foreground">—</span>;
  }
  return (
    <div className="flex flex-wrap gap-1">
      {list.map((f) => {
        const label = FEATURE_LABELS[f.feature_type] ?? f.feature_type;
        // A disabled feature is the dangerous case: basic auth that is present
        // but switched off looks, at a glance, exactly like basic auth. Say so.
        return f.enabled ? (
          <Badge key={f.id} variant="light">
            {label}
          </Badge>
        ) : (
          <Badge key={f.id} variant="outline" className="text-muted-foreground">
            {label} · off
          </Badge>
        );
      })}
    </div>
  );
}

function DomainActions({
  domain,
  projectId,
  applicationId,
  onEdit,
  onEditFeatures,
}: {
  domain: DomainExpanded;
  projectId: string;
  applicationId: string;
  onEdit: () => void;
  onEditFeatures: () => void;
}) {
  const removeDomain = useRemoveDomain(projectId, applicationId);

  const handleCopyHostname = () => {
    navigator.clipboard
      .writeText(domain.hostname)
      .then(() => toast.success("Hostname copied"))
      .catch(() => toast.error("Failed to copy"));
  };

  return (
    <div className="flex justify-end">
      <AlertDialog>
        <DropdownMenu>
          <DropdownMenuTrigger
            render={<Button size="icon" variant="ghost" aria-label="Actions" />}
          >
            <MoreHorizontal className="h-4 w-4" />
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end">
            <DropdownMenuItem onClick={onEdit}>
              <PencilIcon aria-hidden="true" />
              Edit
            </DropdownMenuItem>
            <DropdownMenuItem onClick={onEditFeatures}>
              <SlidersHorizontalIcon aria-hidden="true" />
              Route features
            </DropdownMenuItem>
            <DropdownMenuItem onClick={handleCopyHostname}>
              <CopyIcon aria-hidden="true" />
              Copy hostname
            </DropdownMenuItem>
            <DropdownMenuSeparator />
            <AlertDialogTrigger
              render={<DropdownMenuItem variant="destructive" />}
              nativeButton={false}
            >
              <Trash2Icon aria-hidden="true" />
              Delete
            </AlertDialogTrigger>
          </DropdownMenuContent>
        </DropdownMenu>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Delete {domain.hostname}?</AlertDialogTitle>
            <AlertDialogDescription>
              This will delete the domain and all its route features.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction
              onClick={() => {
                toast.promise(removeDomain.mutateAsync(domain.id), {
                  loading: "Deleting domain…",
                  success: "Domain deleted",
                  error: (err) => err.message,
                });
              }}
              className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
            >
              Delete
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}
