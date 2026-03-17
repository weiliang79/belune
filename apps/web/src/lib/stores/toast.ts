import { writable } from 'svelte/store';

interface Toast {
	id: string;
	message: string;
	type: 'success' | 'error' | 'info';
}

export const toasts = writable<Toast[]>([]);

export function addToast(message: string, type: Toast['type'] = 'info') {
	const id = crypto.randomUUID();
	toasts.update((t) => [...t, { id, message, type }]);
	setTimeout(() => {
		toasts.update((t) => t.filter((toast) => toast.id !== id));
	}, 5000);
}
