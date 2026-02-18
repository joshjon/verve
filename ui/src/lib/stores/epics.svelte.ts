import type { Epic, EpicStatus } from '$lib/models/epic';

class EpicStore {
	epics = $state<Epic[]>([]);
	loading = $state(false);
	error = $state<string | null>(null);

	get epicsByStatus(): Record<EpicStatus, Epic[]> {
		const grouped: Record<EpicStatus, Epic[]> = {
			draft: [],
			planning: [],
			ready: [],
			active: [],
			completed: [],
			closed: []
		};
		for (const epic of this.epics) {
			if (grouped[epic.status]) {
				grouped[epic.status].push(epic);
			}
		}
		return grouped;
	}

	setEpics(epics: Epic[]) {
		this.epics = epics;
	}

	addEpic(epic: Epic) {
		this.epics = [epic, ...this.epics];
	}

	updateEpic(epic: Epic) {
		const idx = this.epics.findIndex((e) => e.id === epic.id);
		if (idx >= 0) {
			this.epics[idx] = epic;
		} else {
			this.epics = [epic, ...this.epics];
		}
	}

	removeEpic(id: string) {
		this.epics = this.epics.filter((e) => e.id !== id);
	}

	clear() {
		this.epics = [];
		this.error = null;
	}
}

export const epicStore = new EpicStore();
