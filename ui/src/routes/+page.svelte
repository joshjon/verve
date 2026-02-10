<script lang="ts">
	import { onMount } from 'svelte';
	import { client } from '$lib/api-client';
	import { taskStore } from '$lib/stores/tasks.svelte';
	import TaskColumn from '$lib/components/TaskColumn.svelte';
	import CreateTaskDialog from '$lib/components/CreateTaskDialog.svelte';
	import { Button } from '$lib/components/ui/button';

	let openCreate = $state(false);
	let syncing = $state(false);
	let syncResult = $state<{ synced: number; merged: number } | null>(null);

	onMount(() => {
		loadTasks();
		const interval = setInterval(loadTasks, 5000);
		return () => clearInterval(interval);
	});

	async function loadTasks() {
		taskStore.loading = true;
		try {
			const tasks = await client.listTasks();
			taskStore.setTasks(tasks);
			taskStore.error = null;
		} catch (e) {
			taskStore.error = (e as Error).message;
		} finally {
			taskStore.loading = false;
		}
	}

	async function syncAllPRs() {
		syncing = true;
		syncResult = null;
		try {
			syncResult = await client.syncAllTasks();
			await loadTasks();
			// Clear result after 3 seconds
			setTimeout(() => {
				syncResult = null;
			}, 3000);
		} catch (e) {
			taskStore.error = (e as Error).message;
		} finally {
			syncing = false;
		}
	}
</script>

<div class="p-6 h-full flex flex-col">
	<header class="flex justify-between items-center mb-6">
		<div>
			<h1 class="text-2xl font-bold">Verve Dashboard</h1>
			<p class="text-muted-foreground">AI Task Orchestrator</p>
		</div>
		<div class="flex items-center gap-3">
			{#if syncResult}
				<span class="text-sm text-muted-foreground">
					Synced {syncResult.synced} PRs, {syncResult.merged} merged
				</span>
			{/if}
			<Button variant="outline" onclick={syncAllPRs} disabled={syncing}>
				{syncing ? 'Syncing...' : 'Sync All PRs'}
			</Button>
			<Button onclick={() => (openCreate = true)}>New Task</Button>
		</div>
	</header>

	{#if taskStore.error}
		<div class="bg-destructive/10 text-destructive p-4 rounded-md mb-4">
			{taskStore.error}
		</div>
	{/if}

	<div class="grid grid-cols-6 gap-4 flex-1 min-h-0">
		{#each taskStore.statuses as status}
			<TaskColumn {status} tasks={taskStore.tasksByStatus[status]} />
		{/each}
	</div>
</div>

<CreateTaskDialog bind:open={openCreate} onCreated={loadTasks} />
