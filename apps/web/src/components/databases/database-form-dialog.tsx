import { useEffect, useState } from "react";
import { Loader2, ChevronDown, ChevronUp } from "lucide-react";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { useCreateDatabase } from "@/lib/hooks/use-databases";

function slugify(name: string): string {
  return name
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-|-$/g, "");
}

const DB_TYPES = ["postgres", "mysql", "redis", "mongo"] as const;
const DEFAULT_VERSIONS: Record<string, string> = {
  postgres: "16",
  mysql: "8",
  redis: "7",
  mongo: "7",
};
const DEFAULT_USERS: Record<string, string> = {
  postgres: "postgres",
  mysql: "root",
  redis: "default",
  mongo: "admin",
};

interface Props {
  projectId: string;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

export function DatabaseFormDialog({ projectId, open, onOpenChange }: Props) {
  const createDb = useCreateDatabase(projectId);

  const [dbName, setDbName] = useState("");
  const [dbSlug, setDbSlug] = useState("");
  const [dbSlugManual, setDbSlugManual] = useState(false);
  const [dbType, setDbType] = useState<string>("postgres");
  const [dbVersion, setDbVersion] = useState("");
  const [showCredentials, setShowCredentials] = useState(false);
  const [dbUser, setDbUser] = useState("");
  const [dbPassword, setDbPassword] = useState("");
  const [dbDatabaseName, setDbDatabaseName] = useState("");
  const [dbRootPassword, setDbRootPassword] = useState("");
  const [dbError, setDbError] = useState("");

  useEffect(() => {
    if (!open) {
      setDbName("");
      setDbSlug("");
      setDbSlugManual(false);
      setDbType("postgres");
      setDbVersion("");
      setShowCredentials(false);
      setDbUser("");
      setDbPassword("");
      setDbDatabaseName("");
      setDbRootPassword("");
      setDbError("");
    }
  }, [open]);

  const handleCreateDb = () => {
    if (!dbName.trim()) return;
    setDbError("");
    const credentials =
      showCredentials &&
      (dbUser || dbPassword || dbDatabaseName || dbRootPassword)
        ? {
            user: dbUser || undefined,
            password: dbPassword || undefined,
            database_name: dbDatabaseName || undefined,
            root_password: dbRootPassword || undefined,
          }
        : undefined;
    createDb.mutate(
      {
        name: dbName.trim(),
        slug: dbSlug || undefined,
        type: dbType,
        version: dbVersion || undefined,
        credentials,
      },
      {
        onSuccess: () => {
          onOpenChange(false);
        },
        onError: (e) => {
          setDbError(
            e instanceof Error ? e.message : "Failed to create database",
          );
        },
      },
    );
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Create Database</DialogTitle>
          <DialogDescription>
            Provision a new managed database instance.
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-4 py-2">
          {dbError && (
            <div className="bg-destructive/10 text-destructive rounded-md px-3 py-2 text-sm">
              {dbError}
            </div>
          )}
          <div className="space-y-2">
            <Label htmlFor="db-name">Name</Label>
            <Input
              id="db-name"
              value={dbName}
              onChange={(e) => {
                setDbName(e.target.value);
                if (!dbSlugManual) {
                  setDbSlug(slugify(e.target.value));
                }
              }}
              placeholder="my-database"
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="db-slug">Slug</Label>
            <Input
              id="db-slug"
              value={dbSlug}
              onChange={(e) => {
                setDbSlug(slugify(e.target.value));
                setDbSlugManual(true);
              }}
              placeholder={dbName ? slugify(dbName) : "auto-generated"}
            />
            <p className="text-muted-foreground text-xs">
              Used in container naming. Auto-generated from name unless
              overridden.
            </p>
          </div>
          <div className="space-y-2">
            <Label htmlFor="db-type">Type</Label>
            <select
              id="db-type"
              value={dbType}
              onChange={(e) => {
                setDbType(e.target.value);
                setDbVersion("");
                setDbUser("");
                setDbPassword("");
                setDbDatabaseName("");
                setDbRootPassword("");
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
            <Label htmlFor="db-version">Image Tag</Label>
            <Input
              id="db-version"
              value={dbVersion}
              onChange={(e) => setDbVersion(e.target.value)}
              placeholder={`e.g. ${DEFAULT_VERSIONS[dbType] || "latest"}, ${DEFAULT_VERSIONS[dbType] || "latest"}-alpine`}
            />
          </div>
          <div className="space-y-2">
            <Button
              type="button"
              variant="ghost"
              size="sm"
              className="w-full justify-between"
              onClick={() => setShowCredentials(!showCredentials)}
            >
              Credential Overrides
              {showCredentials ? (
                <ChevronUp className="ml-1 h-4 w-4" />
              ) : (
                <ChevronDown className="ml-1 h-4 w-4" />
              )}
            </Button>
            {showCredentials && (
              <div className="space-y-3 rounded-md border p-3">
                {dbType !== "redis" && (
                  <div className="space-y-1">
                    <Label htmlFor="db-user">User</Label>
                    <Input
                      id="db-user"
                      value={dbUser}
                      onChange={(e) => setDbUser(e.target.value)}
                      placeholder={DEFAULT_USERS[dbType] || ""}
                    />
                  </div>
                )}
                <div className="space-y-1">
                  <Label htmlFor="db-password">Password</Label>
                  <Input
                    id="db-password"
                    type="password"
                    value={dbPassword}
                    onChange={(e) => setDbPassword(e.target.value)}
                    placeholder="auto-generated"
                  />
                </div>
                {dbType === "mysql" && (
                  <div className="space-y-1">
                    <Label htmlFor="db-root-password">Root Password</Label>
                    <Input
                      id="db-root-password"
                      type="password"
                      value={dbRootPassword}
                      onChange={(e) => setDbRootPassword(e.target.value)}
                      placeholder="auto-generated"
                    />
                  </div>
                )}
                {(dbType === "postgres" || dbType === "mysql") && (
                  <div className="space-y-1">
                    <Label htmlFor="db-database-name">Database Name</Label>
                    <Input
                      id="db-database-name"
                      value={dbDatabaseName}
                      onChange={(e) => setDbDatabaseName(e.target.value)}
                      placeholder={dbName || "same as name"}
                    />
                  </div>
                )}
                <p className="text-muted-foreground text-xs">
                  Leave empty to use defaults.
                </p>
              </div>
            )}
          </div>
        </div>
        <DialogFooter>
          <Button
            onClick={handleCreateDb}
            disabled={!dbName.trim() || createDb.isPending}
          >
            {createDb.isPending && (
              <Loader2 className="mr-1 h-4 w-4 animate-spin" />
            )}
            Create
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
