<script lang="ts">
	import './layout.css';
	import { onMount } from 'svelte';
	import { Zap } from 'lucide-svelte';
	import { client } from '$lib/api-client';
	import { repoStore } from '$lib/stores/repos.svelte';
	import RepoSelector from '$lib/components/RepoSelector.svelte';

	let { children } = $props();

	onMount(async () => {
		repoStore.loading = true;
		try {
			const repos = await client.listRepos();
			repoStore.setRepos(repos);
		} catch {
			// Ignore errors on initial load
		} finally {
			repoStore.loading = false;
		}
	});
</script>

<svelte:head>
	<title>Verve - AI Task Orchestrator</title>
</svelte:head>

<div class="min-h-screen bg-background flex flex-col">
	<header class="border-b bg-card/50 backdrop-blur-sm sticky top-0 z-50">
		<div class="px-6 h-14 flex items-center justify-between">
			<div class="flex items-center gap-4">
				<a href="/" class="flex items-center gap-2 hover:opacity-80 transition-opacity">
					<div class="w-8 h-8 rounded-lg bg-primary flex items-center justify-center">
						<Zap class="w-5 h-5 text-primary-foreground" />
					</div>
					<span class="font-bold text-xl tracking-tight">Verve</span>
				</a>
				<RepoSelector />
			</div>
		</div>
	</header>
	<main class="flex-1">
		{@render children()}
	</main>
</div>
