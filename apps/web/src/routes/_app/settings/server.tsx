import { createFileRoute } from "@tanstack/react-router";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { SettingsNav } from "@/lib/components/settings-nav";

export const Route = createFileRoute("/_app/settings/server")({
  component: ServerSettingsPage,
});

function ServerSettingsPage() {
  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold">Settings</h1>
        <p className="text-muted-foreground">
          Manage your account and platform settings.
        </p>
      </div>

      <SettingsNav />

      <Card>
        <CardHeader>
          <CardTitle>Server Info</CardTitle>
        </CardHeader>
        <CardContent className="text-muted-foreground py-8 text-center text-sm">
          Server monitoring and metrics coming soon.
        </CardContent>
      </Card>
    </div>
  );
}
