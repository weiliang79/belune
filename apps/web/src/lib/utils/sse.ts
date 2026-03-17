export function createEventSource(
	url: string,
	onMessage: (data: string) => void,
	onError?: (error: Event) => void
): EventSource {
	const source = new EventSource(url);

	source.onmessage = (event) => {
		onMessage(event.data);
	};

	source.onerror = (event) => {
		if (onError) onError(event);
	};

	return source;
}
