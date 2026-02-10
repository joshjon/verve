<script lang="ts">
	import type { Task, TaskStatus } from '$lib/models/task';
	import TaskCard from './TaskCard.svelte';
	import { Badge } from '$lib/components/ui/badge';
	import { cn } from '$lib/utils';

	let { status, tasks }: { status: TaskStatus; tasks: Task[] } = $props();

	const statusConfig: Record<TaskStatus, { label: string; class: string }> = {
		pending: { label: 'Pending', class: 'bg-yellow-500 hover:bg-yellow-500' },
		running: { label: 'Running', class: 'bg-blue-500 hover:bg-blue-500' },
		review: { label: 'Review', class: 'bg-purple-500 hover:bg-purple-500' },
		merged: { label: 'Merged', class: 'bg-green-500 hover:bg-green-500' },
		closed: { label: 'Closed', class: 'bg-gray-500 hover:bg-gray-500' },
		failed: { label: 'Failed', class: 'bg-red-500 hover:bg-red-500' }
	};

	const config = $derived(statusConfig[status]);
</script>

<div class="bg-muted/50 rounded-lg p-3 min-h-[500px] flex flex-col">
	<div class="flex items-center gap-2 mb-3">
		<Badge class={cn('text-white', config.class)}>{config.label}</Badge>
		<span class="text-sm text-muted-foreground">({tasks.length})</span>
	</div>
	<div class="space-y-2 flex-1 overflow-y-auto">
		{#each tasks as task (task.id)}
			<TaskCard {task} />
		{/each}
		{#if tasks.length === 0}
			<p class="text-sm text-muted-foreground text-center py-4">No tasks</p>
		{/if}
	</div>
</div>
