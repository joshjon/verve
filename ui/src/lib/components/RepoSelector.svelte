<script lang="ts">
	import { repoStore } from '$lib/stores/repos.svelte';
	import { ChevronDown, GitBranch, Plus, Trash2 } from 'lucide-svelte';
	import * as Popover from '$lib/components/ui/popover';
	import { Button } from '$lib/components/ui/button';
	import AddRepoDialog from './AddRepoDialog.svelte';
	import { client } from '$lib/api-client';

	let open = $state(false);
	let openAddRepo = $state(false);
	let removing = $state<string | null>(null);

	async function handleRemove(e: MouseEvent, repoId: string) {
		e.stopPropagation();
		removing = repoId;
		try {
			await client.removeRepo(repoId);
			repoStore.removeRepo(repoId);
		} catch {
			// Ignore errors for now
		} finally {
			removing = null;
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
							onclick={(e) => handleRemove(e, repo.id)}
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
