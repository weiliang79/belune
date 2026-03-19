import type { User } from "@/lib/types";
import { api } from "./client";

export function listUsers() {
  return api.get<User[]>("/users");
}

export function createUser(data: {
  email: string;
  password: string;
  role: string;
}) {
  return api.post<User>("/users", data);
}

export function updateUserRole(userId: string, role: string) {
  return api.put<User>(`/users/${userId}/role`, { role });
}

export function deleteUser(userId: string) {
  return api.delete<void>(`/users/${userId}`);
}

export function resetUserPassword(userId: string, password: string) {
  return api.put<void>(`/users/${userId}/password`, { password });
}

export function changeOwnPassword(data: {
  current_password: string;
  new_password: string;
}) {
  return api.put<void>("/auth/password", data);
}
