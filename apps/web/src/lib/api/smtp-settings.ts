import { api } from "./client";

export type SmtpTLSMode = "none" | "starttls" | "tls";

/** Effective SMTP config; the password is masked to a presence flag. */
export interface SmtpSettings {
  host: string;
  port: number;
  user: string;
  from_email: string;
  from_name: string;
  tls_mode: SmtpTLSMode;
  password_set: boolean;
}

/** Save payload. Leave `password` blank to keep the stored secret. */
export interface SaveSmtpSettings {
  host: string;
  port: number;
  user: string;
  from_email: string;
  from_name: string;
  tls_mode: SmtpTLSMode;
  password: string;
}

export interface TestResult {
  ok: boolean;
  error?: string;
}

export function getSmtpSettings() {
  return api.get<SmtpSettings>("/settings/smtp");
}

export function updateSmtpSettings(data: SaveSmtpSettings) {
  return api.put<{ status: string }>("/settings/smtp", data);
}

export function testSmtpSettings(data: SaveSmtpSettings & { to: string }) {
  return api.post<TestResult>("/settings/smtp/test", data);
}
