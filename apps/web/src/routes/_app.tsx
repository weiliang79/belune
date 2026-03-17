import { createFileRoute, Outlet, redirect } from "@tanstack/react-router";
import { Sidebar } from "@/lib/components/layout/sidebar";
import { useAuthStore } from "@/lib/stores/auth";

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
  return (
    <div className="flex h-screen">
      <Sidebar />
      <main className="flex-1 overflow-y-auto p-6">
        <Outlet />
      </main>
    </div>
  );
}
