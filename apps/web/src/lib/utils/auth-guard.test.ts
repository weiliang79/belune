import { beforeEach, describe, expect, it } from "vitest";
import { isRedirect } from "@tanstack/react-router";
import { useAuthStore } from "@/lib/stores/auth";
import type { User } from "@/lib/types";
import { requireAdmin } from "./auth-guard";

const userWithRole = (role: User["role"]): User => ({
  id: "user-1",
  email: "user@example.com",
  role,
  username: "user",
  first_name: "Test",
  last_name: "User",
});

function thrownBy(fn: () => void): unknown {
  try {
    fn();
  } catch (e) {
    return e;
  }
  return undefined;
}

describe("requireAdmin", () => {
  beforeEach(() => {
    useAuthStore.getState().clearUser();
  });

  it("lets an admin through", () => {
    useAuthStore.getState().setUser(userWithRole("admin"));
    expect(() => requireAdmin()).not.toThrow();
  });

  it("sends a member to /projects instead of rendering the page", () => {
    // The sidebar only hides the link. Typing /server used to render the page.
    useAuthStore.getState().setUser(userWithRole("member"));
    const thrown = thrownBy(requireAdmin);
    expect(isRedirect(thrown)).toBe(true);
    if (isRedirect(thrown)) expect(thrown.options.to).toBe("/projects");
  });

  it("treats an unknown user as not an admin", () => {
    // Fail closed: no user in the store is not an admin.
    expect(isRedirect(thrownBy(requireAdmin))).toBe(true);
  });
});
