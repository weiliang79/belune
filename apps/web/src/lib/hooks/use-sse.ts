import { useEffect, useRef, useState } from "react";

/**
 * Manages an EventSource connection with exponential-backoff reconnection.
 * Reconnects up to `maxRetries` times; backs off starting at `initialDelayMs`,
 * doubling each attempt up to 30 seconds. Passes `null` for `url` to disable.
 */
export function useSSEWithReconnect(
  url: string | null,
  onMessage: (data: string) => void,
  maxRetries = 10,
  initialDelayMs = 1_000,
) {
  const [connected, setConnected] = useState(false);
  const onMessageRef = useRef(onMessage);
  onMessageRef.current = onMessage;
  const retryCount = useRef(0);
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(() => {
    if (!url) return;

    let source: EventSource | null = null;
    let cancelled = false;

    function connect() {
      if (cancelled) return;
      source = new EventSource(url!, { withCredentials: true });

      source.onopen = () => {
        setConnected(true);
        retryCount.current = 0;
      };

      source.onmessage = (event) => {
        onMessageRef.current(event.data);
      };

      source.onerror = () => {
        setConnected(false);
        source?.close();
        source = null;

        if (!cancelled && retryCount.current < maxRetries) {
          const delay = Math.min(
            initialDelayMs * 2 ** retryCount.current,
            30_000,
          );
          retryCount.current++;
          timerRef.current = setTimeout(connect, delay);
        }
      };
    }

    connect();

    return () => {
      cancelled = true;
      if (timerRef.current !== null) {
        clearTimeout(timerRef.current);
        timerRef.current = null;
      }
      source?.close();
      setConnected(false);
      retryCount.current = 0;
    };
  }, [url, maxRetries, initialDelayMs]);

  return { connected };
}
