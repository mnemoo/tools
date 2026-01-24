<script lang="ts">
	import { api, type EventInfo, type LGSSessionSummary } from '$lib/api';
	import { loadGameSettings, openGame as openGameHelper, openReplay } from '$lib/openGame';
	import { _ } from '$lib/i18n';

	let {
		mode
	}: {
		mode: string;
	} = $props();

	// Search state
	let searchInput = $state('');
	let searchedSimId = $state<number | null>(null);
	let eventInfo = $state<EventInfo | null>(null);
	let loading = $state(false);
	let error = $state<string | null>(null);

	// LGS state
	let sessions = $state<LGSSessionSummary[]>([]);
	let selectedSession = $state<string>('');
	let forceLoading = $state(false);
	let forceMessage = $state<string | null>(null);
	let forceError = $state<string | null>(null);

	// Open game state
	let openGameLoading = $state(false);
	let openGameError = $state<string | null>(null);

	// Replay state
	let replayLoading = $state(false);

	// Load sessions on mount
	$effect(() => {
		loadSessions();
	});

	// Clear event when mode changes
	$effect(() => {
		if (mode) {
			eventInfo = null;
			searchedSimId = null;
			error = null;
			forceMessage = null;
			forceError = null;
		}
	});

	async function loadSessions() {
		try {
			const response = await api.lgsSessions();
			sessions = response.sessions;
			if (sessions.length > 0 && !selectedSession) {
				selectedSession = sessions[0].sessionID;
			}
		} catch {
			// Ignore - LGS may not be active
		}
	}

	async function searchEvent() {
		const simId = parseInt(searchInput.trim());
		if (isNaN(simId) || simId < 0) {
			error = 'Please enter a valid Event ID (positive number)';
			return;
		}

		loading = true;
		error = null;
		eventInfo = null;
		forceMessage = null;
		forceError = null;

		try {
			eventInfo = await api.getEvent(mode, simId);
			searchedSimId = simId;
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load event';
		} finally {
			loading = false;
		}
	}

	function handleKeyDown(e: KeyboardEvent) {
		if (e.key === 'Enter') {
			searchEvent();
		}
	}

	async function forceOutcome() {
		if (!selectedSession || searchedSimId === null) return;

		forceLoading = true;
		forceError = null;
		forceMessage = null;

		try {
			const response = await api.lgsForceOutcome(selectedSession, mode, searchedSimId);
			forceMessage = `Forced ${response.payout.toFixed(2)}x for next spin`;
		} catch (e) {
			forceError = e instanceof Error ? e.message : 'Failed to force outcome';
		} finally {
			forceLoading = false;
		}
	}

	async function forceAndOpenGame() {
		if (!selectedSession || searchedSimId === null) return;

		openGameLoading = true;
		openGameError = null;
		forceError = null;
		forceMessage = null;

		try {
			// First force the outcome
			const forceResponse = await api.lgsForceOutcome(selectedSession, mode, searchedSimId);
			forceMessage = `Forced ${forceResponse.payout.toFixed(2)}x`;

			// Load game settings and open game
			const gameSettings = await loadGameSettings();
			if (!gameSettings?.gameUrl) {
				throw new Error('Game URL not configured');
			}

			openGameHelper(gameSettings.gameUrl, selectedSession);
		} catch (e) {
			openGameError = e instanceof Error ? e.message : 'Failed to open game';
		} finally {
			openGameLoading = false;
		}
	}

	async function handleReplay() {
		if (searchedSimId === null) return;

		replayLoading = true;
		try {
			const gameSettings = await loadGameSettings();
			if (!gameSettings?.gameUrl) {
				throw new Error('Game URL not configured');
			}

			openReplay(gameSettings.gameUrl, mode, searchedSimId);
		} catch (e) {
			openGameError = e instanceof Error ? e.message : 'Failed to open replay';
		} finally {
			replayLoading = false;
		}
	}

	function formatOdds(odds: string): string {
		return odds || 'N/A';
	}

	function formatProbability(prob: number): string {
		if (prob >= 0.01) return `${(prob * 100).toFixed(2)}%`;
		if (prob >= 0.0001) return `${(prob * 100).toFixed(4)}%`;
		return `${(prob * 100).toExponential(2)}%`;
	}

	function copyToClipboard(text: string) {
		navigator.clipboard.writeText(text);
	}

	// Parse event JSON for display
	function parseEventData(event: unknown): Record<string, unknown> | null {
		if (!event) return null;
		if (typeof event === 'object') return event as Record<string, unknown>;
		try {
			return JSON.parse(String(event));
		} catch {
			return null;
		}
	}

	const eventData = $derived(eventInfo ? parseEventData(eventInfo.event) : null);
</script>

<div class="space-y-6">
	<!-- Search Section -->
	<div class="glass-panel rounded-2xl p-6">
		<div class="flex items-center gap-3 mb-6">
			<div class="w-1 h-5 rounded-full bg-cyan-400"></div>
			<h3 class="font-display text-lg text-[var(--color-light)] tracking-wider">{$_('events.viewer')}</h3>
			<span class="text-xs font-mono text-[var(--color-mist)]">Mode: {mode}</span>
		</div>

		<!-- Search Input -->
		<div class="flex gap-3">
			<div class="flex-1 relative">
				<input
					type="text"
					bind:value={searchInput}
					onkeydown={handleKeyDown}
					placeholder={$_('events.searchPlaceholder')}
					class="w-full px-4 py-3 bg-slate-800/50 border border-slate-700 rounded-xl text-white placeholder-slate-500 font-mono focus:outline-none focus:border-cyan-500 focus:ring-1 focus:ring-cyan-500/50 transition-all"
				/>
				{#if searchInput}
					<button
						onclick={() => { searchInput = ''; eventInfo = null; searchedSimId = null; error = null; }}
						class="absolute right-3 top-1/2 -translate-y-1/2 text-slate-500 hover:text-white transition-colors"
					>
						<svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
							<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
						</svg>
					</button>
				{/if}
			</div>
			<button
				onclick={searchEvent}
				disabled={loading || !searchInput.trim()}
				class="px-6 py-3 bg-gradient-to-r from-cyan-500 to-blue-500 text-white font-semibold rounded-xl hover:from-cyan-400 hover:to-blue-400 disabled:opacity-50 disabled:cursor-not-allowed transition-all flex items-center gap-2"
			>
				{#if loading}
					<svg class="w-5 h-5 animate-spin" fill="none" viewBox="0 0 24 24">
						<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
						<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"></path>
					</svg>
				{:else}
					<svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
						<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
					</svg>
				{/if}
				{$_('events.search')}
			</button>
		</div>

		<!-- Error Message -->
		{#if error}
			<div class="mt-4 p-4 rounded-xl bg-red-500/10 border border-red-500/30 text-red-400 text-sm">
				{error}
			</div>
		{/if}
	</div>

	<!-- Event Info Section -->
	{#if eventInfo}
		<div class="glass-panel rounded-2xl p-6">
			<!-- Header with badges -->
			<div class="flex items-center justify-between mb-6">
				<div class="flex items-center gap-3">
					<div class="w-1 h-5 rounded-full bg-emerald-400"></div>
					<h3 class="font-display text-lg text-[var(--color-light)] tracking-wider">EVENT #{searchedSimId}</h3>

					<!-- Status badges -->
					<div class="flex gap-2">
						{#if eventInfo.lazy_load}
							<span class="px-2 py-0.5 text-xs font-medium rounded-full bg-purple-500/20 text-purple-400 border border-purple-500/30">
								{$_('events.lazyLoaded')}
							</span>
						{/if}
						{#if eventInfo.events_loaded}
							<span class="px-2 py-0.5 text-xs font-medium rounded-full bg-emerald-500/20 text-emerald-400 border border-emerald-500/30">
								{$_('events.fromCache')}
							</span>
						{/if}
						{#if eventInfo.event_missing}
							<span class="px-2 py-0.5 text-xs font-medium rounded-full bg-amber-500/20 text-amber-400 border border-amber-500/30">
								{$_('events.eventMissing')}
							</span>
						{/if}
					</div>
				</div>

				<button
					onclick={() => copyToClipboard(String(searchedSimId))}
					class="px-3 py-1.5 text-xs font-medium rounded-lg bg-slate-700/50 text-slate-400 hover:bg-slate-700 hover:text-white transition-colors flex items-center gap-1.5"
				>
					<svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
						<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z" />
					</svg>
					{$_('events.copyId')}
				</button>
			</div>

			<!-- Stats Grid -->
			<div class="grid grid-cols-2 md:grid-cols-4 gap-4 mb-6">
				<!-- Payout -->
				<div class="p-4 rounded-xl bg-gradient-to-br from-emerald-500/10 to-emerald-600/5 border border-emerald-500/20">
					<div class="text-xs text-slate-400 mb-1">{$_('events.payout')}</div>
					<div class="text-2xl font-bold text-emerald-400">{eventInfo.payout.toFixed(2)}x</div>
				</div>

				<!-- Weight -->
				<div class="p-4 rounded-xl bg-gradient-to-br from-blue-500/10 to-blue-600/5 border border-blue-500/20">
					<div class="text-xs text-slate-400 mb-1">{$_('events.weight')}</div>
					<div class="text-2xl font-bold text-blue-400">{eventInfo.weight.toLocaleString()}</div>
				</div>

				<!-- Odds -->
				<div class="p-4 rounded-xl bg-gradient-to-br from-purple-500/10 to-purple-600/5 border border-purple-500/20">
					<div class="text-xs text-slate-400 mb-1">{$_('events.odds')}</div>
					<div class="text-lg font-bold text-purple-400">{formatOdds(eventInfo.odds)}</div>
				</div>

				<!-- Probability -->
				<div class="p-4 rounded-xl bg-gradient-to-br from-amber-500/10 to-amber-600/5 border border-amber-500/20">
					<div class="text-xs text-slate-400 mb-1">{$_('events.probability')}</div>
					<div class="text-lg font-bold text-amber-400">{formatProbability(eventInfo.probability)}</div>
				</div>
			</div>

			<!-- LGS Integration -->
			{#if sessions.length > 0}
				<div class="p-4 rounded-xl bg-slate-800/30 border border-slate-700/50 mb-6">
					<div class="flex items-center gap-3 mb-4">
						<svg class="w-5 h-5 text-cyan-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
							<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 12h14M12 5l7 7-7 7" />
						</svg>
						<span class="text-sm font-semibold text-white">{$_('events.lgsIntegration')}</span>
					</div>

					<div class="flex flex-wrap items-center gap-3">
						<!-- Session Selector -->
						<select
							bind:value={selectedSession}
							class="px-3 py-2 bg-slate-700/50 border border-slate-600 rounded-lg text-white text-sm focus:outline-none focus:border-cyan-500"
						>
							{#each sessions as session}
								<option value={session.sessionID}>{session.sessionID}</option>
							{/each}
						</select>

						<!-- Force Outcome Button -->
						<button
							onclick={forceOutcome}
							disabled={forceLoading || !selectedSession}
							class="px-4 py-2 text-sm font-semibold rounded-lg bg-gradient-to-r from-orange-500 to-amber-500 text-slate-900 hover:from-orange-400 hover:to-amber-400 disabled:opacity-50 disabled:cursor-not-allowed transition-all flex items-center gap-2"
						>
							{#if forceLoading}
								<svg class="w-4 h-4 animate-spin" fill="none" viewBox="0 0 24 24">
									<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
									<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"></path>
								</svg>
							{:else}
								<svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
									<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 10V3L4 14h7v7l9-11h-7z" />
								</svg>
							{/if}
							{$_('events.forceOutcome')}
						</button>

						<!-- Force & Open Game -->
						<button
							onclick={forceAndOpenGame}
							disabled={openGameLoading || !selectedSession}
							class="px-4 py-2 text-sm font-semibold rounded-lg bg-gradient-to-r from-emerald-500 to-cyan-500 text-slate-900 hover:from-emerald-400 hover:to-cyan-400 disabled:opacity-50 disabled:cursor-not-allowed transition-all flex items-center gap-2"
						>
							{#if openGameLoading}
								<svg class="w-4 h-4 animate-spin" fill="none" viewBox="0 0 24 24">
									<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
									<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"></path>
								</svg>
							{:else}
								<svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
									<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M14.752 11.168l-3.197-2.132A1 1 0 0010 9.87v4.263a1 1 0 001.555.832l3.197-2.132a1 1 0 000-1.664z" />
									<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
								</svg>
							{/if}
							{$_('events.forceAndPlay')}
						</button>

						<!-- Replay Button -->
						<button
							onclick={handleReplay}
							disabled={replayLoading || eventInfo.event_missing}
							class="px-4 py-2 text-sm font-semibold rounded-lg bg-slate-700 text-white hover:bg-slate-600 disabled:opacity-50 disabled:cursor-not-allowed transition-all flex items-center gap-2"
						>
							{#if replayLoading}
								<svg class="w-4 h-4 animate-spin" fill="none" viewBox="0 0 24 24">
									<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
									<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"></path>
								</svg>
							{:else}
								<svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
									<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
								</svg>
							{/if}
							{$_('events.replay')}
						</button>
					</div>

					<!-- Force Messages -->
					{#if forceMessage}
						<div class="mt-3 p-3 rounded-lg bg-emerald-500/10 border border-emerald-500/30 text-emerald-400 text-sm">
							{forceMessage}
						</div>
					{/if}
					{#if forceError}
						<div class="mt-3 p-3 rounded-lg bg-red-500/10 border border-red-500/30 text-red-400 text-sm">
							{forceError}
						</div>
					{/if}
					{#if openGameError}
						<div class="mt-3 p-3 rounded-lg bg-red-500/10 border border-red-500/30 text-red-400 text-sm">
							{openGameError}
						</div>
					{/if}
				</div>
			{:else}
				<div class="p-4 rounded-xl bg-slate-800/30 border border-slate-700/50 mb-6 text-center text-slate-500 text-sm">
					{$_('events.noLgsSessions')}
				</div>
			{/if}

			<!-- Event Data -->
			{#if eventData && !eventInfo.event_missing}
				<div class="rounded-xl border border-slate-700/50 overflow-hidden">
					<div class="px-4 py-3 bg-slate-800/50 border-b border-slate-700/50 flex items-center justify-between">
						<span class="text-sm font-semibold text-white">{$_('events.eventData')}</span>
						<button
							onclick={() => copyToClipboard(JSON.stringify(eventData, null, 2))}
							class="px-3 py-1 text-xs font-medium rounded-lg bg-slate-700/50 text-slate-400 hover:bg-slate-700 hover:text-white transition-colors flex items-center gap-1.5"
						>
							<svg class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
								<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z" />
							</svg>
							{$_('events.copy')}
						</button>
					</div>
					<div class="p-4 bg-slate-900/50 max-h-96 overflow-auto">
						<pre class="text-xs font-mono text-slate-300 whitespace-pre-wrap">{JSON.stringify(eventData, null, 2)}</pre>
					</div>
				</div>
			{:else if eventInfo.event_missing}
				<div class="p-6 rounded-xl bg-amber-500/10 border border-amber-500/30 text-center">
					<svg class="w-12 h-12 mx-auto text-amber-400 mb-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
						<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
					</svg>
					<p class="text-amber-400 font-semibold">{$_('events.notAvailable')}</p>
					<p class="text-sm text-slate-400 mt-2">{$_('events.notFoundDesc')}</p>
				</div>
			{/if}
		</div>
	{:else if !loading && !error}
		<!-- Empty State -->
		<div class="glass-panel rounded-2xl p-12 text-center">
			<svg class="w-16 h-16 mx-auto text-slate-600 mb-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
				<path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
			</svg>
			<p class="text-lg text-slate-400 mb-2">{$_('events.searchForEvent')}</p>
			<p class="text-sm text-slate-500">{$_('events.searchDesc')}</p>
			<p class="text-xs text-slate-600 mt-4">{$_('events.lazyLoadNote')}</p>
		</div>
	{/if}
</div>
