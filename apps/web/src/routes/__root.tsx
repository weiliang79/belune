import { createRootRoute, Outlet, redirect } from "@tanstack/react-router";
import { TanStackRouterDevtools } from "@tanstack/react-router-devtools";
import { ReactQueryDevtools } from "@tanstack/react-query-devtools";
import { ThemeProvider } from "next-themes";
import { Toaster } from "@/components/ui/sonner";
import { checkSetup, getMe } from "@/lib/api/auth";
import { useAuthStore } from "@/lib/stores/auth";
import { useAccentSync } from "@/lib/stores/accent";
import { ApiError } from "@/lib/api/client";
import { RootErrorBoundary, NotFoundPage } from "@/lib/components/status-pages";
import { ProgressProvider } from "@bprogress/react";
import { RouteProgress } from "@/lib/components/route-progress";

export const Route = createRootRoute({
  errorComponent: RootErrorBoundary,
  notFoundComponent: NotFoundPage,
  beforeLoad: async ({ location }) => {
    // Skip auth checks for login/setup pages
    if (location.pathname === "/login" || location.pathname === "/setup") {
      return;
    }

    // Check if setup is required
    try {
      const { setup_required } = await checkSetup();
      if (setup_required) {
        throw redirect({ to: "/setup" });
      }
    } catch (e) {
      if (e instanceof ApiError) {
        // API not reachable, continue to render
        return;
      }
      throw e;
    }

    // Check if user is authenticated
    try {
      const user = await getMe();
      useAuthStore.getState().setUser(user);
    } catch (e) {
      if (e instanceof ApiError && e.status === 401) {
        throw redirect({ to: "/login", search: { redirect: location.href } });
      }
      // Don't swallow non-401 failures (429, 5xx, network). Proceeding would
      // render a half-authed shell that _app then bounces to /login, hiding the
      // real error; surface it through the root error boundary instead.
      throw e;
    }
  },
  component: RootLayout,
});

function RootLayout() {
  useAccentSync();
  return (
    <ThemeProvider
      attribute="class"
      defaultTheme="dark"
      enableSystem
      disableTransitionOnChange
    >
      <ProgressProvider
        color="var(--brand)"
        height="2px"
        options={{ showSpinner: false }}
      >
        <RouteProgress />
        <Outlet />
        <Toaster />
        {import.meta.env.DEV && (
          <>
            <TanStackRouterDevtools position="bottom-right" />
            <ReactQueryDevtools buttonPosition="bottom-left" />
          </>
        )}
      </ProgressProvider>
    </ThemeProvider>
  );
}
