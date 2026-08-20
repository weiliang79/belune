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
 *  this secret has been verified. Takes the password because enrolling is at
 *  least as dangerous as disabling: whoever enrolls holds the authenticator. */
export function enrollTotp(password: string) {
  return api.post<TOTPEnrollment>("/auth/totp/enroll", { password });
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

/** Needs a current code as well as the password: ten fresh recovery codes are
 *  ten working second factors. */
export function regenerateRecoveryCodes(password: string, code: string) {
  return api.post<{ recovery_codes: string[] }>("/auth/totp/recovery-codes", {
    password,
    code,
  });
}

export function resetUserTotp(userId: string) {
  return api.post<{ status: string }>(`/users/${userId}/totp/reset`);
}
