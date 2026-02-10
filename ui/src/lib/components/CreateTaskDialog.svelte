<script lang="ts">
	import { client } from '$lib/api-client';
	import { taskStore } from '$lib/stores/tasks.svelte';
	import { Button } from '$lib/components/ui/button';
	import * as Dialog from '$lib/components/ui/dialog';
	import { Badge } from '$lib/components/ui/badge';

	let {
		open = $bindable(false),
		onCreated
	}: { open: boolean; onCreated: () => void } = $props();

	let description = $state('');
	let loading = $state(false);
	let error = $state<string | null>(null);
	let selectedDeps = $state<string[]>([]);
	let searchQuery = $state('');
	let showDepDropdown = $state(false);
	let inputElement = $state<HTMLInputElement | null>(null);
	let dropdownStyle = $state('');

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

	function updateDropdownPosition() {
		if (inputElement) {
			const rect = inputElement.getBoundingClientRect();
			dropdownStyle = `top: ${rect.bottom + 4}px; left: ${rect.left}px; width: ${rect.width}px;`;
		}
	}

	function handleInputFocus() {
		updateDropdownPosition();
		showDepDropdown = true;
	}

	async function handleSubmit(e: SubmitEvent) {
		e.preventDefault();
		if (!description.trim()) return;

		loading = true;
		error = null;

		try {
			await client.createTask(description, selectedDeps.length > 0 ? selectedDeps : undefined);
			description = '';
			selectedDeps = [];
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
		error = null;
		showDepDropdown = false;
	}

	function addDependency(taskId: string) {
		selectedDeps = [...selectedDeps, taskId];
		searchQuery = '';
		showDepDropdown = false;
	}

	function removeDependency(taskId: string) {
		selectedDeps = selectedDeps.filter((id) => id !== taskId);
	}
</script>

<Dialog.Root bind:open>
	<Dialog.Content class="sm:max-w-[500px]">
		<Dialog.Header>
			<Dialog.Title>Create New Task</Dialog.Title>
			<Dialog.Description>
				Describe the task you want the AI agent to complete.
			</Dialog.Description>
		</Dialog.Header>
		<form onsubmit={handleSubmit}>
			<div class="py-4 space-y-4">
				<div>
					<label for="description" class="text-sm font-medium mb-2 block">Description</label>
					<textarea
						id="description"
						bind:value={description}
						class="w-full border rounded-md p-3 min-h-[120px] bg-background text-foreground resize-none focus:outline-none focus:ring-2 focus:ring-ring"
						placeholder="e.g., Add a function that calculates the Fibonacci sequence..."
						disabled={loading}
					></textarea>
				</div>

				<div>
					<label for="dep-search" class="text-sm font-medium mb-2 block"
						>Dependencies (optional)</label
					>
					<p class="text-xs text-muted-foreground mb-2">
						Select tasks that must complete before this task can run.
					</p>

					{#if selectedDeps.length > 0}
						<div class="flex flex-wrap gap-1 mb-2">
							{#each selectedDeps as depId}
								<Badge variant="secondary" class="gap-1">
									{depId}
									<button
										type="button"
										class="ml-1 hover:text-destructive"
										onclick={() => removeDependency(depId)}
									>
										&times;
									</button>
								</Badge>
							{/each}
						</div>
					{/if}

					<input
						id="dep-search"
						type="text"
						bind:value={searchQuery}
						bind:this={inputElement}
						onfocus={handleInputFocus}
						class="w-full border rounded-md p-2 bg-background text-foreground focus:outline-none focus:ring-2 focus:ring-ring"
						placeholder="Search tasks to add as dependency..."
						disabled={loading}
						autocomplete="off"
					/>
				</div>

				{#if error}
					<p class="text-sm text-destructive">{error}</p>
				{/if}
			</div>
			<Dialog.Footer>
				<Button type="button" variant="outline" onclick={handleClose} disabled={loading}>
					Cancel
				</Button>
				<Button type="submit" disabled={loading || !description.trim()}>
					{loading ? 'Creating...' : 'Create Task'}
				</Button>
			</Dialog.Footer>
		</form>
	</Dialog.Content>
</Dialog.Root>

{#if showDepDropdown}
	<!-- Backdrop to close dropdown -->
	<button
		type="button"
		class="fixed inset-0 z-[100]"
		onclick={() => (showDepDropdown = false)}
		aria-label="Close dropdown"
	></button>

	<!-- Dropdown rendered as fixed position portal -->
	{#if availableTasks.length > 0}
		<div
			class="fixed z-[101] bg-popover border rounded-md shadow-lg max-h-48 overflow-y-auto"
			style={dropdownStyle}
		>
			{#each availableTasks as task (task.id)}
				<button
					type="button"
					class="w-full text-left px-3 py-2 hover:bg-accent cursor-pointer"
					onclick={() => addDependency(task.id)}
				>
					<div class="font-mono text-xs">{task.id}</div>
					<div class="text-sm text-muted-foreground truncate">
						{task.description}
					</div>
				</button>
			{/each}
		</div>
	{:else if searchQuery}
		<div
			class="fixed z-[101] bg-popover border rounded-md shadow-lg p-3 text-sm text-muted-foreground"
			style={dropdownStyle}
		>
			No matching tasks found.
		</div>
	{/if}
{/if}
