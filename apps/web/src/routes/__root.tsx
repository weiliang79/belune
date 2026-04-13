import { createRootRoute, Outlet, redirect, useRouter } from "@tanstack/react-router";
import { TanStackRouterDevtools } from "@tanstack/react-router-devtools";
import { ReactQueryDevtools } from "@tanstack/react-query-devtools";
import { Toaster } from "@/components/ui/sonner";
import { Button } from "@/components/ui/button";
import { checkSetup, getMe } from "@/lib/api/auth";
import { useAuthStore } from "@/lib/stores/auth";
import { ApiError } from "@/lib/api/client";

function RootErrorBoundary({ error }: { error: Error }) {
  const router = useRouter();
  return (
    <div className="flex min-h-screen items-center justify-center">
      <div className="max-w-md space-y-4 text-center">
        <h2 className="text-xl font-semibold">Something went wrong</h2>
        <p className="text-muted-foreground text-sm">
          {error.message || "An unexpected error occurred."}
        </p>
        <div className="flex justify-center gap-2">
          <Button variant="outline" onClick={() => router.invalidate()}>
            Try again
          </Button>
          <Button onClick={() => window.location.reload()}>Reload page</Button>
        </div>
      </div>
    </div>
  );
}

export const Route = createRootRoute({
  errorComponent: RootErrorBoundary,
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
  return (
    <>
      <Outlet />
      <Toaster />
      {import.meta.env.DEV && (
        <>
          <TanStackRouterDevtools position="bottom-right" />
          <ReactQueryDevtools buttonPosition="bottom-left" />
        </>
      )}
    </>
  );
}
