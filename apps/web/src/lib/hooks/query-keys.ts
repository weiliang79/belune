export const queryKeys = {
  auth: {
    me: ["auth", "me"] as const,
    setup: ["auth", "setup"] as const,
  },
  projects: {
    all: ["projects"] as const,
    detail: (id: string) => ["projects", id] as const,
  },
  applications: {
    all: (projectId: string) =>
      ["projects", projectId, "applications"] as const,
    detail: (projectId: string, applicationId: string) =>
      ["projects", projectId, "applications", applicationId] as const,
    buildCache: (projectId: string, applicationId: string) =>
      ["projects", projectId, "applications", applicationId, "cache"] as const,
  },
  deployments: {
    all: (projectId: string, applicationId: string) =>
      [
        "projects",
        projectId,
        "applications",
        applicationId,
        "deployments",
      ] as const,
    detail: (projectId: string, applicationId: string, deploymentId: string) =>
      [
        "projects",
        projectId,
        "applications",
        applicationId,
        "deployments",
        deploymentId,
      ] as const,
  },
  volumes: {
    all: (projectId: string, applicationId: string) =>
      [
        "projects",
        projectId,
        "applications",
        applicationId,
        "volumes",
      ] as const,
  },
  fileMounts: {
    all: (projectId: string, applicationId: string) =>
      [
        "projects",
        projectId,
        "applications",
        applicationId,
        "file-mounts",
      ] as const,
  },
  volumeBackupConfigs: (
    projectId: string,
    applicationId: string,
    volumeId: string,
  ) =>
    [
      "projects",
      projectId,
      "applications",
      applicationId,
      "volumes",
      volumeId,
      "backup-configs",
    ] as const,
  volumeBackups: (
    projectId: string,
    applicationId: string,
    volumeId: string,
  ) =>
    [
      "projects",
      projectId,
      "applications",
      applicationId,
      "volumes",
      volumeId,
      "backups",
    ] as const,
  domains: {
    all: (projectId: string, applicationId: string) =>
      [
        "projects",
        projectId,
        "applications",
        applicationId,
        "domains",
      ] as const,
  },
  envvars: {
    all: (projectId: string, applicationId: string) =>
      ["projects", projectId, "applications", applicationId, "env"] as const,
  },
  projectEnvvars: {
    all: (projectId: string) => ["projects", projectId, "env"] as const,
  },
  users: {
    all: ["users"] as const,
  },
  features: ["features"] as const,
  metrics: ["metrics"] as const,
  projectMetrics: (projectId: string) =>
    ["project-metrics", projectId] as const,
  serverServices: ["server-services"] as const,
  stats: ["stats"] as const,
  notifications: {
    list: ["notifications"] as const,
    unread: ["notifications", "unread"] as const,
  },
  hostMetrics: (range: string) => ["metrics", "host", range] as const,
  hostMetricsRange: (from: string, to: string) =>
    ["metrics", "host", "range", from, to] as const,
  settings: ["settings"] as const,
  databases: {
    all: (projectId: string) => ["projects", projectId, "databases"] as const,
    detail: (projectId: string, databaseId: string) =>
      ["projects", projectId, "databases", databaseId] as const,
    backups: (projectId: string, databaseId: string) =>
      ["projects", projectId, "databases", databaseId, "backups"] as const,
    backupConfigs: (projectId: string, databaseId: string) =>
      ["projects", projectId, "databases", databaseId, "backup-configs"] as const,
  },
  backupDestinations: (projectId: string) =>
    ["projects", projectId, "backup-destinations"] as const,
  projectBackups: (projectId: string) =>
    ["projects", projectId, "project-backups"] as const,
  applicationLogs: {
    history: (projectId: string, applicationId: string, params?: object) =>
      [
        "projects",
        projectId,
        "applications",
        applicationId,
        "logs",
        "history",
        params,
      ] as const,
  },
  globalDeployments: (params?: object) => ["deployments", params] as const,
  requestLogs: (params?: object) => ["requests", params] as const,
  requestSummary: (params?: object) => ["requests-summary", params] as const,
  auditLogs: (params?: object) => ["audit-logs", params] as const,
  auditActions: ["audit-actions"] as const,
  gitProviders: ["git-providers"] as const,
  gitIntegrations: ["git-integrations"] as const,
  gitAvailableProviders: ["git-available-providers"] as const,
  routeFeatures: (projectId: string, applicationId: string, domainId: string) =>
    [
      "projects",
      projectId,
      "applications",
      applicationId,
      "domains",
      domainId,
      "features",
    ] as const,
  quotas: {
    all: ["quotas"] as const,
    detail: (scope: string, scopeId: string) =>
      ["quotas", scope, scopeId] as const,
  },
  previews: {
    all: (projectId: string, applicationId: string) =>
      [
        "projects",
        projectId,
        "applications",
        applicationId,
        "previews",
      ] as const,
  },
  backups: {
    runs: ["backups", "runs"] as const,
    status: ["backups", "status"] as const,
  },
  alertPreferences: ["alert-preferences"] as const,
  invitations: ["invitations"] as const,
};
