import type { User } from "@/lib/types";
import { api } from "./client";

export interface TOTPStatus {
  enabled: boolean;
  recovery_codes_remaining: number;
}

export interface TOTPEnrollment {
  secret: string;
  uri: string;
  qr_code: string;
}

/** The session handed back after a factor change: every other session is ended,
 *  so the browser that made the change is issued a replacement. */
interface RotatedSession {
  session: { token: string; user: User } | null;
}

export function getTotpStatus() {
  return api.get<TOTPStatus>("/auth/totp");
}

/** Starts enrollment. Enables nothing — the factor is only on once a code from
 *  this secret has been verified. */
export function enrollTotp() {
  return api.post<TOTPEnrollment>("/auth/totp/enroll");
}

export function verifyTotpEnrollment(code: string) {
  return api.post<{ recovery_codes: string[] } & RotatedSession>(
    "/auth/totp/enroll/verify",
    { code },
  );
}

export function disableTotp(password: string, code: string) {
  return api.post<{ status: string } & RotatedSession>("/auth/totp/disable", {
    password,
    code,
  });
}

export function regenerateRecoveryCodes(password: string) {
  return api.post<{ recovery_codes: string[] }>("/auth/totp/recovery-codes", {
    password,
  });
}

export function resetUserTotp(userId: string) {
  return api.post<{ status: string }>(`/users/${userId}/totp/reset`);
}
