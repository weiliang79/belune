import type { ApiToken, CreatedApiToken } from "@/lib/types";
import { api } from "./client";

export function listTokens() {
  return api.get<ApiToken[]>("/tokens");
}

/** expiresInDays omitted (or undefined) means the token never expires. */
export function createToken(name: string, expiresInDays?: number) {
  return api.post<CreatedApiToken>("/tokens", {
    name,
    expires_in_days: expiresInDays,
  });
}

export function deleteToken(id: string) {
  return api.delete<{ status: string }>(`/tokens/${id}`);
}
