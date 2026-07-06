import { createFileRoute, useNavigate } from "@tanstack/react-router";
import { SiDocker } from "@icons-pack/react-simple-icons";
import { RouteError } from "@/lib/components/route-error";
import { PageHeader } from "@/components/ui/page-header";
import { useAuthStore } from "@/lib/stores/auth";
import { cn } from "@/lib/utils";
import { DockerOverviewTab } from "@/components/docker/overview-tab";
import { DockerContainersTab } from "@/components/docker/containers-tab";
import { DockerImagesTab } from "@/components/docker/images-tab";
import { DockerVolumesTab } from "@/components/docker/volumes-tab";
import { DockerNetworksTab } from "@/components/docker/networks-tab";

type DockerTab = "overview" | "containers" | "images" | "volumes" | "networks";

const TABS: { value: DockerTab; label: string }[] = [
  { value: "overview", label: "Overview" },
  { value: "containers", label: "Containers" },
  { value: "images", label: "Images" },
  { value: "volumes", label: "Volumes" },
  { value: "networks", label: "Networks" },
];

function isTab(v: unknown): v is Exclude<DockerTab, "overview"> {
  return v === "containers" || v === "images" || v === "volumes" || v === "networks";
}

export const Route = createFileRoute("/_app/docker")({
  component: DockerPage,
  errorComponent: RouteError,
  validateSearch: (search: Record<string, unknown>) => ({
    tab: isTab(search.tab) ? search.tab : undefined,
  }),
});

function DockerPage() {
  const { tab } = Route.useSearch();
  const navigate = useNavigate({ from: "/docker" });
  const isAdmin = useAuthStore((s) => s.user?.role === "admin");

  const activeTab: DockerTab = isTab(tab) ? tab : "overview";
  const setTab = (next: DockerTab) =>
    navigate({ search: () => ({ tab: next === "overview" ? undefined : next }) });

  return (
    <div className="space-y-6">
      <PageHeader
        icon={<SiDocker className="size-5" />}
        title="Docker"
        description="Read-only view of containers, images, volumes, and networks on this host."
      />

      {!isAdmin ? (
        <p className="text-muted-foreground text-sm">
          Docker inspection is restricted to administrators.
        </p>
      ) : (
        <>
          <nav className="flex gap-1 overflow-x-auto border-b">
            {TABS.map((t) => (
              <button
                key={t.value}
                type="button"
                onClick={() => setTab(t.value)}
                className={cn(
                  "border-b-2 px-4 py-2 text-sm font-medium whitespace-nowrap transition-colors",
                  activeTab === t.value
                    ? "border-primary text-foreground"
                    : "text-muted-foreground hover:text-foreground border-transparent",
                )}
              >
                {t.label}
              </button>
            ))}
          </nav>

          {activeTab === "overview" && (
            <DockerOverviewTab enabled={activeTab === "overview"} />
          )}
          {activeTab === "containers" && (
            <DockerContainersTab enabled={activeTab === "containers"} />
          )}
          {activeTab === "images" && (
            <DockerImagesTab enabled={activeTab === "images"} />
          )}
          {activeTab === "volumes" && (
            <DockerVolumesTab enabled={activeTab === "volumes"} />
          )}
          {activeTab === "networks" && (
            <DockerNetworksTab enabled={activeTab === "networks"} />
          )}
        </>
      )}
    </div>
  );
}
