import type { ApiToken, CreatedApiToken, TokenScope } from "@/lib/types";
import { api } from "./client";

export function listTokens() {
  return api.get<ApiToken[]>("/tokens");
}

/** expiresInDays omitted (or undefined) means the token never expires. scopes
 *  must be non-empty — the API rejects a token with none. projectId omitted
 *  (or undefined) means unpinned — every project the owner can reach,
 *  evaluated at use time; the API rejects one the caller cannot reach. */
export function createToken(
  name: string,
  scopes: TokenScope[],
  expiresInDays?: number,
  projectId?: string,
) {
  return api.post<CreatedApiToken>("/tokens", {
    name,
    scopes,
    expires_in_days: expiresInDays,
    project_id: projectId,
  });
}

export function deleteToken(id: string) {
  return api.delete<{ status: string }>(`/tokens/${id}`);
}
