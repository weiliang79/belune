import type { ColumnDef, DisplayColumnDef } from "@tanstack/react-table";

type ActionColumnDef<TData, TValue> = Omit<
  DisplayColumnDef<TData, TValue>,
  "id"
> & {
  id?: string;
};

/**
 * Helper for a trailing "Actions" column. Defaults to a non-sortable display
 * column with a sensible width; callers supply the `cell` (buttons/menu).
 */
export function buildActionColumnDef<TData, TValue = unknown>(
  def: ActionColumnDef<TData, TValue>,
): ColumnDef<TData, TValue> {
  return {
    id: "actions",
    header: "",
    enableSorting: false,
    enableGlobalFilter: false,
    ...def,
  };
}
