export type ConversationStatus = 'active' | 'archived';

export interface Message {
	role: 'user' | 'assistant';
	content: string;
	timestamp: number;
}

export interface Conversation {
	id: string;
	repo_id: string;
	title: string;
	status: ConversationStatus;
	messages: Message[];
	model?: string;
	pending_message?: string;
	claimed_at?: string;
	epic_id?: string;
	created_at: string;
	updated_at: string;
}
