import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Label } from "@/components/ui/label";
import { useProjects } from "@/lib/hooks/use-projects";

interface ProjectPinSelectProps {
  value: string[];
  onValueChange: (value: string[]) => void;
  label?: string;
  helperText?: string;
}

/**
 * The project-pinning multi-select for a personal access token: empty means
 * unpinned (every project the owner can reach), one or more ids narrows it.
 * Extracted after the same Select+multiple block turned up independently in
 * the general Create Token dialog and the Connect-an-AI-Assistant dialog —
 * same picker, same "N projects" trigger summary, different surrounding
 * copy.
 */
export function ProjectPinSelect({
  value,
  onValueChange,
  label = "Projects",
  helperText = "Narrows the token to exactly these projects instead of everything you can reach. Leave empty for every project.",
}: ProjectPinSelectProps) {
  const { data: projects } = useProjects();

  return (
    <div className="space-y-2">
      <Label>{label}</Label>
      <Select multiple value={value} onValueChange={onValueChange}>
        <SelectTrigger>
          <SelectValue>
            {(v: string[]) => {
              if (!v || v.length === 0) return "All projects";
              if (v.length === 1) {
                return (
                  (projects ?? []).find((p) => p.id === v[0])?.name ??
                  "1 project"
                );
              }
              return `${v.length} projects`;
            }}
          </SelectValue>
        </SelectTrigger>
        <SelectContent>
          {(projects ?? []).map((p) => (
            <SelectItem key={p.id} value={p.id}>
              {p.name}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
      <p className="text-muted-foreground text-xs">{helperText}</p>
    </div>
  );
}
