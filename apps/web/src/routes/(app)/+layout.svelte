<script lang="ts">
	let { children } = $props();
</script>

<div class="flex h-screen">
	<!-- Sidebar -->
	<aside class="w-64 border-r bg-zinc-50 p-4">
		<div class="mb-8 text-lg font-bold">PaaS</div>
		<nav class="space-y-1">
			<a href="/dashboard" class="block rounded px-3 py-2 hover:bg-zinc-100">Dashboard</a>
			<a href="/projects" class="block rounded px-3 py-2 hover:bg-zinc-100">Projects</a>
			<a href="/settings" class="block rounded px-3 py-2 hover:bg-zinc-100">Settings</a>
		</nav>
	</aside>

	<!-- Main content -->
	<main class="flex-1 overflow-y-auto p-8">
		{@render children()}
	</main>
</div>
