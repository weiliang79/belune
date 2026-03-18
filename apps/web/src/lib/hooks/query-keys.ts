export const queryKeys = {
  auth: {
    me: ["auth", "me"] as const,
    setup: ["auth", "setup"] as const,
  },
  projects: {
    all: ["projects"] as const,
    detail: (id: string) => ["projects", id] as const,
  },
  services: {
    all: (projectId: string) => ["projects", projectId, "services"] as const,
    detail: (projectId: string, serviceId: string) =>
      ["projects", projectId, "services", serviceId] as const,
  },
  deployments: {
    all: (projectId: string, serviceId: string) =>
      ["projects", projectId, "services", serviceId, "deployments"] as const,
    detail: (projectId: string, serviceId: string, deploymentId: string) =>
      [
        "projects",
        projectId,
        "services",
        serviceId,
        "deployments",
        deploymentId,
      ] as const,
  },
  domains: {
    all: (projectId: string, serviceId: string) =>
      ["projects", projectId, "services", serviceId, "domains"] as const,
  },
  envvars: {
    all: (projectId: string, serviceId: string) =>
      ["projects", projectId, "services", serviceId, "env"] as const,
  },
  metrics: ["metrics"] as const,
  databases: {
    all: (projectId: string) =>
      ["projects", projectId, "databases"] as const,
    detail: (projectId: string, databaseId: string) =>
      ["projects", projectId, "databases", databaseId] as const,
  },
};
