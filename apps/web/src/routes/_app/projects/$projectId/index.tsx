import { useState } from "react";
import { createFileRoute, Link, useNavigate } from "@tanstack/react-router";
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
} from "@/components/ui/dialog";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { useServices, useCreateService } from "@/lib/hooks/use-services";
import { useDatabases, useCreateDatabase } from "@/lib/hooks/use-databases";
import {
  Plus,
  Database as DatabaseIcon,
  AppWindow,
  Loader2,
  ChevronDown,
  ChevronUp,
} from "lucide-react";

export const Route = createFileRoute("/_app/projects/$projectId/")({
  component: ProjectOverview,
});

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
  redis: "",
  mongo: "admin",
};

function ProjectOverview() {
  const { projectId } = Route.useParams();
  const navigate = useNavigate();
  const { data: services, isLoading: servicesLoading } =
    useServices(projectId);
  const { data: databases, isLoading: databasesLoading } =
    useDatabases(projectId);
  const createService = useCreateService(projectId);
  const createDb = useCreateDatabase(projectId);

  // App dialog state
  const [appDialogOpen, setAppDialogOpen] = useState(false);
  const [appName, setAppName] = useState("");
  const [appSlug, setAppSlug] = useState("");
  const [appSlugManual, setAppSlugManual] = useState(false);
  const [serviceType, setServiceType] = useState<"image" | "git">("image");
  const [sourceImage, setSourceImage] = useState("");
  const [sourceRepo, setSourceRepo] = useState("");
  const [dockerfilePath, setDockerfilePath] = useState("Dockerfile");
  const [buildType, setBuildType] = useState("dockerfile");
  const [appError, setAppError] = useState("");

  // DB dialog state
  const [dbDialogOpen, setDbDialogOpen] = useState(false);
  const [dbName, setDbName] = useState("");
  const [dbSlug, setDbSlug] = useState("");
  const [dbSlugManual, setDbSlugManual] = useState(false);
  const [dbType, setDbType] = useState<string>("postgres");
  const [dbVersion, setDbVersion] = useState("");
  const [showCredentials, setShowCredentials] = useState(false);
  const [dbUser, setDbUser] = useState("");
  const [dbPassword, setDbPassword] = useState("");
  const [dbDatabaseName, setDbDatabaseName] = useState("");

  const handleCreateApp = async () => {
    setAppError("");
    if (!appName.trim()) {
      setAppError("Application name is required");
      return;
    }
    if (serviceType === "image" && !sourceImage.trim()) {
      setAppError("Image name is required");
      return;
    }
    if (serviceType === "git" && !sourceRepo.trim()) {
      setAppError("Repository URL is required");
      return;
    }
    try {
      const service = await createService.mutateAsync({
        name: appName.trim(),
        slug: appSlug || undefined,
        type: serviceType,
        ...(serviceType === "image"
          ? { source_image: sourceImage, build_type: "image" }
          : {
              source_repo: sourceRepo,
              dockerfile_path: dockerfilePath,
              build_type: buildType,
            }),
      });
      setAppDialogOpen(false);
      setAppName("");
      setAppSlug("");
      setAppSlugManual(false);
      setSourceImage("");
      setSourceRepo("");
      setDockerfilePath("Dockerfile");
      setBuildType("dockerfile");
      setServiceType("image");
      setAppError("");
      navigate({
        to: "/projects/$projectId/services/$serviceId",
        params: { projectId, serviceId: service.id },
      });
    } catch (e) {
      setAppError(
        e instanceof Error ? e.message : "Failed to create application",
      );
    }
  };

  const handleCreateDb = () => {
    if (!dbName.trim()) return;
    const credentials =
      showCredentials && (dbUser || dbPassword || dbDatabaseName)
        ? {
            user: dbUser || undefined,
            password: dbPassword || undefined,
            database_name: dbDatabaseName || undefined,
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
          setDbDialogOpen(false);
          setDbName("");
          setDbSlug("");
          setDbSlugManual(false);
          setDbType("postgres");
          setDbVersion("");
          setShowCredentials(false);
          setDbUser("");
          setDbPassword("");
          setDbDatabaseName("");
        },
      },
    );
  };

  if (servicesLoading || databasesLoading) {
    return <div className="text-muted-foreground">Loading resources...</div>;
  }

  const hasServices = services && services.length > 0;
  const hasDatabases = databases && databases.length > 0;
  const isEmpty = !hasServices && !hasDatabases;

  return (
    <div className="space-y-8">
      <div className="flex items-center justify-between">
        <h2 className="text-lg font-semibold">Resources</h2>
        <DropdownMenu>
          <DropdownMenuTrigger
            render={
              <Button>
                <Plus className="mr-1 h-4 w-4" />
                Add New
              </Button>
            }
          />
          <DropdownMenuContent align="end">
            <DropdownMenuItem onClick={() => setAppDialogOpen(true)}>
              <AppWindow className="mr-2 h-4 w-4" />
              Application
            </DropdownMenuItem>
            <DropdownMenuItem onClick={() => setDbDialogOpen(true)}>
              <DatabaseIcon className="mr-2 h-4 w-4" />
              Database
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </div>

      {/* Create Application Dialog */}
      <Dialog open={appDialogOpen} onOpenChange={setAppDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>New Application</DialogTitle>
            <DialogDescription>
              Add an application to your project.
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-4 py-2">
            {appError && (
              <div className="bg-destructive/10 text-destructive rounded-md px-3 py-2 text-sm">
                {appError}
              </div>
            )}
            <div className="space-y-2">
              <Label htmlFor="app-name">Application Name</Label>
              <Input
                id="app-name"
                value={appName}
                onChange={(e) => {
                  setAppName(e.target.value);
                  if (!appSlugManual) {
                    setAppSlug(slugify(e.target.value));
                  }
                }}
                placeholder="my-api"
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="app-slug">Slug</Label>
              <Input
                id="app-slug"
                value={appSlug}
                onChange={(e) => {
                  setAppSlug(slugify(e.target.value));
                  setAppSlugManual(true);
                }}
                placeholder={appName ? slugify(appName) : "auto-generated"}
              />
              <p className="text-muted-foreground text-xs">
                Used in container naming. Auto-generated from name unless
                overridden.
              </p>
            </div>
            <div className="space-y-2">
              <Label>Source Type</Label>
              <div className="flex gap-2">
                <Button
                  type="button"
                  variant={serviceType === "image" ? "default" : "outline"}
                  size="sm"
                  onClick={() => setServiceType("image")}
                >
                  Docker Image
                </Button>
                <Button
                  type="button"
                  variant={serviceType === "git" ? "default" : "outline"}
                  size="sm"
                  onClick={() => setServiceType("git")}
                >
                  Git Repository
                </Button>
              </div>
            </div>
            {serviceType === "image" ? (
              <div className="space-y-2">
                <Label htmlFor="source-image">Docker Image</Label>
                <Input
                  id="source-image"
                  value={sourceImage}
                  onChange={(e) => setSourceImage(e.target.value)}
                  placeholder="nginx:latest"
                />
              </div>
            ) : (
              <>
                <div className="space-y-2">
                  <Label htmlFor="source-repo">Repository URL</Label>
                  <Input
                    id="source-repo"
                    value={sourceRepo}
                    onChange={(e) => setSourceRepo(e.target.value)}
                    placeholder="https://github.com/user/repo.git"
                  />
                </div>
                <div className="space-y-2">
                  <Label>Build Type</Label>
                  <div className="flex flex-wrap gap-2">
                    {["dockerfile", "buildpacks", "railpack"].map((bt) => (
                      <Button
                        key={bt}
                        type="button"
                        variant={buildType === bt ? "default" : "outline"}
                        size="sm"
                        onClick={() => setBuildType(bt)}
                      >
                        {bt}
                      </Button>
                    ))}
                  </div>
                </div>
                {buildType === "dockerfile" && (
                  <div className="space-y-2">
                    <Label htmlFor="dockerfile-path">Dockerfile Path</Label>
                    <Input
                      id="dockerfile-path"
                      value={dockerfilePath}
                      onChange={(e) => setDockerfilePath(e.target.value)}
                      placeholder="Dockerfile"
                    />
                  </div>
                )}
              </>
            )}
          </div>
          <DialogFooter>
            <Button
              onClick={handleCreateApp}
              disabled={createService.isPending}
            >
              {createService.isPending && (
                <Loader2 className="mr-1 h-4 w-4 animate-spin" />
              )}
              Create Application
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Create Database Dialog */}
      <Dialog open={dbDialogOpen} onOpenChange={setDbDialogOpen}>
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
                value={dbVersion}
                onChange={(e) => setDbVersion(e.target.value)}
                placeholder={DEFAULT_VERSIONS[dbType] || "latest"}
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
                      value={dbPassword}
                      onChange={(e) => setDbPassword(e.target.value)}
                      placeholder="auto-generated"
                    />
                  </div>
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

      {isEmpty ? (
        <Card>
          <CardContent className="flex flex-col items-center justify-center py-12">
            <p className="text-muted-foreground mb-4">
              No applications or databases yet. Click "Add New" above to get
              started.
            </p>
          </CardContent>
        </Card>
      ) : (
        <div className="grid gap-3 sm:grid-cols-2">
          {services?.map((service) => (
            <Link
              key={service.id}
              to="/projects/$projectId/services/$serviceId"
              params={{ projectId, serviceId: service.id }}
            >
              <Card className="hover:bg-muted/50 cursor-pointer transition-colors">
                <CardHeader className="pb-2">
                  <div className="flex items-start justify-between">
                    <CardTitle className="text-base">{service.name}</CardTitle>
                    <Badge
                      variant={
                        service.status === "running" ? "default" : "secondary"
                      }
                    >
                      {service.status}
                    </Badge>
                  </div>
                </CardHeader>
                <CardContent>
                  <div className="text-muted-foreground flex gap-2 text-xs">
                    <Badge variant="outline">{service.type}</Badge>
                    <Badge variant="outline">{service.build_type}</Badge>
                  </div>
                </CardContent>
              </Card>
            </Link>
          ))}
          {databases?.map((db) => (
            <Link
              key={db.id}
              to="/projects/$projectId/databases/$databaseId"
              params={{ projectId, databaseId: db.id }}
            >
              <Card className="hover:bg-muted/50 cursor-pointer transition-colors">
                <CardHeader className="pb-2">
                  <div className="flex items-start justify-between">
                    <div className="flex items-center gap-2">
                      <DatabaseIcon className="text-muted-foreground h-4 w-4" />
                      <CardTitle className="text-base">{db.name}</CardTitle>
                    </div>
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
