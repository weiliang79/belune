import { useMemo, useState } from "react";
import { createFileRoute } from "@tanstack/react-router";
import { useForm } from "@tanstack/react-form";
import { z } from "zod";
import type { ColumnDef } from "@tanstack/react-table";
import { Lock, Trash2 } from "lucide-react";
import { RouteError } from "@/lib/components/route-error";
import {
  useCertificates,
  useUploadCertificate,
  useDeleteCertificate,
} from "@/lib/hooks/use-certificates";
import type { Certificate } from "@/lib/api/certificates";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { PageHeader } from "@/components/ui/page-header";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { DataTable, buildActionColumnDef } from "@/components/ui/data-table";
import {
  Tooltip,
  TooltipContent,
  TooltipPositioner,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";

export const Route = createFileRoute("/_app/certificates")({
  component: CertificatesPage,
  errorComponent: RouteError,
});

// EXPIRY_WARNING_DAYS mirrors the threshold the backend will notify on.
const EXPIRY_WARNING_DAYS = 14;

function daysUntil(iso: string | null): number | null {
  if (!iso) return null;
  return Math.floor(
    (new Date(iso).getTime() - Date.now()) / (1000 * 60 * 60 * 24),
  );
}

function ExpiryCell({ notAfter }: { notAfter: string | null }) {
  const days = daysUntil(notAfter);
  if (days === null) {
    return <span className="text-muted-foreground">—</span>;
  }

  const formatted = new Date(notAfter!).toLocaleDateString();
  if (days < 0) {
    return <Badge variant="destructive">Expired {formatted}</Badge>;
  }
  if (days <= EXPIRY_WARNING_DAYS) {
    return (
      <span className="text-status-building">
        {formatted} · {days}d left
      </span>
    );
  }
  return <span>{formatted}</span>;
}

function CertificatesPage() {
  const { data: certificates, isLoading } = useCertificates();
  const [uploadOpen, setUploadOpen] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<Certificate | null>(null);

  const expiringSoon = useMemo(
    () =>
      (certificates ?? []).filter((c) => {
        const days = daysUntil(c.not_after);
        return days !== null && days <= EXPIRY_WARNING_DAYS;
      }).length,
    [certificates],
  );

  const columns = useMemo<ColumnDef<Certificate>[]>(
    () => [
      {
        accessorKey: "name",
        header: "Name",
        cell: ({ row }) => (
          <span className="font-medium">{row.original.name}</span>
        ),
      },
      {
        accessorKey: "subjects",
        header: "Subjects",
        cell: ({ row }) => (
          <div className="flex flex-wrap gap-1">
            {row.original.subjects.map((s) => (
              <Badge key={s} variant="secondary" className="font-mono text-xs">
                {s}
              </Badge>
            ))}
          </div>
        ),
      },
      {
        accessorKey: "issuer",
        header: "Issuer",
        cell: ({ row }) => (
          <span className="text-muted-foreground truncate">
            {row.original.issuer || "—"}
          </span>
        ),
      },
      {
        accessorKey: "not_after",
        header: "Expires",
        cell: ({ row }) => <ExpiryCell notAfter={row.original.not_after} />,
      },
      {
        accessorKey: "domain_count",
        header: "In use",
        cell: ({ row }) => {
          const count = row.original.domain_count;
          return count === 0 ? (
            <span className="text-muted-foreground">Unused</span>
          ) : (
            <span>
              {count} {count === 1 ? "domain" : "domains"}
            </span>
          );
        },
      },
      buildActionColumnDef<Certificate>({
        meta: { headerClassName: "text-right", className: "text-right" },
        cell: ({ row: { original: cert } }) => (
          <div className="flex justify-end">
            <Tooltip>
              <TooltipTrigger
                render={
                  <Button
                    variant="ghost"
                    size="icon-sm"
                    aria-label="Delete certificate"
                    className="text-destructive hover:bg-destructive/10 hover:text-destructive"
                    onClick={() => setDeleteTarget(cert)}
                  />
                }
              >
                <Trash2 className="h-4 w-4" />
              </TooltipTrigger>
              <TooltipPositioner>
                <TooltipContent>Delete</TooltipContent>
              </TooltipPositioner>
            </Tooltip>
          </div>
        ),
      }),
    ],
    [],
  );

  return (
    <div className="space-y-6">
      <PageHeader
        icon={<Lock className="size-5" />}
        title={
          <>
            Certificates
            {certificates && certificates.length > 0 && (
              <span className="text-muted-foreground ml-2 text-base font-normal">
                {certificates.length}{" "}
                {certificates.length === 1 ? "certificate" : "certificates"}
                {expiringSoon > 0 && (
                  <span className="text-status-building">
                    {" "}
                    · {expiringSoon} expiring soon
                  </span>
                )}
              </span>
            )}
          </>
        }
        description="Certificates you upload once and select on any domain set to Custom SSL. Domains using Automatic get a certificate from Let's Encrypt instead — nothing to upload here."
      />

      <Card>
        <CardHeader className="flex flex-row items-center justify-between">
          <CardTitle>Uploaded Certificates</CardTitle>
          <Button size="sm" onClick={() => setUploadOpen(true)}>
            Upload Certificate
          </Button>
        </CardHeader>
        <CardContent>
          <DataTable
            columns={columns}
            data={certificates ?? []}
            isLoading={isLoading}
            getRowId={(c) => c.id}
            enableSorting
            emptyMessage='No certificates uploaded. Click "Upload Certificate" to add one.'
          />
        </CardContent>
      </Card>

      <UploadCertificateDialog open={uploadOpen} onOpenChange={setUploadOpen} />
      {deleteTarget && (
        <DeleteCertificateDialog
          certificate={deleteTarget}
          open={true}
          onOpenChange={(open) => !open && setDeleteTarget(null)}
        />
      )}
    </div>
  );
}

const uploadSchema = z.object({
  name: z.string().min(1, "Name is required"),
  cert_pem: z
    .string()
    .includes("BEGIN CERTIFICATE", {
      message: "Expected a PEM block starting with -----BEGIN CERTIFICATE-----",
    }),
  key_pem: z
    .string()
    .includes("PRIVATE KEY", {
      message: "Expected a PEM block containing -----BEGIN PRIVATE KEY-----",
    }),
});

function UploadCertificateDialog({
  open,
  onOpenChange,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const upload = useUploadCertificate();

  const form = useForm({
    defaultValues: { name: "", cert_pem: "", key_pem: "" },
    validators: { onSubmit: uploadSchema },
    onSubmit: async ({ value }) => {
      await upload.mutateAsync(value);
      form.reset();
      onOpenChange(false);
    },
  });

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-2xl">
        <DialogHeader>
          <DialogTitle>Upload Certificate</DialogTitle>
          <DialogDescription>
            Paste the full certificate chain and its private key. The key is
            encrypted at rest and never shown again.
          </DialogDescription>
        </DialogHeader>

        <form
          onSubmit={(e) => {
            e.preventDefault();
            void form.handleSubmit();
          }}
          className="space-y-4"
        >
          <form.Field name="name">
            {(field) => (
              <div className="space-y-2">
                <Label htmlFor="name">Name</Label>
                <Input
                  id="name"
                  placeholder="example.com origin"
                  value={field.state.value}
                  onChange={(e) => field.handleChange(e.target.value)}
                  onBlur={field.handleBlur}
                />
                <FieldError field={field} />
              </div>
            )}
          </form.Field>

          <form.Field name="cert_pem">
            {(field) => (
              <div className="space-y-2">
                <Label htmlFor="cert_pem">Certificate (PEM)</Label>
                <Textarea
                  id="cert_pem"
                  rows={7}
                  spellCheck={false}
                  className="font-mono text-xs"
                  placeholder="-----BEGIN CERTIFICATE-----"
                  value={field.state.value}
                  onChange={(e) => field.handleChange(e.target.value)}
                  onBlur={field.handleBlur}
                />
                <p className="text-muted-foreground text-xs">
                  Include any intermediate certificates below the leaf.
                </p>
                <FieldError field={field} />
              </div>
            )}
          </form.Field>

          <form.Field name="key_pem">
            {(field) => (
              <div className="space-y-2">
                <Label htmlFor="key_pem">Private Key (PEM)</Label>
                <Textarea
                  id="key_pem"
                  rows={7}
                  spellCheck={false}
                  className="font-mono text-xs"
                  placeholder="-----BEGIN PRIVATE KEY-----"
                  value={field.state.value}
                  onChange={(e) => field.handleChange(e.target.value)}
                  onBlur={field.handleBlur}
                />
                <FieldError field={field} />
              </div>
            )}
          </form.Field>

          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              onClick={() => onOpenChange(false)}
            >
              Cancel
            </Button>
            <Button type="submit" disabled={upload.isPending}>
              {upload.isPending ? "Uploading…" : "Upload"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

// FieldError renders the first validation message for a TanStack form field.
function FieldError({
  field,
}: {
  field: { state: { meta: { errors: unknown[]; isTouched: boolean } } };
}) {
  const error = field.state.meta.errors[0];
  if (!error || !field.state.meta.isTouched) return null;
  const message =
    typeof error === "string"
      ? error
      : ((error as { message?: string }).message ?? "Invalid value");
  return <p className="text-destructive text-xs">{message}</p>;
}

function DeleteCertificateDialog({
  certificate,
  open,
  onOpenChange,
}: {
  certificate: Certificate;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const del = useDeleteCertificate();
  const inUse = certificate.domain_count > 0;

  return (
    <AlertDialog open={open} onOpenChange={onOpenChange}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>Delete {certificate.name}?</AlertDialogTitle>
          <AlertDialogDescription>
            {inUse
              ? `${certificate.domain_count} ${certificate.domain_count === 1 ? "domain is" : "domains are"} still serving this certificate. Point them at another certificate first — deleting it would break TLS for them.`
              : "This certificate is not in use. Deleting it cannot be undone; you would need to upload the PEM pair again."}
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>Cancel</AlertDialogCancel>
          <AlertDialogAction
            disabled={inUse || del.isPending}
            onClick={async () => {
              await del.mutateAsync(certificate.id);
              onOpenChange(false);
            }}
          >
            Delete
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}
