<script lang="ts">
	import { client } from '$lib/api-client';
	import { epicStore } from '$lib/stores/epics.svelte';
	import { repoStore } from '$lib/stores/repos.svelte';
	import { Button } from '$lib/components/ui/button';
	import * as Dialog from '$lib/components/ui/dialog';
	import { Loader2, Layers, FileText, Type } from 'lucide-svelte';

	let {
		open = $bindable(false),
		onCreated
	}: { open: boolean; onCreated: (epicId: string) => void } = $props();

	let title = $state('');
	let description = $state('');
	let loading = $state(false);
	let error = $state<string | null>(null);

	async function handleSubmit(e: SubmitEvent) {
		e.preventDefault();
		if (!title.trim()) return;

		loading = true;
		error = null;

		try {
			const repoId = repoStore.selectedRepoId;
			if (!repoId) throw new Error('No repository selected');
			const epic = await client.createEpic(repoId, title, description);
			epicStore.addEpic(epic);
			title = '';
			description = '';
			open = false;
			onCreated(epic.id);
		} catch (err) {
			error = (err as Error).message;
		} finally {
			loading = false;
		}
	}

	function handleClose() {
		open = false;
		title = '';
		description = '';
		error = null;
	}
</script>

<Dialog.Root bind:open>
	<Dialog.Content class="sm:max-w-[650px] max-h-[90vh] overflow-y-auto">
		<Dialog.Header>
			<Dialog.Title class="flex items-center gap-2">
				<div class="w-8 h-8 rounded-lg bg-violet-500/10 flex items-center justify-center">
					<Layers class="w-4 h-4 text-violet-500" />
				</div>
				Create New Epic
			</Dialog.Title>
			<Dialog.Description>
				An epic is a large deliverable with multiple related tasks. After creating it, you'll start a planning session where an AI agent helps break it into tasks.
			</Dialog.Description>
		</Dialog.Header>
		<form onsubmit={handleSubmit}>
			<div class="py-4 space-y-5">
				<div class="rounded-lg border border-violet-500/20 bg-violet-500/5 p-4">
					<h4 class="text-sm font-medium text-violet-300 mb-2">Tips for a good epic</h4>
					<ul class="text-xs text-muted-foreground space-y-1.5 list-disc list-inside">
						<li>Give a clear, specific title that describes the deliverable</li>
						<li>In the description, explain the goal, scope, and any constraints</li>
						<li>Mention technologies, patterns, or approaches you want used</li>
						<li>Note any files or areas of the codebase that are relevant</li>
						<li>Include acceptance criteria for the overall epic if possible</li>
					</ul>
				</div>

				<div>
					<label for="epic-title" class="text-sm font-medium mb-2 flex items-center gap-2">
						<Type class="w-4 h-4 text-muted-foreground" />
						Epic Title
					</label>
					<input
						id="epic-title"
						type="text"
						bind:value={title}
						autofocus
						maxlength={200}
						class="w-full border rounded-lg p-3 bg-background text-foreground focus:outline-none focus:ring-2 focus:ring-ring transition-shadow"
						placeholder="e.g., Implement user authentication system"
						disabled={loading}
					/>
					<p class="text-xs text-muted-foreground text-right mt-1">{title.length}/200</p>
				</div>

				<div>
					<label for="epic-desc" class="text-sm font-medium mb-2 flex items-center gap-2">
						<FileText class="w-4 h-4 text-muted-foreground" />
						Description & Guidelines
					</label>
					<textarea
						id="epic-desc"
						bind:value={description}
						class="w-full border rounded-lg p-3 min-h-[180px] bg-background text-foreground resize-none focus:outline-none focus:ring-2 focus:ring-ring transition-shadow"
						placeholder="Describe the epic in detail. Include:&#10;- What you want to build&#10;- Technical requirements and constraints&#10;- Relevant files or code areas&#10;- How tasks should be structured&#10;- Any dependencies or ordering requirements"
						disabled={loading}
					></textarea>
				</div>

				{#if error}
					<div class="bg-destructive/10 text-destructive text-sm p-3 rounded-lg">
						{error}
					</div>
				{/if}
			</div>
			<Dialog.Footer>
				<div class="flex justify-end gap-2 w-full">
					<Button type="button" variant="outline" onclick={handleClose} disabled={loading}>
						Cancel
					</Button>
					<Button type="submit" disabled={loading || !title.trim()} class="gap-2 bg-violet-600 hover:bg-violet-700">
						{#if loading}
							<Loader2 class="w-4 h-4 animate-spin" />
							Creating...
						{:else}
							<Layers class="w-4 h-4" />
							Create Epic
						{/if}
					</Button>
				</div>
			</Dialog.Footer>
		</form>
	</Dialog.Content>
</Dialog.Root>
