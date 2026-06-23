import { useMemo, useState } from "react";
import { createFileRoute } from "@tanstack/react-router";
import { toast } from "sonner";
import { useForm, useStore } from "@tanstack/react-form";
import { z } from "zod";
import { RouteError } from "@/lib/components/route-error";
import {
  useQuotas,
  useUpsertQuota,
  useDeleteQuota,
} from "@/lib/hooks/use-quotas";
import { useUsers } from "@/lib/hooks/use-users";
import { useProjects } from "@/lib/hooks/use-projects";
import type { QuotaLimits, QuotaScope, QuotaView } from "@/lib/api/quotas";
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
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { formatBytes } from "@/lib/utils/format";

export const Route = createFileRoute("/_app/quotas")({
  component: QuotasPage,
  errorComponent: RouteError,
});

/** A quota is "near limit" when any capped resource is at ≥ 80% usage. */
function isNearLimit(q: QuotaView): boolean {
  const ratios = [
    q.limits.max_applications
      ? q.usage.applications / q.limits.max_applications
      : 0,
    q.limits.max_cpu ? q.usage.cpu / q.limits.max_cpu : 0,
    q.limits.max_memory_mb
      ? q.usage.memory_bytes / (1024 * 1024) / q.limits.max_memory_mb
      : 0,
  ];
  return ratios.some((r) => r >= 0.8);
}

function QuotasPage() {
  const { data: quotas, isLoading } = useQuotas();
  const [editTarget, setEditTarget] = useState<QuotaView | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<QuotaView | null>(null);
  const [createOpen, setCreateOpen] = useState(false);

  const nearLimit = quotas?.filter(isNearLimit).length ?? 0;

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-semibold tracking-tight">
          Quotas
          {quotas && quotas.length > 0 && (
            <span className="text-muted-foreground ml-2 text-base font-normal">
              {quotas.length} {quotas.length === 1 ? "rule" : "rules"}
              {nearLimit > 0 && (
                <span className="text-status-building"> · {nearLimit} near limit</span>
              )}
            </span>
          )}
        </h1>
        <p className="text-muted-foreground text-sm">
          Aggregate caps on top of per-container limits. Unset fields mean
          unlimited.
        </p>
      </div>

      <Card>
        <CardHeader className="flex flex-row items-center justify-between">
          <CardTitle>Configured Quotas</CardTitle>
          <Button size="sm" onClick={() => setCreateOpen(true)}>
            Set Quota
          </Button>
        </CardHeader>
        <CardContent>
          {isLoading ? (
            <div className="space-y-3">
              {[1, 2].map((i) => (
                <div key={i} className="bg-muted h-12 animate-pulse rounded" />
              ))}
            </div>
          ) : !quotas || quotas.length === 0 ? (
            <p className="text-muted-foreground text-sm">
              No quotas configured. Click "Set Quota" to add one.
            </p>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Scope</TableHead>
                  <TableHead>Target</TableHead>
                  <TableHead>Applications</TableHead>
                  <TableHead>CPU</TableHead>
                  <TableHead>Memory</TableHead>
                  <TableHead className="text-right">Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {quotas.map((q) => (
                  <QuotaRow
                    key={`${q.scope}:${q.scope_id}`}
                    quota={q}
                    onEdit={() => setEditTarget(q)}
                    onDelete={() => setDeleteTarget(q)}
                  />
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>

      <QuotaDialog
        open={createOpen}
        onOpenChange={setCreateOpen}
        existing={null}
      />
      {editTarget && (
        <QuotaDialog
          open={true}
          onOpenChange={(open) => !open && setEditTarget(null)}
          existing={editTarget}
        />
      )}
      {deleteTarget && (
        <DeleteQuotaDialog
          quota={deleteTarget}
          open={true}
          onOpenChange={(open) => !open && setDeleteTarget(null)}
        />
      )}
    </div>
  );
}

function QuotaRow({
  quota,
  onEdit,
  onDelete,
}: {
  quota: QuotaView;
  onEdit: () => void;
  onDelete: () => void;
}) {
  const label =
    quota.scope === "user"
      ? (quota.meta?.email ?? quota.scope_id.slice(0, 8))
      : (quota.meta?.name ?? quota.scope_id.slice(0, 8));

  const memUsedMB = Math.round(quota.usage.memory_bytes / (1024 * 1024));

  return (
    <TableRow>
      <TableCell>
        <Badge variant={quota.scope === "user" ? "default" : "secondary"}>
          {quota.scope}
        </Badge>
      </TableCell>
      <TableCell className="font-medium">{label}</TableCell>
      <TableCell>
        <UsageCell
          current={quota.usage.applications}
          limit={quota.limits.max_applications}
        />
      </TableCell>
      <TableCell>
        <UsageCell
          current={quota.usage.cpu}
          limit={quota.limits.max_cpu}
          format={(v) => `${v.toFixed(2)} cores`}
        />
      </TableCell>
      <TableCell>
        <UsageCell
          current={memUsedMB}
          limit={quota.limits.max_memory_mb}
          format={(v) => formatBytes(v * 1024 * 1024)}
        />
      </TableCell>
      <TableCell className="text-right">
        <div className="flex justify-end gap-2">
          <Button variant="outline" size="sm" onClick={onEdit}>
            Edit
          </Button>
          <Button variant="destructive" size="sm" onClick={onDelete}>
            Remove
          </Button>
        </div>
      </TableCell>
    </TableRow>
  );
}

function UsageCell({
  current,
  limit,
  format,
}: {
  current: number;
  limit: number | null;
  format?: (v: number) => string;
}) {
  const fmt = format ?? ((v: number) => `${v}`);
  if (limit == null) {
    return (
      <div className="text-sm">
        <div>{fmt(current)}</div>
        <div className="text-muted-foreground text-xs">unlimited</div>
      </div>
    );
  }
  const pct =
    limit === 0 ? 100 : Math.min(100, Math.round((current / limit) * 100));
  const color =
    pct >= 100 ? "bg-destructive" : pct >= 80 ? "bg-amber-500" : "bg-primary";
  return (
    <div className="min-w-[120px] space-y-1 text-sm">
      <div>
        {fmt(current)}{" "}
        <span className="text-muted-foreground">/ {fmt(limit)}</span>
      </div>
      <div className="bg-muted h-1.5 w-full overflow-hidden rounded">
        <div className={`h-full ${color}`} style={{ width: `${pct}%` }} />
      </div>
    </div>
  );
}

// Quota fields are stored as strings ("" = unlimited) and parsed at submit
// time. Validators run on the string input directly so they fit TanStack
// Form's StandardSchemaV1 contract; numeric coercion happens after .refine.
const optionalInt = z.string().refine((v) => v === "" || /^\d+$/.test(v), {
  message: "must be a non-negative integer or blank",
});
const optionalNumber = z
  .string()
  .refine((v) => v === "" || (!isNaN(Number(v)) && Number(v) >= 0), {
    message: "must be a non-negative number or blank",
  });

function parseLimit(value: string | number): number | null {
  if (value === "" || value === null || value === undefined) return null;
  const n = typeof value === "number" ? value : Number(value);
  return Number.isFinite(n) ? n : null;
}

function QuotaDialog({
  open,
  onOpenChange,
  existing,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  existing: QuotaView | null;
}) {
  const upsert = useUpsertQuota();
  const { data: users } = useUsers();
  const { data: projects } = useProjects();

  const form = useForm({
    defaultValues: {
      scope: (existing?.scope ?? "user") as QuotaScope,
      scopeId: existing?.scope_id ?? "",
      maxApps:
        existing?.limits.max_applications != null
          ? String(existing.limits.max_applications)
          : "",
      maxCpu:
        existing?.limits.max_cpu != null ? String(existing.limits.max_cpu) : "",
      maxMemMb:
        existing?.limits.max_memory_mb != null
          ? String(existing.limits.max_memory_mb)
          : "",
    },
    onSubmit: async ({ value }) => {
      const limits: QuotaLimits = {
        max_applications: parseLimit(value.maxApps),
        max_cpu: parseLimit(value.maxCpu),
        max_memory_mb: parseLimit(value.maxMemMb),
      };
      toast.promise(
        upsert
          .mutateAsync({ scope: value.scope, scopeId: value.scopeId, limits })
          .then(() => onOpenChange(false)),
        {
          loading: "Saving quota...",
          success: "Quota saved",
          error: (err) => err.message,
        },
      );
    },
  });

  const scope = useStore(form.store, (s) => s.values.scope);
  const targetOptions = useMemo(() => {
    if (scope === "user") {
      return (users ?? []).map((u) => ({ id: u.id, label: u.email }));
    }
    return (projects ?? []).map((p) => ({
      id: p.id,
      label: `${p.name} (${p.slug})`,
    }));
  }, [scope, users, projects]);

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{existing ? "Edit Quota" : "Set Quota"}</DialogTitle>
          <DialogDescription>
            Leave a field blank to remove that cap (unlimited).
          </DialogDescription>
        </DialogHeader>
        <form
          onSubmit={(e) => {
            e.preventDefault();
            e.stopPropagation();
            form.handleSubmit();
          }}
          className="space-y-4"
        >
          {!existing && (
            <>
              <form.Field
                name="scope"
                children={(field) => (
                  <div className="space-y-2">
                    <Label htmlFor="scope">Scope</Label>
                    <Select
                      value={field.state.value}
                      onValueChange={(v) => {
                        field.handleChange(v as QuotaScope);
                        form.setFieldValue("scopeId", "");
                      }}
                    >
                      <SelectTrigger id="scope">
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="user">User</SelectItem>
                        <SelectItem value="project">Project</SelectItem>
                      </SelectContent>
                    </Select>
                  </div>
                )}
              />
              <form.Field
                name="scopeId"
                validators={{ onChange: z.string().min(1, "Pick a target") }}
                children={(field) => (
                  <div className="space-y-2">
                    <Label htmlFor="target">Target</Label>
                    <Select
                      value={field.state.value}
                      onValueChange={(v) => field.handleChange(v ?? "")}
                    >
                      <SelectTrigger id="target">
                        <SelectValue placeholder={`Pick a ${scope}`} />
                      </SelectTrigger>
                      <SelectContent>
                        {targetOptions.map((o) => (
                          <SelectItem key={o.id} value={o.id}>
                            {o.label}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                    {field.state.meta.errors.length > 0 && (
                      <p className="text-destructive text-sm">
                        {typeof field.state.meta.errors[0] === "string"
                          ? field.state.meta.errors[0]
                          : field.state.meta.errors[0]?.message}
                      </p>
                    )}
                  </div>
                )}
              />
            </>
          )}
          <form.Field
            name="maxApps"
            validators={{ onChange: optionalInt }}
            children={(field) => (
              <div className="space-y-2">
                <Label htmlFor="max-apps">Max Applications</Label>
                <Input
                  id="max-apps"
                  type="number"
                  min="0"
                  value={field.state.value}
                  onChange={(e) => field.handleChange(e.target.value)}
                  placeholder="unlimited"
                />
                {field.state.meta.errors.length > 0 && (
                  <p className="text-destructive text-sm">
                    {typeof field.state.meta.errors[0] === "string"
                      ? field.state.meta.errors[0]
                      : field.state.meta.errors[0]?.message}
                  </p>
                )}
              </div>
            )}
          />
          <form.Field
            name="maxCpu"
            validators={{ onChange: optionalNumber }}
            children={(field) => (
              <div className="space-y-2">
                <Label htmlFor="max-cpu">Max CPU (cores)</Label>
                <Input
                  id="max-cpu"
                  type="number"
                  min="0"
                  step="0.1"
                  value={field.state.value}
                  onChange={(e) => field.handleChange(e.target.value)}
                  placeholder="unlimited"
                />
                {field.state.meta.errors.length > 0 && (
                  <p className="text-destructive text-sm">
                    {typeof field.state.meta.errors[0] === "string"
                      ? field.state.meta.errors[0]
                      : field.state.meta.errors[0]?.message}
                  </p>
                )}
              </div>
            )}
          />
          <form.Field
            name="maxMemMb"
            validators={{ onChange: optionalInt }}
            children={(field) => (
              <div className="space-y-2">
                <Label htmlFor="max-mem">Max Memory (MB)</Label>
                <Input
                  id="max-mem"
                  type="number"
                  min="0"
                  value={field.state.value}
                  onChange={(e) => field.handleChange(e.target.value)}
                  placeholder="unlimited"
                />
                {field.state.meta.errors.length > 0 && (
                  <p className="text-destructive text-sm">
                    {typeof field.state.meta.errors[0] === "string"
                      ? field.state.meta.errors[0]
                      : field.state.meta.errors[0]?.message}
                  </p>
                )}
              </div>
            )}
          />
          <DialogFooter>
            <form.Subscribe
              selector={(s) => s.isSubmitting}
              children={(isSubmitting) => (
                <Button type="submit" disabled={isSubmitting}>
                  {isSubmitting ? "Saving..." : "Save Quota"}
                </Button>
              )}
            />
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

function DeleteQuotaDialog({
  quota,
  open,
  onOpenChange,
}: {
  quota: QuotaView;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const del = useDeleteQuota();
  const label =
    quota.scope === "user"
      ? (quota.meta?.email ?? quota.scope_id)
      : (quota.meta?.name ?? quota.scope_id);

  const handleDelete = () => {
    toast.promise(
      del
        .mutateAsync({ scope: quota.scope, scopeId: quota.scope_id })
        .then(() => onOpenChange(false)),
      {
        loading: "Removing quota...",
        success: "Quota removed",
        error: (err) => err.message,
      },
    );
  };

  return (
    <AlertDialog open={open} onOpenChange={onOpenChange}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>Remove quota for {label}?</AlertDialogTitle>
          <AlertDialogDescription>
            This will restore unlimited caps for this {quota.scope}. Existing
            applications are unaffected.
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>Cancel</AlertDialogCancel>
          <AlertDialogAction
            onClick={handleDelete}
            className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
          >
            {del.isPending ? "Removing..." : "Remove"}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}
