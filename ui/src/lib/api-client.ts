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

	async createTaskInRepo(
		repoId: string,
		title: string,
		description: string,
		dependsOn?: string[],
		acceptanceCriteria?: string[],
		maxCostUsd?: number,
		skipPr?: boolean,
		model?: string
	): Promise<Task> {
		const body: Record<string, unknown> = { title, description, depends_on: dependsOn };
		if (acceptanceCriteria && acceptanceCriteria.length > 0)
			body.acceptance_criteria = acceptanceCriteria;
		if (maxCostUsd && maxCostUsd > 0) body.max_cost_usd = maxCostUsd;
		if (skipPr) body.skip_pr = true;
		if (model) body.model = model;
		const res = await fetch(`${this.baseUrl}/repos/${repoId}/tasks`, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify(body)
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

	async getTaskChecks(id: string): Promise<{
		status: 'pending' | 'success' | 'failure' | 'error';
		summary?: string;
		failed_names?: string[];
		check_runs_skipped?: boolean;
		checks?: { name: string; status: string; conclusion: string; url: string }[];
	}> {
		const res = await fetch(`${this.baseUrl}/tasks/${id}/checks`);
		if (!res.ok) {
			throw new Error('Failed to fetch check status');
		}
		return res.json();
	}

	async retryTask(id: string, instructions?: string): Promise<Task> {
		const res = await fetch(`${this.baseUrl}/tasks/${id}/retry`, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ instructions })
		});
		if (!res.ok) {
			throw new Error('Failed to retry task');
		}
		return res.json();
	}

	// --- Settings APIs ---

	async getGitHubTokenStatus(): Promise<{ configured: boolean; fine_grained?: boolean }> {
		const res = await fetch(`${this.baseUrl}/settings/github-token`);
		if (!res.ok) {
			throw new Error('Failed to check GitHub token status');
		}
		return res.json();
	}

	async saveGitHubToken(token: string): Promise<void> {
		const res = await fetch(`${this.baseUrl}/settings/github-token`, {
			method: 'PUT',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ token })
		});
		if (!res.ok) {
			const body = await res.json().catch(() => null);
			throw new Error(body?.error || 'Failed to save GitHub token');
		}
	}

	async deleteGitHubToken(): Promise<void> {
		const res = await fetch(`${this.baseUrl}/settings/github-token`, {
			method: 'DELETE'
		});
		if (!res.ok) {
			throw new Error('Failed to delete GitHub token');
		}
	}

	async getDefaultModel(): Promise<{ model: string }> {
		const res = await fetch(`${this.baseUrl}/settings/default-model`);
		if (!res.ok) {
			throw new Error('Failed to get default model');
		}
		return res.json();
	}

	async saveDefaultModel(model: string): Promise<void> {
		const res = await fetch(`${this.baseUrl}/settings/default-model`, {
			method: 'PUT',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ model })
		});
		if (!res.ok) {
			throw new Error('Failed to save default model');
		}
	}

	async deleteDefaultModel(): Promise<void> {
		const res = await fetch(`${this.baseUrl}/settings/default-model`, {
			method: 'DELETE'
		});
		if (!res.ok) {
			throw new Error('Failed to delete default model');
		}
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
