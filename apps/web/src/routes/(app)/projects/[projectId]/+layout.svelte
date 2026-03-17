<script lang="ts">
	import { page } from '$app/stores';
	let { children } = $props();
</script>

<div>
	<nav class="mb-6 flex gap-4 border-b pb-2">
		<a href="/projects/{$page.params.projectId}" class="px-2 py-1 hover:text-zinc-900">Overview</a>
		<a href="/projects/{$page.params.projectId}/services" class="px-2 py-1 hover:text-zinc-900">Services</a>
		<a href="/projects/{$page.params.projectId}/databases" class="px-2 py-1 hover:text-zinc-900">Databases</a>
	</nav>
	{@render children()}
</div>
