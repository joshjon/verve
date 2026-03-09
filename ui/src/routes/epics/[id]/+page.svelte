<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';
	import { client } from '$lib/api-client';
	import { repoStore } from '$lib/stores/repos.svelte';
	import { epicUrl } from '$lib/utils';
	import { Loader2 } from 'lucide-svelte';

	const epicId = $derived($page.params.id as string);
	let redirected = $state(false);

	// Use $effect to wait for repo store to be populated before redirecting.
	// The layout loads repos asynchronously, so repos may be empty on first render.
	$effect(() => {
		if (repoStore.repos.length > 0 && !redirected) {
			redirected = true;
			doRedirect();
		}
	});

	async function doRedirect() {
		try {
			const epic = await client.getEpic(epicId);
			const repo = repoStore.repos.find((r) => r.id === epic.repo_id);
			if (repo && epic.number) {
				await goto(epicUrl(repo.owner, repo.name, epic.number), { replaceState: true });
			} else {
				await goto('/', { replaceState: true });
			}
		} catch {
			await goto('/', { replaceState: true });
		}
	}
</script>

<div class="flex flex-col items-center justify-center py-16">
	<Loader2 class="w-8 h-8 animate-spin text-primary mb-4" />
	<p class="text-muted-foreground">Redirecting...</p>
</div>
