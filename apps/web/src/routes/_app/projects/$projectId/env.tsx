import { createFileRoute } from "@tanstack/react-router";
import {
  useProjectEnvVars,
  useUpsertProjectEnvVars,
} from "@/lib/hooks/use-project-envvars";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { useState, useEffect } from "react";
import { parseEnvContent } from "@/lib/utils/parse-env";

export const Route = createFileRoute("/_app/projects/$projectId/env")({
  component: ProjectEnvVarsPage,
});

interface EnvRow {
  key: string;
  value: string;
  is_secret: boolean;
}

function ProjectEnvVarsPage() {
  const { projectId } = Route.useParams();
  const { data: envVars, isLoading } = useProjectEnvVars(projectId);
  const upsert = useUpsertProjectEnvVars(projectId);
  const [rows, setRows] = useState<EnvRow[]>([]);
  const [error, setError] = useState("");
  const [importOpen, setImportOpen] = useState(false);
  const [importText, setImportText] = useState("");

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

  const handleSave = async () => {
    setError("");
    const validRows = rows.filter((r) => r.key.trim());
    try {
      await upsert.mutateAsync(validRows);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to save");
    }
  };

  if (isLoading) {
    return (
      <div className="text-muted-foreground">
        Loading environment variables...
      </div>
    );
  }

  return (
    <div className="space-y-4">
      <Card>
        <CardHeader>
          <div className="flex items-center justify-between">
            <div>
              <CardTitle>Project Environment Variables</CardTitle>
              <p className="text-muted-foreground mt-1 text-sm">
                These variables are inherited by all applications in this
                project. Application-level variables with the same key will
                override these.
              </p>
            </div>
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
          {error && (
            <div className="bg-destructive/10 text-destructive mb-4 rounded-md px-3 py-2 text-sm">
              {error}
            </div>
          )}
          <div className="space-y-2">
            {rows.length === 0 ? (
              <p className="text-muted-foreground py-8 text-center text-sm">
                No project environment variables. Click "Add Variable" to add
                one.
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
          {rows.length > 0 && (
            <div className="mt-4">
              <Button onClick={handleSave} disabled={upsert.isPending}>
                {upsert.isPending ? "Saving..." : "Save Variables"}
              </Button>
            </div>
          )}
        </CardContent>
      </Card>

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
