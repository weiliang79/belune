export interface User {
  id: string;
  email: string;
  role: string;
}

export interface Project {
  id: string;
  name: string;
  slug: string;
  user_id: string;
  created_at: string;
  updated_at: string;
}

export interface Service {
  id: string;
  project_id: string;
  name: string;
  slug: string;
  type: "git" | "image";
  source_repo: string | null;
  source_image: string | null;
  dockerfile_path: string;
  build_type: "dockerfile" | "buildpacks" | "railpack" | "image";
  build_type_override: string | null;
  builder_image: string | null;
  status: string;
  webhook_secret: string | null;
  auto_deploy_branch: string | null;
  created_at: string;
  updated_at: string;
}

export interface Deployment {
  id: string;
  service_id: string;
  status: "pending" | "building" | "deploying" | "success" | "failed";
  triggered_by: "push" | "manual" | "api";
  commit_sha: string | null;
  build_logs: string;
  error_message: string | null;
  started_at: string;
  finished_at: string | null;
}

export interface Domain {
  id: string;
  service_id: string;
  hostname: string;
  ssl_enabled: boolean;
  verified_at: string | null;
  created_at: string;
}

export interface EnvVar {
  id: string;
  key: string;
  value: string;
  is_secret: boolean;
}

export interface MetricsOverview {
  projects: number;
  services: number;
  databases: number;
  deployments: number;
  containers: {
    running: number;
    stopped: number;
    total: number;
  };
}

export interface Database {
  id: string;
  project_id: string;
  type: "postgres" | "mysql" | "redis" | "mongo";
  name: string;
  slug: string;
  version: string;
  status: string;
  internal_host: string;
  internal_port: number;
  created_at: string;
  credentials?: Record<string, string>;
  connection_string?: string;
}
