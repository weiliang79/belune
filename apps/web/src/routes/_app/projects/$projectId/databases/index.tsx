import { createFileRoute } from "@tanstack/react-router";
import { Card, CardContent } from "@/components/ui/card";

export const Route = createFileRoute("/_app/projects/$projectId/databases/")({
  component: DatabasesPage,
});

function DatabasesPage() {
  return (
    <div className="space-y-4">
      <h2 className="text-lg font-semibold">Databases</h2>
      <Card>
        <CardContent className="flex flex-col items-center justify-center py-12">
          <p className="text-muted-foreground">
            Database provisioning coming soon.
          </p>
        </CardContent>
      </Card>
    </div>
  );
}
