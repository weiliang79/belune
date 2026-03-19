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
  users: {
    all: ["users"] as const,
  },
  features: ["features"] as const,
  metrics: ["metrics"] as const,
  databases: {
    all: (projectId: string) =>
      ["projects", projectId, "databases"] as const,
    detail: (projectId: string, databaseId: string) =>
      ["projects", projectId, "databases", databaseId] as const,
  },
};
