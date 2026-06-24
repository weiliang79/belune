import type { Table as TableInstance } from "@tanstack/react-table";

import { Button } from "@/components/ui/button";
import type { DataTablePaginationConfig } from "@/components/ui/data-table/data-table";

interface DataTablePaginationProps<TData> {
  table: TableInstance<TData>;
  pagination: DataTablePaginationConfig;
  /** Rows on the current page (for the manual-mode range label). */
  rowCount: number;
}

export function DataTablePagination<TData>({
  table,
  pagination,
  rowCount,
}: DataTablePaginationProps<TData>) {
  let canPrev: boolean;
  let canNext: boolean;
  let onPrev: () => void;
  let onNext: () => void;
  let label: string;

  if (pagination.mode === "manual") {
    const { offset, pageSize, hasMore, onOffsetChange } = pagination;
    canPrev = offset > 0;
    canNext = hasMore;
    onPrev = () => onOffsetChange(Math.max(0, offset - pageSize));
    onNext = () => onOffsetChange(offset + pageSize);
    label = rowCount > 0 ? `${offset + 1}–${offset + rowCount}` : "0";
    // Hide the footer entirely when there is only a single, full-or-partial page.
    if (!canPrev && !canNext) return null;
  } else {
    const { pageIndex, pageSize } = table.getState().pagination;
    const total = table.getFilteredRowModel().rows.length;
    canPrev = table.getCanPreviousPage();
    canNext = table.getCanNextPage();
    onPrev = () => table.previousPage();
    onNext = () => table.nextPage();
    const start = total === 0 ? 0 : pageIndex * pageSize + 1;
    const end = Math.min(total, (pageIndex + 1) * pageSize);
    label = `${start}–${end} of ${total}`;
    if (total <= pageSize) return null;
  }

  return (
    <div className="flex items-center justify-between">
      <Button variant="outline" size="sm" disabled={!canPrev} onClick={onPrev}>
        Previous
      </Button>
      <span className="text-muted-foreground text-sm">{label}</span>
      <Button variant="outline" size="sm" disabled={!canNext} onClick={onNext}>
        Next
      </Button>
    </div>
  );
}
