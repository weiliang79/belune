export interface User {
  id: string;
  email: string;
  role: "admin" | "member";
  username: string;
  first_name: string;
  last_name: string;
  created_at?: string;
  last_active_at?: string | null;
}

export interface Project {
  id: string;
  name: string;
  slug: string;
  user_id: string;
  created_at: string;
  updated_at: string;
  /** Most recent deployment start across the project's apps; null if never deployed. */
  last_deployed_at: string | null;
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

export interface ApplicationVolume {
  id: string;
  name: string;
  mount_path: string;
  size_bytes: number;
  created_at: string;
}

export interface VolumeBackupConfig {
  id: string;
  application_volume_id: string;
  destination_id: string;
  prefix: string;
  schedule: string;
  keep_latest?: number;
  enabled: boolean;
  quiesce: boolean;
  last_run_at?: string;
  created_at: string;
}

export interface VolumeBackup {
  id: string;
  status: "running" | "succeeded" | "failed";
  size_bytes: number;
  has_remote: boolean;
  config_id?: string;
  started_at: string;
  finished_at?: string;
  error?: string;
  log?: string;
}

export interface AppVolumeBackupConfig extends VolumeBackupConfig {
  volume_name: string;
  mount_path: string;
}

export interface VolumeRestore {
  id: string;
  backup_id?: string;
  status: "running" | "succeeded" | "failed";
  started_at: string;
  finished_at?: string;
  error?: string;
  log?: string;
}

export interface FileMount {
  id: string;
  mount_path: string;
  /** Decrypted content for non-secret mounts; empty when content_masked. */
  content?: string;
  is_secret: boolean;
  file_mode: string;
  /** True when the mount holds secret content masked in this response. */
  content_masked: boolean;
  created_at: string;
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
    error: number;
    total: number;
    /** Per-category running/total; categories the platform models (application, database). */
    by_type: Record<string, { running: number; total: number }>;
  };
}

export interface ServiceMetrics {
  cpu_percent: number;
  memory_used: number;
  memory_limit: number;
  uptime_seconds: number;
  status: string;
  domain?: string;
  port?: number;
}

/** Per-service runtime snapshot keyed by application id. */
export type ProjectMetrics = Record<string, ServiceMetrics>;

export interface ServerService {
  name: string;
  description: string;
  status: "running" | "error";
}

export interface ServerServices {
  healthy: number;
  total: number;
  services: ServerService[];
}

export interface HostMetricPoint {
  cpu_percent: number | null;
  memory_used: number | null;
  memory_total: number | null;
  disk_used: number | null;
  disk_total: number | null;
  // Swap is shown as its own series, never summed with RAM. swap_total is 0 on
  // hosts with no swap configured.
  swap_used: number | null;
  swap_total: number | null;
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

export interface ContainerLog {
  id: string;
  source_type: "application" | "database";
  source_id: string;
  level: "debug" | "info" | "warning" | "error";
  stream: "stdout" | "stderr";
  message: string;
  recorded_at: string;
}

export interface Database {
  id: string;
  project_id: string;
  type: "postgres" | "mysql" | "redis" | "mongo" | "other";
  name: string;
  slug: string;
  version: string;
  status: string;
  internal_host: string;
  internal_port: number;
  cpu_limit: number;
  memory_limit: number;
  host_port: number | null;
  // "other" type only:
  image: string | null;
  container_port: number | null;
  data_dir: string | null;
  backup_mode: "none" | "volume_snapshot" | "command";
  backup_command: string | null;
  restore_command: string | null;
  created_at: string;
  credentials?: Record<string, string>;
  connection_string?: string;
  volume?: { name: string; size_bytes: number };
  external_access?: {
    enabled: boolean;
    host_port?: number;
    ssh_host?: string;
    ssh_user?: string;
  };
}

export interface DatabaseBackup {
  id: string;
  status: "running" | "succeeded" | "failed";
  size_bytes: number;
  has_remote: boolean;
  remote_key?: string;
  config_id?: string;
  target_database?: string;
  started_at: string;
  finished_at?: string;
  error?: string;
  log?: string;
}

export type BackupProvider = "s3" | "r2" | "b2" | "wasabi" | "minio" | "other";

export interface BackupDestination {
  id: string;
  project_id: string;
  name: string;
  provider: BackupProvider;
  endpoint: string;
  region: string;
  bucket: string;
  prefix: string;
  use_ssl: boolean;
  created_at: string;
  updated_at: string;
}

export interface DatabaseRestore {
  id: string;
  backup_id?: string;
  status: "running" | "succeeded" | "failed";
  started_at: string;
  finished_at?: string;
  error?: string;
  log?: string;
}

export interface DatabaseBackupConfig {
  id: string;
  database_id: string;
  destination_id: string;
  prefix: string;
  schedule: string;
  keep_latest?: number;
  enabled: boolean;
  databases: string[]; // specific databases to back up; empty = all
  all_databases: boolean;
  last_run_at?: string;
  created_at: string;
  updated_at: string;
}

export interface ProjectBackupActivity extends DatabaseBackup {
  database_id: string;
  database_name: string;
  database_slug: string;
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

/** What the server last observed on the wire for a domain — not what its
 *  configuration says should happen. */
export type TLSStatus =
  | "unknown"
  | "disabled"
  | "pending"
  | "active"
  | "expiring"
  | "expired"
  | "failed"
  /** The proxy's own internal CA, when it is configured to issue that way (dev).
   *  Not the same as pending, which is that certificate when a public CA was
   *  expected and has not delivered. */
  | "local";

export interface DomainExpanded extends Domain {
  /** The public URL prefix this domain answers on; "/" means the whole host. */
  path: string;
  /** Whether the prefix is removed before the app sees the request. */
  strip_path: boolean;
  /** Prefix prepended after the strip, for an app that insists on a base path. */
  internal_path: string;
  container_port?: number | null;
  force_https: boolean;
  ssl_mode: string;
  ssl_provider?: string | null;
  certificate_id?: string | null;
  advanced_config?: unknown;
  /** The API names this `route_features` (the SQL alias is returned verbatim).
   *  It was declared here as `features`, so every read was silently undefined:
   *  the domain row never showed its feature count and the features list always
   *  said "none configured" — for features that existed and were live in Caddy. */
  route_features?: RouteFeature[];
  tls_status?: TLSStatus;
  tls_issuer?: string | null;
  tls_not_after?: string | null;
  tls_last_checked_at?: string | null;
  tls_error?: string | null;
  /** A suspicion that explains a pending domain; never a verdict on it. */
  tls_advisory?: string | null;
}

export interface AuditLog {
  id: string;
  user_id: string | null;
  action: string;
  resource_type: string;
  resource_id: string | null;
  resource_name?: string;
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
  log: string;
}

export interface BackupRemoteConfig {
  endpoint: string;
  region: string;
  bucket: string;
  prefix: string;
  use_ssl: boolean;
}

export interface BackupStatus {
  last_succeeded_at: string | null;
  last_attempted_at: string | null;
  last_error: string | null;
  remote_enabled: boolean;
  remote: BackupRemoteConfig | null;
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
  deploy_7d: {
    succeeded: number;
    failed: number;
    total: number;
    median_build_ms: number;
  };
  needs_attention: {
    failed_deploys: number;
    error_services: number;
    failed_backups: number;
    total: number;
  };
  host: HostResources | null;
}

// ---- Docker inspect (read-only admin pages) --------------------------------

/** Links a Docker resource back to the platform app/database that owns it. */
export interface DockerOwner {
  type: "application" | "database";
  id: string;
  name: string;
  project_id: string;
}

export interface DockerSystemInfo {
  server_version: string;
  operating_system: string;
  os_type: string;
  architecture: string;
  kernel_version: string;
  storage_driver: string;
  logging_driver: string;
  cgroup_driver: string;
  ncpu: number;
  mem_total: number;
  docker_root_dir: string;
  name: string;
  containers: number;
  containers_running: number;
  containers_paused: number;
  containers_stopped: number;
  images: number;
}

export interface DockerDiskUsageEntry {
  count: number;
  size: number;
  reclaimable: number;
}

export interface DockerDiskUsage {
  layers_size: number;
  images: DockerDiskUsageEntry;
  containers: DockerDiskUsageEntry;
  volumes: DockerDiskUsageEntry;
  build_cache: DockerDiskUsageEntry;
}

export interface DockerOverview {
  info: DockerSystemInfo;
  /** Null while it is still being computed: `docker system df` is far too slow
   *  to run inside the request (33s on a small VPS), so it is refreshed in the
   *  background and arrives on a later poll rather than failing the page. */
  disk_usage: DockerDiskUsage | null;
  counts: {
    containers_running: number;
    containers_total: number;
    images: number;
    volumes: number;
  };
}

export interface DockerContainer {
  id: string;
  name: string;
  image: string;
  status: string;
  ports: Record<string, string>;
  managed: boolean;
  owner?: DockerOwner;
  created_at: string;
}

export interface DockerImage {
  id: string;
  repo_tags: string[] | null;
  size: number;
  shared_size: number;
  containers: number;
  dangling: boolean;
  managed: boolean;
  owner?: DockerOwner;
  created_at: string;
}

export interface DockerVolume {
  name: string;
  driver: string;
  mountpoint: string;
  scope: string;
  size: number;
  ref_count: number;
  managed: boolean;
  kind: "" | "data" | "cache";
  owner?: DockerOwner;
  created_at: string;
}

export interface DockerNetworkContainer {
  id: string;
  name: string;
  ipv4_address: string;
}

// ---- Maintenance (Server page) ---------------------------------------------

export interface ReconcilerStatus {
  interval_seconds: number;
  last_run_at: string;
  last_duration_ms: number;
  last_added: number;
  last_removed: number;
  last_error?: string;
  run_count: number;
  total_drift: number;
}

export interface QueueDepth {
  queue: string;
  pending: number;
  active: number;
  retry: number;
  archived: number;
}

export interface QueueStatus {
  queues: QueueDepth[];
  total_stuck: number;
}

export interface DockerNetwork {
  id: string;
  name: string;
  driver: string;
  scope: string;
  internal: boolean;
  managed: boolean;
  containers: DockerNetworkContainer[] | null;
  created_at: string;
}
