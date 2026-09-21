import { useState, type ComponentType } from "react";
import { useForm } from "@tanstack/react-form";
import {
  Loader2,
  ChevronDown,
  ChevronUp,
  Database as OtherIcon,
} from "lucide-react";
import {
  SiPostgresql,
  SiMysql,
  SiRedis,
  SiMongodb,
} from "@icons-pack/react-simple-icons";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Field, FieldError, FieldLabel } from "@/components/ui/field";
import { fieldError } from "@/lib/utils/field-error";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { useCreateDatabase } from "@/lib/hooks/use-databases";

function slugify(name: string): string {
  return name
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-|-$/g, "");
}

const DB_TYPES = ["postgres", "mysql", "redis", "mongo", "other"] as const;
const DEFAULT_VERSIONS: Record<string, string> = {
  postgres: "16",
  mysql: "8",
  redis: "7",
  mongo: "7",
};
// mysql has no entry: its default user is derived from the slug (shown as the
// "Same as Slug" placeholder below), not a fixed name — see database-form-dialog.
const DEFAULT_USERS: Record<string, string> = {
  postgres: "postgres",
  redis: "default",
  mongo: "admin",
};

const DB_TYPE_ICON: Record<string, ComponentType<{ className?: string }>> = {
  postgres: SiPostgresql,
  mysql: SiMysql,
  redis: SiRedis,
  mongo: SiMongodb,
  other: OtherIcon,
};

interface Props {
  projectId: string;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

export function DatabaseFormDialog({ projectId, open, onOpenChange }: Props) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[85vh] grid-rows-[auto_minmax(0,1fr)_auto]">
        {/* Remount on every open so fields always start blank without an effect. */}
        {open && (
          <DatabaseForm
            key="new"
            projectId={projectId}
            onDone={() => onOpenChange(false)}
          />
        )}
      </DialogContent>
    </Dialog>
  );
}

function DatabaseForm({
  projectId,
  onDone,
}: {
  projectId: string;
  onDone: () => void;
}) {
  const createDb = useCreateDatabase(projectId);

  const [dbSlugManual, setDbSlugManual] = useState(false);
  const [showCredentials, setShowCredentials] = useState(false);
  const [dbError, setDbError] = useState("");

  const form = useForm({
    defaultValues: {
      name: "",
      slug: "",
      type: "postgres",
      version: "",
      user: "",
      password: "",
      databaseName: "",
      rootPassword: "",
      otherImage: "",
      otherPort: "",
      otherDataDir: "/data",
      otherEnv: "",
      otherBackupMode: "volume_snapshot" as "volume_snapshot" | "command",
      otherBackupCmd: "",
      otherRestoreCmd: "",
    },
    onSubmit: ({ value }) => {
      setDbError("");

      let payload: Parameters<typeof createDb.mutate>[0];
      if (value.type === "other") {
        const port = Number(value.otherPort);
        // Parse "KEY=VALUE" lines into an env map.
        const env: Record<string, string> = {};
        for (const line of value.otherEnv.split("\n")) {
          const trimmed = line.trim();
          if (!trimmed) continue;
          const eq = trimmed.indexOf("=");
          if (eq === -1) continue;
          env[trimmed.slice(0, eq).trim()] = trimmed.slice(eq + 1).trim();
        }
        payload = {
          name: value.name.trim(),
          slug: value.slug || undefined,
          type: "other",
          image: value.otherImage.trim(),
          container_port: port,
          data_dir: value.otherDataDir.trim() || "/data",
          env: Object.keys(env).length ? env : undefined,
          backup_mode: value.otherBackupMode,
          backup_command:
            value.otherBackupMode === "command"
              ? value.otherBackupCmd.trim()
              : undefined,
          restore_command:
            value.otherBackupMode === "command"
              ? value.otherRestoreCmd.trim()
              : undefined,
        };
      } else {
        const credentials =
          showCredentials &&
          (value.user ||
            value.password ||
            value.databaseName ||
            value.rootPassword)
            ? {
                user: value.user || undefined,
                password: value.password || undefined,
                database_name: value.databaseName || undefined,
                root_password: value.rootPassword || undefined,
              }
            : undefined;
        payload = {
          name: value.name.trim(),
          slug: value.slug || undefined,
          type: value.type,
          version: value.version || undefined,
          credentials,
        };
      }

      createDb.mutate(payload, {
        onSuccess: () => {
          onDone();
        },
        onError: (e) => {
          setDbError(
            e instanceof Error ? e.message : "Failed to create database",
          );
        },
      });
    },
  });

  return (
    <>
      <DialogHeader>
        <DialogTitle>Create Database</DialogTitle>
        <DialogDescription>
          Provision a new managed database instance.
        </DialogDescription>
      </DialogHeader>
      <div className="space-y-4 overflow-y-auto py-2">
        {dbError && (
          <div className="bg-destructive/10 text-destructive rounded-md px-3 py-2 text-sm">
            {dbError}
          </div>
        )}
        <form.Field
          name="name"
          validators={{
            onChange: ({ value }) =>
              value.trim() === "" ? "Name is required" : undefined,
          }}
          children={(field) => {
            const error = fieldError(field.state.meta.errors);
            return (
              <Field data-invalid={!!error}>
                <FieldLabel htmlFor="db-name">Name</FieldLabel>
                <Input
                  id="db-name"
                  value={field.state.value}
                  onBlur={field.handleBlur}
                  onChange={(e) => {
                    field.handleChange(e.target.value);
                    if (!dbSlugManual) {
                      form.setFieldValue("slug", slugify(e.target.value));
                    }
                  }}
                  placeholder="my-database"
                  aria-invalid={!!error}
                />
                {error && <FieldError>{error}</FieldError>}
              </Field>
            );
          }}
        />
        <form.Field
          name="slug"
          children={(field) => (
            <div className="space-y-2">
              <Label htmlFor="db-slug">Slug</Label>
              <form.Subscribe
                selector={(s) => s.values.name}
                children={(name) => (
                  <Input
                    id="db-slug"
                    value={field.state.value}
                    onChange={(e) => {
                      field.handleChange(slugify(e.target.value));
                      setDbSlugManual(true);
                    }}
                    placeholder={name ? slugify(name) : "auto-generated"}
                  />
                )}
              />
              <p className="text-muted-foreground text-xs">
                Used in container naming. Auto-generated from name unless
                overridden.
              </p>
            </div>
          )}
        />
        <form.Field
          name="type"
          children={(field) => (
            <div className="space-y-2">
              <Label>Type</Label>
              <Select
                value={field.state.value}
                onValueChange={(v) => {
                  field.handleChange((v as string) ?? "postgres");
                  form.setFieldValue("version", "");
                  form.setFieldValue("user", "");
                  form.setFieldValue("password", "");
                  form.setFieldValue("databaseName", "");
                  form.setFieldValue("rootPassword", "");
                }}
              >
                <SelectTrigger>
                  <SelectValue placeholder="Select type" />
                </SelectTrigger>
                <SelectContent>
                  {DB_TYPES.map((t) => {
                    const Icon = DB_TYPE_ICON[t];
                    return (
                      <SelectItem key={t} value={t} icon={<Icon />}>
                        {t.charAt(0).toUpperCase() + t.slice(1)}
                      </SelectItem>
                    );
                  })}
                </SelectContent>
              </Select>
            </div>
          )}
        />
        <form.Subscribe
          selector={(s) => [s.values.type, s.values.name] as const}
          children={([dbType, dbName]) => (
            <>
              {dbType !== "other" && (
                <>
                  <form.Field
                    name="version"
                    children={(field) => (
                      <div className="space-y-2">
                        <Label htmlFor="db-version">Image Tag</Label>
                        <Input
                          id="db-version"
                          value={field.state.value}
                          onChange={(e) => field.handleChange(e.target.value)}
                          placeholder={`e.g. ${DEFAULT_VERSIONS[dbType] || "latest"}, ${DEFAULT_VERSIONS[dbType] || "latest"}-alpine`}
                        />
                      </div>
                    )}
                  />
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
                          <form.Field
                            name="user"
                            children={(field) => (
                              <div className="space-y-1">
                                <Label htmlFor="db-user">User</Label>
                                <Input
                                  id="db-user"
                                  value={field.state.value}
                                  onChange={(e) =>
                                    field.handleChange(e.target.value)
                                  }
                                  placeholder={
                                    dbType === "mysql"
                                      ? "Same as Slug"
                                      : DEFAULT_USERS[dbType] || ""
                                  }
                                />
                              </div>
                            )}
                          />
                        )}
                        <form.Field
                          name="password"
                          children={(field) => (
                            <div className="space-y-1">
                              <Label htmlFor="db-password">Password</Label>
                              <Input
                                id="db-password"
                                type="password"
                                value={field.state.value}
                                onChange={(e) =>
                                  field.handleChange(e.target.value)
                                }
                                placeholder="auto-generated"
                              />
                            </div>
                          )}
                        />
                        {dbType === "mysql" && (
                          <form.Field
                            name="rootPassword"
                            children={(field) => (
                              <div className="space-y-1">
                                <Label htmlFor="db-root-password">
                                  Root Password
                                </Label>
                                <Input
                                  id="db-root-password"
                                  type="password"
                                  value={field.state.value}
                                  onChange={(e) =>
                                    field.handleChange(e.target.value)
                                  }
                                  placeholder="auto-generated"
                                />
                              </div>
                            )}
                          />
                        )}
                        {(dbType === "postgres" || dbType === "mysql") && (
                          <form.Field
                            name="databaseName"
                            children={(field) => (
                              <div className="space-y-1">
                                <Label htmlFor="db-database-name">
                                  Database Name
                                </Label>
                                <Input
                                  id="db-database-name"
                                  value={field.state.value}
                                  onChange={(e) =>
                                    field.handleChange(e.target.value)
                                  }
                                  placeholder={
                                    dbType === "mysql"
                                      ? "Same as Slug"
                                      : dbName || "same as name"
                                  }
                                />
                              </div>
                            )}
                          />
                        )}
                        <p className="text-muted-foreground text-xs">
                          Leave empty to use defaults.
                        </p>
                      </div>
                    )}
                  </div>
                </>
              )}

              {dbType === "other" && (
                <div className="space-y-4">
                  <form.Field
                    name="otherImage"
                    validators={{
                      onChangeListenTo: ["type"],
                      onChange: ({ value, fieldApi }) =>
                        fieldApi.form.state.values.type === "other" &&
                        value.trim() === ""
                          ? "Image is required"
                          : undefined,
                    }}
                    children={(field) => {
                      const error = fieldError(field.state.meta.errors);
                      return (
                        <Field data-invalid={!!error}>
                          <FieldLabel htmlFor="other-image">Image</FieldLabel>
                          <Input
                            id="other-image"
                            value={field.state.value}
                            onBlur={field.handleBlur}
                            onChange={(e) => field.handleChange(e.target.value)}
                            placeholder="e.g. clickhouse/clickhouse-server:24.3"
                            aria-invalid={!!error}
                          />
                          {error ? (
                            <FieldError>{error}</FieldError>
                          ) : (
                            <p className="text-muted-foreground text-xs">
                              Full image reference. Pin a version tag.
                            </p>
                          )}
                        </Field>
                      );
                    }}
                  />
                  <form.Field
                    name="otherPort"
                    validators={{
                      onChangeListenTo: ["type"],
                      onChange: ({ value, fieldApi }) => {
                        if (fieldApi.form.state.values.type !== "other")
                          return undefined;
                        const port = Number(value);
                        return !port || port <= 0
                          ? "A valid container port is required"
                          : undefined;
                      },
                    }}
                    children={(field) => {
                      const error = fieldError(field.state.meta.errors);
                      return (
                        <Field data-invalid={!!error}>
                          <FieldLabel htmlFor="other-port">
                            Container Port
                          </FieldLabel>
                          <Input
                            id="other-port"
                            type="number"
                            min={1}
                            value={field.state.value}
                            onBlur={field.handleBlur}
                            onChange={(e) => field.handleChange(e.target.value)}
                            placeholder="e.g. 9000"
                            aria-invalid={!!error}
                          />
                          {error ? (
                            <FieldError>{error}</FieldError>
                          ) : (
                            <p className="text-muted-foreground text-xs">
                              The port the database listens on inside the
                              container.
                            </p>
                          )}
                        </Field>
                      );
                    }}
                  />
                  <form.Field
                    name="otherDataDir"
                    children={(field) => (
                      <div className="space-y-2">
                        <Label htmlFor="other-datadir">Data Directory</Label>
                        <Input
                          id="other-datadir"
                          value={field.state.value}
                          onChange={(e) => field.handleChange(e.target.value)}
                          placeholder="/data"
                        />
                        <p className="text-muted-foreground text-xs">
                          Where this image stores data (e.g.
                          /var/lib/clickhouse). The volume mounts here. Cannot
                          be changed after creation.
                        </p>
                      </div>
                    )}
                  />
                  <form.Field
                    name="otherEnv"
                    children={(field) => (
                      <div className="space-y-2">
                        <Label htmlFor="other-env">Environment Variables</Label>
                        <Textarea
                          id="other-env"
                          value={field.state.value}
                          onChange={(e) => field.handleChange(e.target.value)}
                          rows={3}
                          placeholder={"KEY=VALUE\nONE_PER_LINE=true"}
                          className="font-mono text-xs"
                        />
                        <p className="text-muted-foreground text-xs">
                          One KEY=VALUE per line. Passed to the container
                          (credentials and config the image needs).
                        </p>
                      </div>
                    )}
                  />
                  <form.Field
                    name="otherBackupMode"
                    children={(field) => (
                      <div className="space-y-2">
                        <Label htmlFor="other-backup-mode">Backup Mode</Label>
                        <select
                          id="other-backup-mode"
                          value={field.state.value}
                          onChange={(e) =>
                            field.handleChange(
                              e.target.value as "volume_snapshot" | "command",
                            )
                          }
                          className="border-input bg-background ring-offset-background focus-visible:ring-ring flex h-9 w-full rounded-md border px-3 py-1 text-sm focus-visible:ring-2 focus-visible:ring-offset-2 focus-visible:outline-none"
                        >
                          <option value="volume_snapshot">
                            Volume snapshot (cold tar; brief downtime)
                          </option>
                          <option value="command">
                            Custom commands (online; you provide dump/restore)
                          </option>
                        </select>
                      </div>
                    )}
                  />
                  <form.Subscribe
                    selector={(s) => s.values.otherBackupMode}
                    children={(otherBackupMode) =>
                      otherBackupMode === "command" && (
                        <div className="space-y-3 rounded-md border p-3">
                          <form.Field
                            name="otherBackupCmd"
                            validators={{
                              onChangeListenTo: ["type", "otherBackupMode"],
                              onChange: ({ value, fieldApi }) =>
                                fieldApi.form.state.values.type === "other" &&
                                fieldApi.form.state.values.otherBackupMode ===
                                  "command" &&
                                value.trim() === ""
                                  ? "Backup command is required"
                                  : undefined,
                            }}
                            children={(field) => {
                              const error = fieldError(field.state.meta.errors);
                              return (
                                <Field data-invalid={!!error} className="gap-1">
                                  <FieldLabel htmlFor="other-backup-cmd">
                                    Backup Command
                                  </FieldLabel>
                                  <Textarea
                                    id="other-backup-cmd"
                                    value={field.state.value}
                                    onBlur={field.handleBlur}
                                    onChange={(e) =>
                                      field.handleChange(e.target.value)
                                    }
                                    rows={2}
                                    placeholder="dump into $BELUNE_BACKUP_DIR"
                                    className="font-mono text-xs"
                                    aria-invalid={!!error}
                                  />
                                  {error && <FieldError>{error}</FieldError>}
                                </Field>
                              );
                            }}
                          />
                          <form.Field
                            name="otherRestoreCmd"
                            validators={{
                              onChangeListenTo: ["type", "otherBackupMode"],
                              onChange: ({ value, fieldApi }) =>
                                fieldApi.form.state.values.type === "other" &&
                                fieldApi.form.state.values.otherBackupMode ===
                                  "command" &&
                                value.trim() === ""
                                  ? "Restore command is required"
                                  : undefined,
                            }}
                            children={(field) => {
                              const error = fieldError(field.state.meta.errors);
                              return (
                                <Field data-invalid={!!error} className="gap-1">
                                  <FieldLabel htmlFor="other-restore-cmd">
                                    Restore Command
                                  </FieldLabel>
                                  <Textarea
                                    id="other-restore-cmd"
                                    value={field.state.value}
                                    onBlur={field.handleBlur}
                                    onChange={(e) =>
                                      field.handleChange(e.target.value)
                                    }
                                    rows={2}
                                    placeholder="restore from $BELUNE_BACKUP_DIR"
                                    className="font-mono text-xs"
                                    aria-invalid={!!error}
                                  />
                                  {error && <FieldError>{error}</FieldError>}
                                </Field>
                              );
                            }}
                          />
                          <p className="text-muted-foreground text-xs">
                            Run inside the container. Backup writes into
                            <code className="mx-1">$BELUNE_BACKUP_DIR</code>;
                            restore reads from it. The image must include
                            <code className="mx-1">sh</code> and
                            <code className="mx-1">tar</code> (used to archive
                            that directory).
                          </p>
                        </div>
                      )
                    }
                  />
                </div>
              )}
            </>
          )}
        />
      </div>
      <DialogFooter>
        <Button
          onClick={() => form.handleSubmit()}
          disabled={createDb.isPending}
        >
          {createDb.isPending && (
            <Loader2 className="mr-1 h-4 w-4 animate-spin" />
          )}
          Create
        </Button>
      </DialogFooter>
    </>
  );
}
