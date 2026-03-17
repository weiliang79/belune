<script lang="ts">
	let email = $state('');
	let password = $state('');
</script>

<div class="flex min-h-screen items-center justify-center">
	<div class="w-full max-w-sm space-y-4 p-8">
		<h1 class="text-2xl font-bold text-center">Sign In</h1>
		<form class="space-y-4">
			<input type="email" placeholder="Email" bind:value={email} class="w-full rounded border p-2" />
			<input type="password" placeholder="Password" bind:value={password} class="w-full rounded border p-2" />
			<button type="submit" class="w-full rounded bg-zinc-900 p-2 text-white">Sign In</button>
		</form>
	</div>
</div>
