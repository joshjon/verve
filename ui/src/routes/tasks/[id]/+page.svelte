<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';
	import { client } from '$lib/api-client';
	import { repoStore } from '$lib/stores/repos.svelte';
	import { onMount } from 'svelte';
	import { Loader2 } from 'lucide-svelte';

	const taskId = $derived($page.params.id as string);

	onMount(async () => {
		try {
			const task = await client.getTask(taskId);
			const repo = repoStore.repos.find((r) => r.id === task.repo_id);
			if (repo && task.number) {
				await goto(`/${repo.owner}/${repo.name}/tasks/${task.number}`, { replaceState: true });
			} else {
				await goto('/', { replaceState: true });
			}
		} catch {
			await goto('/', { replaceState: true });
		}
	});
</script>

<div class="flex flex-col items-center justify-center py-16">
	<Loader2 class="w-8 h-8 animate-spin text-primary mb-4" />
	<p class="text-muted-foreground">Redirecting...</p>
</div>
