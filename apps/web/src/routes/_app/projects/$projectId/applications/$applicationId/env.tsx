import { createFileRoute } from "@tanstack/react-router";
import { useEnvVars, useUpsertEnvVars } from "@/lib/hooks/use-envvars";
import { useProjectEnvVars } from "@/lib/hooks/use-project-envvars";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { Badge } from "@/components/ui/badge";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { toast } from "sonner";
import { useState, useEffect, useMemo } from "react";
import { parseEnvContent } from "@/lib/utils/parse-env";

export const Route = createFileRoute(
  "/_app/projects/$projectId/applications/$applicationId/env",
)({
  component: EnvVarsPage,
});

interface EnvRow {
  key: string;
  value: string;
  is_secret: boolean;
}

function EnvVarsPage() {
  const { projectId, applicationId } = Route.useParams();
  const { data: envVars, isLoading } = useEnvVars(projectId, applicationId);
  const { data: projectEnvVars } = useProjectEnvVars(projectId);
  const upsert = useUpsertEnvVars(projectId, applicationId);
  const [rows, setRows] = useState<EnvRow[]>([]);
  const [importOpen, setImportOpen] = useState(false);
  const [importText, setImportText] = useState("");

  // Keys that this application overrides from the project level
  const appKeys = useMemo(() => new Set(rows.map((r) => r.key)), [rows]);

  // Project-level vars not overridden by app vars
  const inheritedVars = useMemo(
    () => (projectEnvVars ?? []).filter((v) => !appKeys.has(v.key)),
    [projectEnvVars, appKeys],
  );

  // Project-level keys that ARE overridden by app vars
  const overriddenKeys = useMemo(
    () =>
      new Set(
        (projectEnvVars ?? [])
          .filter((v) => appKeys.has(v.key))
          .map((v) => v.key),
      ),
    [projectEnvVars, appKeys],
  );

  useEffect(() => {
    if (envVars) {
      setRows(
        envVars.map((v) => ({
          key: v.key,
          value: v.value,
          is_secret: v.is_secret,
        })),
      );
    }
  }, [envVars]);

  const addRow = () =>
    setRows([...rows, { key: "", value: "", is_secret: false }]);

  const handleImport = () => {
    const parsed = parseEnvContent(importText);
    if (parsed.length === 0) return;
    setRows((prev) => {
      const updated = [...prev];
      for (const { key, value } of parsed) {
        const existing = updated.findIndex((r) => r.key === key);
        if (existing !== -1) {
          updated[existing] = { ...updated[existing], value };
        } else {
          updated.push({ key, value, is_secret: false });
        }
      }
      return updated;
    });
    setImportText("");
    setImportOpen(false);
  };

  const removeRow = (index: number) =>
    setRows(rows.filter((_, i) => i !== index));

  const updateRow = (
    index: number,
    field: keyof EnvRow,
    value: string | boolean,
  ) => {
    const updated = [...rows];
    updated[index] = { ...updated[index], [field]: value };
    setRows(updated);
  };

  const handleSave = () => {
    const validRows = rows.filter((r) => r.key.trim());
    const envKeyRegex = /^[A-Za-z_][A-Za-z0-9_]*$/;
    const invalidRow = validRows.find((r) => !envKeyRegex.test(r.key));
    if (invalidRow) {
      toast.error(`Invalid variable name: ${invalidRow.key}`);
      return;
    }
    toast.promise(upsert.mutateAsync(validRows), {
      loading: "Saving variables...",
      success: "Environment variables saved",
      error: (err) => err.message,
    });
  };

  if (isLoading) {
    return (
      <Card>
        <CardHeader>
          <Skeleton className="h-6 w-48" />
        </CardHeader>
        <CardContent className="space-y-2">
          {[1, 2, 3].map((i) => (
            <div key={i} className="flex items-center gap-2">
              <Skeleton className="h-9 flex-1" />
              <Skeleton className="h-9 flex-1" />
              <Skeleton className="h-5 w-14" />
            </div>
          ))}
        </CardContent>
      </Card>
    );
  }

  return (
    <div className="space-y-4">
      <Card>
        <CardHeader>
          <div className="flex items-center justify-between">
            <CardTitle>Environment Variables</CardTitle>
            <div className="flex gap-2">
              <Button
                size="sm"
                variant="outline"
                onClick={() => setImportOpen(true)}
              >
                Paste .env
              </Button>
              <Button size="sm" variant="outline" onClick={addRow}>
                Add Variable
              </Button>
            </div>
          </div>
        </CardHeader>
        <CardContent>
          <div className="space-y-2">
            {rows.length === 0 ? (
              <p className="text-muted-foreground py-8 text-center text-sm">
                No environment variables. Click "Add Variable" to add one.
              </p>
            ) : (
              rows.map((row, i) => (
                <div key={i} className="flex items-center gap-2">
                  <Input
                    placeholder="KEY"
                    className="font-mono text-xs"
                    value={row.key}
                    onChange={(e) => updateRow(i, "key", e.target.value)}
                  />
                  <Input
                    placeholder="value"
                    className="font-mono text-xs"
                    type={row.is_secret ? "password" : "text"}
                    value={row.value}
                    onChange={(e) => updateRow(i, "value", e.target.value)}
                  />
                  <label className="text-muted-foreground flex items-center gap-1 text-xs whitespace-nowrap">
                    <input
                      type="checkbox"
                      checked={row.is_secret}
                      onChange={(e) =>
                        updateRow(i, "is_secret", e.target.checked)
                      }
                    />
                    Secret
                  </label>
                  {overriddenKeys.has(row.key) && (
                    <Badge variant="secondary" className="text-xs whitespace-nowrap">
                      Overrides project
                    </Badge>
                  )}
                  <Button
                    size="sm"
                    variant="ghost"
                    className="text-destructive"
                    onClick={() => removeRow(i)}
                  >
                    X
                  </Button>
                </div>
              ))
            )}
          </div>
          {/* Also shown when the editor is empty but variables are still
              stored, otherwise removing the last row hid the Save button and
              the deletion could never be committed — the page then read "No
              environment variables" while one was still stored and still being
              injected into the container. */}
          {(rows.length > 0 || (envVars?.length ?? 0) > 0) && (
            <div className="mt-4">
              <Button onClick={handleSave} disabled={upsert.isPending}>
                {upsert.isPending ? "Saving..." : "Save Variables"}
              </Button>
            </div>
          )}
        </CardContent>
      </Card>

      {inheritedVars.length > 0 && (
        <Card>
          <CardHeader>
            <CardTitle className="text-base">
              Inherited from Project
            </CardTitle>
            <p className="text-muted-foreground text-sm">
              These variables are set at the project level. Add a variable with
              the same key above to override.
            </p>
          </CardHeader>
          <CardContent>
            <div className="space-y-2">
              {inheritedVars.map((v) => (
                <div
                  key={v.id}
                  className="flex items-center gap-2 opacity-60"
                >
                  <Input
                    className="font-mono text-xs"
                    value={v.key}
                    disabled
                  />
                  <Input
                    className="font-mono text-xs"
                    type="password"
                    value={v.is_secret ? "••••••••" : v.value}
                    disabled
                  />
                  <Badge
                    variant="outline"
                    className="text-xs whitespace-nowrap"
                  >
                    Inherited
                  </Badge>
                </div>
              ))}
            </div>
          </CardContent>
        </Card>
      )}
      <Dialog open={importOpen} onOpenChange={setImportOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Import .env</DialogTitle>
            <DialogDescription>
              Paste your .env file content below. Existing variables with
              matching keys will be updated.
            </DialogDescription>
          </DialogHeader>
          <Textarea
            className="min-h-50 font-mono text-xs"
            placeholder={"# Paste your .env content\nKEY=value\nANOTHER_KEY=another_value"}
            value={importText}
            onChange={(e) => setImportText(e.target.value)}
          />
          <DialogFooter>
            <Button variant="outline" onClick={() => setImportOpen(false)}>
              Cancel
            </Button>
            <Button onClick={handleImport} disabled={!importText.trim()}>
              Import
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
