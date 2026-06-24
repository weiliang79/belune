import { useEffect, useState } from "react";

/**
 * Returns a debounced copy of `value` that only updates after `delay`ms of
 * no changes. Used by the DataTable search input so client-side global
 * filtering doesn't run on every keystroke.
 */
export function useDebouncedValue<T>(value: T, delay = 250): T {
  const [debounced, setDebounced] = useState(value);

  useEffect(() => {
    const id = setTimeout(() => setDebounced(value), delay);
    return () => clearTimeout(id);
  }, [value, delay]);

  return debounced;
}
