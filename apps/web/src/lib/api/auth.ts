import type { User } from "@/lib/types";
import { api } from "./client";

/** A login either finishes or comes back with a challenge. The challenge names
 *  the methods that can complete it, so the client never hard-codes which
 *  factors exist — adding one later changes nothing here. */
export type LoginResponse =
  | { token: string; user: User; challenge?: undefined }
  | {
      challenge: string;
      methods: string[];
      expires_in: number;
      token?: undefined;
    };

export function login(email: string, password: string) {
  return api.post<LoginResponse>("/auth/login", {
    email,
    password,
  });
}

/** Second step of the same login. The method travels as data, not in the URL. */
export function verifyLogin(challenge: string, method: string, code: string) {
  return api.post<{ token: string; user: User }>("/auth/login/verify", {
    challenge,
    method,
    code,
  });
}

export function logout() {
  return api.post<void>("/auth/logout");
}

export function refreshSession() {
  return api.post<{
    token: string;
    refresh_token: string;
    expires_in: number;
    user: User;
  }>("/auth/refresh");
}

export function getMe() {
  return api.get<User>("/auth/me");
}

export function checkSetup() {
  return api.get<{ setup_required: boolean }>("/auth/setup");
}

export function setup(
  email: string,
  password: string,
  username?: string,
  instanceName?: string,
) {
  return api.post<User>("/auth/setup", {
    email,
    password,
    username: username ?? "",
    instance_name: instanceName ?? "",
  });
}

export function updateProfile(data: {
  username: string;
  first_name: string;
  last_name: string;
}) {
  return api.put<User>("/auth/profile", data);
}

export function forgotPassword(email: string) {
  return api.post<{ status: string }>("/auth/forgot-password", { email });
}

export function resetPassword(token: string, new_password: string) {
  return api.post<void>("/auth/reset-password", { token, new_password });
}
