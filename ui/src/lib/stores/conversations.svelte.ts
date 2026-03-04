import type { Conversation } from '$lib/models/conversation';

class ConversationStore {
	conversations = $state<Conversation[]>([]);
	loading = $state(false);
	error = $state<string | null>(null);

	get activeConversations(): Conversation[] {
		return this.conversations.filter((c) => c.status === 'active');
	}

	get archivedConversations(): Conversation[] {
		return this.conversations.filter((c) => c.status === 'archived');
	}

	setConversations(convs: Conversation[]) {
		this.conversations = convs;
	}

	addConversation(conv: Conversation) {
		this.conversations = [conv, ...this.conversations];
	}

	updateConversation(conv: Conversation) {
		const idx = this.conversations.findIndex((c) => c.id === conv.id);
		if (idx >= 0) {
			this.conversations[idx] = conv;
		} else {
			this.conversations = [conv, ...this.conversations];
		}
	}

	removeConversation(id: string) {
		this.conversations = this.conversations.filter((c) => c.id !== id);
	}

	clear() {
		this.conversations = [];
		this.error = null;
	}
}

export const conversationStore = new ConversationStore();
