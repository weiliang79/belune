import { useEffect, useState } from "react";

// Matches Tailwind's `md` breakpoint (min-width: 768px); "mobile" is below it.
const MOBILE_BREAKPOINT = 768;
const QUERY = `(max-width: ${MOBILE_BREAKPOINT - 1}px)`;

export function useIsMobile() {
  const [isMobile, setIsMobile] = useState(
    () => typeof window !== "undefined" && window.matchMedia(QUERY).matches,
  );

  useEffect(() => {
    const mql = window.matchMedia(QUERY);
    const onChange = (e: MediaQueryListEvent) => setIsMobile(e.matches);
    mql.addEventListener("change", onChange);
    return () => mql.removeEventListener("change", onChange);
  }, []);

  return isMobile;
}
