import { useState, type ReactNode } from "react";
import { createFileRoute, notFound } from "@tanstack/react-router";
import { Check, Plus, Trash2 } from "lucide-react";
import { BeluneLogo } from "@/lib/components/belune-logo";
import { cn } from "@/lib/utils";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Checkbox } from "@/components/ui/checkbox";
import { FieldError } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { LiveIndicator } from "@/components/ui/live-indicator";
import {
  SegmentedControl,
  SegmentedControlItem,
} from "@/components/ui/segmented-control";
import { Separator } from "@/components/ui/separator";
import { Skeleton } from "@/components/ui/skeleton";
import { Sparkline } from "@/components/ui/sparkline";
import { StatusBar } from "@/components/ui/status-bar";
import { StatusPill } from "@/components/ui/status-pill";
import { Switch } from "@/components/ui/switch";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Textarea } from "@/components/ui/textarea";

/**
 * Dev-only theme matrix: every primitive rendered in all four combinations of
 * accent × mode on one page. Exists so a colour decision can be checked in the
 * one combination it usually fails in (emerald + dark, where `--brand-fg` is
 * near-black) without toggling the app back and forth and remembering.
 *
 * The quadrants are real theme scopes (`.light`/`.dark` + `data-accent`), so
 * they render through the same tokens and the same `dark:` utilities as the
 * app — not a copy of the stylesheet. Anything that portals to <body>
 * (Dialog, Select's popup, DropdownMenu, Tooltip, Sonner) escapes its quadrant
 * and takes the page theme, so overlays are deliberately not shown here.
 *
 * Gated in beforeLoad rather than by omission so the URL 404s in a production
 * build; the component itself is code-split by the router plugin and never
 * fetched there.
 */
export const Route = createFileRoute("/dev/theme")({
  beforeLoad: () => {
    if (!import.meta.env.DEV) throw notFound();
  },
  component: ThemeMatrixPage,
});

type Mode = "light" | "dark";
type Accent = "violet" | "emerald";

const QUADRANTS: { mode: Mode; accent: Accent }[] = [
  { mode: "light", accent: "violet" },
  { mode: "dark", accent: "violet" },
  { mode: "light", accent: "emerald" },
  { mode: "dark", accent: "emerald" },
];

function ThemeMatrixPage() {
  return (
    <div className="bg-background text-foreground min-h-screen p-4 md:p-6">
      <header className="mb-4 flex flex-wrap items-baseline justify-between gap-2">
        <h1 className="text-lg font-semibold">Theme matrix</h1>
        <p className="text-muted-foreground text-sm">
          Development only. Overlays (dialogs, menus, selects, toasts) portal
          out of their quadrant and are not shown.
        </p>
      </header>
      <div className="grid gap-4 xl:grid-cols-2">
        {QUADRANTS.map(({ mode, accent }) => (
          <Quadrant key={`${mode}-${accent}`} mode={mode} accent={accent} />
        ))}
      </div>
    </div>
  );
}

function Quadrant({ mode, accent }: { mode: Mode; accent: Accent }) {
  const id = `${mode}-${accent}`;
  return (
    <section
      // The app only ever sets data-accent for emerald (violet is the absence
      // of it), and the `.light`/`.dark` blocks each re-declare the violet
      // brand, so a violet quadrant needs no attribute to override an emerald
      // page.
      className={cn(
        mode,
        "bg-background text-foreground border-border rounded-xl border p-5 shadow-sm",
      )}
      data-accent={accent === "emerald" ? "emerald" : undefined}
      style={{ colorScheme: mode }}
      aria-labelledby={`${id}-title`}
    >
      <div className="mb-4 flex items-center justify-between">
        <h2 id={`${id}-title`} className="font-mono text-sm font-medium">
          {accent} · {mode}
        </h2>
        <span
          aria-hidden="true"
          className="text-brand-fg grid size-8 place-items-center rounded-lg shadow-sm"
          style={{
            background:
              "linear-gradient(140deg, var(--brand), var(--brand-press))",
          }}
        >
          <BeluneLogo className="size-6" />
        </span>
      </div>
      <Showcase id={id} />
    </section>
  );
}

const SPARK = [3, 5, 4, 8, 6, 9, 7, 11, 10, 12];

function Showcase({ id }: { id: string }) {
  const [segment, setSegment] = useState("all");

  return (
    <div className="space-y-5">
      <Row title="Buttons">
        <Button>Deploy</Button>
        <Button variant="secondary">Secondary</Button>
        <Button variant="outline">Outline</Button>
        <Button variant="ghost">Ghost</Button>
        <Button variant="destructive">Destructive</Button>
        <Button variant="destructive-solid">Delete</Button>
        <Button variant="link">Link</Button>
        <Button size="sm">
          <Plus data-icon="inline-start" /> Small
        </Button>
        <Button size="icon" variant="outline" aria-label="Delete">
          <Trash2 />
        </Button>
        <Button disabled>Disabled</Button>
      </Row>

      <Row title="Text on the accent">
        <span className="bg-primary text-primary-foreground rounded-md px-2.5 py-1 text-sm">
          text-brand-fg
        </span>
        {/* Deliberately wrong, kept as the reference for CLAUDE.md's rule: it
            looks fine in three quadrants and fails in emerald · dark. */}
        <span className="bg-primary rounded-md px-2.5 py-1 text-sm text-white">
          text-white ✕
        </span>
        <span className="text-primary text-sm">text-primary on surface</span>
      </Row>

      <Row title="Badges & status">
        <Badge>Default</Badge>
        <Badge variant="secondary">Secondary</Badge>
        <Badge variant="light">Light</Badge>
        <Badge variant="outline">Outline</Badge>
        <Badge variant="destructive">Destructive</Badge>
        <StatusPill status="running" />
        <StatusPill status="building" />
        <StatusPill status="failed" />
        <StatusPill status="stopped" />
        <LiveIndicator active />
        <LiveIndicator active={false} />
      </Row>

      <Row title="Form controls">
        <div className="grid w-full gap-3 sm:grid-cols-2">
          <div className="space-y-2">
            <Label htmlFor={`${id}-name`}>Name</Label>
            <Input id={`${id}-name`} placeholder="my-service" />
          </div>
          <div className="space-y-2">
            <Label htmlFor={`${id}-invalid`}>Port</Label>
            <Input
              id={`${id}-invalid`}
              defaultValue="99999"
              aria-invalid
              aria-describedby={`${id}-invalid-err`}
            />
            <FieldError id={`${id}-invalid-err`}>
              Must be between 1 and 65535.
            </FieldError>
          </div>
          <div className="space-y-2">
            <Label htmlFor={`${id}-disabled`} aria-disabled>
              Disabled
            </Label>
            <Input id={`${id}-disabled`} disabled placeholder="Read only" />
          </div>
          <div className="space-y-2">
            <Label htmlFor={`${id}-notes`}>Notes</Label>
            <Textarea id={`${id}-notes`} rows={2} placeholder="Optional" />
          </div>
        </div>
        <div className="flex flex-wrap items-center gap-5">
          <Label className="gap-2">
            <Checkbox defaultChecked /> Checked
          </Label>
          <Label className="gap-2">
            <Checkbox /> Unchecked
          </Label>
          <Label className="gap-2">
            <Switch defaultChecked /> On
          </Label>
          <Label className="gap-2">
            <Switch /> Off
          </Label>
          <Label className="gap-2" aria-disabled>
            <Switch disabled defaultChecked /> Disabled
          </Label>
        </div>
        <SegmentedControl value={segment} onValueChange={setSegment} size="sm">
          <SegmentedControlItem value="all">All</SegmentedControlItem>
          <SegmentedControlItem value="running">Running</SegmentedControlItem>
          <SegmentedControlItem value="stopped">Stopped</SegmentedControlItem>
        </SegmentedControl>
        <Tabs defaultValue="overview">
          <TabsList>
            <TabsTrigger value="overview">Overview</TabsTrigger>
            <TabsTrigger value="logs">Logs</TabsTrigger>
            <TabsTrigger value="settings">Settings</TabsTrigger>
          </TabsList>
        </Tabs>
      </Row>

      <Row title="Alerts">
        <div className="grid w-full gap-2">
          <Alert>
            <Check />
            <AlertTitle>Deployed</AlertTitle>
            <AlertDescription>web is serving v42.</AlertDescription>
          </Alert>
          <Alert variant="warning">
            <AlertTitle>Certificate expires in 6 days</AlertTitle>
            <AlertDescription>Renewal is scheduled tonight.</AlertDescription>
          </Alert>
          <Alert variant="destructive">
            <AlertTitle>Build failed</AlertTitle>
            <AlertDescription>Exit code 1 at step 4/9.</AlertDescription>
          </Alert>
        </div>
      </Row>

      <Row title="Surfaces">
        <Card className="w-full">
          <CardHeader>
            <CardTitle>Instance</CardTitle>
            <CardDescription>
              Text hierarchy:{" "}
              <span className="text-foreground">foreground</span>,{" "}
              <span className="text-text-muted">muted</span>,{" "}
              <span className="text-text-faint">faint</span>,{" "}
              <span className="text-text-faintest">faintest</span>.
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-3">
            <StatusBar
              segments={[
                { label: "Running", count: 6, className: "bg-status-ready" },
                {
                  label: "Building",
                  count: 2,
                  className: "bg-status-building",
                },
                { label: "Failed", count: 1, className: "bg-status-error" },
              ]}
            />
            <Sparkline values={SPARK} height={32} />
            <Separator />
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Service</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead className="text-right">CPU</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                <TableRow>
                  <TableCell className="font-mono text-xs">web</TableCell>
                  <TableCell>
                    <StatusPill status="running" />
                  </TableCell>
                  <TableCell className="text-right tabular-nums">12%</TableCell>
                </TableRow>
                <TableRow>
                  <TableCell className="font-mono text-xs">worker</TableCell>
                  <TableCell>
                    <StatusPill status="building" />
                  </TableCell>
                  <TableCell className="text-right tabular-nums">—</TableCell>
                </TableRow>
              </TableBody>
            </Table>
            <div className="space-y-2">
              <Skeleton className="h-4 w-1/2" />
              <Skeleton className="h-4 w-1/3" />
            </div>
          </CardContent>
        </Card>
      </Row>
    </div>
  );
}

function Row({ title, children }: { title: string; children: ReactNode }) {
  return (
    <div>
      <p className="text-text-faint mb-2 text-[11px] font-medium tracking-wide uppercase">
        {title}
      </p>
      <div className="flex flex-wrap items-center gap-2">{children}</div>
    </div>
  );
}
