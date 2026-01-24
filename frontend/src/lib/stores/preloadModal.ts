import { writable } from 'svelte/store';
import type { MemoryEstimate } from '$lib/api/types';

interface PreloadModalState {
	open: boolean;
	memoryEstimate: MemoryEstimate | null;
	onConfirm: (() => void) | null;
}

function createPreloadModalStore() {
	const { subscribe, set, update } = writable<PreloadModalState>({
		open: false,
		memoryEstimate: null,
		onConfirm: null
	});

	return {
		subscribe,
		show: (memoryEstimate: MemoryEstimate | null, onConfirm: () => void) => {
			set({ open: true, memoryEstimate, onConfirm });
		},
		close: () => {
			update(state => ({ ...state, open: false }));
		},
		confirm: () => {
			update(state => {
				if (state.onConfirm) {
					state.onConfirm();
				}
				return { ...state, open: false };
			});
		}
	};
}

export const preloadModal = createPreloadModalStore();
