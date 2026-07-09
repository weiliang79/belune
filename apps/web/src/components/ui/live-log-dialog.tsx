import { Loader2Icon } from "lucide-react";
import { BlobLogViewer } from "@/components/logs/blob-log-viewer";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";

// LiveLogDialog shows a run's log and keeps it scrolled to the bottom as it
// grows. When `running` is true the log is still streaming in (the parent polls
// the run and passes the latest log), so it shows a spinner + a waiting hint.
// The log is rendered with per-line level coloring and a level filter.
export function LiveLogDialog({
  open,
  onOpenChange,
  title,
  log,
  running,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  title: string;
  log: string;
  running: boolean;
}) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-3xl">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            {title}
            {running && (
              <span className="text-status-building inline-flex items-center gap-1 text-xs font-normal">
                <Loader2Icon className="size-3 animate-spin" />
                running
              </span>
            )}
          </DialogTitle>
        </DialogHeader>
        <BlobLogViewer
          blob={log}
          running={running}
          emptyMessage="No log recorded for this run."
          heightClass="h-[60vh]"
        />
      </DialogContent>
    </Dialog>
  );
}
