import { useEffect, useState } from "react";
import { SearchIcon } from "lucide-react";

import { cn } from "@/lib/utils";
import { Input } from "@/components/ui/input";
import { useDebouncedValue } from "@/lib/hooks/use-debounced-value";

interface DataTableSearchProps {
  value: string;
  onChange: (value: string) => void;
  placeholder?: string;
  className?: string;
  /** Debounce before emitting onChange; 0 emits immediately. */
  delay?: number;
}

/**
 * Debounced search box for DataTable's client-side global filter. Shows typed
 * text immediately but only emits `onChange` after `delay`ms idle.
 */
export function DataTableSearch({
  value,
  onChange,
  placeholder = "Search…",
  className,
  delay = 250,
}: DataTableSearchProps) {
  const [raw, setRaw] = useState(value);
  const debounced = useDebouncedValue(raw, delay);

  // Emit upstream when the debounced local value diverges from the prop.
  useEffect(() => {
    if (debounced !== value) onChange(debounced);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [debounced]);

  // Keep local text in sync when the value is reset externally (e.g. clear).
  useEffect(() => {
    setRaw(value);
  }, [value]);

  return (
    <div className={cn("relative", className)}>
      <SearchIcon
        aria-hidden="true"
        className="text-text-faint pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2"
      />
      <Input
        value={raw}
        onChange={(e) => setRaw(e.target.value)}
        placeholder={placeholder}
        className="pl-9"
      />
    </div>
  );
}
