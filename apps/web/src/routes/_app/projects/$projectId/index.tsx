import { useState } from "react";
import { createFileRoute, Link, useNavigate } from "@tanstack/react-router";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
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
import {
  useApplications,
  useCreateApplication,
} from "@/lib/hooks/use-applications";
import { useDatabases, useCreateDatabase } from "@/lib/hooks/use-databases";
import { useFeatures } from "@/lib/hooks/use-features";
import { StatusBadge } from "@/lib/components/status-badge";
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import {
  Plus,
  Database as DatabaseIcon,
  AppWindow,
  Loader2,
  ChevronDown,
  ChevronUp,
  AppWindowIcon,
  TriangleAlert,
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
  redis: "default",
  mongo: "admin",
};

function ProjectOverview() {
  const { projectId } = Route.useParams();
  const navigate = useNavigate();
  const { data: applications, isLoading: applicationsLoading } =
    useApplications(projectId);
  const { data: databases, isLoading: databasesLoading } =
    useDatabases(projectId);
  const createApplication = useCreateApplication(projectId);
  const createDb = useCreateDatabase(projectId);
  const { data: features } = useFeatures();

  // App dialog state
  const [appDialogOpen, setAppDialogOpen] = useState(false);
  const [appName, setAppName] = useState("");
  const [appSlug, setAppSlug] = useState("");
  const [appSlugManual, setAppSlugManual] = useState(false);
  const [appType, setAppType] = useState<"image" | "git">("image");
  const [sourceImage, setSourceImage] = useState("");
  const [sourceRepo, setSourceRepo] = useState("");
  const [dockerfilePath, setDockerfilePath] = useState("Dockerfile");
  const [buildType, setBuildType] = useState("dockerfile");
  const [appPort, setAppPort] = useState(8080);
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
  const [dbRootPassword, setDbRootPassword] = useState("");
  const [dbError, setDbError] = useState("");

  const handleCreateApp = async () => {
    setAppError("");
    if (!appName.trim()) {
      setAppError("Application name is required");
      return;
    }
    if (appType === "image" && !sourceImage.trim()) {
      setAppError("Image name is required");
      return;
    }
    if (appType === "git" && !sourceRepo.trim()) {
      setAppError("Repository URL is required");
      return;
    }
    try {
      const application = await createApplication.mutateAsync({
        name: appName.trim(),
        slug: appSlug || undefined,
        type: appType,
        port: appPort,
        ...(appType === "image"
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
      setAppType("image");
      setAppPort(8080);
      setAppError("");
      navigate({
        to: "/projects/$projectId/applications/$applicationId",
        params: { projectId, applicationId: application.id },
      });
    } catch (e) {
      setAppError(
        e instanceof Error ? e.message : "Failed to create application",
      );
    }
  };

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
          setDbRootPassword("");
          setDbError("");
        },
        onError: (e) => {
          setDbError(e instanceof Error ? e.message : "Failed to create database");
        },
      },
    );
  };

  if (applicationsLoading || databasesLoading) {
    return <div className="text-muted-foreground">Loading resources...</div>;
  }

  const hasApplications = applications && applications.length > 0;
  const hasDatabases = databases && databases.length > 0;
  const isEmpty = !hasApplications && !hasDatabases;

  return (
    <div className="space-y-8">
      <div className="flex items-center justify-between">
        <h2 className="text-lg font-semibold">Applications</h2>
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
              <ToggleGroup
                variant="outline"
                value={[appType]}
                onValueChange={(v) => {
                  if (v.length > 0) setAppType(v[0] as "image" | "git");
                }}
                className="justify-start"
              >
                <ToggleGroupItem value="image">Docker Image</ToggleGroupItem>
                <ToggleGroupItem value="git">Git Repository</ToggleGroupItem>
              </ToggleGroup>
            </div>
            {appType === "image" ? (
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
                  <ToggleGroup
                    variant="outline"
                    value={[buildType]}
                    onValueChange={(v) => {
                      if (v.length > 0) setBuildType(v[0]);
                    }}
                    className="justify-start"
                  >
                    <ToggleGroupItem value="dockerfile">
                      Dockerfile
                    </ToggleGroupItem>
                    <ToggleGroupItem value="buildpacks">
                      Buildpacks
                    </ToggleGroupItem>
                    <ToggleGroupItem value="railpack">Railpack</ToggleGroupItem>
                  </ToggleGroup>
                  {buildType === "railpack" &&
                    features?.buildkit_available === false && (
                      <Alert variant="warning">
                        <TriangleAlert />
                        <AlertTitle>Warning</AlertTitle>
                        <AlertDescription>
                          BuildKit is not reachable. Railpack builds will fail.
                        </AlertDescription>
                      </Alert>
                    )}
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
          <div className="space-y-2">
              <Label htmlFor="app-port">Container Port</Label>
              <Input
                id="app-port"
                type="number"
                min="1"
                max="65535"
                value={appPort}
                onChange={(e) => setAppPort(parseInt(e.target.value) || 8080)}
              />
              <p className="text-muted-foreground text-xs">
                Port the container listens on (default 8080).
              </p>
            </div>
          <DialogFooter>
            <Button
              onClick={handleCreateApp}
              disabled={createApplication.isPending}
            >
              {createApplication.isPending && (
                <Loader2 className="mr-1 h-4 w-4 animate-spin" />
              )}
              Create Application
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Create Database Dialog */}
      <Dialog open={dbDialogOpen} onOpenChange={(open) => { setDbDialogOpen(open); if (!open) setDbError(""); }}>
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
          {applications?.map((application) => (
            <Link
              key={application.id}
              to="/projects/$projectId/applications/$applicationId"
              params={{ projectId, applicationId: application.id }}
            >
              <Card className="hover:bg-muted/50 cursor-pointer transition-colors">
                <CardHeader className="pb-2">
                  <div className="flex items-start justify-between">
                    <div className="flex items-start gap-2">
                      <AppWindowIcon className="text-muted-foreground size-4" />
                      <div className="flex flex-col">
                        <CardTitle className="text-base leading-none">
                          {application.name}
                        </CardTitle>
                        <CardDescription className="text-sm">
                          {application.slug}
                        </CardDescription>
                      </div>
                    </div>
                    <StatusBadge status={application.status} />
                  </div>
                </CardHeader>
                <CardContent>
                  <div className="text-muted-foreground flex gap-2 text-xs">
                    <Badge variant="outline" className="capitalize">
                      {application.type}
                    </Badge>
                    {application.type === "image" ? (
                      <Badge variant="outline">
                        {application.source_image}
                      </Badge>
                    ) : (
                      <Badge variant="outline" className="capitalize">
                        {application.build_type}
                      </Badge>
                    )}
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
                    <div className="flex items-start gap-2">
                      <DatabaseIcon className="text-muted-foreground size-4" />
                      <div className="flex flex-col">
                        <CardTitle className="text-base leading-none">
                          {db.name}
                        </CardTitle>
                        <CardDescription className="text-sm">
                          {db.slug}
                        </CardDescription>
                      </div>
                    </div>
                    <StatusBadge status={db.status} />
                  </div>
                </CardHeader>
                <CardContent>
                  <div className="text-muted-foreground flex gap-2 text-xs">
                    <Badge variant="outline">
                      {db.type}:{db.version}
                    </Badge>
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
