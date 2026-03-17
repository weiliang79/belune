import { createFileRoute } from "@tanstack/react-router";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { SettingsNav } from "@/lib/components/settings-nav";

export const Route = createFileRoute("/_app/settings/team")({
  component: TeamSettingsPage,
});

function TeamSettingsPage() {
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
          <CardTitle>Team Members</CardTitle>
        </CardHeader>
        <CardContent className="text-muted-foreground py-8 text-center text-sm">
          Team management coming soon.
        </CardContent>
      </Card>
    </div>
  );
}
