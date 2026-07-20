import type { Application, DeployHook } from "@/lib/types";
import { api } from "./client";

export function listApplications(projectId: string) {
  return api.get<Application[]>(`/projects/${projectId}/applications`);
}

export function getApplication(projectId: string, applicationId: string) {
  return api.get<Application>(`/projects/${projectId}/applications/${applicationId}`);
}

export function createApplication(
  projectId: string,
  data: {
    name: string;
    slug?: string;
    type: "git" | "image";
    source_repo?: string;
    source_image?: string;
    dockerfile_path?: string;
    build_type?: string;
    git_integration_id?: string;
  },
) {
  return api.post<Application>(`/projects/${projectId}/applications`, data);
}

export function updateApplication(
  projectId: string,
  applicationId: string,
  data: {
    name?: string;
    source_repo?: string;
    source_image?: string;
    dockerfile_path?: string;
    build_type_override?: string;
    builder_image?: string;
    cpu_limit?: number;
    memory_limit?: number;
    git_token?: string;
    git_integration_id?: string;
    health_check_path?: string;
  },
) {
  return api.put<Application>(`/projects/${projectId}/applications/${applicationId}`, data);
}

export function updateApplicationRuntime(
  projectId: string,
  applicationId: string,
  data: { readonly_rootfs: boolean; container_caps: "minimal" | "standard" },
) {
  return api.put<{ readonly_rootfs: boolean; container_caps: string }>(
    `/projects/${projectId}/applications/${applicationId}/runtime`,
    data,
  );
}

export function deleteApplication(projectId: string, applicationId: string) {
  return api.delete<void>(`/projects/${projectId}/applications/${applicationId}`);
}

export function deployApplication(projectId: string, applicationId: string) {
  return api.post<{ message: string }>(
    `/projects/${projectId}/applications/${applicationId}/deploy`,
  );
}

export function stopApplication(projectId: string, applicationId: string) {
  return api.post<Application>(
    `/projects/${projectId}/applications/${applicationId}/stop`,
  );
}

export function startApplication(projectId: string, applicationId: string) {
  return api.post<Application>(
    `/projects/${projectId}/applications/${applicationId}/start`,
  );
}

export function restartApplication(projectId: string, applicationId: string) {
  return api.post<Application>(
    `/projects/${projectId}/applications/${applicationId}/restart`,
  );
}

// Recreate the container from the current image (skip build) to apply config
// changes — volumes, file mounts, env, resource limits — without pulling new code.
export function reloadApplication(projectId: string, applicationId: string) {
  return api.post<{ message: string }>(
    `/projects/${projectId}/applications/${applicationId}/reload`,
  );
}

// Rebuild the currently-deployed commit (git apps only), not branch HEAD.
export function rebuildApplication(projectId: string, applicationId: string) {
  return api.post<{ message: string }>(
    `/projects/${projectId}/applications/${applicationId}/rebuild`,
  );
}

export interface BuildCacheInfo {
  build_cache_bytes: number;
  launch_cache_bytes: number;
  total_bytes: number;
}

export function getBuildCache(projectId: string, applicationId: string) {
  return api.get<BuildCacheInfo>(
    `/projects/${projectId}/applications/${applicationId}/cache`,
  );
}

export function clearBuildCache(projectId: string, applicationId: string) {
  return api.delete<{ status: string }>(
    `/projects/${projectId}/applications/${applicationId}/cache`,
  );
}

export function updateWebhook(
  projectId: string,
  applicationId: string,
  data: {
    webhook_secret?: string;
    auto_deploy_branch?: string;
  },
) {
  return api.put<Application>(
    `/projects/${projectId}/applications/${applicationId}/webhook`,
    data,
  );
}

export function getDeployHook(projectId: string, applicationId: string) {
  return api.get<DeployHook>(
    `/projects/${projectId}/applications/${applicationId}/deploy-hook`,
  );
}

// Returns the stored token so the URL can be copied again later. Audited
// server-side, so only call it when the user asks to see the URL.
export function revealDeployHook(projectId: string, applicationId: string) {
  return api.get<DeployHook>(
    `/projects/${projectId}/applications/${applicationId}/deploy-hook/reveal`,
  );
}

// Generates or rotates the token. Rotating invalidates the previous URL
// immediately, so any CI still using it starts getting 404s.
export function generateDeployHook(projectId: string, applicationId: string) {
  return api.post<DeployHook>(
    `/projects/${projectId}/applications/${applicationId}/deploy-hook`,
    {},
  );
}

export function deleteDeployHook(projectId: string, applicationId: string) {
  return api.delete<DeployHook>(
    `/projects/${projectId}/applications/${applicationId}/deploy-hook`,
  );
}
