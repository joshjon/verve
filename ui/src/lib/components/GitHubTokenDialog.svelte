<script lang="ts">
	import { client } from '$lib/api-client';
	import { Button } from '$lib/components/ui/button';
	import * as Dialog from '$lib/components/ui/dialog';
	import { Key, Eye, EyeOff, Loader2, X, Check, Trash2, Shield, AlertTriangle, Settings, Cpu } from 'lucide-svelte';

	let {
		open = $bindable(false),
		required = false,
		onconfigured
	}: {
		open: boolean;
		required?: boolean;
		onconfigured?: () => void;
	} = $props();

	// GitHub token state
	let token = $state('');
	let showToken = $state(false);
	let saving = $state(false);
	let deleting = $state(false);
	let loading = $state(false);
	let configured = $state(false);
	let fineGrained = $state(false);
	let error = $state<string | null>(null);
	let success = $state<string | null>(null);

	// Default model state
	let defaultModel = $state('');
	let modelConfigured = $state(false);
	let modelLoading = $state(false);
	let modelSaving = $state(false);

	// Available model options (fetched from API)
	let modelOptions = $state<{ value: string; label: string }[]>([]);
	let modelsLoading = $state(false);

	// Whether both required settings are satisfied
	const allConfigured = $derived(configured && modelConfigured);

	$effect(() => {
		if (open) {
			checkStatus();
			loadDefaultModel();
			loadModelOptions();
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
			fineGrained = status.fine_grained ?? false;
		} catch (e) {
			error = (e as Error).message;
		} finally {
			loading = false;
		}
	}

	async function loadDefaultModel() {
		modelLoading = true;
		try {
			const res = await client.getDefaultModel();
			defaultModel = res.model;
			modelConfigured = res.configured;
		} catch {
			// Ignore - defaults stay empty
		} finally {
			modelLoading = false;
		}
	}

	async function loadModelOptions() {
		modelsLoading = true;
		try {
			modelOptions = await client.listModels();
		} catch {
			// Fall back to built-in defaults if API fails
			modelOptions = [
				{ value: 'haiku', label: 'Haiku' },
				{ value: 'sonnet', label: 'Sonnet' },
				{ value: 'opus', label: 'Opus' }
			];
		} finally {
			modelsLoading = false;
		}
	}

	async function handleModelChange(e: Event) {
		const value = (e.target as HTMLSelectElement).value;
		modelSaving = true;
		error = null;
		success = null;
		try {
			await client.saveDefaultModel(value);
			defaultModel = value;
			modelConfigured = true;
			const label = modelOptions.find((m) => m.value === value)?.label || value;
			success = `Default model set to ${label}`;
			setTimeout(() => { success = null; }, 3000);
			// If we were in required mode and now everything is configured, notify parent
			if (required && configured) {
				onconfigured?.();
			}
		} catch (e) {
			error = (e as Error).message;
		} finally {
			modelSaving = false;
		}
	}

	async function handleSave(e: SubmitEvent) {
		e.preventDefault();
		saving = true;
		error = null;
		success = null;
		try {
			const isFineGrained = token.startsWith('github_pat_');
			await client.saveGitHubToken(token);
			configured = true;
			fineGrained = isFineGrained;
			token = '';
			showToken = false;
			success = 'GitHub token saved successfully';
			// If we were in required mode and now everything is configured, notify parent
			if (required && modelConfigured) {
				onconfigured?.();
			} else if (!required) {
				onconfigured?.();
			}
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
		if (required && !isOpen && !allConfigured) return;
		open = isOpen;
	}
</script>

<Dialog.Root open={open} onOpenChange={handleOpenChange}>
	<Dialog.Content
		class="sm:max-w-[520px]"
		showCloseButton={!required || allConfigured}
		onInteractOutside={(e: PointerEvent) => {
			if (required && !allConfigured) e.preventDefault();
		}}
		onEscapeKeydown={(e: KeyboardEvent) => {
			if (required && !allConfigured) e.preventDefault();
		}}
	>
		<Dialog.Header>
			<Dialog.Title class="flex items-center gap-2">
				<div class="w-8 h-8 rounded-lg bg-primary/10 flex items-center justify-center">
					<Settings class="w-4 h-4 text-primary" />
				</div>
				Settings
			</Dialog.Title>
			<Dialog.Description>
				{#if required && !allConfigured}
					{#if !configured && !modelConfigured}
						A GitHub token and default model are required to get started.
					{:else if !configured}
						A GitHub personal access token is required to get started.
					{:else}
						A default model must be selected to get started.
					{/if}
				{:else}
					Configure Verve settings.
				{/if}
			</Dialog.Description>
		</Dialog.Header>

		<div class="py-4 space-y-6">
			<!-- Default Model Section -->
			<div class="space-y-3">
				<div class="flex items-center gap-2">
					<Cpu class="w-4 h-4 text-muted-foreground" />
					<span class="text-sm font-medium">Default Model</span>
					{#if required && !modelConfigured}
						<span class="text-xs text-destructive font-medium">Required</span>
					{/if}
				</div>
				<p class="text-xs text-muted-foreground">
					Set the default AI model for new tasks. Can be overridden per task.
				</p>
				<div class="flex items-center gap-2">
					{#if modelsLoading || modelLoading}
						<div class="flex-1 flex items-center gap-2 text-muted-foreground">
							<Loader2 class="w-4 h-4 animate-spin" />
							<span class="text-sm">Loading models...</span>
						</div>
					{:else}
						<select
							class="flex-1 border rounded-lg px-3 py-2 bg-background text-foreground text-sm focus:outline-none focus:ring-2 focus:ring-ring transition-shadow"
							class:border-destructive={required && !modelConfigured}
							value={defaultModel}
							onchange={handleModelChange}
							disabled={modelSaving}
						>
							{#if !modelConfigured}
								<option value="" disabled>Select a model...</option>
							{/if}
							{#each modelOptions as option}
								<option value={option.value}>{option.label}</option>
							{/each}
						</select>
					{/if}
					{#if modelSaving}
						<Loader2 class="w-4 h-4 animate-spin text-muted-foreground" />
					{/if}
				</div>
			</div>

			<div class="border-t"></div>

			<!-- GitHub Token Section -->
			<div class="space-y-3">
				<div class="flex items-center gap-2">
					<Key class="w-4 h-4 text-muted-foreground" />
					<span class="text-sm font-medium">GitHub Token</span>
					{#if required && !configured}
						<span class="text-xs text-destructive font-medium">Required</span>
					{/if}
				</div>

				{#if loading}
					<div class="flex items-center justify-center py-6 gap-2 text-muted-foreground">
						<Loader2 class="w-4 h-4 animate-spin" />
						<span class="text-sm">Checking token status...</span>
					</div>
				{:else if configured}
					<div class="space-y-2">
						<div class="flex items-center justify-between p-3 rounded-lg border bg-muted/20">
							<div class="flex items-center gap-2 text-sm">
								<Check class="w-4 h-4 text-green-500" />
								<span>Token configured</span>
								{#if fineGrained}
									<span class="text-xs text-muted-foreground bg-muted px-1.5 py-0.5 rounded">Fine-grained</span>
								{:else}
									<span class="text-xs text-muted-foreground bg-muted px-1.5 py-0.5 rounded">Classic</span>
								{/if}
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
						{#if fineGrained}
							<div class="flex items-start gap-2 p-2.5 rounded-lg bg-amber-500/10 text-xs text-amber-700 dark:text-amber-400">
								<AlertTriangle class="w-3.5 h-3.5 shrink-0 mt-0.5" />
								<span>Fine-grained tokens cannot access CI check status due to a GitHub limitation. Automatic CI failure detection and retry is disabled. Use a classic token with <code class="bg-amber-500/20 px-1 py-0.5 rounded text-[11px]">repo</code> scope for full CI visibility.</span>
							</div>
						{/if}
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
									class:border-destructive={required && !configured}
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
							<div class="text-xs text-muted-foreground space-y-2">
							<p class="font-medium">Required permissions:</p>
							<div class="rounded-md border px-3 py-2">
								<span class="font-medium text-foreground/80">Classic token</span>
								<span class="ml-1.5"><code class="bg-muted px-1 py-0.5 rounded text-[11px]">repo</code> scope</span>
							</div>
							<div class="rounded-md border px-3 py-2 space-y-1.5">
								<p class="font-medium text-foreground/80">Fine-grained token</p>
								<ul class="ml-3.5 list-disc space-y-0.5">
									<li><span class="text-foreground/70">Contents</span> — Read and write</li>
									<li><span class="text-foreground/70">Pull requests</span> — Read and write</li>
									<li><span class="text-foreground/70">Metadata</span> — Read-only</li>
								</ul>
								<p class="text-muted-foreground/70 pt-0.5">
									Note: CI check status (GitHub Actions) is not available with fine-grained tokens due to a GitHub limitation. Use a classic token for full CI visibility.
								</p>
							</div>
						</div>
						</div>

						{#if required && !configured}
							<Dialog.Footer class="gap-2 mt-4">
								<Button type="submit" disabled={saving || !token.trim()} class="gap-2">
									{#if saving}
										<Loader2 class="w-4 h-4 animate-spin" />
									{/if}
									Save Token
								</Button>
							</Dialog.Footer>
						{:else}
							<Dialog.Footer class="gap-2 mt-4">
								<Button
									type="button"
									variant="outline"
									onclick={() => (open = false)}
									disabled={saving}
								>
									Cancel
								</Button>
								<Button type="submit" disabled={saving || !token.trim()} class="gap-2">
									{#if saving}
										<Loader2 class="w-4 h-4 animate-spin" />
									{/if}
									Save Token
								</Button>
							</Dialog.Footer>
						{/if}
					</form>
				{/if}

				<div class="flex items-center gap-1.5 text-[11px] text-muted-foreground/60 mt-2">
					<Shield class="w-3 h-3 flex-shrink-0" />
					<span>Tokens are encrypted with AES-256 before being stored</span>
				</div>
			</div>

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

		{#if !required || allConfigured}
			<Dialog.Footer class="mt-4">
				<Button onclick={() => (open = false)}>Done</Button>
			</Dialog.Footer>
		{/if}
	</Dialog.Content>
</Dialog.Root>
