import type { User } from "@/lib/types";
import { api } from "./client";

export function listUsers() {
  return api.get<User[]>("/users");
}

export function createUser(data: {
  email: string;
  password: string;
  role: string;
  username?: string;
}) {
  return api.post<User>("/users", data);
}

export function updateProfile(data: {
  username: string;
  first_name: string;
  last_name: string;
}) {
  return api.put<User>("/auth/profile", data);
}

export function updateUserRole(userId: string, role: string) {
  return api.put<User>(`/users/${userId}/role`, { role });
}

export function deleteUser(userId: string) {
  return api.delete<void>(`/users/${userId}`);
}

/** Setting someone else's password steps up on the caller's OWN current
 *  password, not the target's — a hijacked or borrowed admin session must
 *  not be able to take over any account with nothing re-checked. */
export function resetUserPassword(
  userId: string,
  password: string,
  currentPassword: string,
) {
  return api.put<void>(`/users/${userId}/password`, {
    password,
    current_password: currentPassword,
  });
}

export function changeOwnPassword(data: {
  current_password: string;
  new_password: string;
}) {
  return api.put<void>("/auth/password", data);
}
