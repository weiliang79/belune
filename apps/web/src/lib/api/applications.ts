import type { Application } from "@/lib/types";
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
    git_credential_id?: string;
    health_check_path?: string;
  },
) {
  return api.put<Application>(`/projects/${projectId}/applications/${applicationId}`, data);
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
