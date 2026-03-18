import type { Database } from "@/lib/types";
import { api } from "./client";

export function listDatabases(projectId: string) {
  return api.get<Database[]>(`/projects/${projectId}/databases`);
}

export function getDatabase(projectId: string, databaseId: string) {
  return api.get<Database>(`/projects/${projectId}/databases/${databaseId}`);
}

export function createDatabase(
  projectId: string,
  data: {
    name: string;
    slug?: string;
    type: string;
    version?: string;
    credentials?: {
      user?: string;
      password?: string;
      database_name?: string;
    };
  },
) {
  return api.post<Database>(`/projects/${projectId}/databases`, data);
}

export function deleteDatabase(projectId: string, databaseId: string) {
  return api.delete<void>(`/projects/${projectId}/databases/${databaseId}`);
}
