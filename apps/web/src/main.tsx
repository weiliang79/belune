import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { RouterProvider, createRouter } from "@tanstack/react-router";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { routeTree } from "./routeTree.gen";
import { initFavicon } from "./lib/favicon";
import { RouteSkeleton } from "./lib/components/route-skeleton";
import "./index.css";

// Repaint the favicon from the live brand color (theme + accent aware).
initFavicon();

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 1000 * 60,
      retry: 1,
    },
  },
});

const router = createRouter({
  routeTree,
  defaultPendingComponent: RouteSkeleton,
  // Matches route-progress.tsx's own 150ms start delay, so the bar and the
  // skeleton appear together instead of one covering for the other.
  defaultPendingMs: 150,
  defaultPendingMinMs: 300,
});

declare module "@tanstack/react-router" {
  interface Register {
    router: typeof router;
  }
}

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      <RouterProvider router={router} />
    </QueryClientProvider>
  </StrictMode>,
);
