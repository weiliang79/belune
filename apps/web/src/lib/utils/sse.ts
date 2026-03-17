export function createEventSource(
  url: string,
  onMessage: (data: string) => void,
  onError?: (event: Event) => void,
): EventSource {
  const source = new EventSource(url, { withCredentials: true });

  source.onmessage = (event) => {
    onMessage(event.data);
  };

  if (onError) {
    source.onerror = onError;
  }

  return source;
}
