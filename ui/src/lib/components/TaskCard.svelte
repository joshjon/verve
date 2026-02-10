<script lang="ts">
	import type { Task } from '$lib/models/task';
	import * as Card from '$lib/components/ui/card';
	import { goto } from '$app/navigation';
	import { GitPullRequest, Link2, ChevronRight } from 'lucide-svelte';

	let { task }: { task: Task } = $props();

	function handleClick() {
		goto(`/tasks/${task.id}`);
	}

	function handlePRClick(e: MouseEvent) {
		e.stopPropagation();
	}

	const hasDependencies = $derived(task.depends_on && task.depends_on.length > 0);
</script>

<Card.Root
	class="group p-3 cursor-pointer bg-card shadow-sm hover:bg-accent/50 hover:border-accent transition-all duration-200 hover:shadow-md"
	onclick={handleClick}
	role="button"
	tabindex="0"
>
	<div class="flex items-start justify-between gap-2">
		<p class="font-medium text-sm line-clamp-2 flex-1">{task.description}</p>
		<ChevronRight
			class="w-4 h-4 text-muted-foreground opacity-0 group-hover:opacity-100 transition-opacity flex-shrink-0 mt-0.5"
		/>
	</div>
	<div class="flex items-center gap-2 mt-2">
		<span class="text-[10px] text-muted-foreground font-mono bg-muted px-1.5 py-0.5 rounded">
			{task.id}
		</span>
		{#if hasDependencies}
			<span class="text-[10px] text-muted-foreground flex items-center gap-0.5" title="Has dependencies">
				<Link2 class="w-3 h-3" />
				{task.depends_on?.length}
			</span>
		{/if}
	</div>
	{#if task.pull_request_url}
		<a
			href={task.pull_request_url}
			class="inline-flex items-center gap-1 text-xs text-primary hover:underline mt-2"
			onclick={handlePRClick}
			target="_blank"
			rel="noopener noreferrer"
		>
			<GitPullRequest class="w-3 h-3" />
			PR #{task.pr_number}
		</a>
	{/if}
</Card.Root>
