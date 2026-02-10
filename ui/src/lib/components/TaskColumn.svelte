<script lang="ts">
	import type { Task, TaskStatus } from '$lib/models/task';
	import TaskCard from './TaskCard.svelte';
	import {
		Clock,
		Play,
		Eye,
		GitMerge,
		CheckCircle,
		XCircle,
		type Icon
	} from 'lucide-svelte';
	import type { ComponentType } from 'svelte';

	let { status, tasks }: { status: TaskStatus; tasks: Task[] } = $props();

	const statusConfig: Record<
		TaskStatus,
		{ label: string; icon: ComponentType<Icon>; headerBg: string; iconClass: string }
	> = {
		pending: {
			label: 'Pending',
			icon: Clock,
			headerBg: 'bg-amber-500/10',
			iconClass: 'text-amber-600 dark:text-amber-400'
		},
		running: {
			label: 'Running',
			icon: Play,
			headerBg: 'bg-blue-500/10',
			iconClass: 'text-blue-600 dark:text-blue-400'
		},
		review: {
			label: 'In Review',
			icon: Eye,
			headerBg: 'bg-purple-500/10',
			iconClass: 'text-purple-600 dark:text-purple-400'
		},
		merged: {
			label: 'Merged',
			icon: GitMerge,
			headerBg: 'bg-green-500/10',
			iconClass: 'text-green-600 dark:text-green-400'
		},
		closed: {
			label: 'Closed',
			icon: CheckCircle,
			headerBg: 'bg-gray-500/10',
			iconClass: 'text-gray-600 dark:text-gray-400'
		},
		failed: {
			label: 'Failed',
			icon: XCircle,
			headerBg: 'bg-red-500/10',
			iconClass: 'text-red-600 dark:text-red-400'
		}
	};

	const config = $derived(statusConfig[status]);
	const StatusIcon = $derived(config.icon);
</script>

<div class="rounded-xl border bg-muted/50 min-h-[500px] flex flex-col overflow-hidden">
	<div class="flex items-center gap-2 px-3 py-2.5 {config.headerBg} border-b">
		<StatusIcon class="w-4 h-4 {config.iconClass}" />
		<span class="font-medium text-sm {config.iconClass}">{config.label}</span>
		<span
			class="ml-auto inline-flex items-center justify-center w-5 h-5 rounded-full bg-background text-xs font-medium"
		>
			{tasks.length}
		</span>
	</div>
	<div class="space-y-2 flex-1 overflow-y-auto p-2">
		{#each tasks as task (task.id)}
			<TaskCard {task} />
		{/each}
		{#if tasks.length === 0}
			<div class="flex flex-col items-center justify-center py-8 text-muted-foreground">
				<StatusIcon class="w-8 h-8 opacity-20 mb-2" />
				<p class="text-sm">No tasks</p>
			</div>
		{/if}
	</div>
</div>
