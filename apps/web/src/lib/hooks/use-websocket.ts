import { useEffect, useRef, useCallback, useState } from "react";

interface OutboundMessage {
  channel: string;
  event: string;
  data: unknown;
}

interface InboundMessage {
  action: "subscribe" | "unsubscribe";
  channel: string;
}

type MessageHandler = (event: string, data: unknown) => void;

// Singleton WebSocket manager
let ws: WebSocket | null = null;
let reconnectTimer: ReturnType<typeof setTimeout> | null = null;
let retryCount = 0;
const MAX_RETRIES = 20;
const listeners = new Map<string, Set<MessageHandler>>();
let connectPromise: Promise<void> | null = null;

function getWsUrl() {
  const proto = window.location.protocol === "https:" ? "wss:" : "ws:";
  return `${proto}//${window.location.host}/api/ws`;
}

function sendJSON(msg: InboundMessage) {
  if (ws && ws.readyState === WebSocket.OPEN) {
    ws.send(JSON.stringify(msg));
  }
}

function handleMessage(event: MessageEvent) {
  try {
    const msg: OutboundMessage = JSON.parse(event.data);
    const handlers = listeners.get(msg.channel);
    if (handlers) {
      for (const handler of handlers) {
        handler(msg.event, msg.data);
      }
    }
  } catch {
    // ignore parse errors
  }
}

function connect(): Promise<void> {
  if (ws && (ws.readyState === WebSocket.OPEN || ws.readyState === WebSocket.CONNECTING)) {
    return connectPromise ?? Promise.resolve();
  }

  connectPromise = new Promise<void>((resolve) => {
    ws = new WebSocket(getWsUrl());

    ws.onopen = () => {
      retryCount = 0;
      // Re-subscribe to all active channels
      for (const channel of listeners.keys()) {
        sendJSON({ action: "subscribe", channel });
      }
      resolve();
    };

    ws.onmessage = handleMessage;

    ws.onclose = () => {
      ws = null;
      connectPromise = null;
      if (listeners.size > 0 && retryCount < MAX_RETRIES) {
        const delay = Math.min(1000 * 2 ** retryCount, 30000);
        retryCount++;
        reconnectTimer = setTimeout(() => {
          connect();
        }, delay);
      }
    };

    ws.onerror = () => {
      // onclose will fire after this
    };
  });

  return connectPromise;
}

function ensureConnected() {
  if (!ws || ws.readyState === WebSocket.CLOSED || ws.readyState === WebSocket.CLOSING) {
    connect();
  }
}

function subscribe(channel: string, handler: MessageHandler) {
  let handlers = listeners.get(channel);
  const isNew = !handlers || handlers.size === 0;
  if (!handlers) {
    handlers = new Set();
    listeners.set(channel, handlers);
  }
  handlers.add(handler);

  if (isNew) {
    ensureConnected();
    sendJSON({ action: "subscribe", channel });
  }
}

function unsubscribe(channel: string, handler: MessageHandler) {
  const handlers = listeners.get(channel);
  if (!handlers) return;
  handlers.delete(handler);
  if (handlers.size === 0) {
    listeners.delete(channel);
    sendJSON({ action: "unsubscribe", channel });

    // Close WS if no more listeners
    if (listeners.size === 0) {
      if (reconnectTimer) {
        clearTimeout(reconnectTimer);
        reconnectTimer = null;
      }
      ws?.close();
      ws = null;
      connectPromise = null;
      retryCount = 0;
    }
  }
}

/**
 * Subscribe to a WebSocket channel. The callback receives (event, data) for
 * every message on that channel. Automatically subscribes on mount and
 * unsubscribes on unmount.
 */
export function useChannel<T = unknown>(
  channel: string | null,
  onMessage: (event: string, data: T) => void,
) {
  const [connected, setConnected] = useState(false);
  const onMessageRef = useRef(onMessage);
  onMessageRef.current = onMessage;

  const handler = useCallback((event: string, data: unknown) => {
    onMessageRef.current(event, data as T);
  }, []);

  useEffect(() => {
    if (!channel) return;

    subscribe(channel, handler);
    setConnected(true);

    return () => {
      unsubscribe(channel, handler);
      setConnected(false);
    };
  }, [channel, handler]);

  return { connected };
}
