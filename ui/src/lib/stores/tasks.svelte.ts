import type { Task, TaskStatus } from '$lib/models/task';

const ALL_STATUSES: TaskStatus[] = ['pending', 'running', 'review', 'merged', 'closed', 'failed'];

class TaskStore {
	tasks = $state<Task[]>([]);
	loading = $state(false);
	error = $state<string | null>(null);

	get tasksByStatus(): Record<TaskStatus, Task[]> {
		const grouped: Record<TaskStatus, Task[]> = {
			pending: [],
			running: [],
			review: [],
			merged: [],
			closed: [],
			failed: []
		};
		for (const task of this.tasks) {
			if (grouped[task.status]) {
				grouped[task.status].push(task);
			}
		}
		return grouped;
	}

	get totalCost(): number {
		return this.tasks.reduce((sum, t) => sum + (t.cost_usd || 0), 0);
	}

	get statuses(): TaskStatus[] {
		return ALL_STATUSES;
	}

	setTasks(tasks: Task[]) {
		this.tasks = tasks;
	}

	addTask(task: Task) {
		this.tasks = [...this.tasks, task];
	}

	updateTask(task: Task) {
		const idx = this.tasks.findIndex((t) => t.id === task.id);
		if (idx >= 0) {
			// Preserve existing logs when incoming task has null logs (SSE omits them).
			if (task.logs == null) {
				task = { ...task, logs: this.tasks[idx].logs };
			}
			this.tasks[idx] = task;
		} else {
			// Upsert: task not found, add it.
			this.tasks = [...this.tasks, task];
		}
	}

	clear() {
		this.tasks = [];
		this.error = null;
	}

	deleteTask(taskId: string) {
		this.tasks = this.tasks.filter(t => t.id !== taskId);
	}
}

export const taskStore = new TaskStore();
