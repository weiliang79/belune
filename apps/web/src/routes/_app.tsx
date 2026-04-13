import { createFileRoute, Outlet, redirect } from "@tanstack/react-router";
import { Sidebar } from "@/lib/components/layout/sidebar";
import { useAuthStore } from "@/lib/stores/auth";
import { useWebSocketStatus } from "@/lib/hooks/use-websocket";

export const Route = createFileRoute("/_app")({
  beforeLoad: () => {
    const { isAuthenticated } = useAuthStore.getState();
    if (!isAuthenticated) {
      throw redirect({ to: "/login" });
    }
  },
  component: AppLayout,
});

function AppLayout() {
  const wsStatus = useWebSocketStatus();

  return (
    <div className="flex h-screen flex-col">
      {wsStatus === "failed" && (
        <div className="flex items-center justify-center gap-3 border-b border-destructive/20 bg-destructive/10 px-4 py-2 text-sm text-destructive">
          <span>Real-time connection lost. Live updates are unavailable.</span>
          <button
            className="underline underline-offset-2"
            onClick={() => window.location.reload()}
          >
            Reload
          </button>
        </div>
      )}
      <div className="flex min-h-0 flex-1">
        <Sidebar />
        <main className="flex-1 overflow-y-auto p-6">
          <Outlet />
        </main>
      </div>
    </div>
  );
}
