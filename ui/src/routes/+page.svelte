<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { client } from '$lib/api-client';
	import { taskStore } from '$lib/stores/tasks.svelte';
	import { repoStore } from '$lib/stores/repos.svelte';
	import { epicStore } from '$lib/stores/epics.svelte';
	import TaskColumn from '$lib/components/TaskColumn.svelte';
	import CreateTaskDialog from '$lib/components/CreateTaskDialog.svelte';
	import CreateEpicDialog from '$lib/components/CreateEpicDialog.svelte';
	import EpicCard from '$lib/components/EpicCard.svelte';
	import { Button } from '$lib/components/ui/button';
	import {
		Plus,
		RefreshCw,
		CheckCircle2,
		AlertCircle,
		GitBranch,
		Clock,
		Play,
		Eye,
		XCircle,
		Layers
	} from 'lucide-svelte';

	let openCreate = $state(false);
	let openCreateEpic = $state(false);
	let syncing = $state(false);
	let syncResult = $state<{ synced: number; merged: number } | null>(null);

	// Track current EventSource so we can reconnect when repo changes.
	let currentES: EventSource | null = null;

	function connectSSE(repoId: string | null) {
		if (currentES) {
			currentES.close();
			currentES = null;
		}

		if (!repoId) {
			taskStore.setTasks([]);
			taskStore.loading = false;
			return;
		}

		taskStore.loading = true;
		const es = new EventSource(client.eventsURL(repoId));

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

		currentES = es;
	}

	// Reconnect SSE when selected repo changes, and load epics.
	$effect(() => {
		const repoId = repoStore.selectedRepoId;
		connectSSE(repoId);
		if (repoId) {
			loadEpics(repoId);
		} else {
			epicStore.clear();
		}
		return () => {
			if (currentES) {
				currentES.close();
				currentES = null;
			}
		};
	});

	async function loadEpics(repoId: string) {
		try {
			const epics = await client.listEpicsByRepo(repoId);
			epicStore.setEpics(epics);
		} catch {
			// Ignore errors silently on initial load
		}
	}

	async function syncPRs() {
		const repoId = repoStore.selectedRepoId;
		if (!repoId) return;
		syncing = true;
		syncResult = null;
		try {
			syncResult = await client.syncRepoTasks(repoId);
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
	const hasRepo = $derived(!!repoStore.selectedRepoId);

	const doneTasks = $derived([
		...taskStore.tasksByStatus.merged,
		...taskStore.tasksByStatus.closed
	]);

	const activeEpics = $derived(
		epicStore.epics.filter((e) => !['completed', 'closed'].includes(e.status))
	);
</script>

<div class="p-4 sm:p-6 flex-1 min-h-0 flex flex-col">
	{#if hasRepo}
		<header class="flex flex-col sm:flex-row sm:justify-between sm:items-center gap-3 mb-4 sm:mb-6">
			<div>
				<div class="flex items-center gap-3">
					<h1 class="text-xl sm:text-2xl font-bold">Dashboard</h1>
					{#if totalTasks > 0}
						<div class="flex items-center gap-2">
							<span
								class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-primary/15 text-primary"
							>
								{totalTasks} total
							</span>
							{#if activeTasks > 0}
								<span
									class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-primary/15 text-primary"
								>
									{activeTasks} active
								</span>
							{/if}
						</div>
					{/if}
				</div>
				<p class="text-muted-foreground text-sm mt-1 hidden sm:block">
					Manage and monitor your AI-powered tasks
				</p>
			</div>
			<div class="flex items-center gap-2 sm:gap-3">
				{#if syncResult}
					<div
						class="flex items-center gap-2 text-sm text-green-600 dark:text-green-400 bg-green-500/10 px-3 py-1.5 rounded-md"
					>
						<CheckCircle2 class="w-4 h-4" />
						<span>Synced {syncResult.synced} PRs, {syncResult.merged} merged</span>
					</div>
				{/if}
				<Button variant="outline" onclick={syncPRs} disabled={syncing} class="gap-2">
					<RefreshCw class="w-4 h-4 {syncing ? 'animate-spin' : ''}" />
					<span class="hidden sm:inline">{syncing ? 'Syncing...' : 'Sync PRs'}</span>
				</Button>
				<Button variant="outline" onclick={() => (openCreateEpic = true)} class="gap-2 border-violet-500/30 text-violet-300 hover:bg-violet-500/10">
					<Layers class="w-4 h-4" />
					<span class="hidden sm:inline">New Epic</span>
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

		<!-- Epics section -->
		{#if activeEpics.length > 0}
			<div class="mb-4">
				<h2 class="text-sm font-semibold text-muted-foreground mb-2 flex items-center gap-2">
					<Layers class="w-4 h-4 text-violet-400" />
					Epics
				</h2>
				<div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-2">
					{#each activeEpics as epic (epic.id)}
						<EpicCard {epic} />
					{/each}
				</div>
			</div>
		{/if}

		{#snippet pendingAction()}
			<button
				onclick={() => (openCreate = true)}
				class="h-6 px-2 gap-1 text-xs inline-flex items-center rounded-md font-medium bg-amber-500/20 text-amber-200 hover:bg-amber-500/30 transition-colors"
			>
				<Plus class="w-3 h-3" />
				New
			</button>
		{/snippet}

		<div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-5 gap-3 sm:gap-4 flex-1 min-h-0 sm:auto-rows-[1fr] max-h-96 sm:max-h-none">
			<TaskColumn
				label="Pending"
				icon={Clock}
				headerBg="bg-amber-500/10"
				iconClass="text-amber-600 dark:text-amber-400"
				tasks={taskStore.tasksByStatus.pending}
				headerAction={pendingAction}
			/>
			<TaskColumn
				label="Running"
				icon={Play}
				headerBg="bg-blue-500/10"
				iconClass="text-blue-600 dark:text-blue-400"
				tasks={taskStore.tasksByStatus.running}
			/>
			<TaskColumn
				label="In Review"
				icon={Eye}
				headerBg="bg-purple-500/10"
				iconClass="text-purple-600 dark:text-purple-400"
				tasks={taskStore.tasksByStatus.review}
			/>
			<TaskColumn
				label="Done"
				icon={CheckCircle2}
				headerBg="bg-green-500/10"
				iconClass="text-green-600 dark:text-green-400"
				tasks={doneTasks}
			/>
			<TaskColumn
				label="Failed"
				icon={XCircle}
				headerBg="bg-red-500/10"
				iconClass="text-red-600 dark:text-red-400"
				tasks={taskStore.tasksByStatus.failed}
			/>
		</div>
	{:else}
		<div class="flex-1 flex flex-col items-center justify-center text-center">
			<div
				class="w-16 h-16 rounded-2xl bg-muted flex items-center justify-center mb-4"
			>
				<GitBranch class="w-8 h-8 text-muted-foreground" />
			</div>
			<h2 class="text-xl font-semibold mb-2">No repository selected</h2>
			<p class="text-muted-foreground text-sm max-w-md">
				Add a GitHub repository to get started. Tasks are scoped to individual repositories.
			</p>
		</div>
	{/if}
</div>

{#if hasRepo}
	<CreateTaskDialog bind:open={openCreate} onCreated={() => {}} />
	<CreateEpicDialog bind:open={openCreateEpic} onCreated={(id) => goto(`/epics/${id}`)} />
{/if}
