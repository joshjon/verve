<script lang="ts">
	import { client } from '$lib/api-client';
	import { taskStore } from '$lib/stores/tasks.svelte';
	import { repoStore } from '$lib/stores/repos.svelte';
	import { Button } from '$lib/components/ui/button';
	import * as Dialog from '$lib/components/ui/dialog';
	import { Badge } from '$lib/components/ui/badge';
	import { FileText, Link2, Search, X, Loader2, Sparkles, ChevronDown, ChevronRight, Target, DollarSign } from 'lucide-svelte';

	let {
		open = $bindable(false),
		onCreated
	}: { open: boolean; onCreated: () => void } = $props();

	let description = $state('');
	let loading = $state(false);
	let error = $state<string | null>(null);
	let selectedDeps = $state<string[]>([]);
	let searchQuery = $state('');
	let acceptanceCriteria = $state('');
	let maxCostUsd = $state<number | undefined>(undefined);
	let showAdvanced = $state(false);

	// Filter available tasks (exclude closed/failed and already selected)
	const availableTasks = $derived(
		taskStore.tasks.filter(
			(t) =>
				!['closed', 'failed'].includes(t.status) &&
				!selectedDeps.includes(t.id) &&
				(searchQuery === '' ||
					t.id.toLowerCase().includes(searchQuery.toLowerCase()) ||
					t.description.toLowerCase().includes(searchQuery.toLowerCase()))
		)
	);

	async function handleSubmit(e: SubmitEvent) {
		e.preventDefault();
		if (!description.trim()) return;

		loading = true;
		error = null;

		try {
			const repoId = repoStore.selectedRepoId;
			if (!repoId) throw new Error('No repository selected');
			await client.createTaskInRepo(
				repoId,
				description,
				selectedDeps.length > 0 ? selectedDeps : undefined,
				acceptanceCriteria || undefined,
				maxCostUsd
			);
			description = '';
			selectedDeps = [];
			acceptanceCriteria = '';
			maxCostUsd = undefined;
			showAdvanced = false;
			open = false;
			onCreated();
		} catch (err) {
			error = (err as Error).message;
		} finally {
			loading = false;
		}
	}

	function handleClose() {
		open = false;
		description = '';
		selectedDeps = [];
		acceptanceCriteria = '';
		maxCostUsd = undefined;
		showAdvanced = false;
		error = null;
		searchQuery = '';
	}

	function addDependency(taskId: string) {
		selectedDeps = [...selectedDeps, taskId];
		searchQuery = '';
	}

	function removeDependency(taskId: string) {
		selectedDeps = selectedDeps.filter((id) => id !== taskId);
	}
</script>

<Dialog.Root bind:open>
	<Dialog.Content class="sm:max-w-[540px]">
		<Dialog.Header>
			<Dialog.Title class="flex items-center gap-2">
				<div class="w-8 h-8 rounded-lg bg-primary/10 flex items-center justify-center">
					<Sparkles class="w-4 h-4 text-primary" />
				</div>
				Create New Task
			</Dialog.Title>
			<Dialog.Description>
				Describe the task you want the AI agent to complete. Be specific for best results.
			</Dialog.Description>
		</Dialog.Header>
		<form onsubmit={handleSubmit}>
			<div class="py-4 space-y-5">
				<div>
					<label for="description" class="text-sm font-medium mb-2 flex items-center gap-2">
						<FileText class="w-4 h-4 text-muted-foreground" />
						Description
					</label>
					<textarea
						id="description"
						bind:value={description}
						autofocus
						class="w-full border rounded-lg p-3 min-h-[120px] bg-background text-foreground resize-none focus:outline-none focus:ring-2 focus:ring-ring transition-shadow"
						placeholder="e.g., Add a function that calculates the Fibonacci sequence and include unit tests..."
						disabled={loading}
					></textarea>
				</div>

				<div>
					<label for="acceptance-criteria" class="text-sm font-medium mb-2 flex items-center gap-2">
						<Target class="w-4 h-4 text-muted-foreground" />
						Acceptance Criteria
						<span class="text-xs text-muted-foreground font-normal">(optional)</span>
					</label>
					<textarea
						id="acceptance-criteria"
						bind:value={acceptanceCriteria}
						class="w-full border rounded-lg p-3 min-h-[80px] bg-background text-foreground resize-none focus:outline-none focus:ring-2 focus:ring-ring transition-shadow text-sm"
						placeholder="e.g., All tests pass, No linting errors, Function handles edge cases..."
						disabled={loading}
					></textarea>
				</div>

				<div>
					<label for="dep-search" class="text-sm font-medium mb-2 flex items-center gap-2">
						<Link2 class="w-4 h-4 text-muted-foreground" />
						Dependencies
						<span class="text-xs text-muted-foreground font-normal">(optional)</span>
					</label>

					{#if selectedDeps.length > 0}
						<div class="flex flex-wrap gap-1.5 mb-3 max-h-20 overflow-y-auto">
							{#each selectedDeps as depId}
								<Badge variant="secondary" class="gap-1 pl-2 pr-1 py-1 max-w-48">
									<span class="font-mono text-xs truncate">{depId}</span>
									<button
										type="button"
										class="ml-1 hover:bg-destructive/20 hover:text-destructive rounded p-0.5 transition-colors shrink-0"
										onclick={() => removeDependency(depId)}
									>
										<X class="w-3 h-3" />
									</button>
								</Badge>
							{/each}
						</div>
					{/if}

					<div class="relative">
						<Search class="w-4 h-4 absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground" />
						<input
							id="dep-search"
							type="text"
							bind:value={searchQuery}
							class="w-full border rounded-lg pl-9 pr-3 py-2 bg-background text-foreground focus:outline-none focus:ring-2 focus:ring-ring transition-shadow"
							placeholder="Search tasks to add as dependency..."
							disabled={loading}
							autocomplete="off"
						/>
					</div>

					<div class="mt-2 border rounded-lg max-h-36 overflow-y-auto bg-muted/20">
						{#if availableTasks.length > 0}
							{#each availableTasks as task (task.id)}
								<button
									type="button"
									class="w-full text-left px-3 py-2.5 hover:bg-accent cursor-pointer border-b last:border-b-0 transition-colors overflow-hidden"
									onclick={() => addDependency(task.id)}
								>
									<div class="flex items-center gap-2">
										<span class="font-mono text-xs text-muted-foreground bg-background px-1.5 py-0.5 rounded shrink-0">
											{task.id}
										</span>
									</div>
									<div class="text-sm line-clamp-2 mt-1">{task.description}</div>
								</button>
							{/each}
						{:else if searchQuery}
							<div class="p-4 text-sm text-muted-foreground text-center">
								<Search class="w-5 h-5 mx-auto mb-2 opacity-40" />
								No matching tasks found
							</div>
						{:else}
							<div class="p-4 text-sm text-muted-foreground text-center">
								<Link2 class="w-5 h-5 mx-auto mb-2 opacity-40" />
								No tasks available as dependencies
							</div>
						{/if}
					</div>
				</div>

				<div>
					<button
						type="button"
						class="flex items-center gap-1.5 text-sm text-muted-foreground hover:text-foreground transition-colors"
						onclick={() => (showAdvanced = !showAdvanced)}
					>
						{#if showAdvanced}
							<ChevronDown class="w-4 h-4" />
						{:else}
							<ChevronRight class="w-4 h-4" />
						{/if}
						Advanced Options
					</button>

					{#if showAdvanced}
						<div class="mt-3 space-y-4 pl-1">
							<div>
								<label for="max-cost" class="text-sm font-medium mb-2 flex items-center gap-2">
									<DollarSign class="w-4 h-4 text-muted-foreground" />
									Max Cost (USD)
									<span class="text-xs text-muted-foreground font-normal">(optional)</span>
								</label>
								<input
									id="max-cost"
									type="number"
									step="0.01"
									min="0"
									bind:value={maxCostUsd}
									class="w-full border rounded-lg p-2 bg-background text-foreground focus:outline-none focus:ring-2 focus:ring-ring transition-shadow text-sm"
									placeholder="e.g., 5.00"
									disabled={loading}
								/>
							</div>
						</div>
					{/if}
				</div>

				{#if error}
					<div class="bg-destructive/10 text-destructive text-sm p-3 rounded-lg flex items-center gap-2">
						<X class="w-4 h-4 flex-shrink-0" />
						{error}
					</div>
				{/if}
			</div>
			<Dialog.Footer class="gap-2">
				<Button type="button" variant="outline" onclick={handleClose} disabled={loading}>
					Cancel
				</Button>
				<Button type="submit" disabled={loading || !description.trim()} class="gap-2">
					{#if loading}
						<Loader2 class="w-4 h-4 animate-spin" />
						Creating...
					{:else}
						<Sparkles class="w-4 h-4" />
						Create Task
					{/if}
				</Button>
			</Dialog.Footer>
		</form>
	</Dialog.Content>
</Dialog.Root>
