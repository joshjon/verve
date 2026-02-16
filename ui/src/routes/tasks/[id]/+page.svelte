<script lang="ts">
	import { page } from '$app/stores';
	import { onMount } from 'svelte';
	import { client } from '$lib/api-client';
	import type { Task, TaskStatus } from '$lib/models/task';
	import { Button } from '$lib/components/ui/button';
	import { Badge } from '$lib/components/ui/badge';
	import * as Card from '$lib/components/ui/card';
	import { goto } from '$app/navigation';
	import { repoStore } from '$lib/stores/repos.svelte';
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
		GitBranch,
		Link2,
		Terminal,
		Sparkles,
		RefreshCw,
		X,
		Calendar,
		Loader2,
		Target,
		DollarSign,
		AlertTriangle,
		ChevronDown,
		ExternalLink,
		CircleDot,
		MinusCircle,
		Copy,
		Check,
		MessageSquare,
		Send
	} from 'lucide-svelte';
	import type { ComponentType } from 'svelte';
	import type { Icon } from 'lucide-svelte';
	import { AnsiUp } from 'ansi_up';
	const ansi = new AnsiUp();

	function colorizeLogs(html: string): string {
		// Split on HTML tags so we only colorize text nodes, not tag attributes
		return html.replace(/([^<]+)|(<[^>]*>)/g, (match, text, tag) => {
			if (tag) return tag;
			return text
				// Bracketed prefixes: [agent], [error], [info], [warn], [system], etc.
				.replace(/\[([a-zA-Z_-]+)\]/g, '<span class=log-bracket>[$1]</span>')
				// Lines starting with $ or > (command prompts)
				.replace(/^([\$>]\s)(.*)$/gm, '<span class=log-prompt>$1</span><span class=log-cmd>$2</span>')
				// File paths (word with slashes and optional extension)
				.replace(/(?<!\w)((?:\.{0,2}\/)?(?:[\w.-]+\/)+[\w.-]+)(?!\w)/g, '<span class=log-path>$1</span>')
				// URLs
				.replace(/(https?:\/\/[^\s<]+)/g, '<span class=log-url>$1</span>')
				// Numbers (standalone)
				.replace(/(?<=\s|^|\(|:)(\d+\.?\d*)(?=\s|$|\)|,|;)/gm, '<span class=log-num>$1</span>')
				// Quoted strings
				.replace(/("|')(.*?)(\1)/g, '<span class=log-str>$1$2$3</span>');
		});
	}

	// Configure marked for safe rendering
	marked.setOptions({
		breaks: true,
		gfm: true
	});

	let task = $state<Task | null>(null);
	let logsByAttempt = $state<Record<number, string[]>>({});
	let activeAttemptTab = $state(1);
	let loading = $state(true);
	let error = $state<string | null>(null);
	let syncing = $state(false);
	let closing = $state(false);
	let showCloseForm = $state(false);
	let closeReason = $state('');
	let retrying = $state(false);
	let showRetryForm = $state(false);
	let retryInstructions = $state('');
	let sendingFeedback = $state(false);
	let showFeedbackForm = $state(false);
	let feedbackText = $state('');
	let logsContainer: HTMLDivElement | null = $state(null);
	let autoScroll = $state(true);
	let lastLogCount = $state(0);
	let showRetryContext = $state(false);
	let checkStatus = $state<{
		status: 'pending' | 'success' | 'failure' | 'error';
		summary?: string;
		failed_names?: string[];
		check_runs_skipped?: boolean;
		checks?: { name: string; status: string; conclusion: string; url: string }[];
	} | null>(null);
	let checkStatusLoading = $state(false);
	let checkPollTimer = $state<ReturnType<typeof setTimeout> | null>(null);
	let forceCheckPolls = $state(0);

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
				return 'bg-green-500/20 text-green-400';
			case 'medium':
				return 'bg-amber-500/20 text-amber-400';
			case 'low':
				return 'bg-red-500/20 text-red-400';
			default:
				return 'bg-gray-500/20 text-gray-400';
		}
	});

	interface PrereqFailure {
		detected: string[];
		missing: {
			tool: string;
			reason: string;
			install: string;
		}[];
		dockerfile?: string;
	}

	let dockerfileCopied = $state(false);

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
			bgClass: 'bg-amber-500/20 text-amber-400',
			textClass: 'text-amber-600 dark:text-amber-400'
		},
		running: {
			label: 'Running',
			icon: Play,
			bgClass: 'bg-blue-500/20 text-blue-400',
			textClass: 'text-blue-600 dark:text-blue-400'
		},
		review: {
			label: 'In Review',
			icon: Eye,
			bgClass: 'bg-purple-500/20 text-purple-400',
			textClass: 'text-purple-600 dark:text-purple-400'
		},
		merged: {
			label: 'Merged',
			icon: GitMerge,
			bgClass: 'bg-green-500/20 text-green-400',
			textClass: 'text-green-600 dark:text-green-400'
		},
		closed: {
			label: 'Closed',
			icon: CheckCircle,
			bgClass: 'bg-gray-500/20 text-gray-400',
			textClass: 'text-gray-600 dark:text-gray-400'
		},
		failed: {
			label: 'Failed',
			icon: XCircle,
			bgClass: 'bg-red-500/20 text-red-400',
			textClass: 'text-red-600 dark:text-red-400'
		}
	};

	const branchURL = $derived.by(() => {
		if (!task?.branch_name) return null;
		const r = repoStore.repos.find((r) => r.id === task!.repo_id);
		if (!r) return null;
		return `https://github.com/${r.full_name}/tree/${task.branch_name}`;
	});

	const canClose = $derived(task && !['closed', 'merged', 'failed'].includes(task.status));
	const canRetry = $derived(task?.status === 'failed');
	const canProvideFeedback = $derived(task?.status === 'review');

	const currentStatusConfig = $derived(task ? statusConfig[task.status] : null);
	const StatusIcon = $derived(currentStatusConfig?.icon ?? Clock);

	// Render description as markdown
	const renderedDescription = $derived(task && task.description.trim() ? marked(task.description) : '');

	// Per-attempt log tracking
	const logs = $derived(logsByAttempt[activeAttemptTab] ?? []);
	const renderedLogs = $derived(logs.length > 0 ? colorizeLogs(ansi.ansi_to_html(logs.join('\n'))) : '');
	const attemptNumbers = $derived.by(() => {
		const nums = new Set(Object.keys(logsByAttempt).map(Number));
		if (task) {
			for (let i = 1; i <= task.attempt; i++) nums.add(i);
		}
		return [...nums].sort((a, b) => a - b);
	});
	const showAttemptTabs = $derived(attemptNumbers.length > 1);

	function switchAttemptTab(attempt: number) {
		activeAttemptTab = attempt;
		lastLogCount = 0;
		autoScroll = true;
		requestAnimationFrame(() => {
			if (logsContainer) logsContainer.scrollTop = logsContainer.scrollHeight;
		});
	}

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
				const prev = task.status;
				task = { ...event.task, logs: task.logs };
				// Refresh check status when task enters review
				if (task.status === 'review' && task.pr_number && prev !== 'review') {
					checkStatus = null;
					stopCheckPolling();
					forceCheckPolls = 3;
					// Delay first fetch to let GitHub process the new commit
					checkPollTimer = setTimeout(loadCheckStatus, 5000);
				}
			}
		});

		// Log streaming via dedicated SSE endpoint.
		// Uses double-buffering so reconnects replace logs without flashing.
		// Logs are grouped by attempt number for tabbed display.
		let logBufferMap: Record<number, string[]> = {};
		let historicalDone = false;

		const logsES = new EventSource(client.taskLogsURL(taskId));

		logsES.addEventListener('open', () => {
			logBufferMap = {};
			historicalDone = false;
		});

		logsES.addEventListener('logs_appended', (e) => {
			const event = JSON.parse(e.data);
			const attempt: number = event.attempt || 1;
			if (historicalDone) {
				logsByAttempt[attempt] = [...(logsByAttempt[attempt] ?? []), ...event.logs];
				// Auto-switch to latest attempt on first log of new attempt
				if (attempt > activeAttemptTab) {
					activeAttemptTab = attempt;
					lastLogCount = 0;
				}
			} else {
				logBufferMap[attempt] = [...(logBufferMap[attempt] ?? []), ...event.logs];
			}
		});

		logsES.addEventListener('logs_done', () => {
			logsByAttempt = logBufferMap;
			logBufferMap = {};
			historicalDone = true;
			// Default to latest attempt
			const keys = Object.keys(logsByAttempt).map(Number);
			if (keys.length > 0) {
				activeAttemptTab = Math.max(...keys);
			}
		});

		return () => {
			es.close();
			logsES.close();
			stopCheckPolling();
		};
	});

	async function loadTask() {
		try {
			task = await client.getTask(taskId);
			error = null;
			if (task.status === 'review' && task.pr_number) {
				loadCheckStatus();
			}
		} catch (e) {
			error = (e as Error).message;
		} finally {
			loading = false;
		}
	}

	function stopCheckPolling() {
		if (checkPollTimer) {
			clearTimeout(checkPollTimer);
			checkPollTimer = null;
		}
	}

	async function loadCheckStatus() {
		checkStatusLoading = true;
		stopCheckPolling();
		try {
			checkStatus = await client.getTaskChecks(taskId);
			// Keep polling while checks are pending, or during forced
			// polls after a status transition (handles stale GitHub data).
			const shouldPoll =
				checkStatus.status === 'pending' || forceCheckPolls > 0;
			if (forceCheckPolls > 0) forceCheckPolls--;
			if (shouldPoll && task?.status === 'review') {
				checkPollTimer = setTimeout(loadCheckStatus, 10000);
			}
		} catch {
			checkStatus = { status: 'error', summary: 'Failed to fetch check status' };
		} finally {
			checkStatusLoading = false;
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

	async function handleRetry() {
		if (!task || retrying) return;
		retrying = true;
		try {
			task = await client.retryTask(task.id, retryInstructions || undefined);
			showRetryForm = false;
			retryInstructions = '';
		} catch (e) {
			error = (e as Error).message;
		} finally {
			retrying = false;
		}
	}

	async function handleFeedback() {
		if (!task || sendingFeedback || !feedbackText.trim()) return;
		sendingFeedback = true;
		try {
			task = await client.feedbackTask(task.id, feedbackText.trim());
			showFeedbackForm = false;
			feedbackText = '';
		} catch (e) {
			error = (e as Error).message;
		} finally {
			sendingFeedback = false;
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

<div class="p-4 sm:p-6">
	<Button variant="ghost" onclick={() => goto('/')} class="mb-4 sm:mb-6 gap-2 -ml-2">
		<ArrowLeft class="w-4 h-4" />
		<span class="hidden sm:inline">Back to Dashboard</span>
		<span class="sm:hidden">Back</span>
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
		<!-- Header: Title + Metadata (full width) -->
		<div class="space-y-4 mb-6">
			{#if task.title}
				<h1 class="text-xl sm:text-2xl font-semibold">{task.title}</h1>
			{/if}

			<div class="flex items-center gap-2 sm:gap-3 flex-wrap pb-4 sm:pb-5 border-b">
				<span class="font-mono text-xs sm:text-sm text-muted-foreground bg-muted px-2 py-0.5 rounded truncate max-w-[150px] sm:max-w-none">
					{task.id}
				</span>
				<Badge class="{currentStatusConfig?.bgClass} gap-1">
					<StatusIcon class="w-3 h-3" />
					{currentStatusConfig?.label}
				</Badge>
				{#if task.model}
					<span class="text-xs text-muted-foreground bg-muted px-2 py-0.5 rounded capitalize">{task.model}</span>
				{/if}
				<span class="text-xs sm:text-sm text-muted-foreground flex items-center gap-1.5">
					<Calendar class="w-3.5 h-3.5" />
					{formatRelativeTime(task.created_at)}
				</span>
				<span class="text-xs sm:text-sm text-muted-foreground flex items-center gap-1.5">
					<Clock class="w-3.5 h-3.5" />
					{formatRelativeTime(task.updated_at)}
				</span>
				{#if task.cost_usd > 0}
					<span class="text-xs sm:text-sm text-muted-foreground flex items-center gap-1.5">
						<DollarSign class="w-3.5 h-3.5" />
						{formatCost(task.cost_usd)}
						{#if task.max_cost_usd}
							<span class="text-muted-foreground/60">/ {formatCost(task.max_cost_usd)}</span>
						{/if}
					</span>
				{/if}
				{#if canClose}
					<div class="ml-auto">
						{#if showCloseForm}
							<Button size="sm" variant="ghost" onclick={() => (showCloseForm = false)} class="gap-1">
								<X class="w-4 h-4" />
								Cancel
							</Button>
						{:else}
							<Button size="sm" variant="outline" onclick={() => (showCloseForm = true)} class="gap-1">
								<XCircle class="w-4 h-4" />
								<span class="hidden sm:inline">Close Task</span>
								<span class="sm:hidden">Close</span>
							</Button>
						{/if}
					</div>
				{/if}
			</div>

			<!-- Close Form (full width, above columns) -->
			{#if showCloseForm}
				<Card.Root class="border-destructive/30 bg-destructive/5">
					<Card.Header class="pb-0 gap-0">
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
		</div>

		<!-- Two-column layout -->
		<div class="grid grid-cols-1 lg:grid-cols-2 gap-6">
			<!-- Left column: Task details -->
			<div class="space-y-6">
				<!-- Task Details (unified card) -->
				<div class="rounded-xl border bg-card shadow-sm overflow-hidden">
					<!-- Header -->
					<div class="flex items-center gap-2 px-5 py-3 border-b">
						<FileText class="w-4 h-4 text-muted-foreground" />
						<span class="font-semibold text-sm">Task Details</span>
					</div>

					<!-- Description -->
					<div class="px-5 py-4 {(task.acceptance_criteria && task.acceptance_criteria.length > 0) || (task.depends_on && task.depends_on.length > 0) ? 'border-b' : ''}">
						{#if renderedDescription}
							<div class="prose prose-sm dark:prose-invert max-w-none">
								{@html renderedDescription}
							</div>
						{:else}
							<p class="text-sm text-muted-foreground italic">No description provided</p>
						{/if}
					</div>

					<!-- Acceptance Criteria -->
					{#if task.acceptance_criteria && task.acceptance_criteria.length > 0}
						<div class="px-5 py-4 {task.depends_on && task.depends_on.length > 0 ? 'border-b' : ''}">
							<div class="flex items-center gap-2 mb-3">
								<Target class="w-3.5 h-3.5 text-muted-foreground" />
								<span class="text-sm font-medium">Acceptance Criteria</span>
								<span class="text-xs text-muted-foreground">
									({task.acceptance_criteria.length})
								</span>
							</div>
							<ol class="space-y-2">
								{#each task.acceptance_criteria as criterion, i}
									<li class="flex items-start gap-2.5 text-sm">
										<span class="text-xs text-muted-foreground font-mono mt-0.5 w-5 shrink-0 text-right">{i + 1}.</span>
										<span class="text-foreground/80">{criterion}</span>
									</li>
								{/each}
							</ol>
						</div>
					{/if}

					<!-- Dependencies -->
					{#if task.depends_on && task.depends_on.length > 0}
						<div class="px-5 py-4">
							<div class="flex items-center gap-2 mb-3">
								<Link2 class="w-3.5 h-3.5 text-muted-foreground" />
								<span class="text-sm font-medium">Dependencies</span>
								<span class="text-xs text-muted-foreground">
									({task.depends_on.length})
								</span>
							</div>
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
						</div>
					{/if}
				</div>

				<!-- Pull Request -->
				{#if task.pull_request_url}
					<div class="rounded-xl border shadow-sm overflow-hidden {task.status === 'merged' ? 'border-green-500/30 bg-green-500/10' : task.status === 'closed' || task.status === 'failed' ? 'border-gray-500/30 bg-gray-500/5' : 'border-purple-500/30 bg-purple-500/[0.08]'}">
						<!-- Header -->
						<div class="flex items-center gap-2 px-5 py-3 {task.status === 'review' ? 'border-b' : ''}">
							{#if task.status === 'merged'}
								<GitMerge class="w-4 h-4 text-green-500" />
								<span class="font-semibold text-sm">Pull Request (Merged)</span>
							{:else if task.status === 'closed' || task.status === 'failed'}
								<GitPullRequest class="w-4 h-4 text-gray-500" />
								<span class="font-semibold text-sm">Pull Request (Closed)</span>
							{:else}
								<GitPullRequest class="w-4 h-4 text-purple-500" />
								<span class="font-semibold text-sm">Pull Request</span>
							{/if}
							<div class="ml-auto flex items-center gap-2">
								<a
									href={task.pull_request_url}
									class="text-primary hover:underline font-medium flex items-center gap-2 text-sm"
									target="_blank"
									rel="noopener noreferrer"
								>
									<GitPullRequest class="w-4 h-4" />
									PR #{task.pr_number}
								</a>
								{#if task.status === 'review'}
									<Button
										size="sm"
										variant="ghost"
										class="h-7 w-7 p-0 text-muted-foreground"
										onclick={() => { syncStatus(); loadCheckStatus(); }}
										disabled={checkStatusLoading || syncing}
										title="Sync PR status and refresh checks"
									>
										<RefreshCw class="w-3.5 h-3.5 {checkStatusLoading || syncing ? 'animate-spin' : ''}" />
									</Button>
								{/if}
							</div>
						</div>
						<!-- CI Checks -->
						{#if task.status === 'review'}
							<div class="px-5 py-3 space-y-2">
								<div class="flex items-center gap-2">
									{#if checkStatusLoading && !checkStatus}
										<Loader2 class="w-3.5 h-3.5 animate-spin text-muted-foreground" />
										<span class="text-sm text-muted-foreground">Checking CI status...</span>
									{:else if checkStatus?.check_runs_skipped}
										<AlertTriangle class="w-3.5 h-3.5 text-amber-500" />
										<span class="text-sm text-muted-foreground">CI checks skipped — fine-grained tokens do not support this. Use a classic token for CI visibility.</span>
									{:else if checkStatus?.status === 'success'}
										<CheckCircle class="w-3.5 h-3.5 text-green-600 dark:text-green-400" />
										<span class="text-sm text-green-600 dark:text-green-400">All checks passed</span>
									{:else if checkStatus?.status === 'pending'}
										<Loader2 class="w-3.5 h-3.5 animate-spin text-amber-600 dark:text-amber-400" />
										<span class="text-sm text-amber-600 dark:text-amber-400">Checks in progress</span>
									{:else if checkStatus?.status === 'failure'}
										<XCircle class="w-3.5 h-3.5 text-red-600 dark:text-red-400" />
										<span class="text-sm text-red-600 dark:text-red-400">Checks failed</span>
									{:else if checkStatus?.status === 'error'}
										<AlertTriangle class="w-3.5 h-3.5 text-amber-500" />
										<span class="text-sm text-muted-foreground">{checkStatus.summary}</span>
									{/if}
								</div>
								{#if checkStatus?.checks && checkStatus.checks.length > 0}
									<div class="space-y-1">
										{#each checkStatus.checks as check}
											<div class="flex items-center gap-2 text-sm pl-1">
												{#if check.status !== 'completed'}
													<Loader2 class="w-3.5 h-3.5 animate-spin text-amber-600 dark:text-amber-400 shrink-0" />
												{:else if check.conclusion === 'success'}
													<CheckCircle class="w-3.5 h-3.5 text-green-600 dark:text-green-400 shrink-0" />
												{:else if check.conclusion === 'failure'}
													<XCircle class="w-3.5 h-3.5 text-red-600 dark:text-red-400 shrink-0" />
												{:else if check.conclusion === 'skipped'}
													<MinusCircle class="w-3.5 h-3.5 text-muted-foreground shrink-0" />
												{:else if check.conclusion === 'cancelled'}
													<XCircle class="w-3.5 h-3.5 text-muted-foreground shrink-0" />
												{:else}
													<CircleDot class="w-3.5 h-3.5 text-muted-foreground shrink-0" />
												{/if}
												{#if check.url}
													<a
														href={check.url}
														class="text-muted-foreground hover:text-foreground hover:underline truncate flex items-center gap-1"
														target="_blank"
														rel="noopener noreferrer"
													>
														{check.name}
														<ExternalLink class="w-3 h-3 shrink-0 opacity-50" />
													</a>
												{:else}
													<span class="text-muted-foreground truncate">{check.name}</span>
												{/if}
											</div>
										{/each}
									</div>
								{/if}
							</div>
						{/if}
					</div>
				{/if}

				<!-- Branch (skip-PR mode) -->
				{#if task.branch_name && !task.pull_request_url}
					<Card.Root class="border-cyan-500/30 bg-cyan-500/5">
						<Card.Header class="pb-0 gap-0">
							<Card.Title class="text-base flex items-center gap-2">
								<GitBranch class="w-4 h-4 text-cyan-500" />
								Branch
							</Card.Title>
						</Card.Header>
						<Card.Content class="space-y-3">
							<div class="flex items-center gap-4">
								{#if branchURL}
									<a
										href={branchURL}
										class="text-primary hover:underline font-medium flex items-center gap-2"
										target="_blank"
										rel="noopener noreferrer"
									>
										<GitBranch class="w-4 h-4" />
										{task.branch_name}
									</a>
								{:else}
									<span class="text-sm font-mono text-muted-foreground flex items-center gap-2">
										<GitBranch class="w-4 h-4" />
										{task.branch_name}
									</span>
								{/if}
								{#if task.status === 'review'}
									<Button
										size="sm"
										variant="outline"
										onclick={syncStatus}
										disabled={syncing}
										class="gap-2 shrink-0"
									>
										<RefreshCw class="w-4 h-4 {syncing ? 'animate-spin' : ''}" />
										{syncing ? 'Syncing...' : 'Sync PR'}
									</Button>
								{/if}
							</div>
							{#if task.status === 'review'}
								<p class="text-sm text-muted-foreground">No PR linked yet. Create one from this branch and sync to detect it.</p>
							{/if}
						</Card.Content>
					</Card.Root>
				{/if}

				<!-- Prerequisite Failure -->
				{#if parsedPrereqFailure}
					<Card.Root class="border-red-500/30 bg-red-500/5">
						<Card.Header class="pb-0 gap-0">
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
								Build a custom agent image with the missing tools, then retry the task.
							</p>
							{#each parsedPrereqFailure.missing as item}
								<div class="rounded-lg border bg-background p-3 space-y-1">
									<div class="flex items-center gap-2">
										<Badge variant="destructive" class="text-xs">{item.tool}</Badge>
									</div>
									<p class="text-sm text-muted-foreground">{item.reason}</p>
								</div>
							{/each}
							{#if parsedPrereqFailure.dockerfile}
								<div class="mt-4 space-y-2">
									<div class="flex items-center justify-between">
										<p class="text-sm font-medium">Suggested Dockerfile</p>
										<Button
											variant="ghost"
											size="sm"
											class="h-7 gap-1.5 text-xs"
											onclick={() => {
												navigator.clipboard.writeText(parsedPrereqFailure?.dockerfile ?? '');
												dockerfileCopied = true;
												setTimeout(() => dockerfileCopied = false, 2000);
											}}
										>
											{#if dockerfileCopied}
												<Check class="w-3 h-3" />
												Copied
											{:else}
												<Copy class="w-3 h-3" />
												Copy
											{/if}
										</Button>
									</div>
									<p class="text-xs text-muted-foreground">This Dockerfile was AI-generated and may not be completely accurate. Review and adjust before use.</p>
									<pre class="rounded-lg border bg-background p-3 text-xs font-mono overflow-x-auto whitespace-pre">{parsedPrereqFailure.dockerfile}</pre>
								</div>
							{/if}
						</Card.Content>
					</Card.Root>
				<!-- Close Reason -->
				{:else if task.close_reason}
					<Card.Root class="border-gray-500/30">
						<Card.Header class="pb-0 gap-0">
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
			</div>

			<!-- Right column: Agent Session Pane -->
			<div class="rounded-xl border bg-card shadow-sm overflow-hidden self-start">
				<!-- Header Bar -->
				<div class="flex items-center gap-2 px-5 py-3 border-b">
					<Sparkles class="w-4 h-4 text-muted-foreground" />
					<span class="font-semibold text-sm">Agent</span>
					{#if task.status === 'running'}
						<span class="flex items-center gap-1 text-xs text-blue-500">
							<span class="relative flex h-2 w-2">
								<span class="animate-ping absolute inline-flex h-full w-full rounded-full bg-blue-400 opacity-75"></span>
								<span class="relative inline-flex rounded-full h-2 w-2 bg-blue-500"></span>
							</span>
							Live
						</span>
					{/if}
					{#if parsedAgentStatus?.tests_status}
						<Badge class="text-xs gap-1 {parsedAgentStatus.tests_status === 'pass' ? 'bg-green-500/20 text-green-400' : parsedAgentStatus.tests_status === 'skip' ? 'bg-secondary text-secondary-foreground' : 'bg-red-500/20 text-red-400'}">
							{#if parsedAgentStatus.tests_status === 'pass'}
								<CheckCircle class="w-3 h-3" />
							{:else if parsedAgentStatus.tests_status === 'fail'}
								<XCircle class="w-3 h-3" />
							{:else if parsedAgentStatus.tests_status === 'skip'}
								<MinusCircle class="w-3 h-3" />
							{/if}
							Tests: {parsedAgentStatus.tests_status}
						</Badge>
					{/if}
					{#if parsedAgentStatus?.confidence}
						<Badge class="{confidenceColor} text-xs">
							{parsedAgentStatus.confidence} confidence
						</Badge>
					{/if}
					{#if canRetry}
						<div class="ml-auto">
							{#if showRetryForm}
								<Button size="sm" variant="ghost" onclick={() => (showRetryForm = false)} class="gap-1">
									<X class="w-4 h-4" />
									Cancel
								</Button>
							{:else}
								<Button size="sm" variant="outline" onclick={() => (showRetryForm = true)} class="gap-1">
									<RefreshCw class="w-4 h-4" />
									Retry
								</Button>
							{/if}
						</div>
					{/if}
					{#if canProvideFeedback}
						<div class="ml-auto">
							{#if showFeedbackForm}
								<Button size="sm" variant="ghost" onclick={() => (showFeedbackForm = false)} class="gap-1">
									<X class="w-4 h-4" />
									Cancel
								</Button>
							{:else}
								<Button size="sm" variant="outline" onclick={() => (showFeedbackForm = true)} class="gap-1 border-purple-500/40 text-purple-600 dark:text-purple-400 hover:bg-purple-500/10">
									<MessageSquare class="w-4 h-4" />
									<span class="hidden sm:inline">Provide Feedback</span>
									<span class="sm:hidden">Feedback</span>
								</Button>
							{/if}
						</div>
					{/if}
				</div>

				<!-- Feedback Form -->
			{#if showFeedbackForm}
				<div class="px-5 py-4 border-b bg-purple-500/5">
					<div class="space-y-4">
						<div>
							<label for="feedback-text" class="text-sm font-medium mb-2 block">
								What changes would you like the agent to make?
							</label>
							<textarea
								id="feedback-text"
								bind:value={feedbackText}
								class="w-full border rounded-lg p-3 min-h-[100px] bg-background text-foreground resize-none focus:outline-none focus:ring-2 focus:ring-ring focus:ring-purple-500/40"
								placeholder="Describe what needs to be changed or improved in the current implementation..."
								disabled={sendingFeedback}
							></textarea>
						</div>
						<div class="flex justify-end gap-2">
							<Button variant="outline" onclick={() => (showFeedbackForm = false)} disabled={sendingFeedback}>
								Cancel
							</Button>
							<Button onclick={handleFeedback} disabled={sendingFeedback || !feedbackText.trim()} class="gap-2 bg-purple-600 hover:bg-purple-700 text-white">
								{#if sendingFeedback}
									<Loader2 class="w-4 h-4 animate-spin" />
									Sending...
								{:else}
									<Send class="w-4 h-4" />
									Send Feedback
								{/if}
							</Button>
						</div>
					</div>
				</div>
			{/if}

			<!-- Agent Insights -->
				{#if parsedAgentStatus || task.attempt > 1}
					<div class="px-5 py-4 border-b space-y-4">
						{#if task.attempt > 1}
							<div class="space-y-2">
								<div class="flex items-center gap-2">
									<span class="text-sm font-medium">Last Retry</span>
									<Badge variant="outline" class="text-xs {['review', 'merged'].includes(task.status) ? 'border-green-500/50 text-green-600 dark:text-green-400' : ''}">
										Attempt {task.attempt}/{task.max_attempts}
									</Badge>
								</div>
								{#if task.consecutive_failures >= 2 && !['review', 'merged'].includes(task.status)}
									<Badge class="bg-red-500 text-white gap-1 text-xs">
										<AlertTriangle class="w-3 h-3" />
										{task.consecutive_failures} consecutive failures
									</Badge>
								{/if}
								{#if task.retry_reason}
									<div class="flex items-start gap-2 text-sm">
										<span class="text-muted-foreground shrink-0">Reason:</span>
										<span class="text-foreground/80">{task.retry_reason}</span>
									</div>
								{/if}
								{#if task.retry_context}
									<div>
										<button
											type="button"
											class="inline-flex items-center gap-1.5 text-xs text-amber-700 dark:text-amber-400 hover:text-amber-800 dark:hover:text-amber-300 transition-colors bg-amber-500/10 hover:bg-amber-500/20 px-2 py-1.5 rounded-md"
											onclick={() => (showRetryContext = !showRetryContext)}
										>
											CI Failure Logs
											<ChevronDown class="w-3.5 h-3.5 transition-transform {showRetryContext ? 'rotate-180' : ''}" />
										</button>
										{#if showRetryContext}
											<pre class="mt-2 text-xs font-mono bg-zinc-900/50 text-white rounded-lg p-3 max-h-48 overflow-y-auto whitespace-pre-wrap border border-border">{task.retry_context}</pre>
										{/if}
									</div>
								{/if}
							</div>
						{/if}
						{#if parsedAgentStatus?.files_modified && parsedAgentStatus.files_modified.length > 0}
							<div class="space-y-1.5">
								<span class="text-sm font-medium">Files Changed</span>
								<div class="flex flex-wrap gap-1.5">
									{#each parsedAgentStatus.files_modified as file}
										<span class="text-xs font-mono bg-indigo-500/10 text-indigo-700 dark:text-indigo-400 px-2 py-0.5 rounded-md">{file}</span>
									{/each}
								</div>
							</div>
						{/if}
						{#if parsedAgentStatus?.criteria_met && parsedAgentStatus.criteria_met.length > 0}
							<div class="space-y-1.5">
								<span class="text-sm font-medium">Criteria</span>
								<div class="flex flex-wrap gap-1.5">
									{#each parsedAgentStatus.criteria_met as criterion}
										<span class="inline-flex items-center gap-1 text-xs bg-green-500/10 text-green-700 dark:text-green-400 px-2 py-0.5 rounded-md">
											<CheckCircle class="w-3 h-3" />
											{criterion}
										</span>
									{/each}
								</div>
							</div>
						{/if}
						{#if parsedAgentStatus?.blockers && parsedAgentStatus.blockers.length > 0}
							<div class="space-y-1.5">
								<span class="text-sm font-medium">Blockers</span>
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
						{#if parsedAgentStatus?.notes}
							<div class="space-y-1.5">
								<span class="text-sm font-medium">Notes</span>
								<p class="text-sm text-muted-foreground">{parsedAgentStatus.notes}</p>
							</div>
						{/if}
					</div>
				{/if}

				<!-- Retry Form -->
				{#if showRetryForm}
					<div class="px-5 py-4 border-b bg-blue-500/5">
						<div class="space-y-4">
							<div>
								<label for="retry-instructions" class="text-sm font-medium mb-2 block">
									Instructions (optional)
								</label>
								<textarea
									id="retry-instructions"
									bind:value={retryInstructions}
									class="w-full border rounded-lg p-3 min-h-[80px] bg-background text-foreground resize-none focus:outline-none focus:ring-2 focus:ring-ring"
									placeholder="What should the agent do differently this time?"
									disabled={retrying}
								></textarea>
							</div>
							<div class="flex justify-end gap-2">
								<Button variant="outline" onclick={() => (showRetryForm = false)} disabled={retrying}>
									Cancel
								</Button>
								<Button onclick={handleRetry} disabled={retrying} class="gap-2">
									{#if retrying}
										<Loader2 class="w-4 h-4 animate-spin" />
										Retrying...
									{:else}
										<RefreshCw class="w-4 h-4" />
										Retry Task
									{/if}
								</Button>
							</div>
						</div>
					</div>
				{/if}

				<!-- Logs -->
				<div class="p-4">
					<div class="rounded-lg border border-zinc-800 overflow-hidden">
						{#if showAttemptTabs}
							<div class="flex items-center gap-1 px-3 py-2 bg-zinc-950 border-b border-zinc-800">
								{#each attemptNumbers as num}
									<button
										type="button"
										class="px-3 py-1 text-xs font-medium rounded-md transition-all {activeAttemptTab === num ? 'bg-white/10 text-white' : 'text-zinc-600 hover:text-zinc-400 hover:bg-white/5'}"
										onclick={() => switchAttemptTab(num)}
									>
										Attempt {num}
										{#if task.status === 'running' && num === task.attempt}
											<span class="inline-flex ml-1 h-1.5 w-1.5 rounded-full bg-blue-500 animate-pulse"></span>
										{/if}
									</button>
								{/each}
							</div>
						{/if}
						<div
							bind:this={logsContainer}
							onscroll={handleLogsScroll}
							class="terminal-container h-[250px] sm:h-[400px] lg:h-[500px] w-full bg-zinc-950 p-3 sm:p-4 overflow-y-auto"
						>
						{#if logs.length > 0}
							<pre class="log-output text-xs font-mono whitespace-pre-wrap leading-relaxed">{@html renderedLogs}</pre>
						{:else}
							<div class="flex flex-col items-center justify-center h-full text-muted-foreground">
								<Terminal class="w-8 h-8 opacity-20 mb-2" />
								<p class="text-sm">No logs available yet</p>
							</div>
						{/if}
					</div>
					</div>
				</div>
			</div>
		</div>
	{/if}
</div>

<style>
	:global(.terminal-container) {
		box-shadow: inset 0 2px 4px rgba(0, 0, 0, 0.3);
	}
	:global(.log-output) {
		color: #bac2cd;
	}
	:global(.log-bracket) {
		color: #79c0ff;
	}
	:global(.log-prompt) {
		color: #7ee787;
	}
	:global(.log-cmd) {
		color: #e2e8f0;
	}
	:global(.log-path) {
		color: #d2a8ff;
	}
	:global(.log-url) {
		color: #79c0ff;
		text-decoration: underline;
		text-decoration-color: #79c0ff40;
	}
	:global(.log-num) {
		color: #ffa657;
	}
	:global(.log-str) {
		color: #a5d6ff;
	}
</style>

