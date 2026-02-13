<script lang="ts">
	import { onMount } from 'svelte';
	import { client } from '$lib/api-client';
	import { taskStore } from '$lib/stores/tasks.svelte';
	import TaskColumn from '$lib/components/TaskColumn.svelte';
	import CreateTaskDialog from '$lib/components/CreateTaskDialog.svelte';
	import { Button } from '$lib/components/ui/button';
	import { Plus, RefreshCw, CheckCircle2, AlertCircle } from 'lucide-svelte';

	let openCreate = $state(false);
	let syncing = $state(false);
	let syncResult = $state<{ synced: number; merged: number } | null>(null);

	onMount(() => {
		const es = new EventSource(client.eventsURL());

		es.addEventListener('init', (e) => {
			taskStore.setTasks(JSON.parse(e.data));
			taskStore.loading = false;
			taskStore.error = null;
		});

		es.addEventListener('task_created', (e) => {
			const event = JSON.parse(e.data);
			taskStore.addTask(event.task);
		});

		es.addEventListener('task_updated', (e) => {
			const event = JSON.parse(e.data);
			taskStore.updateTask(event.task);
		});

		es.onerror = () => {
			taskStore.error = 'Connection lost. Reconnecting...';
		};

		taskStore.loading = true;

		return () => es.close();
	});

	async function syncAllPRs() {
		syncing = true;
		syncResult = null;
		try {
			syncResult = await client.syncAllTasks();
			setTimeout(() => {
				syncResult = null;
			}, 3000);
		} catch (e) {
			taskStore.error = (e as Error).message;
		} finally {
			syncing = false;
		}
	}

	const totalTasks = $derived(taskStore.tasks.length);
	const activeTasks = $derived(
		taskStore.tasks.filter((t) => ['pending', 'running', 'review'].includes(t.status)).length
	);
</script>

<div class="p-6 h-full flex flex-col">
	<header class="flex justify-between items-center mb-6">
		<div>
			<div class="flex items-center gap-3">
				<h1 class="text-2xl font-bold">Dashboard</h1>
				{#if totalTasks > 0}
					<div class="flex items-center gap-2">
						<span
							class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-primary/10 text-primary"
						>
							{totalTasks} total
						</span>
						{#if activeTasks > 0}
							<span
								class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-blue-500/10 text-blue-600 dark:text-blue-400"
							>
								{activeTasks} active
							</span>
						{/if}
					</div>
				{/if}
			</div>
			<p class="text-muted-foreground text-sm mt-1">Manage and monitor your AI-powered tasks</p>
		</div>
		<div class="flex items-center gap-3">
			{#if syncResult}
				<div
					class="flex items-center gap-2 text-sm text-green-600 dark:text-green-400 bg-green-500/10 px-3 py-1.5 rounded-md"
				>
					<CheckCircle2 class="w-4 h-4" />
					<span>Synced {syncResult.synced} PRs, {syncResult.merged} merged</span>
				</div>
			{/if}
			<Button variant="outline" onclick={syncAllPRs} disabled={syncing} class="gap-2">
				<RefreshCw class="w-4 h-4 {syncing ? 'animate-spin' : ''}" />
				{syncing ? 'Syncing...' : 'Sync PRs'}
			</Button>
			<Button onclick={() => (openCreate = true)} class="gap-2">
				<Plus class="w-4 h-4" />
				New Task
			</Button>
		</div>
	</header>

	{#if taskStore.error}
		<div
			class="bg-destructive/10 text-destructive p-4 rounded-lg mb-4 flex items-center gap-3 border border-destructive/20"
		>
			<AlertCircle class="w-5 h-5 flex-shrink-0" />
			<span>{taskStore.error}</span>
		</div>
	{/if}

	<div class="grid grid-cols-6 gap-4 flex-1 min-h-0">
		{#each taskStore.statuses as status}
			<TaskColumn {status} tasks={taskStore.tasksByStatus[status]} />
		{/each}
	</div>
</div>

<CreateTaskDialog bind:open={openCreate} onCreated={() => {}} />
