import { createFileRoute, Link } from "@tanstack/react-router";
import { useServices } from "@/lib/hooks/use-services";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { buttonVariants } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";

export const Route = createFileRoute("/_app/projects/$projectId/services/")({
  component: ServicesPage,
});

function ServicesPage() {
  const { projectId } = Route.useParams();
  const { data: services, isLoading } = useServices(projectId);

  if (isLoading) {
    return <div className="text-muted-foreground">Loading services...</div>;
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h2 className="text-lg font-semibold">Services</h2>
        <Link
          to="/projects/$projectId/services/new"
          params={{ projectId }}
          className={buttonVariants()}
        >
          Deploy Service
        </Link>
      </div>

      {!services || services.length === 0 ? (
        <Card>
          <CardContent className="flex flex-col items-center justify-center py-12">
            <p className="text-muted-foreground mb-4">No services yet.</p>
            <Link
              to="/projects/$projectId/services/new"
              params={{ projectId }}
              className={buttonVariants()}
            >
              Deploy your first service
            </Link>
          </CardContent>
        </Card>
      ) : (
        <div className="grid gap-3 sm:grid-cols-2">
          {services.map((service) => (
            <Link
              key={service.id}
              to="/projects/$projectId/services/$serviceId"
              params={{ projectId, serviceId: service.id }}
            >
              <Card className="hover:bg-muted/50 cursor-pointer transition-colors">
                <CardHeader className="pb-2">
                  <div className="flex items-start justify-between">
                    <CardTitle className="text-base">{service.name}</CardTitle>
                    <Badge
                      variant={
                        service.status === "running" ? "default" : "secondary"
                      }
                    >
                      {service.status}
                    </Badge>
                  </div>
                </CardHeader>
                <CardContent>
                  <div className="text-muted-foreground flex gap-2 text-xs">
                    <Badge variant="outline">{service.type}</Badge>
                    <Badge variant="outline">{service.build_type}</Badge>
                  </div>
                </CardContent>
              </Card>
            </Link>
          ))}
        </div>
      )}
    </div>
  );
}
