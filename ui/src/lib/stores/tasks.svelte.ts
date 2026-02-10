import type { Task, TaskStatus } from '$lib/models/task';

const ALL_STATUSES: TaskStatus[] = ['pending', 'running', 'review', 'merged', 'completed', 'failed'];

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
			completed: [],
			failed: []
		};
		for (const task of this.tasks) {
			if (grouped[task.status]) {
				grouped[task.status].push(task);
			}
		}
		return grouped;
	}

	get statuses(): TaskStatus[] {
		return ALL_STATUSES;
	}

	setTasks(tasks: Task[]) {
		this.tasks = tasks;
	}

	updateTask(task: Task) {
		const idx = this.tasks.findIndex((t) => t.id === task.id);
		if (idx >= 0) {
			this.tasks[idx] = task;
		}
	}

	clear() {
		this.tasks = [];
		this.error = null;
	}
}

export const taskStore = new TaskStore();
