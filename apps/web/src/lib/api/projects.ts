import type { Project } from "@/lib/types";
import { api } from "./client";

export function listProjects() {
  return api.get<Project[]>("/projects");
}

export function getProject(id: string) {
  return api.get<Project>(`/projects/${id}`);
}

export function createProject(data: { name: string; slug: string }) {
  return api.post<Project>("/projects", data);
}

export function updateProject(
  id: string,
  data: { name: string; slug: string },
) {
  return api.put<Project>(`/projects/${id}`, data);
}

export function deleteProject(id: string) {
  return api.delete<void>(`/projects/${id}`);
}
