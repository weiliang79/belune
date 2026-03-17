import { useState } from "react";
import { createFileRoute, Link } from "@tanstack/react-router";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { useDatabases, useCreateDatabase } from "@/lib/hooks/use-databases";
import { Plus, Database as DatabaseIcon, Loader2 } from "lucide-react";

export const Route = createFileRoute("/_app/projects/$projectId/databases/")({
  component: DatabasesPage,
});

const DB_TYPES = ["postgres", "mysql", "redis", "mongo"] as const;
const DEFAULT_VERSIONS: Record<string, string> = {
  postgres: "16",
  mysql: "8",
  redis: "7",
  mongo: "7",
};

function DatabasesPage() {
  const { projectId } = Route.useParams();
  const { data: databases, isLoading } = useDatabases(projectId);
  const createDb = useCreateDatabase(projectId);
  const [open, setOpen] = useState(false);
  const [name, setName] = useState("");
  const [type, setType] = useState<string>("postgres");
  const [version, setVersion] = useState("");

  const handleCreate = () => {
    if (!name.trim()) return;
    createDb.mutate(
      { name: name.trim(), type, version: version || undefined },
      {
        onSuccess: () => {
          setOpen(false);
          setName("");
          setType("postgres");
          setVersion("");
        },
      },
    );
  };

  if (isLoading) {
    return <div className="text-muted-foreground">Loading databases...</div>;
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h2 className="text-lg font-semibold">Databases</h2>
        <Dialog open={open} onOpenChange={setOpen}>
          <DialogTrigger
            render={
              <Button>
                <Plus className="mr-1 h-4 w-4" />
                Create Database
              </Button>
            }
          />
          <DialogContent>
            <DialogHeader>
              <DialogTitle>Create Database</DialogTitle>
              <DialogDescription>
                Provision a new managed database instance.
              </DialogDescription>
            </DialogHeader>
            <div className="space-y-4 py-2">
              <div className="space-y-2">
                <Label htmlFor="db-name">Name</Label>
                <Input
                  id="db-name"
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  placeholder="my-database"
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="db-type">Type</Label>
                <select
                  id="db-type"
                  value={type}
                  onChange={(e) => {
                    setType(e.target.value);
                    setVersion("");
                  }}
                  className="border-input bg-background ring-offset-background focus-visible:ring-ring flex h-9 w-full rounded-md border px-3 py-1 text-sm focus-visible:ring-2 focus-visible:ring-offset-2 focus-visible:outline-none"
                >
                  {DB_TYPES.map((t) => (
                    <option key={t} value={t}>
                      {t.charAt(0).toUpperCase() + t.slice(1)}
                    </option>
                  ))}
                </select>
              </div>
              <div className="space-y-2">
                <Label htmlFor="db-version">Version</Label>
                <Input
                  id="db-version"
                  value={version}
                  onChange={(e) => setVersion(e.target.value)}
                  placeholder={DEFAULT_VERSIONS[type] || "latest"}
                />
              </div>
            </div>
            <DialogFooter>
              <Button
                onClick={handleCreate}
                disabled={!name.trim() || createDb.isPending}
              >
                {createDb.isPending && (
                  <Loader2 className="mr-1 h-4 w-4 animate-spin" />
                )}
                Create
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      </div>

      {!databases || databases.length === 0 ? (
        <Card>
          <CardContent className="flex flex-col items-center justify-center py-12">
            <DatabaseIcon className="text-muted-foreground mb-3 h-10 w-10" />
            <p className="text-muted-foreground mb-1">No databases yet.</p>
            <p className="text-muted-foreground text-xs">
              Create a database to get started.
            </p>
          </CardContent>
        </Card>
      ) : (
        <div className="grid gap-3 sm:grid-cols-2">
          {databases.map((db) => (
            <Link
              key={db.id}
              to="/projects/$projectId/databases/$databaseId"
              params={{ projectId, databaseId: db.id }}
            >
              <Card className="hover:bg-muted/50 cursor-pointer transition-colors">
                <CardHeader className="pb-2">
                  <div className="flex items-start justify-between">
                    <CardTitle className="text-base">{db.name}</CardTitle>
                    <Badge
                      variant={
                        db.status === "running"
                          ? "default"
                          : db.status === "failed"
                            ? "destructive"
                            : "secondary"
                      }
                    >
                      {db.status}
                    </Badge>
                  </div>
                </CardHeader>
                <CardContent>
                  <div className="text-muted-foreground flex gap-2 text-xs">
                    <Badge variant="outline">{db.type}</Badge>
                    <Badge variant="outline">v{db.version}</Badge>
                  </div>
                </CardContent>
              </Card>
            </Link>
          ))}
        </div>
      )}
    </div>
  );
}
