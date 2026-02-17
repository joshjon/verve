<script lang="ts">
	import type { Task } from '$lib/models/task';
	import * as Card from '$lib/components/ui/card';
	import { goto } from '$app/navigation';
	import { GitPullRequest, GitMerge, GitBranch, Ban, Link2, ChevronRight, RefreshCw, DollarSign, AlertTriangle, Loader2 } from 'lucide-svelte';
	import { repoStore } from '$lib/stores/repos.svelte';

	let { task }: { task: Task } = $props();

	function handleClick() {
		goto(`/tasks/${task.id}`);
	}

	function handlePRClick(e: MouseEvent) {
		e.stopPropagation();
	}

	const hasDependencies = $derived(task.depends_on && task.depends_on.length > 0);
	const branchURL = $derived.by(() => {
		if (!task.branch_name) return null;
		const r = repoStore.repos.find((r) => r.id === task.repo_id);
		if (!r) return null;
		return `https://github.com/${r.full_name}/tree/${task.branch_name}`;
	});
</script>

<Card.Root
	class="group p-3 cursor-pointer bg-[oklch(0.18_0.005_285.823)] shadow-sm hover:bg-accent/50 hover:border-accent transition-all duration-200 hover:shadow-md"
	onclick={handleClick}
	role="button"
	tabindex={0}
>
	<div class="flex items-start justify-between gap-2">
		<p class="font-medium text-sm line-clamp-2 flex-1">{task.title || task.description}</p>
		{#if task.status === 'merged'}
			<span class="inline-flex items-center gap-1 text-[11px] font-semibold text-green-700 dark:text-green-300 bg-green-500/15 px-2 py-0.5 rounded-full border border-green-500/20 shrink-0">
				<GitMerge class="w-3 h-3" />
				Merged
			</span>
		{:else if task.status === 'closed'}
			<span class="inline-flex items-center gap-1 text-[11px] font-semibold text-gray-600 dark:text-gray-300 bg-gray-500/15 px-2 py-0.5 rounded-full border border-gray-500/20 shrink-0">
				<Ban class="w-3 h-3" />
				Closed
			</span>
		{:else}
			<ChevronRight
				class="w-4 h-4 text-muted-foreground opacity-0 group-hover:opacity-100 transition-opacity flex-shrink-0 mt-0.5"
			/>
		{/if}
	</div>
	<div class="flex items-center gap-2 mt-2 flex-wrap">
		<span class="text-[10px] text-muted-foreground font-mono bg-muted px-1.5 py-0.5 rounded">
			{task.id}
		</span>
		{#if task.attempt > 1}
			<span
				class="text-[10px] text-amber-600 dark:text-amber-400 flex items-center gap-0.5"
				title="Retry attempt {task.attempt} of {task.max_attempts}"
			>
				<RefreshCw class="w-3 h-3" />
				#{task.attempt}
			</span>
		{/if}
		{#if task.consecutive_failures >= 2}
			<span
				class="text-[10px] text-red-600 dark:text-red-400 flex items-center gap-0.5"
				title="{task.consecutive_failures} consecutive failures"
			>
				<AlertTriangle class="w-3 h-3" />
			</span>
		{/if}
		{#if hasDependencies}
			<span class="text-[10px] text-muted-foreground flex items-center gap-0.5" title="Has dependencies">
				<Link2 class="w-3 h-3" />
				{task.depends_on?.length}
			</span>
		{/if}
		{#if task.cost_usd > 0}
			<span
				class="text-[10px] text-muted-foreground flex items-center gap-0.5"
				title="Cost: ${task.cost_usd.toFixed(2)}"
			>
				<DollarSign class="w-3 h-3" />
				{task.cost_usd.toFixed(2)}
			</span>
		{/if}
		{#if task.branch_name && !task.pull_request_url}
			<span
				class="inline-flex items-center gap-0.5 text-[10px] font-medium text-cyan-600 dark:text-cyan-400 bg-cyan-500/10 px-1.5 py-0.5 rounded-full border border-cyan-500/20"
				title="Branch only — no PR created yet"
			>
				<GitBranch class="w-3 h-3" />
				Branch only
			</span>
		{/if}
	</div>
	{#if task.pull_request_url}
		<div class="flex items-center gap-2 mt-2">
			<a
				href={task.pull_request_url}
				class="inline-flex items-center gap-1 text-xs text-primary hover:underline"
				onclick={handlePRClick}
				target="_blank"
				rel="noopener noreferrer"
			>
				<GitPullRequest class="w-3 h-3" />
				PR #{task.pr_number}
			</a>
			{#if task.status === 'running' || task.status === 'pending'}
				<span class="inline-flex items-center gap-1 text-[10px] text-blue-500">
					<Loader2 class="w-3 h-3 animate-spin" />
					Updating
				</span>
			{/if}
		</div>
	{:else if task.branch_name}
		<a
			href={branchURL ?? '#'}
			class="inline-flex items-center gap-1 text-xs text-primary hover:underline mt-2 max-w-full"
			onclick={handlePRClick}
			target="_blank"
			rel="noopener noreferrer"
		>
			<GitBranch class="w-3 h-3 shrink-0" />
			<span class="truncate">{task.branch_name}</span>
		</a>
	{/if}
</Card.Root>
