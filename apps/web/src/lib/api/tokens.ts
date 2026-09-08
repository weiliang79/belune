import type { ApiToken, CreatedApiToken, TokenScope } from "@/lib/types";
import { api } from "./client";

export function listTokens() {
  return api.get<ApiToken[]>("/tokens");
}

/** expiresInDays omitted (or undefined) means the token never expires. scopes
 *  must be non-empty — the API rejects a token with none. */
export function createToken(
  name: string,
  scopes: TokenScope[],
  expiresInDays?: number,
) {
  return api.post<CreatedApiToken>("/tokens", {
    name,
    scopes,
    expires_in_days: expiresInDays,
  });
}

export function deleteToken(id: string) {
  return api.delete<{ status: string }>(`/tokens/${id}`);
}
