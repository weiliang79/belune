import { useMemo, useState } from "react";
import { createFileRoute } from "@tanstack/react-router";
import {
  BookOpen,
  Database,
  Globe,
  LayoutTemplate,
  Package2,
  SearchIcon,
  Tag,
} from "lucide-react";
import { useTemplates } from "@/lib/hooks/use-templates";
import { TemplateWizardDialog } from "@/components/templates/template-wizard-dialog";
import { TemplateLogo } from "@/components/templates/template-logo";
import { Card } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Separator } from "@/components/ui/separator";
import { PageHeader } from "@/components/ui/page-header";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import { cn } from "@/lib/utils";
import type { TemplateSummary } from "@/lib/api/templates";

export const Route = createFileRoute("/_app/templates")({
  component: TemplatesPage,
});

function TemplateCard({
  template,
  onSelect,
}: {
  template: TemplateSummary;
  onSelect: () => void;
}) {
  const hasLinks = !!(template.website || template.docs_url);
  return (
    <Card className="hover:ring-border-strong h-full gap-0 py-0 transition-shadow">
      <button
        type="button"
        onClick={onSelect}
        className="hover:bg-card-hover focus-visible:ring-ring flex flex-1 flex-col gap-3 rounded-t-xl p-4 text-left outline-none transition-colors focus-visible:ring-2 focus-visible:ring-inset"
      >
        <div className="flex items-start justify-between gap-2">
          <div className="flex min-w-0 items-center gap-3">
            <TemplateLogo logoUrl={template.logo_url} />
            <div className="min-w-0">
              <h3 className="group-hover/card:text-primary truncate text-sm font-semibold transition-colors">
                {template.name}
              </h3>
              <span className="text-text-faint text-xs capitalize">
                {template.category}
              </span>
            </div>
          </div>
          {template.version && (
            <Badge
              variant="outline"
              className="shrink-0 gap-1 font-normal"
            >
              <Tag aria-hidden="true" className="size-3" />
              {template.version}
            </Badge>
          )}
        </div>

        <p className="text-muted-foreground line-clamp-2 text-sm">
          {template.description}
        </p>

        {template.tags && template.tags.length > 0 && (
          <div className="flex flex-wrap gap-1">
            {template.tags.map((tag) => (
              <Badge key={tag} variant="secondary" className="font-normal">
                {tag}
              </Badge>
            ))}
          </div>
        )}

        <div className="mt-auto flex flex-wrap gap-1.5 pt-1">
          <Badge variant="outline" className="gap-1 font-normal">
            <Package2 aria-hidden="true" className="size-3" />
            {template.services} app{template.services === 1 ? "" : "s"}
          </Badge>
          {template.databases > 0 && (
            <Badge variant="outline" className="gap-1 font-normal">
              <Database aria-hidden="true" className="size-3" />
              {template.databases} database{template.databases === 1 ? "" : "s"}
            </Badge>
          )}
        </div>
      </button>

      {hasLinks && (
        <>
          <Separator />
          <div className="text-text-faint flex items-center gap-4 px-4 py-2.5 text-xs">
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
          </div>
        </>
      )}
    </Card>
  );
}

function TemplatesPage() {
  const { data: templates, isLoading } = useTemplates();
  const [search, setSearch] = useState("");
  const [category, setCategory] = useState<string | null>(null);
  const [selected, setSelected] = useState<TemplateSummary | null>(null);
  const [wizardOpen, setWizardOpen] = useState(false);

  const categories = useMemo(() => {
    const set = new Set<string>();
    for (const t of templates ?? []) set.add(t.category);
    return Array.from(set).sort();
  }, [templates]);

  const filtered = useMemo(() => {
    const q = search.trim().toLowerCase();
    return (templates ?? []).filter((t) => {
      if (category && t.category !== category) return false;
      if (!q) return true;
      return (
        t.name.toLowerCase().includes(q) ||
        t.description.toLowerCase().includes(q) ||
        (t.tags ?? []).some((tag) => tag.toLowerCase().includes(q))
      );
    });
  }, [templates, search, category]);

  const openWizard = (t: TemplateSummary) => {
    setSelected(t);
    setWizardOpen(true);
  };

  return (
    <div className="space-y-6">
      <PageHeader
        icon={<LayoutTemplate aria-hidden="true" className="size-5" />}
        title="Templates"
        description="Deploy popular self-hosted apps in one click — with managed databases, backups, and TLS built in."
      />

      <div className="flex flex-col gap-3 sm:flex-row sm:items-center">
        <div className="relative sm:max-w-xs">
          <SearchIcon
            aria-hidden="true"
            className="text-text-faint pointer-events-none absolute left-2.5 top-1/2 size-4 -translate-y-1/2"
          />
          <Input
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="Search templates…"
            className="pl-8"
            aria-label="Search templates"
          />
        </div>
        {categories.length > 1 && (
          <div className="flex flex-wrap gap-1.5">
            <CategoryChip
              label="All"
              active={category === null}
              onClick={() => setCategory(null)}
            />
            {categories.map((c) => (
              <CategoryChip
                key={c}
                label={c}
                active={category === c}
                onClick={() => setCategory(c)}
              />
            ))}
          </div>
        )}
      </div>

      {isLoading ? (
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {Array.from({ length: 6 }).map((_, i) => (
            <Skeleton key={i} className="h-32 rounded-xl" />
          ))}
        </div>
      ) : filtered.length === 0 ? (
        <p className="text-muted-foreground py-12 text-center text-sm">
          No templates match your search.
        </p>
      ) : (
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {filtered.map((t) => (
            <TemplateCard key={t.id} template={t} onSelect={() => openWizard(t)} />
          ))}
        </div>
      )}

      <TemplateWizardDialog
        template={selected}
        open={wizardOpen}
        onOpenChange={setWizardOpen}
      />
    </div>
  );
}

function CategoryChip({
  label,
  active,
  onClick,
}: {
  label: string;
  active: boolean;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={cn(
        "rounded-full border px-3 py-1 text-xs font-medium capitalize transition-colors",
        active
          ? "bg-primary text-primary-foreground border-primary"
          : "text-muted-foreground hover:bg-elev border-transparent",
      )}
    >
      {label}
    </button>
  );
}
