import { Globe } from "lucide-react";
import { Card, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";

export function DomainEmptyState({ onAdd }: { onAdd: () => void }) {
  return (
    <Card>
      <CardContent className="flex flex-col items-center gap-3 py-10 text-center">
        <div className="bg-muted flex h-12 w-12 items-center justify-center rounded-full">
          <Globe className="text-muted-foreground h-6 w-6" />
        </div>
        <div className="space-y-1">
          <p className="text-sm font-medium">No domains configured</p>
          <p className="text-muted-foreground text-xs">
            Point a domain at this application to serve traffic over HTTPS.
          </p>
        </div>
        <Button size="sm" onClick={onAdd}>
          Add your first domain
        </Button>
      </CardContent>
    </Card>
  );
}
