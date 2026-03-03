<script lang="ts">
	import { repoStore } from '$lib/stores/repos.svelte';
	import { ChevronDown, GitBranch, Plus, Trash2, AlertTriangle } from 'lucide-svelte';
	import * as Popover from '$lib/components/ui/popover';
	import * as Dialog from '$lib/components/ui/dialog';
	import { Button } from '$lib/components/ui/button';
	import AddRepoDialog from './AddRepoDialog.svelte';
	import { client } from '$lib/api-client';

	let open = $state(false);
	let openAddRepo = $state(false);
	let removing = $state<string | null>(null);
	let confirmRemoveRepo = $state<{ id: string; fullName: string } | null>(null);
	let dialogWasOpen = false;

	// When the add repo dialog closes, prevent focus returning to the popover trigger
	// from re-opening the dropdown
	$effect(() => {
		if (openAddRepo) {
			dialogWasOpen = true;
		} else if (dialogWasOpen) {
			dialogWasOpen = false;
			requestAnimationFrame(() => { open = false; });
		}
	});

	function promptRemove(e: MouseEvent, repoId: string, fullName: string) {
		e.stopPropagation();
		open = false;
		confirmRemoveRepo = { id: repoId, fullName };
	}

	async function handleConfirmRemove() {
		if (!confirmRemoveRepo) return;
		const repoId = confirmRemoveRepo.id;
		removing = repoId;
		try {
			await client.removeRepo(repoId);
			repoStore.removeRepo(repoId);
		} catch {
			// Ignore errors for now
		} finally {
			removing = null;
			confirmRemoveRepo = null;
		}
	}

	function selectRepo(id: string) {
		repoStore.selectRepo(id);
		open = false;
	}
</script>

{#if repoStore.repos.length > 0}
	<Popover.Root bind:open>
		<Popover.Trigger>
			<Button variant="outline" class="gap-2 min-w-0 sm:min-w-40 justify-between max-w-[200px] sm:max-w-none">
				<div class="flex items-center gap-2 min-w-0">
					<GitBranch class="w-4 h-4 text-muted-foreground shrink-0" />
					<span class="truncate max-w-[120px] sm:max-w-48">
						{repoStore.selectedRepo?.full_name ?? 'Select repo'}
					</span>
				</div>
				<ChevronDown class="w-4 h-4 text-muted-foreground shrink-0" />
			</Button>
		</Popover.Trigger>
		<Popover.Content class="w-72 p-0" align="start">
			<div class="max-h-60 overflow-y-auto">
				{#each repoStore.repos as repo (repo.id)}
					<!-- svelte-ignore a11y_click_events_have_key_events -->
					<!-- svelte-ignore a11y_no_static_element_interactions -->
					<div
						class="w-full flex items-center justify-between px-3 py-2 text-sm hover:bg-accent cursor-pointer transition-colors {repo.id === repoStore.selectedRepoId ? 'bg-accent' : ''}"
						onclick={() => selectRepo(repo.id)}
					>
						<div class="flex items-center gap-2 truncate">
							<GitBranch class="w-3.5 h-3.5 text-muted-foreground flex-shrink-0" />
							<span class="truncate">{repo.full_name}</span>
						</div>
						<button
							class="p-1 hover:bg-destructive/20 hover:text-destructive rounded transition-colors flex-shrink-0"
							onclick={(e) => promptRemove(e, repo.id, repo.full_name)}
							disabled={removing === repo.id}
							title="Remove repo"
						>
							<Trash2 class="w-3 h-3" />
						</button>
					</div>
				{/each}
			</div>
			<div class="border-t p-1">
				<button
					class="w-full flex items-center gap-2 px-3 py-2 text-sm hover:bg-accent cursor-pointer rounded transition-colors text-muted-foreground"
					onclick={() => {
						open = false;
						openAddRepo = true;
					}}
				>
					<Plus class="w-3.5 h-3.5" />
					Add Repository
				</button>
			</div>
		</Popover.Content>
	</Popover.Root>
{:else}
	<Button variant="outline" class="gap-2" onclick={() => (openAddRepo = true)}>
		<Plus class="w-4 h-4" />
		Add Repository
	</Button>
{/if}

<AddRepoDialog bind:open={openAddRepo} />

<Dialog.Root open={confirmRemoveRepo !== null} onOpenChange={(v) => { if (!v) confirmRemoveRepo = null; }}>
	<Dialog.Content class="sm:max-w-md">
		<Dialog.Header>
			<Dialog.Title class="flex items-center gap-2">
				<div class="w-8 h-8 rounded-lg bg-destructive/10 flex items-center justify-center">
					<AlertTriangle class="w-4 h-4 text-destructive" />
				</div>
				Delete Repository
			</Dialog.Title>
			<Dialog.Description>
				Are you sure you want to delete <strong>{confirmRemoveRepo?.fullName}</strong>? This will permanently delete all tasks, epics, and logs associated with this repository. This action cannot be undone.
			</Dialog.Description>
		</Dialog.Header>
		<Dialog.Footer>
			<Button variant="outline" onclick={() => (confirmRemoveRepo = null)}>Cancel</Button>
			<Button variant="destructive" onclick={handleConfirmRemove} disabled={removing !== null}>
				{#if removing}
					Deleting...
				{:else}
					Delete Repository
				{/if}
			</Button>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>
