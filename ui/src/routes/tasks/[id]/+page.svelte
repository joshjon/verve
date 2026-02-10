<script lang="ts">
	import { page } from '$app/stores';
	import { onMount } from 'svelte';
	import { client } from '$lib/api-client';
	import type { Task } from '$lib/models/task';
	import { Button } from '$lib/components/ui/button';
	import { Badge } from '$lib/components/ui/badge';
	import * as Card from '$lib/components/ui/card';
	import { ScrollArea } from '$lib/components/ui/scroll-area';
	import { goto } from '$app/navigation';
	import { cn } from '$lib/utils';

	let task = $state<Task | null>(null);
	let loading = $state(true);
	let error = $state<string | null>(null);
	let syncing = $state(false);
	let closing = $state(false);
	let showCloseForm = $state(false);
	let closeReason = $state('');

	const taskId = $derived($page.params.id);

	const statusConfig: Record<string, { label: string; class: string }> = {
		pending: { label: 'Pending', class: 'bg-yellow-500 hover:bg-yellow-500' },
		running: { label: 'Running', class: 'bg-blue-500 hover:bg-blue-500' },
		review: { label: 'Review', class: 'bg-purple-500 hover:bg-purple-500' },
		merged: { label: 'Merged', class: 'bg-green-500 hover:bg-green-500' },
		closed: { label: 'Closed', class: 'bg-gray-500 hover:bg-gray-500' },
		failed: { label: 'Failed', class: 'bg-red-500 hover:bg-red-500' }
	};

	// Check if task can be closed (not already closed, merged, or failed)
	const canClose = $derived(
		task && !['closed', 'merged', 'failed'].includes(task.status)
	);

	onMount(() => {
		loadTask();
		const interval = setInterval(loadTask, 2000);
		return () => clearInterval(interval);
	});

	async function loadTask() {
		try {
			task = await client.getTask(taskId);
			error = null;
		} catch (e) {
			error = (e as Error).message;
		} finally {
			loading = false;
		}
	}

	async function syncStatus() {
		if (!task || syncing) return;
		syncing = true;
		try {
			task = await client.syncTask(task.id);
		} catch (e) {
			error = (e as Error).message;
		} finally {
			syncing = false;
		}
	}

	async function handleClose() {
		if (!task || closing) return;
		closing = true;
		try {
			task = await client.closeTask(task.id, closeReason || undefined);
			showCloseForm = false;
			closeReason = '';
		} catch (e) {
			error = (e as Error).message;
		} finally {
			closing = false;
		}
	}

	function formatDate(dateStr: string): string {
		return new Date(dateStr).toLocaleString();
	}
</script>

<div class="p-6 max-w-4xl mx-auto">
	<Button variant="ghost" onclick={() => goto('/')} class="mb-4">
		<span class="mr-2">&larr;</span> Back to Dashboard
	</Button>

	{#if loading}
		<div class="flex items-center justify-center py-12">
			<p class="text-muted-foreground">Loading task...</p>
		</div>
	{:else if error && !task}
		<div class="bg-destructive/10 text-destructive p-4 rounded-md">
			{error}
		</div>
	{:else if task}
		<div class="space-y-6">
			<div class="flex items-start justify-between">
				<div>
					<h1 class="text-2xl font-bold font-mono">{task.id}</h1>
					<p class="text-sm text-muted-foreground mt-1">
						Created: {formatDate(task.created_at)}
					</p>
				</div>
				<div class="flex items-center gap-2">
					{#if canClose}
						{#if showCloseForm}
							<Button size="sm" variant="ghost" onclick={() => (showCloseForm = false)}>
								Cancel
							</Button>
						{:else}
							<Button size="sm" variant="outline" onclick={() => (showCloseForm = true)}>
								Close Task
							</Button>
						{/if}
					{/if}
					<Badge class={cn('text-white', statusConfig[task.status]?.class)}>
						{statusConfig[task.status]?.label ?? task.status}
					</Badge>
				</div>
			</div>

			{#if showCloseForm}
				<Card.Root class="border-destructive/50">
					<Card.Header>
						<Card.Title>Close Task</Card.Title>
					</Card.Header>
					<Card.Content>
						<div class="space-y-4">
							<div>
								<label for="close-reason" class="text-sm font-medium mb-2 block">
									Reason (optional)
								</label>
								<textarea
									id="close-reason"
									bind:value={closeReason}
									class="w-full border rounded-md p-3 min-h-[80px] bg-background text-foreground resize-none focus:outline-none focus:ring-2 focus:ring-ring"
									placeholder="Why is this task being closed?"
									disabled={closing}
								></textarea>
							</div>
							<div class="flex justify-end gap-2">
								<Button variant="outline" onclick={() => (showCloseForm = false)} disabled={closing}>
									Cancel
								</Button>
								<Button variant="destructive" onclick={handleClose} disabled={closing}>
									{closing ? 'Closing...' : 'Close Task'}
								</Button>
							</div>
						</div>
					</Card.Content>
				</Card.Root>
			{/if}

			<Card.Root>
				<Card.Header>
					<Card.Title>Description</Card.Title>
				</Card.Header>
				<Card.Content>
					<p class="whitespace-pre-wrap">{task.description}</p>
				</Card.Content>
			</Card.Root>

			{#if task.close_reason}
				<Card.Root class="border-gray-500/50">
					<Card.Header>
						<Card.Title>Close Reason</Card.Title>
					</Card.Header>
					<Card.Content>
						<p class="whitespace-pre-wrap text-muted-foreground">{task.close_reason}</p>
					</Card.Content>
				</Card.Root>
			{/if}

			{#if task.pull_request_url}
				<Card.Root>
					<Card.Header>
						<Card.Title>Pull Request</Card.Title>
					</Card.Header>
					<Card.Content>
						<div class="flex items-center gap-4">
							<a
								href={task.pull_request_url}
								class="text-primary hover:underline"
								target="_blank"
								rel="noopener noreferrer"
							>
								PR #{task.pr_number}
							</a>
							<Button size="sm" variant="outline" onclick={syncStatus} disabled={syncing}>
								{syncing ? 'Syncing...' : 'Sync Status'}
							</Button>
						</div>
					</Card.Content>
				</Card.Root>
			{/if}

			{#if task.depends_on && task.depends_on.length > 0}
				<Card.Root>
					<Card.Header>
						<Card.Title>Dependencies</Card.Title>
					</Card.Header>
					<Card.Content>
						<div class="flex flex-wrap gap-2">
							{#each task.depends_on as depId}
								<Badge
									variant="outline"
									class="cursor-pointer hover:bg-accent"
									onclick={() => goto(`/tasks/${depId}`)}
								>
									{depId}
								</Badge>
							{/each}
						</div>
					</Card.Content>
				</Card.Root>
			{/if}

			<Card.Root>
				<Card.Header>
					<Card.Title>Logs</Card.Title>
				</Card.Header>
				<Card.Content>
					<ScrollArea class="h-[400px] w-full rounded-md border bg-black p-4">
						{#if task.logs && task.logs.length > 0}
							<pre
								class="text-green-400 text-xs font-mono whitespace-pre-wrap">{task.logs.join('\n')}</pre>
						{:else}
							<p class="text-muted-foreground text-sm">No logs available yet.</p>
						{/if}
					</ScrollArea>
				</Card.Content>
			</Card.Root>

			<div class="text-sm text-muted-foreground">
				Last updated: {formatDate(task.updated_at)}
			</div>
		</div>
	{/if}
</div>
