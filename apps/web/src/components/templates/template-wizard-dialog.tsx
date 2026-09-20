import { useState } from "react";
import { useNavigate } from "@tanstack/react-router";
import { useForm } from "@tanstack/react-form";
import {
  ArrowRight,
  BookOpen,
  CheckCircle2,
  Database,
  Globe,
  Loader2,
  Package,
  Tag,
} from "lucide-react";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { TemplateLogo } from "@/components/templates/template-logo";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  SegmentedControl,
  SegmentedControlItem,
} from "@/components/ui/segmented-control";
import { useTemplate, useInstantiateTemplate } from "@/lib/hooks/use-templates";
import { useProjects } from "@/lib/hooks/use-projects";
import { ApiError } from "@/lib/api/client";
import type { TemplateSummary } from "@/lib/api/templates";

interface Props {
  template: TemplateSummary | null;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

type Target = "new" | "existing";

function fieldError(errors: unknown[]): string | undefined {
  const first = errors[0];
  if (!first) return undefined;
  return typeof first === "string"
    ? first
    : (first as { message?: string }).message;
}

export function TemplateWizardDialog({ template, open, onOpenChange }: Props) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="flex max-h-[85vh] flex-col gap-0 overflow-hidden p-0 sm:max-w-lg">
        {/* Remount per open/template so the wizard resets without an effect. */}
        {open && template && (
          <WizardBody
            key={template.id}
            template={template}
            onOpenChange={onOpenChange}
          />
        )}
      </DialogContent>
    </Dialog>
  );
}

function WizardBody({
  template,
  onOpenChange,
}: {
  template: TemplateSummary;
  onOpenChange: (open: boolean) => void;
}) {
  const navigate = useNavigate();
  const { data: detail, isLoading } = useTemplate(template.id, true);
  const { data: projects } = useProjects();
  const instantiate = useInstantiateTemplate();

  const [inputs, setInputs] = useState<Record<string, string>>({});
  const [error, setError] = useState("");
  const [done, setDone] = useState<{ projectId: string; notes?: string } | null>(
    null,
  );

  // Input values fall back to the manifest default for display; the backend
  // applies the same default server-side, so we never need to seed state.
  const inputValue = (key: string, def?: string) => inputs[key] ?? def ?? "";

  const needsHostname = detail?.needs_hostname ?? false;
  const hasInputs = (detail?.inputs?.length ?? 0) > 0;

  const form = useForm({
    defaultValues: {
      target: "new" as Target,
      projectName: template.name ?? "",
      existingProjectId: "",
      hostname: "",
    },
    onSubmit: async ({ value }) => {
      setError("");
      try {
        const res = await instantiate.mutateAsync({
          id: template.id,
          data: {
            project_id:
              value.target === "existing" ? value.existingProjectId : undefined,
            new_project_name:
              value.target === "new" ? value.projectName.trim() : undefined,
            hostname: value.hostname.trim() || undefined,
            inputs,
          },
        });
        setDone({ projectId: res.project_id, notes: res.notes });
      } catch (e) {
        setError(e instanceof ApiError ? e.message : "Failed to create from template");
      }
    },
  });

  const goToProject = () => {
    if (!done) return;
    onOpenChange(false);
    navigate({ to: "/projects/$projectId", params: { projectId: done.projectId } });
  };

  if (done) {
    return (
      <>
        <DialogHeader className="shrink-0 p-4 pb-2">
          <DialogTitle className="flex items-center gap-2">
            <CheckCircle2 aria-hidden="true" className="size-5 text-success" />
            {template.name} is being created
          </DialogTitle>
          <DialogDescription>
            Databases provision first, then the app deploys automatically. Follow
            progress on the project page.
          </DialogDescription>
        </DialogHeader>
        <div className="flex-1 overflow-y-auto px-4 pb-4">
          {done.notes && (
            <div className="rounded-lg border bg-elev/50 p-3">
              <p className="mb-1 text-xs font-semibold uppercase tracking-wide text-text-faint">
                Next steps
              </p>
              <p className="whitespace-pre-wrap text-sm leading-relaxed">
                {done.notes}
              </p>
            </div>
          )}
        </div>
        <DialogFooter className="m-0 shrink-0">
          <Button onClick={goToProject}>Go to project</Button>
        </DialogFooter>
      </>
    );
  }

  return (
    <form
      onSubmit={(e) => {
        e.preventDefault();
        e.stopPropagation();
        form.handleSubmit();
      }}
      // display: contents so DialogHeader/content/DialogFooter stay direct flex
      // children of DialogContent -- wrapping them in a block-level <form>
      // would break the flex-1/overflow-y-auto scroll area below.
      className="contents"
    >
      <DialogHeader className="flex-row items-center gap-3 shrink-0 p-4 pb-3">
        <TemplateLogo logoUrl={template.logo_url} />
        <div className="min-w-0 flex-1 space-y-1">
          <DialogTitle className="flex items-center gap-2">
            Deploy {template.name}
            {template.version && (
              <Badge variant="outline" className="gap-1 font-normal">
                <Tag aria-hidden="true" className="size-3" />
                {template.version}
              </Badge>
            )}
          </DialogTitle>
          <DialogDescription>{template.description}</DialogDescription>
        </div>
      </DialogHeader>

      {isLoading || !detail ? (
        <div className="flex-1 space-y-3 overflow-y-auto px-4 pb-4">
          <Skeleton className="h-10 w-full" />
          <Skeleton className="h-10 w-full" />
        </div>
      ) : (
        <div className="flex-1 space-y-5 overflow-y-auto px-4 pb-4">
          {/* Target project */}
          <form.Field name="target">
            {(targetField) => (
              <div className="space-y-2">
                <Label>Where should this go?</Label>
                <SegmentedControl
                  size="sm"
                  value={targetField.state.value}
                  onValueChange={(v) => targetField.handleChange(v as Target)}
                >
                  <SegmentedControlItem value="new">New project</SegmentedControlItem>
                  <SegmentedControlItem value="existing">
                    Existing project
                  </SegmentedControlItem>
                </SegmentedControl>
                {targetField.state.value === "new" ? (
                  <form.Field
                    name="projectName"
                    validators={{
                      onChange: ({ value }) =>
                        value.trim() === "" ? "Project name is required" : undefined,
                    }}
                    children={(field) => {
                      const error = fieldError(field.state.meta.errors);
                      return (
                        <>
                          <Input
                            value={field.state.value}
                            onBlur={field.handleBlur}
                            onChange={(e) => field.handleChange(e.target.value)}
                            placeholder="Project Name"
                            aria-label="New project name"
                          />
                          {error && (
                            <p className="text-destructive text-xs">{error}</p>
                          )}
                        </>
                      );
                    }}
                  />
                ) : (
                  <form.Field
                    name="existingProjectId"
                    validators={{
                      onChange: ({ value }) =>
                        value === "" ? "Select a project" : undefined,
                    }}
                    children={(field) => {
                      const error = fieldError(field.state.meta.errors);
                      return (
                        <>
                          <Select
                            value={field.state.value}
                            onValueChange={(v) => field.handleChange(v ?? "")}
                          >
                            <SelectTrigger className="h-8">
                              <SelectValue placeholder="Select a Project" />
                            </SelectTrigger>
                            <SelectContent>
                              {(projects ?? []).map((p) => (
                                <SelectItem key={p.id} value={p.id}>
                                  {p.name}
                                </SelectItem>
                              ))}
                            </SelectContent>
                          </Select>
                          {error && (
                            <p className="text-destructive text-xs">{error}</p>
                          )}
                        </>
                      );
                    }}
                  />
                )}
              </div>
            )}
          </form.Field>

          {/* Hostname */}
          <form.Field
            name="hostname"
            validators={{
              onChange: ({ value }) =>
                needsHostname && value.trim() === ""
                  ? "This app needs a hostname"
                  : undefined,
            }}
            children={(field) => {
              const error = fieldError(field.state.meta.errors);
              return (
                <div className="space-y-2">
                  <Label htmlFor="tpl-hostname">
                    Hostname{" "}
                    {needsHostname ? (
                      <span className="text-destructive">*</span>
                    ) : (
                      <span className="text-text-faint font-normal">(optional)</span>
                    )}
                  </Label>
                  <Input
                    id="tpl-hostname"
                    value={field.state.value}
                    onBlur={field.handleBlur}
                    onChange={(e) => field.handleChange(e.target.value)}
                    placeholder="app.example.com"
                  />
                  {error ? (
                    <p className="text-destructive text-xs">{error}</p>
                  ) : (
                    <p className="text-xs text-text-faint">
                      {needsHostname
                        ? "This app needs its public URL to work correctly."
                        : "Add one now to get a routed URL, or configure it later."}
                    </p>
                  )}
                </div>
              );
            }}
          />

          {/* Inputs */}
          {hasInputs && (
            <div className="space-y-3">
              {detail.inputs!.map((i) => (
                <div key={i.key} className="space-y-1.5">
                  <Label htmlFor={`tpl-input-${i.key}`}>
                    {i.label}
                    {i.required && <span className="text-destructive"> *</span>}
                  </Label>
                  <Input
                    id={`tpl-input-${i.key}`}
                    value={inputValue(i.key, i.default)}
                    onChange={(e) =>
                      setInputs((prev) => ({ ...prev, [i.key]: e.target.value }))
                    }
                    type={i.validation === "email" ? "email" : "text"}
                  />
                  {i.description && (
                    <p className="text-xs text-text-faint">{i.description}</p>
                  )}
                </div>
              ))}
            </div>
          )}

          {/* Summary */}
          <form.Subscribe
            selector={(s) => s.values.hostname}
            children={(hostname) => (
              <div className="flex flex-wrap gap-x-4 gap-y-1 rounded-lg border bg-elev/40 px-3 py-2 text-xs text-muted-foreground">
                <span className="flex items-center gap-1.5">
                  <Package aria-hidden="true" className="size-3.5" />
                  {detail.services} app{detail.services === 1 ? "" : "s"}
                </span>
                {detail.databases > 0 && (
                  <span className="flex items-center gap-1.5">
                    <Database aria-hidden="true" className="size-3.5" />
                    {detail.databases} database{detail.databases === 1 ? "" : "s"}
                  </span>
                )}
                {hostname.trim() && (
                  <span className="flex items-center gap-1.5">
                    <Globe aria-hidden="true" className="size-3.5" />
                    {hostname.trim()}
                  </span>
                )}
              </div>
            )}
          />

          {error && (
            <Alert variant="destructive">
              <AlertTitle>Could not deploy</AlertTitle>
              <AlertDescription>{error}</AlertDescription>
            </Alert>
          )}
        </div>
      )}

      <DialogFooter className="m-0 shrink-0 sm:items-center sm:justify-between">
        <div className="text-text-faint flex items-center gap-4 text-sm">
          {template.docs_url && (
            <a
              href={template.docs_url}
              target="_blank"
              rel="noopener noreferrer"
              className="hover:text-foreground inline-flex items-center gap-1 transition-colors"
            >
              <BookOpen aria-hidden="true" className="size-3.5" />
              Docs
            </a>
          )}
          {template.website && (
            <a
              href={template.website}
              target="_blank"
              rel="noopener noreferrer"
              className="hover:text-foreground inline-flex items-center gap-1 transition-colors"
            >
              <Globe aria-hidden="true" className="size-3.5" />
              Website
            </a>
          )}
        </div>
        <div className="flex gap-2">
          <Button type="button" variant="ghost" onClick={() => onOpenChange(false)}>
            Cancel
          </Button>
          <form.Subscribe
            selector={(s) => [
              s.values.target,
              s.values.projectName,
              s.values.existingProjectId,
              s.values.hostname,
            ] as const}
            children={([target, projectName, existingProjectId, hostname]) => {
              let canSubmit = !!detail;
              if (target === "new" && projectName.trim() === "") canSubmit = false;
              if (target === "existing" && existingProjectId === "") canSubmit = false;
              if (needsHostname && hostname.trim() === "") canSubmit = false;
              for (const i of detail?.inputs ?? []) {
                if (i.required && inputValue(i.key, i.default).trim() === "") {
                  canSubmit = false;
                }
              }
              return (
                <Button
                  type="submit"
                  disabled={!canSubmit || instantiate.isPending}
                >
                  {instantiate.isPending ? (
                    <>
                      <Loader2 aria-hidden="true" className="size-4 animate-spin" />
                      Creating…
                    </>
                  ) : (
                    <>
                      Deploy
                      <ArrowRight aria-hidden="true" className="size-4" />
                    </>
                  )}
                </Button>
              );
            }}
          />
        </div>
      </DialogFooter>
    </form>
  );
}
