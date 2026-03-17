import { createFileRoute } from "@tanstack/react-router";
import { useEnvVars, useUpsertEnvVars } from "@/lib/hooks/use-envvars";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { useState, useEffect } from "react";

export const Route = createFileRoute(
  "/_app/projects/$projectId/services/$serviceId/env",
)({
  component: EnvVarsPage,
});

interface EnvRow {
  key: string;
  value: string;
  is_secret: boolean;
}

function EnvVarsPage() {
  const { projectId, serviceId } = Route.useParams();
  const { data: envVars, isLoading } = useEnvVars(projectId, serviceId);
  const upsert = useUpsertEnvVars(projectId, serviceId);
  const [rows, setRows] = useState<EnvRow[]>([]);
  const [error, setError] = useState("");

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
            <CardTitle>Environment Variables</CardTitle>
            <Button size="sm" variant="outline" onClick={addRow}>
              Add Variable
            </Button>
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
    </div>
  );
}
