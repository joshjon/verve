<script lang="ts">
	import { page } from '$app/stores';
	import { onMount } from 'svelte';
	import { client } from '$lib/api-client';
	import type { Task, TaskStatus } from '$lib/models/task';
	import { Button } from '$lib/components/ui/button';
	import { Badge } from '$lib/components/ui/badge';
	import * as Card from '$lib/components/ui/card';
	import { goto } from '$app/navigation';
	import { marked } from 'marked';
	import {
		ArrowLeft,
		Clock,
		Play,
		Eye,
		GitMerge,
		CheckCircle,
		XCircle,
		FileText,
		GitPullRequest,
		Link2,
		Terminal,
		RefreshCw,
		X,
		Calendar,
		Loader2,
		Target,
		DollarSign,
		AlertTriangle,
		ChevronDown,
		ChevronRight,
		Activity
	} from 'lucide-svelte';
	import type { ComponentType } from 'svelte';
	import type { Icon } from 'lucide-svelte';

	// Configure marked for safe rendering
	marked.setOptions({
		breaks: true,
		gfm: true
	});

	let task = $state<Task | null>(null);
	let logs = $state<string[]>([]);
	let loading = $state(true);
	let error = $state<string | null>(null);
	let syncing = $state(false);
	let closing = $state(false);
	let showCloseForm = $state(false);
	let closeReason = $state('');
	let logsContainer: HTMLDivElement | null = $state(null);
	let autoScroll = $state(true);
	let lastLogCount = $state(0);
	let showRetryContext = $state(false);

	interface AgentStatusParsed {
		files_modified?: string[];
		tests_status?: string;
		confidence?: string;
		blockers?: string[];
		criteria_met?: string[];
		notes?: string;
	}

	const parsedAgentStatus = $derived.by(() => {
		if (!task?.agent_status) return null;
		try {
			return JSON.parse(task.agent_status) as AgentStatusParsed;
		} catch {
			return null;
		}
	});

	const confidenceColor = $derived.by(() => {
		if (!parsedAgentStatus?.confidence) return '';
		switch (parsedAgentStatus.confidence) {
			case 'high':
				return 'bg-green-500';
			case 'medium':
				return 'bg-amber-500';
			case 'low':
				return 'bg-red-500';
			default:
				return 'bg-gray-500';
		}
	});

	interface PrereqFailure {
		detected: string[];
		missing: {
			tool: string;
			reason: string;
			install: string;
		}[];
	}

	const parsedPrereqFailure = $derived.by(() => {
		if (!task?.close_reason || task.status !== 'failed') return null;
		try {
			const parsed = JSON.parse(task.close_reason);
			if (parsed.missing && Array.isArray(parsed.missing)) {
				return parsed as PrereqFailure;
			}
			return null;
		} catch {
			return null;
		}
	});

	function formatCost(cost: number): string {
		return `$${cost.toFixed(2)}`;
	}

	const taskId = $derived($page.params.id);

	const statusConfig: Record<
		TaskStatus,
		{ label: string; icon: ComponentType<Icon>; bgClass: string; textClass: string }
	> = {
		pending: {
			label: 'Pending',
			icon: Clock,
			bgClass: 'bg-amber-500',
			textClass: 'text-amber-600 dark:text-amber-400'
		},
		running: {
			label: 'Running',
			icon: Play,
			bgClass: 'bg-blue-500',
			textClass: 'text-blue-600 dark:text-blue-400'
		},
		review: {
			label: 'In Review',
			icon: Eye,
			bgClass: 'bg-purple-500',
			textClass: 'text-purple-600 dark:text-purple-400'
		},
		merged: {
			label: 'Merged',
			icon: GitMerge,
			bgClass: 'bg-green-500',
			textClass: 'text-green-600 dark:text-green-400'
		},
		closed: {
			label: 'Closed',
			icon: CheckCircle,
			bgClass: 'bg-gray-500',
			textClass: 'text-gray-600 dark:text-gray-400'
		},
		failed: {
			label: 'Failed',
			icon: XCircle,
			bgClass: 'bg-red-500',
			textClass: 'text-red-600 dark:text-red-400'
		}
	};

	const canClose = $derived(task && !['closed', 'merged', 'failed'].includes(task.status));

	const currentStatusConfig = $derived(task ? statusConfig[task.status] : null);
	const StatusIcon = $derived(currentStatusConfig?.icon ?? Clock);

	// Render description as markdown
	const renderedDescription = $derived(task ? marked(task.description) : '');

	// Auto-scroll logs when new logs arrive
	$effect(() => {
		if (logs.length > lastLogCount) {
			lastLogCount = logs.length;
			if (autoScroll && logsContainer) {
				requestAnimationFrame(() => {
					if (logsContainer) {
						logsContainer.scrollTop = logsContainer.scrollHeight;
					}
				});
			}
		}
	});

	function handleLogsScroll(e: Event) {
		const el = e.target as HTMLDivElement;
		// Check if user is near bottom (within 50px)
		const isNearBottom = el.scrollHeight - el.scrollTop - el.clientHeight < 50;
		autoScroll = isNearBottom;
	}

	onMount(() => {
		loadTask();

		// Task metadata updates via global SSE.
		const es = new EventSource(client.eventsURL());

		es.addEventListener('task_updated', (e) => {
			const event = JSON.parse(e.data);
			if (event.task?.id === taskId && task) {
				task = { ...event.task, logs: task.logs };
			}
		});

		// Log streaming via dedicated SSE endpoint.
		// Uses double-buffering so reconnects replace logs without flashing.
		let logBuffer: string[] = [];
		let historicalDone = false;

		const logsES = new EventSource(client.taskLogsURL(taskId));

		logsES.addEventListener('open', () => {
			logBuffer = [];
			historicalDone = false;
		});

		logsES.addEventListener('logs_appended', (e) => {
			const event = JSON.parse(e.data);
			if (historicalDone) {
				logs = [...logs, ...event.logs];
			} else {
				logBuffer.push(...event.logs);
			}
		});

		logsES.addEventListener('logs_done', () => {
			logs = logBuffer;
			logBuffer = [];
			historicalDone = true;
		});

		return () => {
			es.close();
			logsES.close();
		};
	});

	async function loadTask() {
		try {
			task = await client.getTask(taskId);
			error = null;
		} catch (e) {
			error = (e as Error).message;
		} finally {
			loading = false;
		}
	}

	async function syncStatus() {
		if (!task || syncing) return;
		syncing = true;
		try {
			task = await client.syncTask(task.id);
		} catch (e) {
			error = (e as Error).message;
		} finally {
			syncing = false;
		}
	}

	async function handleClose() {
		if (!task || closing) return;
		closing = true;
		try {
			task = await client.closeTask(task.id, closeReason || undefined);
			showCloseForm = false;
			closeReason = '';
		} catch (e) {
			error = (e as Error).message;
		} finally {
			closing = false;
		}
	}

	function formatDate(dateStr: string): string {
		return new Date(dateStr).toLocaleString();
	}

	function formatRelativeTime(dateStr: string): string {
		const date = new Date(dateStr);
		const now = new Date();
		const diffMs = now.getTime() - date.getTime();
		const diffMins = Math.floor(diffMs / 60000);
		const diffHours = Math.floor(diffMs / 3600000);
		const diffDays = Math.floor(diffMs / 86400000);

		if (diffMins < 1) return 'Just now';
		if (diffMins < 60) return `${diffMins}m ago`;
		if (diffHours < 24) return `${diffHours}h ago`;
		return `${diffDays}d ago`;
	}
</script>

<div class="p-6 max-w-4xl mx-auto">
	<Button variant="ghost" onclick={() => goto('/')} class="mb-6 gap-2 -ml-2">
		<ArrowLeft class="w-4 h-4" />
		Back to Dashboard
	</Button>

	{#if loading}
		<div class="flex flex-col items-center justify-center py-16">
			<Loader2 class="w-8 h-8 animate-spin text-primary mb-4" />
			<p class="text-muted-foreground">Loading task...</p>
		</div>
	{:else if error && !task}
		<div
			class="bg-destructive/10 text-destructive p-4 rounded-lg flex items-center gap-3 border border-destructive/20"
		>
			<XCircle class="w-5 h-5 flex-shrink-0" />
			<span>{error}</span>
		</div>
	{:else if task}
		<div class="space-y-6">
			<!-- Header -->
			<div class="flex items-start justify-between gap-4">
				<div class="flex-1">
					<div class="flex items-center gap-3 mb-2">
						<span class="font-mono text-sm text-muted-foreground bg-muted px-2 py-1 rounded">
							{task.id}
						</span>
						<Badge class="{currentStatusConfig?.bgClass} text-white gap-1">
							<StatusIcon class="w-3 h-3" />
							{currentStatusConfig?.label}
						</Badge>
					</div>
					<div class="flex items-center gap-4 text-sm text-muted-foreground">
						<span class="flex items-center gap-1">
							<Calendar class="w-4 h-4" />
							Created {formatRelativeTime(task.created_at)}
						</span>
					</div>
				</div>
				<div class="flex items-center gap-2">
					{#if canClose}
						{#if showCloseForm}
							<Button size="sm" variant="ghost" onclick={() => (showCloseForm = false)} class="gap-1">
								<X class="w-4 h-4" />
								Cancel
							</Button>
						{:else}
							<Button size="sm" variant="outline" onclick={() => (showCloseForm = true)} class="gap-1">
								<XCircle class="w-4 h-4" />
								Close Task
							</Button>
						{/if}
					{/if}
				</div>
			</div>

			<!-- Close Form -->
			{#if showCloseForm}
				<Card.Root class="border-destructive/30 bg-destructive/5">
					<Card.Header class="pb-3">
						<Card.Title class="text-base flex items-center gap-2">
							<XCircle class="w-4 h-4 text-destructive" />
							Close Task
						</Card.Title>
					</Card.Header>
					<Card.Content>
						<div class="space-y-4">
							<div>
								<label for="close-reason" class="text-sm font-medium mb-2 block">
									Reason (optional)
								</label>
								<textarea
									id="close-reason"
									bind:value={closeReason}
									class="w-full border rounded-lg p-3 min-h-[80px] bg-background text-foreground resize-none focus:outline-none focus:ring-2 focus:ring-ring"
									placeholder="Why is this task being closed?"
									disabled={closing}
								></textarea>
							</div>
							<div class="flex justify-end gap-2">
								<Button variant="outline" onclick={() => (showCloseForm = false)} disabled={closing}>
									Cancel
								</Button>
								<Button variant="destructive" onclick={handleClose} disabled={closing} class="gap-2">
									{#if closing}
										<Loader2 class="w-4 h-4 animate-spin" />
										Closing...
									{:else}
										<XCircle class="w-4 h-4" />
										Close Task
									{/if}
								</Button>
							</div>
						</div>
					</Card.Content>
				</Card.Root>
			{/if}

			<!-- Retry Info -->
			{#if task.attempt > 1}
				<Card.Root class="border-amber-500/30 bg-amber-500/5">
					<Card.Header class="pb-3">
						<Card.Title class="text-base flex items-center gap-2">
							<RefreshCw class="w-4 h-4 text-amber-500" />
							Retry Attempt {task.attempt} of {task.max_attempts}
							{#if task.consecutive_failures >= 2}
								<Badge class="bg-red-500 text-white gap-1 text-xs">
									<AlertTriangle class="w-3 h-3" />
									{task.consecutive_failures} consecutive
								</Badge>
							{/if}
						</Card.Title>
					</Card.Header>
					<Card.Content class="space-y-3">
						{#if task.retry_reason}
							<p class="text-sm text-muted-foreground">{task.retry_reason}</p>
						{/if}
						{#if task.retry_context}
							<div>
								<button
									type="button"
									class="flex items-center gap-1.5 text-sm text-muted-foreground hover:text-foreground transition-colors"
									onclick={() => (showRetryContext = !showRetryContext)}
								>
									{#if showRetryContext}
										<ChevronDown class="w-4 h-4" />
									{:else}
										<ChevronRight class="w-4 h-4" />
									{/if}
									CI Failure Logs
								</button>
								{#if showRetryContext}
									<pre class="mt-2 text-xs font-mono bg-zinc-950 text-green-400 rounded-lg p-3 max-h-48 overflow-y-auto whitespace-pre-wrap">{task.retry_context}</pre>
								{/if}
							</div>
						{/if}
					</Card.Content>
				</Card.Root>
			{/if}

			<!-- Description -->
			<Card.Root>
				<Card.Header class="pb-3">
					<Card.Title class="text-base flex items-center gap-2">
						<FileText class="w-4 h-4 text-muted-foreground" />
						Description
					</Card.Title>
				</Card.Header>
				<Card.Content class="max-h-64 overflow-y-auto">
					<div class="prose prose-sm dark:prose-invert max-w-none">
						{@html renderedDescription}
					</div>
				</Card.Content>
			</Card.Root>

			<!-- Acceptance Criteria -->
			{#if task.acceptance_criteria}
				<Card.Root class="border-blue-500/30 bg-blue-500/5">
					<Card.Header class="pb-3">
						<Card.Title class="text-base flex items-center gap-2">
							<Target class="w-4 h-4 text-blue-500" />
							Acceptance Criteria
						</Card.Title>
					</Card.Header>
					<Card.Content>
						<p class="whitespace-pre-wrap text-sm text-muted-foreground">{task.acceptance_criteria}</p>
					</Card.Content>
				</Card.Root>
			{/if}

			<!-- Prerequisite Failure -->
			{#if parsedPrereqFailure}
				<Card.Root class="border-red-500/30 bg-red-500/5">
					<Card.Header class="pb-3">
						<Card.Title class="text-base flex items-center gap-2">
							<AlertTriangle class="w-4 h-4 text-red-500" />
							Missing Prerequisites
							<Badge class="bg-red-500 text-white text-xs">
								{parsedPrereqFailure.missing.length} missing
							</Badge>
						</Card.Title>
					</Card.Header>
					<Card.Content class="space-y-3">
						<p class="text-sm text-muted-foreground">
							The agent detected project types that require tools not installed in the worker image.
							Update your agent Docker image to include the missing tools, then retry the task.
						</p>
						{#each parsedPrereqFailure.missing as item}
							<div class="rounded-lg border bg-background p-3 space-y-1">
								<div class="flex items-center gap-2">
									<Badge variant="destructive" class="text-xs">{item.tool}</Badge>
								</div>
								<p class="text-sm text-muted-foreground">{item.reason}</p>
								<p class="text-xs text-muted-foreground font-mono">{item.install}</p>
							</div>
						{/each}
					</Card.Content>
				</Card.Root>
			<!-- Close Reason -->
			{:else if task.close_reason}
				<Card.Root class="border-gray-500/30">
					<Card.Header class="pb-3">
						<Card.Title class="text-base flex items-center gap-2">
							<CheckCircle class="w-4 h-4 text-gray-500" />
							Close Reason
						</Card.Title>
					</Card.Header>
					<Card.Content>
						<p class="whitespace-pre-wrap text-muted-foreground">{task.close_reason}</p>
					</Card.Content>
				</Card.Root>
			{/if}

			<!-- Pull Request -->
			{#if task.pull_request_url}
				<Card.Root class={task.status === 'merged' ? 'border-green-500/30 bg-green-500/10' : 'border-purple-500/30 bg-purple-500/5'}>
					<Card.Header class="pb-3">
						<Card.Title class="text-base flex items-center gap-2">
							{#if task.status === 'merged'}
								<GitMerge class="w-4 h-4 text-green-500" />
								Pull Request (Merged)
							{:else}
								<GitPullRequest class="w-4 h-4 text-purple-500" />
								Pull Request
							{/if}
						</Card.Title>
					</Card.Header>
					<Card.Content>
						<div class="flex items-center gap-4">
							<a
								href={task.pull_request_url}
								class="text-primary hover:underline font-medium flex items-center gap-2"
								target="_blank"
								rel="noopener noreferrer"
							>
								<GitPullRequest class="w-4 h-4" />
								PR #{task.pr_number}
							</a>
							{#if task.status !== 'merged'}
								<Button
									size="sm"
									variant="outline"
									onclick={syncStatus}
									disabled={syncing}
									class="gap-2"
								>
									<RefreshCw class="w-4 h-4 {syncing ? 'animate-spin' : ''}" />
									{syncing ? 'Syncing...' : 'Sync Status'}
								</Button>
							{/if}
						</div>
					</Card.Content>
				</Card.Root>
			{/if}

			<!-- Dependencies -->
			{#if task.depends_on && task.depends_on.length > 0}
				<Card.Root>
					<Card.Header class="pb-3">
						<Card.Title class="text-base flex items-center gap-2">
							<Link2 class="w-4 h-4 text-muted-foreground" />
							Dependencies
							<span class="text-xs text-muted-foreground font-normal">
								({task.depends_on.length})
							</span>
						</Card.Title>
					</Card.Header>
					<Card.Content>
						<div class="flex flex-wrap gap-2">
							{#each task.depends_on as depId}
								<button
									class="inline-flex items-center gap-1 px-3 py-1.5 rounded-md bg-muted hover:bg-accent text-sm font-mono transition-colors"
									onclick={() => goto(`/tasks/${depId}`)}
								>
									<Link2 class="w-3 h-3" />
									{depId}
								</button>
							{/each}
						</div>
					</Card.Content>
				</Card.Root>
			{/if}

			<!-- Agent Status -->
			{#if parsedAgentStatus}
				<Card.Root class="border-indigo-500/30 bg-indigo-500/5">
					<Card.Header class="pb-3">
						<Card.Title class="text-base flex items-center gap-2">
							<Activity class="w-4 h-4 text-indigo-500" />
							Agent Status
							{#if parsedAgentStatus.confidence}
								<Badge class="{confidenceColor} text-white text-xs">
									{parsedAgentStatus.confidence} confidence
								</Badge>
							{/if}
							{#if parsedAgentStatus.tests_status}
								<Badge variant={parsedAgentStatus.tests_status === 'pass' ? 'default' : 'destructive'} class="text-xs gap-1">
									{#if parsedAgentStatus.tests_status === 'pass'}
										<CheckCircle class="w-3 h-3" />
									{:else if parsedAgentStatus.tests_status === 'fail'}
										<XCircle class="w-3 h-3" />
									{/if}
									Tests: {parsedAgentStatus.tests_status}
								</Badge>
							{/if}
						</Card.Title>
					</Card.Header>
					<Card.Content class="space-y-3">
						{#if parsedAgentStatus.criteria_met && parsedAgentStatus.criteria_met.length > 0}
							<div>
								<p class="text-sm font-medium mb-1.5">Criteria Met</p>
								<div class="flex flex-wrap gap-1.5">
									{#each parsedAgentStatus.criteria_met as criterion}
										<Badge variant="secondary" class="gap-1 text-xs">
											<CheckCircle class="w-3 h-3 text-green-500" />
											{criterion}
										</Badge>
									{/each}
								</div>
							</div>
						{/if}
						{#if parsedAgentStatus.blockers && parsedAgentStatus.blockers.length > 0}
							<div>
								<p class="text-sm font-medium mb-1.5">Blockers</p>
								<div class="flex flex-wrap gap-1.5">
									{#each parsedAgentStatus.blockers as blocker}
										<Badge variant="destructive" class="gap-1 text-xs">
											<AlertTriangle class="w-3 h-3" />
											{blocker}
										</Badge>
									{/each}
								</div>
							</div>
						{/if}
						{#if parsedAgentStatus.files_modified && parsedAgentStatus.files_modified.length > 0}
							<div>
								<p class="text-sm font-medium mb-1.5">Files Modified</p>
								<div class="flex flex-wrap gap-1.5">
									{#each parsedAgentStatus.files_modified as file}
										<span class="text-xs font-mono bg-muted px-2 py-1 rounded">{file}</span>
									{/each}
								</div>
							</div>
						{/if}
						{#if parsedAgentStatus.notes}
							<div>
								<p class="text-sm font-medium mb-1">Notes</p>
								<p class="text-sm text-muted-foreground">{parsedAgentStatus.notes}</p>
							</div>
						{/if}
					</Card.Content>
				</Card.Root>
			{/if}

			<!-- Logs -->
			<Card.Root>
				<Card.Header class="pb-3">
					<Card.Title class="text-base flex items-center gap-2">
						<Terminal class="w-4 h-4 text-muted-foreground" />
						Logs
						{#if task.status === 'running'}
							<span class="flex items-center gap-1 text-xs text-blue-500 font-normal">
								<span class="relative flex h-2 w-2">
									<span
										class="animate-ping absolute inline-flex h-full w-full rounded-full bg-blue-400 opacity-75"
									></span>
									<span class="relative inline-flex rounded-full h-2 w-2 bg-blue-500"></span>
								</span>
								Live
							</span>
						{/if}
					</Card.Title>
				</Card.Header>
				<Card.Content>
					<div
						bind:this={logsContainer}
						onscroll={handleLogsScroll}
						class="h-[400px] w-full rounded-lg border bg-zinc-950 p-4 overflow-y-auto"
					>
						{#if logs.length > 0}
							<pre class="text-green-400 text-xs font-mono whitespace-pre-wrap leading-relaxed">{logs.join('\n')}</pre>
						{:else}
							<div class="flex flex-col items-center justify-center h-full text-muted-foreground">
								<Terminal class="w-8 h-8 opacity-20 mb-2" />
								<p class="text-sm">No logs available yet</p>
							</div>
						{/if}
					</div>
				</Card.Content>
			</Card.Root>

			<!-- Footer -->
			<div class="flex items-center justify-between text-sm text-muted-foreground pt-2">
				<span class="flex items-center gap-1">
					<Clock class="w-4 h-4" />
					Last updated: {formatDate(task.updated_at)}
				</span>
				{#if task.cost_usd > 0}
					<span class="flex items-center gap-1">
						<DollarSign class="w-4 h-4" />
						Cost: {formatCost(task.cost_usd)}
						{#if task.max_cost_usd}
							<span class="text-muted-foreground">/ {formatCost(task.max_cost_usd)}</span>
						{/if}
					</span>
				{/if}
			</div>
		</div>
	{/if}
</div>
