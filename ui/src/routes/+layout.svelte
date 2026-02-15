<script lang="ts">
	import './layout.css';
	import { onMount } from 'svelte';
	import { Zap, Settings } from 'lucide-svelte';
	import { client } from '$lib/api-client';
	import { repoStore } from '$lib/stores/repos.svelte';
	import { Button } from '$lib/components/ui/button';
	import RepoSelector from '$lib/components/RepoSelector.svelte';
	import GitHubTokenDialog from '$lib/components/GitHubTokenDialog.svelte';

	let { children } = $props();
	let openTokenDialog = $state(false);
	let tokenConfigured = $state<boolean | null>(null);

	onMount(async () => {
		try {
			const status = await client.getGitHubTokenStatus();
			tokenConfigured = status.configured;
			if (!status.configured) {
				openTokenDialog = true;
			}
		} catch {
			tokenConfigured = false;
			openTokenDialog = true;
		}

		if (tokenConfigured) {
			await loadRepos();
		}
	});

	async function loadRepos() {
		repoStore.loading = true;
		try {
			const repos = await client.listRepos();
			repoStore.setRepos(repos);
		} catch {
			// Ignore errors on initial load
		} finally {
			repoStore.loading = false;
		}
	}

	function handleTokenConfigured() {
		tokenConfigured = true;
		openTokenDialog = false;
		loadRepos();
	}
</script>

<svelte:head>
	<title>Verve - AI Task Orchestrator</title>
</svelte:head>

<div class="min-h-screen bg-background flex flex-col">
	<header class="border-b bg-card/50 backdrop-blur-sm sticky top-0 z-50">
		<div class="px-4 sm:px-6 h-14 flex items-center justify-between gap-2">
			<div class="flex items-center gap-2 sm:gap-4 min-w-0">
				<a href="/" class="flex items-center gap-2 hover:opacity-80 transition-opacity shrink-0">
					<div class="w-8 h-8 rounded-lg bg-primary flex items-center justify-center">
						<Zap class="w-5 h-5 text-primary-foreground" />
					</div>
					<span class="font-bold text-xl tracking-tight hidden sm:inline">Verve</span>
				</a>
				{#if tokenConfigured}
					<RepoSelector />
				{/if}
			</div>
			<div class="flex items-center shrink-0">
				<Button
					variant="ghost"
					size="icon"
					onclick={() => (openTokenDialog = true)}
					title="Settings"
				>
					<Settings class="w-5 h-5 text-muted-foreground" />
				</Button>
			</div>
		</div>
	</header>
	<main class="flex-1">
		{#if tokenConfigured}
			{@render children()}
		{:else if tokenConfigured === false}
			<div class="flex items-center justify-center h-[60vh] text-muted-foreground text-sm">
				Configure your GitHub token to get started.
			</div>
		{/if}
	</main>
</div>

<GitHubTokenDialog
	bind:open={openTokenDialog}
	required={!tokenConfigured}
	onconfigured={handleTokenConfigured}
/>
