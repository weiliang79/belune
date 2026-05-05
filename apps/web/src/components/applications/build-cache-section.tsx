import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from "@/components/ui/alert-dialog";
import {
  useBuildCache,
  useClearBuildCache,
} from "@/lib/hooks/use-applications";
import { formatBytes } from "@/lib/utils/format";

interface Props {
  projectId: string;
  applicationId: string;
}

export function BuildCacheSection({ projectId, applicationId }: Props) {
  const { data: buildCache } = useBuildCache(projectId, applicationId);
  const clearBuildCache = useClearBuildCache(projectId, applicationId);

  return (
    <Card>
      <CardHeader>
        <CardTitle>Build Cache</CardTitle>
      </CardHeader>
      <CardContent className="space-y-4">
        <p className="text-muted-foreground text-sm">
          Persistent layer cache reused across builds. Clearing forces the next
          build to start from scratch.
        </p>
        <div className="grid grid-cols-3 gap-4 text-sm">
          <div>
            <div className="text-muted-foreground">Build layers</div>
            <div className="font-mono">
              {buildCache
                ? formatBytes(Math.max(0, buildCache.build_cache_bytes))
                : "—"}
            </div>
          </div>
          <div>
            <div className="text-muted-foreground">Launch layers</div>
            <div className="font-mono">
              {buildCache
                ? formatBytes(Math.max(0, buildCache.launch_cache_bytes))
                : "—"}
            </div>
          </div>
          <div>
            <div className="text-muted-foreground">Total</div>
            <div className="font-mono font-semibold">
              {buildCache
                ? formatBytes(Math.max(0, buildCache.total_bytes))
                : "—"}
            </div>
          </div>
        </div>
        <AlertDialog>
          <AlertDialogTrigger
            render={
              <Button
                variant="outline"
                disabled={!buildCache || buildCache.total_bytes <= 0}
              />
            }
          >
            Clear Build Cache
          </AlertDialogTrigger>
          <AlertDialogContent>
            <AlertDialogHeader>
              <AlertDialogTitle>Clear build cache?</AlertDialogTitle>
              <AlertDialogDescription>
                The next build will re-download and rebuild all layers. This can
                take significantly longer than a cached build.
              </AlertDialogDescription>
            </AlertDialogHeader>
            <AlertDialogFooter>
              <AlertDialogCancel>Cancel</AlertDialogCancel>
              <AlertDialogAction
                onClick={() => {
                  toast.promise(clearBuildCache.mutateAsync(), {
                    loading: "Clearing cache...",
                    success: "Build cache cleared",
                    error: (err) => err.message,
                  });
                }}
              >
                Clear
              </AlertDialogAction>
            </AlertDialogFooter>
          </AlertDialogContent>
        </AlertDialog>
      </CardContent>
    </Card>
  );
}
