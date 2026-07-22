import { useQuery } from "@tanstack/react-query";
import { queryKeys } from "./query-keys";
import * as versionApi from "@/lib/api/version";

/**
 * The running build's version, read from the server rather than compiled into
 * the bundle.
 *
 * It used to be a literal in `brand.ts` that had to be hand-edited every
 * release — and when it was missed, the UI confidently reported the previous
 * version. Asking the binary cannot go stale that way. Returns an empty string
 * until the (public) request resolves, so callers render nothing rather than a
 * placeholder.
 */
export function useVersion(): string {
  const { data } = useQuery({
    queryKey: queryKeys.version,
    queryFn: versionApi.getVersion,
    // The version cannot change without the page reloading with it.
    staleTime: Infinity,
    retry: false,
  });
  return data?.version ?? "";
}
