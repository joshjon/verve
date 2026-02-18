<script lang="ts">
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { onMount } from 'svelte';
	import { client } from '$lib/api-client';
	import { epicStore } from '$lib/stores/epics.svelte';
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import type { Epic, ProposedTask } from '$lib/models/epic';
	import {
		ArrowLeft,
		Layers,
		Loader2,
		Plus,
		Trash2,
		Send,
		Check,
		X,
		Edit3,
		GripVertical,
		Link2,
		MessageSquare,
		PauseCircle,
		Play,
		Square,
		CheckCircle2,
		AlertCircle
	} from 'lucide-svelte';

	let epic = $state<Epic | null>(null);
	let loading = $state(true);
	let error = $state<string | null>(null);

	// Planning session state
	let planningPrompt = $state('');
	let sessionMessage = $state('');
	let sendingMessage = $state(false);
	let isPlanning = $derived(epic?.status === 'planning');
	let isEditable = $derived(epic?.status === 'draft' || epic?.status === 'ready');

	// Task editing state
	let editingTaskIdx = $state<number | null>(null);
	let editTitle = $state('');
	let editDescription = $state('');
	let editCriteria = $state<string[]>([]);

	// Confirmation state
	let confirming = $state(false);
	let notReady = $state(false);
	let closing = $state(false);
	let startingPlanning = $state(false);
	let finishingPlanning = $state(false);

	const epicId = $derived($page.params.id);

	onMount(async () => {
		await loadEpic();
	});

	async function loadEpic() {
		loading = true;
		error = null;
		try {
			epic = await client.getEpic(epicId);
		} catch (err) {
			error = (err as Error).message;
		} finally {
			loading = false;
		}
	}

	async function handleStartPlanning() {
		if (!planningPrompt.trim() || !epic) return;
		startingPlanning = true;
		error = null;
		try {
			epic = await client.startPlanning(epic.id, planningPrompt);
			planningPrompt = '';
		} catch (err) {
			error = (err as Error).message;
		} finally {
			startingPlanning = false;
		}
	}

	async function handleSendMessage() {
		if (!sessionMessage.trim() || !epic) return;
		sendingMessage = true;
		error = null;
		try {
			epic = await client.sendSessionMessage(epic.id, sessionMessage);
			sessionMessage = '';
		} catch (err) {
			error = (err as Error).message;
		} finally {
			sendingMessage = false;
		}
	}

	async function handleFinishPlanning() {
		if (!epic) return;
		finishingPlanning = true;
		error = null;
		try {
			epic = await client.finishPlanning(epic.id);
		} catch (err) {
			error = (err as Error).message;
		} finally {
			finishingPlanning = false;
		}
	}

	function startEditTask(idx: number) {
		if (!epic || isPlanning) return;
		const task = epic.proposed_tasks[idx];
		editingTaskIdx = idx;
		editTitle = task.title;
		editDescription = task.description;
		editCriteria = [...(task.acceptance_criteria ?? [])];
	}

	function cancelEditTask() {
		editingTaskIdx = null;
		editTitle = '';
		editDescription = '';
		editCriteria = [];
	}

	async function saveEditTask() {
		if (!epic || editingTaskIdx === null) return;
		const tasks = [...epic.proposed_tasks];
		tasks[editingTaskIdx] = {
			...tasks[editingTaskIdx],
			title: editTitle,
			description: editDescription,
			acceptance_criteria: editCriteria.filter((c) => c.trim() !== '')
		};
		try {
			epic = await client.updateProposedTasks(epic.id, tasks);
			cancelEditTask();
		} catch (err) {
			error = (err as Error).message;
		}
	}

	async function addNewTask() {
		if (!epic) return;
		const newTask: ProposedTask = {
			temp_id: `temp_${Date.now()}`,
			title: 'New task',
			description: '',
			depends_on_temp_ids: [],
			acceptance_criteria: []
		};
		const tasks = [...epic.proposed_tasks, newTask];
		try {
			epic = await client.updateProposedTasks(epic.id, tasks);
			// Auto-edit the newly added task
			startEditTask(tasks.length - 1);
		} catch (err) {
			error = (err as Error).message;
		}
	}

	async function removeTask(idx: number) {
		if (!epic) return;
		const removedId = epic.proposed_tasks[idx].temp_id;
		const tasks = epic.proposed_tasks
			.filter((_, i) => i !== idx)
			.map((t) => ({
				...t,
				depends_on_temp_ids: (t.depends_on_temp_ids ?? []).filter((id) => id !== removedId)
			}));
		try {
			epic = await client.updateProposedTasks(epic.id, tasks);
			if (editingTaskIdx === idx) cancelEditTask();
		} catch (err) {
			error = (err as Error).message;
		}
	}

	async function handleConfirm() {
		if (!epic) return;
		confirming = true;
		error = null;
		try {
			epic = await client.confirmEpic(epic.id, notReady);
			epicStore.updateEpic(epic);
		} catch (err) {
			error = (err as Error).message;
		} finally {
			confirming = false;
		}
	}

	async function handleClose() {
		if (!epic) return;
		closing = true;
		error = null;
		try {
			epic = await client.closeEpic(epic.id);
			epicStore.updateEpic(epic);
		} catch (err) {
			error = (err as Error).message;
		} finally {
			closing = false;
		}
	}

	function getStatusColor(status: string) {
		switch (status) {
			case 'draft':
				return 'bg-gray-500/15 text-gray-400';
			case 'planning':
				return 'bg-violet-500/15 text-violet-400';
			case 'ready':
				return 'bg-amber-500/15 text-amber-400';
			case 'active':
				return 'bg-blue-500/15 text-blue-400';
			case 'completed':
				return 'bg-green-500/15 text-green-400';
			case 'closed':
				return 'bg-red-500/15 text-red-400';
			default:
				return 'bg-gray-500/15 text-gray-400';
		}
	}

	function getDependencyLabel(tempId: string): string {
		if (!epic) return tempId;
		const t = epic.proposed_tasks.find((pt) => pt.temp_id === tempId);
		return t ? t.title : tempId;
	}
</script>

<div class="p-4 sm:p-6 flex-1 min-h-0 flex flex-col max-w-6xl mx-auto w-full">
	<div class="mb-4">
		<button
			onclick={() => goto('/')}
			class="inline-flex items-center gap-1.5 text-sm text-muted-foreground hover:text-foreground transition-colors"
		>
			<ArrowLeft class="w-4 h-4" />
			Back to Dashboard
		</button>
	</div>

	{#if loading}
		<div class="flex items-center justify-center py-20">
			<Loader2 class="w-8 h-8 animate-spin text-muted-foreground" />
		</div>
	{:else if error && !epic}
		<div class="bg-destructive/10 text-destructive p-4 rounded-lg flex items-center gap-3">
			<AlertCircle class="w-5 h-5" />
			{error}
		</div>
	{:else if epic}
		<!-- Header -->
		<header class="flex flex-col sm:flex-row sm:items-center gap-3 mb-6">
			<div class="flex items-center gap-3 flex-1 min-w-0">
				<div class="w-10 h-10 rounded-lg bg-violet-500/10 flex items-center justify-center shrink-0">
					<Layers class="w-5 h-5 text-violet-500" />
				</div>
				<div class="min-w-0">
					<h1 class="text-xl font-bold truncate">{epic.title}</h1>
					<div class="flex items-center gap-2 mt-0.5">
						<span class="text-xs font-mono text-muted-foreground">{epic.id}</span>
						<span class="px-2 py-0.5 rounded-full text-[11px] font-semibold {getStatusColor(epic.status)}">
							{epic.status}
						</span>
					</div>
				</div>
			</div>
			<div class="flex items-center gap-2 shrink-0">
				{#if epic.status === 'draft' || epic.status === 'ready'}
					<Button variant="outline" size="sm" onclick={handleClose} disabled={closing} class="gap-1.5 text-red-400 border-red-500/30 hover:bg-red-500/10">
						{#if closing}
							<Loader2 class="w-3.5 h-3.5 animate-spin" />
						{:else}
							<X class="w-3.5 h-3.5" />
						{/if}
						Close Epic
					</Button>
				{/if}
			</div>
		</header>

		{#if error}
			<div class="bg-destructive/10 text-destructive p-3 rounded-lg text-sm mb-4 flex items-center gap-2">
				<AlertCircle class="w-4 h-4 shrink-0" />
				{error}
			</div>
		{/if}

		{#if epic.description}
			<Card.Root class="mb-6 bg-[oklch(0.18_0.005_285.823)]">
				<Card.Content class="p-4">
					<p class="text-sm text-muted-foreground whitespace-pre-wrap">{epic.description}</p>
				</Card.Content>
			</Card.Root>
		{/if}

		<div class="grid grid-cols-1 lg:grid-cols-3 gap-6 flex-1">
			<!-- Left: Proposed Tasks -->
			<div class="lg:col-span-2 flex flex-col min-h-0">
				<div class="flex items-center justify-between mb-3">
					<h2 class="text-sm font-semibold flex items-center gap-2">
						Proposed Tasks
						{#if epic.proposed_tasks.length > 0}
							<span class="px-2 py-0.5 rounded-full text-xs bg-muted text-muted-foreground">
								{epic.proposed_tasks.length}
							</span>
						{/if}
					</h2>
					{#if isEditable}
						<Button variant="outline" size="sm" onclick={addNewTask} class="gap-1.5 text-xs" disabled={isPlanning}>
							<Plus class="w-3.5 h-3.5" />
							Add Task
						</Button>
					{/if}
				</div>

				{#if epic.proposed_tasks.length === 0}
					<Card.Root class="bg-[oklch(0.18_0.005_285.823)] flex-1">
						<Card.Content class="p-8 text-center">
							<div class="w-12 h-12 rounded-xl bg-muted flex items-center justify-center mx-auto mb-3">
								<Layers class="w-6 h-6 text-muted-foreground" />
							</div>
							<p class="text-sm text-muted-foreground">
								{#if epic.status === 'draft' && !epic.planning_prompt}
									Start a planning session to generate task proposals.
								{:else}
									No tasks have been proposed yet.
								{/if}
							</p>
						</Card.Content>
					</Card.Root>
				{:else}
					<div class="space-y-2 overflow-y-auto flex-1 min-h-0 max-h-[60vh]">
						{#each epic.proposed_tasks as task, idx (task.temp_id)}
							<Card.Root class="bg-[oklch(0.18_0.005_285.823)] {isPlanning ? 'opacity-60' : ''}">
								<Card.Content class="p-3">
									{#if editingTaskIdx === idx}
										<!-- Edit mode -->
										<div class="space-y-3">
											<input
												type="text"
												bind:value={editTitle}
												class="w-full border rounded-lg px-3 py-2 bg-background text-foreground focus:outline-none focus:ring-2 focus:ring-ring text-sm"
												placeholder="Task title"
											/>
											<textarea
												bind:value={editDescription}
												class="w-full border rounded-lg px-3 py-2 bg-background text-foreground resize-none focus:outline-none focus:ring-2 focus:ring-ring text-sm min-h-[80px]"
												placeholder="Task description"
											></textarea>
											<div>
												<label class="text-xs font-medium text-muted-foreground mb-1 block">Acceptance Criteria</label>
												{#each editCriteria as criterion, ci}
													<div class="flex items-center gap-2 mb-1">
														<input
															type="text"
															value={criterion}
															oninput={(e) => {
																editCriteria = editCriteria.map((c, i) => (i === ci ? (e.target as HTMLInputElement).value : c));
															}}
															class="flex-1 border rounded-lg px-2 py-1 bg-background text-foreground text-xs"
															placeholder="Criterion"
														/>
														<button
															type="button"
															class="p-1 hover:bg-destructive/10 hover:text-destructive rounded"
															onclick={() => {
																editCriteria = editCriteria.filter((_, i) => i !== ci);
															}}
														>
															<X class="w-3 h-3" />
														</button>
													</div>
												{/each}
												<button
													type="button"
													class="text-xs text-primary hover:underline mt-1"
													onclick={() => {
														editCriteria = [...editCriteria, ''];
													}}
												>
													+ Add criterion
												</button>
											</div>
											<div class="flex items-center gap-2 justify-end">
												<Button variant="ghost" size="sm" onclick={cancelEditTask}>Cancel</Button>
												<Button size="sm" onclick={saveEditTask} class="gap-1">
													<Check class="w-3.5 h-3.5" />
													Save
												</Button>
											</div>
										</div>
									{:else}
										<!-- Display mode -->
										<div class="flex items-start gap-2">
											<span class="text-xs text-muted-foreground font-mono mt-0.5 shrink-0">{idx + 1}.</span>
											<div class="flex-1 min-w-0">
												<div class="flex items-start justify-between gap-2">
													<p class="text-sm font-medium">{task.title}</p>
													{#if isEditable && !isPlanning}
														<div class="flex items-center gap-1 shrink-0">
															<button
																class="p-1 hover:bg-accent rounded transition-colors"
																onclick={() => startEditTask(idx)}
																title="Edit task"
															>
																<Edit3 class="w-3.5 h-3.5 text-muted-foreground" />
															</button>
															<button
																class="p-1 hover:bg-destructive/10 hover:text-destructive rounded transition-colors"
																onclick={() => removeTask(idx)}
																title="Remove task"
															>
																<Trash2 class="w-3.5 h-3.5 text-muted-foreground" />
															</button>
														</div>
													{/if}
												</div>
												{#if task.description}
													<p class="text-xs text-muted-foreground mt-1 line-clamp-2">{task.description}</p>
												{/if}
												<div class="flex items-center gap-3 mt-2 flex-wrap">
													{#if task.depends_on_temp_ids && task.depends_on_temp_ids.length > 0}
														<span class="text-[10px] text-muted-foreground flex items-center gap-0.5">
															<Link2 class="w-3 h-3" />
															{task.depends_on_temp_ids.map((id) => getDependencyLabel(id)).join(', ')}
														</span>
													{/if}
													{#if task.acceptance_criteria && task.acceptance_criteria.length > 0}
														<span class="text-[10px] text-muted-foreground flex items-center gap-0.5">
															<CheckCircle2 class="w-3 h-3" />
															{task.acceptance_criteria.length} criteria
														</span>
													{/if}
												</div>
											</div>
										</div>
									{/if}
								</Card.Content>
							</Card.Root>
						{/each}
					</div>
				{/if}

				<!-- Confirm / Ready section -->
				{#if isEditable && epic.proposed_tasks.length > 0}
					<Card.Root class="mt-4 bg-[oklch(0.18_0.005_285.823)] border-green-500/20">
						<Card.Content class="p-4">
							<div class="flex flex-col sm:flex-row items-start sm:items-center gap-3">
								<div class="flex-1">
									<p class="text-sm font-medium">Ready to create tasks?</p>
									<p class="text-xs text-muted-foreground mt-0.5">
										This will create {epic.proposed_tasks.length} task{epic.proposed_tasks.length !== 1 ? 's' : ''} from the proposed plan.
									</p>
								</div>
								<div class="flex items-center gap-3">
									<label class="flex items-center gap-2 cursor-pointer">
										<input
											type="checkbox"
											bind:checked={notReady}
											class="w-3.5 h-3.5 rounded border-input accent-primary"
										/>
										<span class="text-xs flex items-center gap-1">
											<PauseCircle class="w-3 h-3" />
											Hold tasks
										</span>
									</label>
									<Button onclick={handleConfirm} disabled={confirming} class="gap-1.5 bg-green-600 hover:bg-green-700">
										{#if confirming}
											<Loader2 class="w-4 h-4 animate-spin" />
											Confirming...
										{:else}
											<Check class="w-4 h-4" />
											Confirm Epic
										{/if}
									</Button>
								</div>
							</div>
						</Card.Content>
					</Card.Root>
				{/if}
			</div>

			<!-- Right: Planning Session -->
			<div class="flex flex-col min-h-0">
				<h2 class="text-sm font-semibold mb-3 flex items-center gap-2">
					<MessageSquare class="w-4 h-4 text-violet-400" />
					Planning Session
				</h2>

				{#if !epic.planning_prompt && epic.status === 'draft'}
					<!-- Initial planning prompt -->
					<Card.Root class="bg-[oklch(0.18_0.005_285.823)] flex-1 flex flex-col">
						<Card.Content class="p-4 flex-1 flex flex-col">
							<p class="text-xs text-muted-foreground mb-3">
								Provide additional instructions for the AI planning agent. It will analyze the epic and propose a task breakdown.
							</p>
							<textarea
								bind:value={planningPrompt}
								class="flex-1 min-h-[120px] border rounded-lg p-3 bg-background text-foreground resize-none focus:outline-none focus:ring-2 focus:ring-ring text-sm"
								placeholder="e.g., Break this into small, independently testable tasks. Each task should be completable in one PR. Prioritize the data model and API first, then the UI."
							></textarea>
							<Button
								onclick={handleStartPlanning}
								disabled={!planningPrompt.trim() || startingPlanning}
								class="mt-3 gap-1.5 bg-violet-600 hover:bg-violet-700 w-full"
							>
								{#if startingPlanning}
									<Loader2 class="w-4 h-4 animate-spin" />
									Starting...
								{:else}
									<Play class="w-4 h-4" />
									Start Planning
								{/if}
							</Button>
						</Card.Content>
					</Card.Root>
				{:else}
					<!-- Session log & chat -->
					<Card.Root class="bg-[oklch(0.18_0.005_285.823)] flex-1 flex flex-col min-h-[300px]">
						<Card.Content class="p-3 flex-1 flex flex-col min-h-0">
							{#if epic.planning_prompt}
								<div class="text-xs text-muted-foreground mb-2 pb-2 border-b border-border/50">
									<span class="font-medium text-violet-400">Planning prompt:</span>
									<p class="mt-1 line-clamp-3">{epic.planning_prompt}</p>
								</div>
							{/if}

							<!-- Session log -->
							<div class="flex-1 overflow-y-auto space-y-2 min-h-0 mb-3 max-h-[40vh]">
								{#each epic.session_log as line}
									<div class="text-xs {line.startsWith('user:') ? 'text-blue-400' : 'text-muted-foreground'}">
										{line}
									</div>
								{/each}
								{#if epic.session_log.length === 0}
									<p class="text-xs text-muted-foreground text-center py-4">
										{#if isPlanning}
											Planning session is active. Send messages to guide the agent.
										{:else}
											Session log will appear here.
										{/if}
									</p>
								{/if}
							</div>

							<!-- Message input -->
							{#if isPlanning || isEditable}
								<div class="flex items-center gap-2">
									<input
										type="text"
										bind:value={sessionMessage}
										class="flex-1 border rounded-lg px-3 py-2 bg-background text-foreground text-sm focus:outline-none focus:ring-2 focus:ring-ring"
										placeholder={isPlanning ? 'Send feedback to the agent...' : 'Resume planning with a message...'}
										disabled={sendingMessage}
										onkeydown={(e) => {
											if (e.key === 'Enter' && !e.shiftKey) {
												e.preventDefault();
												handleSendMessage();
											}
										}}
									/>
									<Button
										size="sm"
										onclick={handleSendMessage}
										disabled={!sessionMessage.trim() || sendingMessage}
										class="shrink-0"
									>
										{#if sendingMessage}
											<Loader2 class="w-4 h-4 animate-spin" />
										{:else}
											<Send class="w-4 h-4" />
										{/if}
									</Button>
								</div>
								{#if isPlanning}
									<Button
										variant="outline"
										size="sm"
										onclick={handleFinishPlanning}
										disabled={finishingPlanning}
										class="mt-2 w-full gap-1.5 text-xs"
									>
										{#if finishingPlanning}
											<Loader2 class="w-3.5 h-3.5 animate-spin" />
										{:else}
											<Square class="w-3.5 h-3.5" />
										{/if}
										Stop Planning Session
									</Button>
								{/if}
							{/if}
						</Card.Content>
					</Card.Root>
				{/if}

				<!-- Task IDs (after confirmation) -->
				{#if epic.task_ids.length > 0}
					<Card.Root class="mt-4 bg-[oklch(0.18_0.005_285.823)]">
						<Card.Content class="p-4">
							<h3 class="text-xs font-semibold mb-2">Created Tasks</h3>
							<div class="space-y-1.5">
								{#each epic.task_ids as taskId}
									<a
										href="/tasks/{taskId}"
										class="flex items-center gap-2 text-xs text-primary hover:underline"
									>
										<CheckCircle2 class="w-3 h-3" />
										{taskId}
									</a>
								{/each}
							</div>
						</Card.Content>
					</Card.Root>
				{/if}
			</div>
		</div>
	{/if}
</div>
