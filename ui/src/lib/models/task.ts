export type TaskStatus = 'pending' | 'running' | 'review' | 'merged' | 'closed' | 'failed';

export interface Task {
	id: string;
	description: string;
	status: TaskStatus;
	logs: string[];
	pull_request_url?: string;
	pr_number?: number;
	depends_on?: string[];
	close_reason?: string;
	created_at: string;
	updated_at: string;
}
