<script lang="ts">
	import { api } from '$lib/api';
	import type {
		LoaderModeStatus,
		MemoryEstimate,
		WSMessage,
		WSLoadingProgress,
		WSLoadingStarted,
		WSLoadingComplete,
		WSLoadingError,
		WSPriorityChanged
	} from '$lib/api/types';
	import { preloadModal } from '$lib/stores/preloadModal';
	import { _ } from '$lib/i18n';

	// State
	let modes = $state<Record<string, LoaderModeStatus>>({});
	let priority = $state<'low' | 'high'>('low');
	let started = $state(true); // Assume started by default, will be updated from API
	let wsConnected = $state(false);
	let ws: WebSocket | null = null;
	let isBoostLoading = $state(false);
	let isStartLoading = $state(false);
	let memoryEstimate = $state<MemoryEstimate | null>(null);

	// Derived state
	let allComplete = $derived(
		Object.keys(modes).length > 0 &&
			Object.values(modes).every((m) => m.status === 'complete')
	);

	let loadingModes = $derived(
		Object.values(modes).filter((m) => m.status === 'loading')
	);

	let pendingModes = $derived(
		Object.values(modes).filter((m) => m.status === 'pending')
	);

	let completeModes = $derived(
		Object.values(modes).filter((m) => m.status === 'complete')
	);

	let errorModes = $derived(
		Object.values(modes).filter((m) => m.status === 'error')
	);

	let totalModes = $derived(Object.keys(modes).length);

	let overallProgress = $derived(() => {
		if (totalModes === 0) return 0;
		const totalBytes = Object.values(modes).reduce((sum, m) => sum + m.total_bytes, 0);
		const readBytes = Object.values(modes).reduce((sum, m) => sum + m.bytes_read, 0);
		return totalBytes > 0 ? (readBytes / totalBytes) * 100 : 0;
	});

	// Format bytes
	function formatBytes(bytes: number): string {
		if (bytes < 1024) return `${bytes} B`;
		if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
		return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
	}

	// Connect WebSocket
	function connectWebSocket() {
		const wsUrl = api.getWebSocketUrl();
		ws = new WebSocket(wsUrl);

		ws.onopen = () => {
			wsConnected = true;
		};

		ws.onclose = () => {
			wsConnected = false;
			setTimeout(connectWebSocket, 3000);
		};

		ws.onerror = () => {};

		ws.onmessage = (event) => {
			try {
				const msg: WSMessage = JSON.parse(event.data);
				handleMessage(msg);
			} catch (e) {
				console.error('Failed to parse WebSocket message:', e);
			}
		};
	}

	// Handle WebSocket messages
	function handleMessage(msg: WSMessage) {
		switch (msg.type) {
			case 'loading_started': {
				const data = msg.payload as WSLoadingStarted;
				modes = {
					...modes,
					[data.mode]: {
						mode: data.mode,
						events_file: data.events_file,
						status: 'loading',
						current_line: 0,
						bytes_read: 0,
						total_bytes: data.total_bytes,
						percent_bytes: 0,
						started_at: Date.now()
					}
				};
				break;
			}
			case 'loading_progress': {
				const data = msg.payload as WSLoadingProgress;
				if (modes[data.mode]) {
					modes = {
						...modes,
						[data.mode]: {
							...modes[data.mode],
							current_line: data.current_line,
							bytes_read: data.bytes_read,
							total_bytes: data.total_bytes,
							percent_bytes: data.percent_bytes
						}
					};
				}
				priority = data.priority;
				break;
			}
			case 'loading_complete': {
				const data = msg.payload as WSLoadingComplete;
				if (modes[data.mode]) {
					modes = {
						...modes,
						[data.mode]: {
							...modes[data.mode],
							status: 'complete',
							current_line: data.total_lines,
							total_lines: data.total_lines,
							bytes_read: data.total_bytes,
							percent_bytes: 100,
							completed_at: Date.now()
						}
					};
				}
				break;
			}
			case 'loading_error': {
				const data = msg.payload as WSLoadingError;
				if (modes[data.mode]) {
					modes = {
						...modes,
						[data.mode]: {
							...modes[data.mode],
							status: 'error',
							error: data.error
						}
					};
				}
				break;
			}
			case 'priority_changed': {
				const data = msg.payload as WSPriorityChanged;
				priority = data.new_priority as 'low' | 'high';
				break;
			}
		}
	}

	// Toggle boost
	async function toggleBoost() {
		isBoostLoading = true;
		try {
			if (priority === 'low') {
				await api.loaderBoost();
				priority = 'high';
			} else {
				await api.loaderUnboost();
				priority = 'low';
			}
		} catch (e) {
			console.error('Failed to toggle boost:', e);
		} finally {
			isBoostLoading = false;
		}
	}

	// Show preload warning modal via store (renders at root level)
	function showPreloadModal() {
		preloadModal.show(memoryEstimate, async () => {
			isStartLoading = true;
			try {
				await api.loaderStart();
				started = true;
				await fetchStatus();
			} catch (e) {
				console.error('Failed to start loader:', e);
			} finally {
				isStartLoading = false;
			}
		});
	}

	// Fetch initial status
	async function fetchStatus() {
		try {
			const status = await api.loaderStatus();
			modes = status.modes;
			priority = status.priority;
			started = status.started ?? true;
			memoryEstimate = status.memory_estimate ?? null;
		} catch (e) {
			console.error('Failed to fetch loader status:', e);
		}
	}

	// Initialize
	$effect(() => {
		fetchStatus();
		connectWebSocket();

		return () => {
			if (ws) {
				ws.close();
			}
		};
	});
</script>

<div>
	<div class="flex items-center gap-3 mb-6">
		<div class="w-1 h-5 rounded-full {allComplete ? 'bg-emerald-400' : 'bg-blue-400'}"></div>
		<h3 class="font-display text-lg text-[var(--color-light)] tracking-wider">{$_('books.title')}</h3>
		<span class="text-xs font-mono text-[var(--color-mist)]">
			({completeModes.length}/{totalModes})
		</span>
		{#if !allComplete}
			<svg class="w-4 h-4 text-blue-400 animate-spin ml-auto" fill="none" viewBox="0 0 24 24">
				<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
				<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
			</svg>
		{:else if wsConnected}
			<span class="w-1.5 h-1.5 bg-emerald-400 rounded-full ml-auto" title="Live"></span>
		{/if}
	</div>

	{#if !started}
		<!-- Lazy Loading Mode -->
		<div class="p-4 rounded-xl bg-gradient-to-br from-emerald-500/10 to-cyan-600/5 border border-emerald-500/20">
			<div class="flex items-center gap-3">
				<div class="w-12 h-12 rounded-xl bg-emerald-500/20 flex items-center justify-center">
					<svg class="w-6 h-6 text-emerald-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
						<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 10V3L4 14h7v7l9-11h-7z" />
					</svg>
				</div>
				<div>
					<div class="text-lg font-semibold text-white">{$_('books.lazyEnabled')}</div>
					<div class="text-sm text-emerald-400/80">{$_('books.lazyDesc')}</div>
				</div>
			</div>

			<div class="mt-4 p-3 rounded-lg bg-slate-800/50 text-xs text-slate-400">
				<p>{$_('books.lazyNote')}</p>
				<p class="mt-1">{$_('books.lazyChunkNote')}</p>
			</div>

			<button
				class="mt-4 w-full px-4 py-2.5 text-sm font-medium rounded-lg transition-all bg-slate-700/50 text-slate-400 hover:bg-slate-700 border border-slate-600/30 disabled:opacity-50"
				onclick={showPreloadModal}
				disabled={isStartLoading}
			>
				{#if isStartLoading}
					<span class="flex items-center justify-center gap-2">
						<svg class="w-4 h-4 animate-spin" fill="none" viewBox="0 0 24 24">
							<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
							<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"></path>
						</svg>
						{$_('books.starting')}
					</span>
				{:else}
					{$_('books.preloadAll')}
				{/if}
			</button>
		</div>
	{:else if started && totalModes === 0}
		<div class="py-6 text-center text-slate-500 text-sm">
			{$_('books.noBooks')}
		</div>
	{:else if allComplete}
		<!-- All Complete State -->
		<div class="p-4 rounded-xl bg-gradient-to-br from-emerald-500/10 to-emerald-600/5 border border-emerald-500/20">
			<div class="flex items-center gap-3">
				<div class="w-12 h-12 rounded-xl bg-emerald-500/20 flex items-center justify-center">
					<svg class="w-6 h-6 text-emerald-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
						<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
					</svg>
				</div>
				<div>
					<div class="text-lg font-semibold text-white">{$_('books.allLoaded')}</div>
					<div class="text-sm text-emerald-400/80">{$_('books.booksReady', { values: { count: totalModes } })}</div>
				</div>
			</div>

			<!-- Mode badges -->
			<div class="mt-4 flex flex-wrap gap-2">
				{#each completeModes as mode}
					<span class="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-lg bg-slate-700/50 text-xs font-medium text-slate-300">
						<svg class="w-3 h-3 text-emerald-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
							<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
						</svg>
						{mode.mode}
						<span class="text-slate-500">{formatBytes(mode.total_bytes)}</span>
					</span>
				{/each}
			</div>
		</div>
	{:else}
		<!-- Loading State -->
		<div class="space-y-3">
			<!-- Overall progress -->
			<div class="p-3 rounded-xl bg-slate-700/30">
				<div class="flex items-center justify-between mb-2">
					<span class="text-xs text-slate-400">{$_('books.overallProgress')}</span>
					<span class="text-xs font-mono text-white">{overallProgress().toFixed(1)}%</span>
				</div>
				<div class="relative h-2 bg-slate-700 rounded-full overflow-hidden">
					<div
						class="absolute inset-y-0 left-0 bg-gradient-to-r from-blue-500 to-purple-500 rounded-full transition-all duration-300"
						style="width: {overallProgress()}%"
					></div>
				</div>
			</div>

			<!-- Individual modes -->
			{#each loadingModes as mode}
				<div class="p-3 rounded-xl bg-slate-700/20 border border-slate-700/50">
					<div class="flex items-center justify-between mb-2">
						<span class="text-sm font-medium text-white font-mono">{mode.mode}</span>
						<span class="text-xs text-slate-400">
							{formatBytes(mode.bytes_read)} / {formatBytes(mode.total_bytes)}
						</span>
					</div>
					<div class="relative h-1.5 bg-slate-700 rounded-full overflow-hidden">
						<div
							class="absolute inset-y-0 left-0 bg-blue-500 rounded-full transition-all duration-300"
							style="width: {mode.percent_bytes}%"
						></div>
					</div>
					<div class="flex items-center justify-between mt-1.5 text-xs text-slate-500">
						<span>{mode.current_line.toLocaleString()} {$_('books.lines')}</span>
						<span>{mode.percent_bytes.toFixed(1)}%</span>
					</div>
				</div>
			{/each}

			<!-- Pending modes -->
			{#if pendingModes.length > 0}
				<div class="flex flex-wrap gap-2">
					{#each pendingModes as mode}
						<span class="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-lg bg-slate-700/30 text-xs text-slate-500">
							<svg class="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
								<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
							</svg>
							{mode.mode}
						</span>
					{/each}
				</div>
			{/if}

			<!-- Completed modes -->
			{#if completeModes.length > 0}
				<div class="flex flex-wrap gap-2">
					{#each completeModes as mode}
						<span class="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-lg bg-emerald-500/10 text-xs text-emerald-400">
							<svg class="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
								<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
							</svg>
							{mode.mode}
						</span>
					{/each}
				</div>
			{/if}

			<!-- Error modes -->
			{#each errorModes as mode}
				<div class="p-2 rounded-lg bg-rose-500/10 border border-rose-500/20 text-xs text-rose-400">
					<span class="font-medium">{mode.mode}:</span> {mode.error}
				</div>
			{/each}

			<!-- Boost control -->
			<div class="flex items-center justify-between pt-2 border-t border-slate-700/50">
				<div class="flex items-center gap-2">
					<span class="text-xs text-slate-500">{$_('books.priority')}:</span>
					<span class="text-xs font-mono font-medium {priority === 'high' ? 'text-orange-400' : 'text-slate-400'}">
						{priority.toUpperCase()}
					</span>
				</div>
				<button
					class="px-3 py-1.5 text-xs font-medium rounded-lg transition-all {priority === 'high'
						? 'bg-orange-500/20 text-orange-400 hover:bg-orange-500/30 border border-orange-500/30'
						: 'bg-blue-500/20 text-blue-400 hover:bg-blue-500/30 border border-blue-500/30'}"
					onclick={toggleBoost}
					disabled={isBoostLoading}
				>
					{#if isBoostLoading}
						<svg class="w-3 h-3 animate-spin" fill="none" viewBox="0 0 24 24">
							<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
							<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"></path>
						</svg>
					{:else if priority === 'high'}
						{$_('books.slowDown')}
					{:else}
						{$_('books.turbo')}
					{/if}
				</button>
			</div>
		</div>
	{/if}
</div>
