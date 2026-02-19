<script lang="ts">
	import type { Task } from '$lib/models/task';
	import TaskCard from './TaskCard.svelte';
	import type { ComponentType, Snippet } from 'svelte';
	import type { Icon } from 'lucide-svelte';

	let {
		label,
		icon: HeaderIcon,
		headerBg,
		iconClass,
		tasks,
		headerAction,
		selectionMode = false,
		selectedTaskIds = new Set<string>(),
		onToggleSelection
	}: {
		label: string;
		icon: ComponentType<Icon>;
		headerBg: string;
		iconClass: string;
		tasks: Task[];
		headerAction?: Snippet;
		selectionMode?: boolean;
		selectedTaskIds?: Set<string>;
		onToggleSelection?: (taskId: string) => void;
	} = $props();
</script>

<div class="rounded-xl border bg-muted/45 flex flex-col overflow-hidden">
	<div class="flex items-center gap-2 px-3 py-2.5 {headerBg} border-b">
		<HeaderIcon class="w-4 h-4 {iconClass}" />
		<span class="font-medium text-sm {iconClass}">{label}</span>
		<div class="ml-auto flex items-center gap-1.5">
			{#if headerAction}
				{@render headerAction()}
			{/if}
			<span
				class="inline-flex items-center justify-center w-5 h-5 rounded-full bg-background text-xs font-medium"
			>
				{tasks.length}
			</span>
		</div>
	</div>
	<div class="space-y-2 sm:flex-1 overflow-y-auto p-2">
		{#each tasks as task (task.id)}
			<TaskCard
				{task}
				{selectionMode}
				selected={selectedTaskIds.has(task.id)}
				{onToggleSelection}
			/>
		{/each}
		{#if tasks.length === 0}
			<div class="flex flex-col items-center justify-center py-3 sm:py-8 text-muted-foreground">
				<HeaderIcon class="w-8 h-8 opacity-20 mb-2 hidden sm:block" />
				<p class="text-sm">No tasks</p>
			</div>
		{/if}
	</div>
</div>
