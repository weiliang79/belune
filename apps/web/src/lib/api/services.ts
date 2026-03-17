import type { Service } from "@/lib/types";
import { api } from "./client";

export function listServices(projectId: string) {
  return api.get<Service[]>(`/projects/${projectId}/services`);
}

export function getService(projectId: string, serviceId: string) {
  return api.get<Service>(`/projects/${projectId}/services/${serviceId}`);
}

export function createService(
  projectId: string,
  data: {
    name: string;
    type: "git" | "image";
    source_repo?: string;
    source_image?: string;
    dockerfile_path?: string;
    build_type?: string;
  },
) {
  return api.post<Service>(`/projects/${projectId}/services`, data);
}

export function updateService(
  projectId: string,
  serviceId: string,
  data: {
    name?: string;
    source_repo?: string;
    source_image?: string;
    dockerfile_path?: string;
    build_type_override?: string;
    builder_image?: string;
  },
) {
  return api.put<Service>(`/projects/${projectId}/services/${serviceId}`, data);
}

export function deleteService(projectId: string, serviceId: string) {
  return api.delete<void>(`/projects/${projectId}/services/${serviceId}`);
}

export function deployService(projectId: string, serviceId: string) {
  return api.post<{ message: string }>(
    `/projects/${projectId}/services/${serviceId}/deploy`,
  );
}

export function stopService(projectId: string, serviceId: string) {
  return api.post<{ status: string }>(
    `/projects/${projectId}/services/${serviceId}/stop`,
  );
}

export function restartService(projectId: string, serviceId: string) {
  return api.post<{ status: string }>(
    `/projects/${projectId}/services/${serviceId}/restart`,
  );
}
