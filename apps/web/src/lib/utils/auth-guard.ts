import { redirect } from "@tanstack/react-router";
import { getMe } from "@/lib/api/auth";
import { ApiError } from "@/lib/api/client";
import { useAuthStore } from "@/lib/stores/auth";

/**
 * beforeLoad guard for the admin-only pages. The sidebar hides their links, but a
 * hidden link is not a guard: a Member who typed the URL got the page, and with it
 * a subscription to live channels only an admin may hear.
 *
 * Reads the store rather than probing the session — the root route's beforeLoad
 * runs first and has already loaded the user. No user reads as non-admin, so this
 * fails closed if that ever stops being true.
 */
export function requireAdmin(): void {
  if (useAuthStore.getState().user?.role !== "admin") {
    throw redirect({ to: "/projects" });
  }
}

/**
 * beforeLoad guard for the logged-out pages (login / setup / forgot-password):
 * an already-authenticated user has no business on them, so send them to `to`.
 *
 * Why this can't just read the store: the root route deliberately skips its auth
 * check on /login and /setup, so `isAuthenticated` is still false there even for a
 * signed-in user — a store read would never fire. So probe the session directly
 * with getMe, treating a 401 as "logged out, show the form" and populating the
 * store on success (which also lets the fast path below short-circuit routes the
 * root does resolve, like /forgot-password).
 */
export async function redirectIfAuthenticated(to: string): Promise<void> {
  if (useAuthStore.getState().isAuthenticated) {
    throw redirect({ to: to as never });
  }
  try {
    const user = await getMe();
    useAuthStore.getState().setUser(user);
  } catch (e) {
    if (e instanceof ApiError && e.status === 401) return; // logged out — show the form
    throw e; // real error (or a redirect thrown elsewhere) — don't swallow it
  }
  throw redirect({ to: to as never });
}
