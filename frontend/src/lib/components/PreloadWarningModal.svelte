<script lang="ts">
	import { preloadModal } from '$lib/stores/preloadModal';

	// Subscribe to store
	let state = $state({ open: false, memoryEstimate: null as any, onConfirm: null as any });

	$effect(() => {
		const unsubscribe = preloadModal.subscribe(s => {
			state = s;
		});
		return unsubscribe;
	});

	const open = $derived(state.open);
	const memoryEstimate = $derived(state.memoryEstimate);
	const estimatedMB = $derived(memoryEstimate?.estimated_mb ?? 0);
	const modeCount = $derived(memoryEstimate?.mode_count ?? 0);
	const compressedMB = $derived(Math.round((memoryEstimate?.compressed_bytes ?? 0) / (1024 * 1024)));

	function handleKeydown(e: KeyboardEvent) {
		if (e.key === 'Escape' && open) preloadModal.close();
	}

	function handleClose() {
		preloadModal.close();
	}

	function handleConfirm() {
		preloadModal.confirm();
	}
</script>

<svelte:window onkeydown={handleKeydown} />

{#if open}
	<!-- Backdrop -->
	<!-- svelte-ignore a11y_click_events_have_key_events a11y_no_static_element_interactions -->
	<div
		class="fixed inset-0 bg-black/80 backdrop-blur-sm"
		style="z-index: 99998;"
		onclick={handleClose}
	></div>

	<!-- Modal -->
	<div
		class="fixed inset-0 flex items-center justify-center p-4"
		style="z-index: 99999; pointer-events: none;"
	>
		<!-- svelte-ignore a11y_click_events_have_key_events a11y_no_noninteractive_element_interactions -->
		<div
			class="w-full max-w-lg rounded-2xl shadow-2xl border border-slate-700"
			style="pointer-events: auto; max-height: 90vh; overflow-y: auto;"
			onclick={(e) => e.stopPropagation()}
			role="dialog"
			aria-modal="true"
		>
			<!-- Header -->
			<div class="p-5 border-b border-slate-700">
				<div class="flex items-center gap-4">
					<div class="w-10 h-10 rounded-xl bg-amber-500/20 flex items-center justify-center shrink-0">
						<svg class="w-7 h-7 text-amber-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
							<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
						</svg>
					</div>
					<div class="flex-1">
						<h3 class="text-xl font-semibold text-white">Preload All Event Books?</h3>
						<p class="text-sm text-slate-400 mt-1">This action requires significant resources</p>
					</div>
					<button
						class="p-2 rounded-lg hover:bg-slate-700 text-slate-400 hover:text-white transition-colors"
						onclick={handleClose}
					>
						<svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
							<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
						</svg>
					</button>
				</div>
			</div>

			<!-- Content -->
			<div class="px-5 py-5 space-y-4">
				<!-- Memory Estimation Card -->
				<div class="p-3 rounded-xl bg-gradient-to-br from-red-500/20 to-orange-500/10 border border-red-500/30">
					<div class="flex items-center gap-3 mb-4">
						<svg class="w-6 h-6 text-red-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
							<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 3v2m6-2v2M9 19v2m6-2v2M5 9H3m2 6H3m18-6h-2m2 6h-2M7 19h10a2 2 0 002-2V7a2 2 0 00-2-2H7a2 2 0 00-2 2v10a2 2 0 002 2zM9 9h6v6H9V9z" />
						</svg>
						<span class="text-lg font-semibold text-red-400">Estimated Memory Usage</span>
					</div>

					<div class="grid grid-cols-2 gap-4">
						<div class="p-3 rounded-lg bg-slate-900/50">
							<div class="text-2xl font-bold text-white">
								{#if estimatedMB > 1024}
									{(estimatedMB / 1024).toFixed(1)} GB
								{:else}
									~{estimatedMB} MB
								{/if}
							</div>
							<div class="text-xs text-slate-400 mt-1">RAM Required</div>
						</div>
						<div class="p-3 rounded-lg bg-slate-900/50">
							<div class="text-2xl font-bold text-white">{modeCount}</div>
							<div class="text-xs text-slate-400 mt-1">Bet Modes</div>
						</div>
					</div>

					<div class="mt-4 text-sm text-red-200/80">
						<p>Compressed size: <strong>{compressedMB} MB</strong> → Decompressed: <strong>~{estimatedMB} MB</strong></p>
						<p class="mt-2 text-red-300">This may exceed available memory and crash the application!</p>
					</div>
				</div>

				<!-- Recommendation Card -->
				<div class="p-3 rounded-xl bg-emerald-500/10 border border-emerald-500/30">
					<div class="flex items-start gap-3">
						<svg class="w-6 h-6 text-emerald-400 mt-0.5 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor">
							<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
						</svg>
						<div class="text-sm">
							<p class="font-semibold text-emerald-400 mb-2">Lazy Loading Already Active</p>
							<p class="text-slate-300">Events are loaded automatically on-demand.</p>
							<p class="text-slate-400 mt-2">Preload only if you need instant access to ALL events simultaneously.</p>
						</div>
					</div>
				</div>
			</div>

			<!-- Actions -->
			<div class="p-6 border-t border-slate-700 flex gap-3">
				<button
					class="flex-1 px-5 py-3 text-sm font-semibold rounded-xl bg-slate-700 text-white hover:bg-slate-600 transition-colors"
					onclick={handleClose}
				>
					Cancel
				</button>
				<button
					class="flex-1 px-5 py-3 text-sm font-semibold rounded-xl bg-gradient-to-r from-amber-500 to-orange-500 text-slate-900 hover:from-amber-400 hover:to-orange-400 transition-colors"
					onclick={handleConfirm}
				>
					Preload Anyway ({estimatedMB} MB)
				</button>
			</div>
		</div>
	</div>
{/if}
