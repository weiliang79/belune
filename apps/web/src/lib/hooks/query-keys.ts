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
    all: (projectId: string) => ["projects", projectId, "applications"] as const,
    detail: (projectId: string, applicationId: string) =>
      ["projects", projectId, "applications", applicationId] as const,
  },
  deployments: {
    all: (projectId: string, applicationId: string) =>
      ["projects", projectId, "applications", applicationId, "deployments"] as const,
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
  domains: {
    all: (projectId: string, applicationId: string) =>
      ["projects", projectId, "applications", applicationId, "domains"] as const,
  },
  envvars: {
    all: (projectId: string, applicationId: string) =>
      ["projects", projectId, "applications", applicationId, "env"] as const,
  },
  projectEnvvars: {
    all: (projectId: string) =>
      ["projects", projectId, "env"] as const,
  },
  users: {
    all: ["users"] as const,
  },
  features: ["features"] as const,
  metrics: ["metrics"] as const,
  hostMetrics: (range: string) => ["metrics", "host", range] as const,
  settings: ["settings"] as const,
  databases: {
    all: (projectId: string) =>
      ["projects", projectId, "databases"] as const,
    detail: (projectId: string, databaseId: string) =>
      ["projects", projectId, "databases", databaseId] as const,
  },
  applicationLogs: {
    history: (projectId: string, applicationId: string, params?: object) =>
      ["projects", projectId, "applications", applicationId, "logs", "history", params] as const,
  },
  globalDeployments: (params?: object) =>
    ["deployments", params] as const,
  requestLogs: (params?: object) =>
    ["requests", params] as const,
  gitCredentials: ["git-credentials"] as const,
  auditLogs: (params?: object) =>
    ["audit-logs", params] as const,
  routeFeatures: (projectId: string, applicationId: string, domainId: string) =>
    ["projects", projectId, "applications", applicationId, "domains", domainId, "features"] as const,
};
