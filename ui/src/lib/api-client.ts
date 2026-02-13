import { API_BASE_URL } from './config/api';
import type { Task } from './models/task';
import type { Repo, GitHubRepo } from './models/repo';

export class VerveClient {
	private baseUrl: string;

	constructor() {
		this.baseUrl = API_BASE_URL + '/api/v1';
	}

	// --- Repo APIs ---

	async listRepos(): Promise<Repo[]> {
		const res = await fetch(`${this.baseUrl}/repos`);
		if (!res.ok) {
			throw new Error('Failed to fetch repos');
		}
		return res.json();
	}

	async addRepo(fullName: string): Promise<Repo> {
		const res = await fetch(`${this.baseUrl}/repos`, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ full_name: fullName })
		});
		if (!res.ok) {
			throw new Error('Failed to add repo');
		}
		return res.json();
	}

	async removeRepo(repoId: string): Promise<void> {
		const res = await fetch(`${this.baseUrl}/repos/${repoId}`, {
			method: 'DELETE'
		});
		if (!res.ok) {
			throw new Error('Failed to remove repo');
		}
	}

	async listAvailableRepos(): Promise<GitHubRepo[]> {
		const res = await fetch(`${this.baseUrl}/repos/available`);
		if (!res.ok) {
			throw new Error('Failed to list available repos');
		}
		return res.json();
	}

	// --- Repo-scoped Task APIs ---

	async listTasksByRepo(repoId: string): Promise<Task[]> {
		const res = await fetch(`${this.baseUrl}/repos/${repoId}/tasks`);
		if (!res.ok) {
			throw new Error('Failed to fetch tasks');
		}
		return res.json();
	}

	async createTaskInRepo(repoId: string, description: string, dependsOn?: string[]): Promise<Task> {
		const res = await fetch(`${this.baseUrl}/repos/${repoId}/tasks`, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ description, depends_on: dependsOn })
		});
		if (!res.ok) {
			throw new Error('Failed to create task');
		}
		return res.json();
	}

	async syncRepoTasks(repoId: string): Promise<{ synced: number; merged: number }> {
		const res = await fetch(`${this.baseUrl}/repos/${repoId}/tasks/sync`, {
			method: 'POST'
		});
		if (!res.ok) {
			throw new Error('Failed to sync tasks');
		}
		return res.json();
	}

	// --- Task APIs (global by ID) ---

	async getTask(id: string): Promise<Task> {
		const res = await fetch(`${this.baseUrl}/tasks/${id}`);
		if (!res.ok) {
			throw new Error('Task not found');
		}
		return res.json();
	}

	async syncTask(id: string): Promise<Task> {
		const res = await fetch(`${this.baseUrl}/tasks/${id}/sync`, {
			method: 'POST'
		});
		if (!res.ok) {
			throw new Error('Failed to sync task');
		}
		return res.json();
	}

	async closeTask(id: string, reason?: string): Promise<Task> {
		const res = await fetch(`${this.baseUrl}/tasks/${id}/close`, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ reason })
		});
		if (!res.ok) {
			throw new Error('Failed to close task');
		}
		return res.json();
	}

	// --- SSE URLs ---

	eventsURL(repoId?: string): string {
		if (repoId) {
			return `${this.baseUrl}/events?repo_id=${repoId}`;
		}
		return `${this.baseUrl}/events`;
	}

	taskLogsURL(id: string): string {
		return `${this.baseUrl}/tasks/${id}/logs`;
	}
}

export const client = new VerveClient();
