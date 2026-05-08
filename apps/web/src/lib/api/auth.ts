import type { User } from "@/lib/types";
import { api } from "./client";

export function login(email: string, password: string) {
  return api.post<{ token: string; user: User }>("/auth/login", {
    email,
    password,
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

export function setup(email: string, password: string, username?: string) {
  return api.post<User>("/auth/setup", { email, password, username: username ?? "" });
}

export function updateProfile(data: { username: string; first_name: string; last_name: string }) {
  return api.put<User>("/auth/profile", data);
}

export function forgotPassword(email: string) {
  return api.post<{ status: string }>("/auth/forgot-password", { email });
}

export function resetPassword(token: string, new_password: string) {
  return api.post<void>("/auth/reset-password", { token, new_password });
}
