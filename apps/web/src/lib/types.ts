export interface User {
  id: string;
  email: string;
  role: "admin" | "member";
  username: string;
  first_name: string;
  last_name: string;
  created_at?: string;
}

export interface Project {
  id: string;
  name: string;
  slug: string;
  user_id: string;
  created_at: string;
  updated_at: string;
}

export interface Application {
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
  cpu_limit: number;
  memory_limit: number;
  status: string;
  webhook_secret: string | null;
  auto_deploy_branch: string | null;
  health_check_path: string | null;
  parent_application_id: string | null;
  branch: string | null;
  preview_branch_pattern: string | null;
  preview_domain_template: string | null;
  last_activity_at: string;
  created_at: string;
  updated_at: string;
}

export interface Deployment {
  id: string;
  application_id: string;
  status: "pending" | "building" | "deploying" | "success" | "failed";
  triggered_by: "push" | "manual" | "api" | "rollback";
  commit_sha: string | null;
  build_logs: string;
  error_message: string | null;
  image_tag: string | null;
  started_at: string;
  build_started_at: string | null;
  build_ended_at: string | null;
  deploy_started_at: string | null;
  finished_at: string | null;
}

export interface Domain {
  id: string;
  application_id: string;
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
  applications: number;
  databases: number;
  deployments: number;
  containers: {
    running: number;
    stopped: number;
    total: number;
  };
}

export interface HostMetricPoint {
  cpu_percent: number | null;
  memory_used: number | null;
  memory_total: number | null;
  disk_used: number | null;
  disk_total: number | null;
  recorded_at: string;
}

export interface AppMetricPoint {
  cpu_percent: number | null;
  memory_usage: number | null;
  memory_limit: number | null;
  network_rx_bytes: number | null;
  network_tx_bytes: number | null;
  recorded_at: string;
}

export interface SettingEntry {
  key: string;
  value: string;
}

export interface GlobalDeployment {
  id: string;
  application_id: string;
  status: string;
  triggered_by: string;
  commit_sha: string | null;
  build_logs: string | null;
  error_message: string | null;
  image_tag: string | null;
  started_at: string;
  finished_at: string | null;
  application_name: string;
  application_slug: string;
  project_id: string;
  project_name: string;
}

export interface RequestLog {
  id: string;
  application_id: string;
  method: string;
  path: string;
  status_code: number;
  latency_ms: number;
  hostname: string;
  request_size: number | null;
  response_size: number | null;
  client_ip: string | null;
  user_agent: string | null;
  recorded_at: string;
}

export interface ApplicationLog {
  id: string;
  application_id: string;
  stream: "stdout" | "stderr";
  message: string;
  recorded_at: string;
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
  cpu_limit: number;
  memory_limit: number;
  host_port: number | null;
  created_at: string;
  credentials?: Record<string, string>;
  connection_string?: string;
  volume?: { name: string; size_bytes: number };
}

export interface GitCredential {
  id: string;
  name: string;
  provider: "github" | "gitlab" | "bitbucket" | "generic";
  username: string;
  created_at: string;
  updated_at: string;
}

export interface RouteFeature {
  id: string;
  domain_id: string;
  feature_type:
    | "basic_auth"
    | "redirect"
    | "headers"
    | "ip_allowlist"
    | "rate_limit";
  config: Record<string, unknown>;
  enabled: boolean;
  created_at: string;
  updated_at: string;
}

export interface DomainExpanded extends Domain {
  container_port?: number | null;
  force_https: boolean;
  ssl_mode: string;
  ssl_provider?: string | null;
  cert_path?: string | null;
  key_path?: string | null;
  advanced_config?: unknown;
  features?: RouteFeature[];
}

export interface AuditLog {
  id: string;
  user_id: string | null;
  action: string;
  resource_type: string;
  resource_id: string | null;
  details: Record<string, unknown> | null;
  ip_address: string | null;
  created_at: string;
  user_email?: string;
}

export interface Invitation {
  id: string;
  email: string;
  role: string;
  invited_by_user_id: string;
  expires_at: string;
  accepted_at: string | null;
  created_at: string;
}

export interface BackupRun {
  id: string;
  started_at: string;
  finished_at: string | null;
  status: "running" | "succeeded" | "failed";
  remote_key: string | null;
  size_bytes: number;
  error: string | null;
}

export interface BackupStatus {
  last_succeeded_at: string | null;
  last_attempted_at: string | null;
  last_error: string | null;
  remote_enabled: boolean;
  retention: { days: number; count: number };
}

export interface AlertPreferences {
  deploy_failures: boolean;
  deploy_success: boolean;
  build_failures: boolean;
  quota_threshold: boolean;
  quota_threshold_percent: number;
}

export interface Notification {
  id: string;
  user_id: string;
  type: string;
  title: string;
  body: string;
  link: string | null;
  read: boolean;
  created_at: string;
}

export interface HostResources {
  cpu_percent: number;
  memory_used: number;
  memory_total: number;
  disk_used: number;
  disk_total: number;
  recorded_at: string;
}

export interface Stats {
  is_admin: boolean;
  app_health: { running: number; total: number };
  deploy_7d: { succeeded: number; failed: number; total: number };
  needs_attention: {
    failed_deploys: number;
    error_services: number;
    failed_backups: number;
    total: number;
  };
  host: HostResources | null;
}
