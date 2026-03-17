import { createFileRoute } from "@tanstack/react-router";
import { Card, CardContent } from "@/components/ui/card";

export const Route = createFileRoute(
  "/_app/projects/$projectId/databases/$databaseId",
)({
  component: DatabaseDetailPage,
});

function DatabaseDetailPage() {
  return (
    <Card>
      <CardContent className="text-muted-foreground py-12 text-center">
        Database detail view coming soon.
      </CardContent>
    </Card>
  );
}
