<script lang="ts">
	import { client } from '$lib/api-client';
	import { Button } from '$lib/components/ui/button';
	import * as Dialog from '$lib/components/ui/dialog';
	import { Key, Eye, EyeOff, Loader2, X, Check, Trash2, Shield } from 'lucide-svelte';

	let {
		open = $bindable(false),
		required = false,
		onconfigured
	}: {
		open: boolean;
		required?: boolean;
		onconfigured?: () => void;
	} = $props();

	let token = $state('');
	let showToken = $state(false);
	let saving = $state(false);
	let deleting = $state(false);
	let loading = $state(false);
	let configured = $state(false);
	let error = $state<string | null>(null);
	let success = $state<string | null>(null);

	$effect(() => {
		if (open) {
			checkStatus();
		} else {
			token = '';
			showToken = false;
			error = null;
			success = null;
		}
	});

	async function checkStatus() {
		loading = true;
		error = null;
		try {
			const status = await client.getGitHubTokenStatus();
			configured = status.configured;
		} catch (e) {
			error = (e as Error).message;
		} finally {
			loading = false;
		}
	}

	async function handleSave(e: SubmitEvent) {
		e.preventDefault();
		saving = true;
		error = null;
		success = null;
		try {
			await client.saveGitHubToken(token);
			configured = true;
			token = '';
			showToken = false;
			success = 'GitHub token saved successfully';
			onconfigured?.();
		} catch (e) {
			error = (e as Error).message;
		} finally {
			saving = false;
		}
	}

	async function handleDelete() {
		deleting = true;
		error = null;
		success = null;
		try {
			await client.deleteGitHubToken();
			configured = false;
			success = 'GitHub token removed';
		} catch (e) {
			error = (e as Error).message;
		} finally {
			deleting = false;
		}
	}

	function handleOpenChange(isOpen: boolean) {
		if (required && !isOpen && !configured) return;
		open = isOpen;
	}
</script>

<Dialog.Root open={open} onOpenChange={handleOpenChange}>
	<Dialog.Content
		class="sm:max-w-[450px]"
		showCloseButton={!required || configured}
		onInteractOutside={(e) => {
			if (required && !configured) e.preventDefault();
		}}
		onEscapeKeydown={(e) => {
			if (required && !configured) e.preventDefault();
		}}
	>
		<Dialog.Header>
			<Dialog.Title class="flex items-center gap-2">
				<div class="w-8 h-8 rounded-lg bg-primary/10 flex items-center justify-center">
					<Key class="w-4 h-4 text-primary" />
				</div>
				GitHub Token
			</Dialog.Title>
			<Dialog.Description>
				{#if required && !configured}
					A GitHub personal access token is required to get started. It is used for repository
					access and PR management.
				{:else}
					Configure your GitHub personal access token for repository access and PR management.
				{/if}
			</Dialog.Description>
		</Dialog.Header>

		<div class="py-4 space-y-4">
			{#if loading}
				<div class="flex items-center justify-center py-6 gap-2 text-muted-foreground">
					<Loader2 class="w-4 h-4 animate-spin" />
					<span class="text-sm">Checking token status...</span>
				</div>
			{:else if configured}
				<div class="flex items-center justify-between p-3 rounded-lg border bg-muted/20">
					<div class="flex items-center gap-2 text-sm">
						<Check class="w-4 h-4 text-green-500" />
						<span>GitHub token is configured</span>
					</div>
					<Button
						variant="ghost"
						size="sm"
						class="text-destructive hover:text-destructive hover:bg-destructive/10 gap-1.5"
						onclick={handleDelete}
						disabled={deleting}
					>
						{#if deleting}
							<Loader2 class="w-3.5 h-3.5 animate-spin" />
						{:else}
							<Trash2 class="w-3.5 h-3.5" />
						{/if}
						Remove
					</Button>
				</div>

				<form onsubmit={handleSave}>
					<p class="text-xs text-muted-foreground mb-2">Replace with a new token:</p>
					<div class="flex gap-2">
						<div class="relative flex-1">
							<input
								type={showToken ? 'text' : 'password'}
								bind:value={token}
								class="w-full border rounded-lg pl-3 pr-9 py-2 bg-background text-foreground text-sm focus:outline-none focus:ring-2 focus:ring-ring transition-shadow"
								placeholder="ghp_..."
								autocomplete="off"
							/>
							<button
								type="button"
								class="absolute right-2.5 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground transition-colors"
								onclick={() => (showToken = !showToken)}
								tabindex={-1}
							>
								{#if showToken}
									<EyeOff class="w-4 h-4" />
								{:else}
									<Eye class="w-4 h-4" />
								{/if}
							</button>
						</div>
						<Button type="submit" size="sm" disabled={saving || !token.trim()}>
							{#if saving}
								<Loader2 class="w-4 h-4 animate-spin" />
							{:else}
								Save
							{/if}
						</Button>
					</div>
				</form>
			{:else}
				<form onsubmit={handleSave}>
					<div class="space-y-3">
						<div class="relative">
							<input
								type={showToken ? 'text' : 'password'}
								bind:value={token}
								class="w-full border rounded-lg pl-3 pr-9 py-2 bg-background text-foreground focus:outline-none focus:ring-2 focus:ring-ring transition-shadow"
								placeholder="ghp_..."
								autocomplete="off"
							/>
							<button
								type="button"
								class="absolute right-2.5 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground transition-colors"
								onclick={() => (showToken = !showToken)}
								tabindex={-1}
							>
								{#if showToken}
									<EyeOff class="w-4 h-4" />
								{:else}
									<Eye class="w-4 h-4" />
								{/if}
							</button>
						</div>
						<p class="text-xs text-muted-foreground">
							Requires <code class="bg-muted px-1 py-0.5 rounded text-[11px]">repo</code> scope
							for repository access and PR management.
						</p>
					</div>

					<Dialog.Footer class="gap-2 mt-4">
						{#if !required}
							<Button
								type="button"
								variant="outline"
								onclick={() => (open = false)}
								disabled={saving}
							>
								Cancel
							</Button>
						{/if}
						<Button type="submit" disabled={saving || !token.trim()} class="gap-2">
							{#if saving}
								<Loader2 class="w-4 h-4 animate-spin" />
							{/if}
							Save Token
						</Button>
					</Dialog.Footer>
				</form>
			{/if}

			{#if error}
				<div
					class="bg-destructive/10 text-destructive text-sm p-3 rounded-lg flex items-center gap-2"
				>
					<X class="w-4 h-4 flex-shrink-0" />
					{error}
				</div>
			{/if}

			{#if success}
				<div
					class="bg-green-500/10 text-green-600 dark:text-green-400 text-sm p-3 rounded-lg flex items-center gap-2"
				>
					<Check class="w-4 h-4 flex-shrink-0" />
					{success}
				</div>
			{/if}

		</div>

		{#if configured}
			<Dialog.Footer>
				<Button variant="outline" onclick={() => (open = false)}>Done</Button>
			</Dialog.Footer>
		{/if}

		<div class="flex items-center justify-center gap-1.5 text-[11px] text-muted-foreground/60 border-t pt-3 -mx-6 px-6">
			<Shield class="w-3 h-3 flex-shrink-0" />
			<span>Encrypted with AES-256 before being stored</span>
		</div>
	</Dialog.Content>
</Dialog.Root>
