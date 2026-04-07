import type { GitCredential } from "@/lib/types";
import { api } from "./client";

export function listGitCredentials() {
  return api.get<GitCredential[]>("/git-credentials");
}

export function createGitCredential(data: {
  name: string;
  provider: string;
  token: string;
  username?: string;
}) {
  return api.post<GitCredential>("/git-credentials", data);
}

export function updateGitCredential(
  id: string,
  data: {
    name?: string;
    provider?: string;
    token?: string;
    username?: string;
  },
) {
  return api.put<GitCredential>(`/git-credentials/${id}`, data);
}

export function deleteGitCredential(id: string) {
  return api.delete<void>(`/git-credentials/${id}`);
}
