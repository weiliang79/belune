import { api } from "./client";

export type GitProvider = "github" | "gitlab" | "bitbucket" | "gitea";

export interface GitProviderConfig {
  id: string;
  provider: GitProvider;
  base_url: string;
  client_id: string;
  app_id: string;
  app_slug: string;
  has_secret: boolean;
  is_public: boolean;
  is_owner: boolean;
}

export interface SaveGitProviderConfig {
  provider: GitProvider;
  base_url?: string;
  client_id?: string;
  app_id?: string;
  app_slug?: string;
  client_secret?: string;
  private_key?: string;
  webhook_secret?: string;
  is_public?: boolean;
}

export interface GitHubManifest {
  post_url: string;
  state: string;
  manifest: Record<string, unknown>;
}

export function listGitProviderConfigs() {
  return api.get<GitProviderConfig[]>("/git/providers");
}

export function saveGitProviderConfig(data: SaveGitProviderConfig) {
  return api.put<GitProviderConfig>("/git/providers", data);
}

export function deleteGitProviderConfig(id: string) {
  return api.delete<{ status: string }>(`/git/providers/${id}`);
}

export function getGitHubManifest(org?: string, isPublic?: boolean) {
  const params = new URLSearchParams();
  if (org) params.set("org", org);
  if (isPublic) params.set("public", "true");
  const q = params.toString() ? `?${params.toString()}` : "";
  return api.get<GitHubManifest>(`/git/providers/github/manifest${q}`);
}

// submitGitHubManifest performs a top-level form POST to GitHub so it creates
// the App and redirects back to our manifest callback.
export function submitGitHubManifest(m: GitHubManifest) {
  const form = document.createElement("form");
  form.method = "POST";
  form.action = `${m.post_url}?state=${encodeURIComponent(m.state)}`;

  const input = document.createElement("input");
  input.type = "hidden";
  input.name = "manifest";
  input.value = JSON.stringify(m.manifest);
  form.appendChild(input);

  document.body.appendChild(form);
  form.submit();
}
