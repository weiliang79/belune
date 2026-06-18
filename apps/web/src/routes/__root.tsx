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
        throw redirect({ to: "/login" });
      }
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
      <Outlet />
      <Toaster />
      {import.meta.env.DEV && (
        <>
          <TanStackRouterDevtools position="bottom-right" />
          <ReactQueryDevtools buttonPosition="bottom-left" />
        </>
      )}
    </ThemeProvider>
  );
}
