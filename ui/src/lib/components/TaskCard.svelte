<script lang="ts">
	import type { Task } from '$lib/models/task';
	import * as Card from '$lib/components/ui/card';
	import { goto } from '$app/navigation';

	let { task }: { task: Task } = $props();

	function handleClick() {
		goto(`/tasks/${task.id}`);
	}

	function handlePRClick(e: MouseEvent) {
		e.stopPropagation();
	}
</script>

<Card.Root
	class="p-3 cursor-pointer hover:bg-accent transition-colors"
	onclick={handleClick}
	role="button"
	tabindex="0"
>
	<p class="font-medium text-sm line-clamp-2">{task.description}</p>
	<p class="text-xs text-muted-foreground mt-1 font-mono">{task.id}</p>
	{#if task.pull_request_url}
		<a
			href={task.pull_request_url}
			class="text-xs text-primary hover:underline mt-1 inline-block"
			onclick={handlePRClick}
			target="_blank"
			rel="noopener noreferrer"
		>
			PR #{task.pr_number}
		</a>
	{/if}
</Card.Root>
