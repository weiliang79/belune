import { writable } from 'svelte/store';

interface User {
	id: string;
	email: string;
	role: string;
}

export const user = writable<User | null>(null);
export const isAuthenticated = writable(false);
