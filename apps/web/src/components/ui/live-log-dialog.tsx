import { useEffect, useRef } from "react";
import { Loader2Icon } from "lucide-react";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";

// LiveLogDialog shows a run's log and keeps it scrolled to the bottom as it
// grows. When `running` is true the log is still streaming in (the parent polls
// the run and passes the latest log), so it shows a spinner + a waiting hint.
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
  const preRef = useRef<HTMLPreElement>(null);
  useEffect(() => {
    const el = preRef.current;
    if (el) el.scrollTop = el.scrollHeight;
  }, [log]);

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-2xl">
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
        <pre
          ref={preRef}
          className="bg-muted/40 max-h-[60vh] overflow-auto rounded-md border p-3 font-mono text-xs whitespace-pre-wrap"
        >
          {log.trim() ||
            (running ? "Waiting for output…" : "No log recorded for this run.")}
        </pre>
      </DialogContent>
    </Dialog>
  );
}
