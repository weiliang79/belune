import { createFileRoute, redirect } from "@tanstack/react-router";

// Dashboard removed in v0.0.3. Redirect old bookmarks to Projects.
export const Route = createFileRoute("/_app/dashboard")({
  beforeLoad: () => {
    throw redirect({ to: "/projects" });
  },
});
