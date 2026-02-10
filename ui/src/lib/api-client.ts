import { API_BASE_URL } from './config/api';
import type { Task } from './models/task';

export class VerveClient {
	private baseUrl: string;

	constructor() {
		this.baseUrl = API_BASE_URL + '/api/v1';
	}

	async listTasks(): Promise<Task[]> {
		const res = await fetch(`${this.baseUrl}/tasks`);
		if (!res.ok) {
			throw new Error('Failed to fetch tasks');
		}
		return res.json();
	}

	async getTask(id: string): Promise<Task> {
		const res = await fetch(`${this.baseUrl}/tasks/${id}`);
		if (!res.ok) {
			throw new Error('Task not found');
		}
		return res.json();
	}

	async createTask(description: string, dependsOn?: string[]): Promise<Task> {
		const res = await fetch(`${this.baseUrl}/tasks`, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ description, depends_on: dependsOn })
		});
		if (!res.ok) {
			throw new Error('Failed to create task');
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

	async syncAllTasks(): Promise<{ synced: number; merged: number }> {
		const res = await fetch(`${this.baseUrl}/tasks/sync`, {
			method: 'POST'
		});
		if (!res.ok) {
			throw new Error('Failed to sync tasks');
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
}

export const client = new VerveClient();
