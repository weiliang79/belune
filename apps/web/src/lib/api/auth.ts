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

export function getMe() {
  return api.get<User>("/auth/me");
}

export function checkSetup() {
  return api.get<{ setup_required: boolean }>("/auth/setup");
}

export function setup(email: string, password: string) {
  return api.post<User>("/auth/setup", { email, password });
}
