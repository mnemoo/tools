<script lang="ts">
	import { api, type EventInfo, type LGSSessionSummary } from '$lib/api';
	import { loadGameSettings, openGame as openGameHelper, openReplay } from '$lib/openGame';
	import { _ } from '$lib/i18n';

	let {
		mode,
		simId,
		onClose
	}: {
		mode: string;
		simId: number;
		onClose: () => void;
	} = $props();

	let eventInfo = $state<EventInfo | null>(null);
	let loading = $state(true);
	let loadingEvents = $state(false);
	let error = $state<string | null>(null);

	// Force outcome state
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

	$effect(() => {
		loadEvent();
		loadSessions();
	});

	async function loadEvent() {
		loading = true;
		error = null;
		try {
			eventInfo = await api.getEvent(mode, simId);
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load event';
		} finally {
			loading = false;
		}
	}

	async function loadSessions() {
		try {
			const response = await api.lgsSessions();
			sessions = response.sessions;
			if (sessions.length > 0 && !selectedSession) {
				selectedSession = sessions[0].sessionID;
			}
		} catch {
			// Ignore errors - LGS may not be active
		}
	}

	async function loadEventsFile() {
		loadingEvents = true;
		error = null;
		try {
			await api.loadEvents(mode);
			await loadEvent();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load events file';
		} finally {
			loadingEvents = false;
		}
	}

	async function forceOutcome() {
		if (!selectedSession) return;

		forceLoading = true;
		forceError = null;
		forceMessage = null;

		try {
			const response = await api.lgsForceOutcome(selectedSession, mode, simId);
			forceMessage = `Set ${response.payout.toFixed(2)}x for next spin in "${selectedSession}"`;
		} catch (e) {
			forceError = e instanceof Error ? e.message : 'Failed to force outcome';
		} finally {
			forceLoading = false;
		}
	}

	async function forceAndOpenGame() {
		if (!selectedSession) return;

		openGameLoading = true;
		openGameError = null;
		forceError = null;
		forceMessage = null;

		try {
			// First, force the outcome
			const response = await api.lgsForceOutcome(selectedSession, mode, simId);
			forceMessage = `Set ${response.payout.toFixed(2)}x for next spin`;

			// Then open the game with saved settings
			const settings = loadGameSettings();
			await openGameHelper({
				sessionID: selectedSession,
				balance: settings.balance,
				currency: settings.currency,
				language: settings.language,
				device: settings.device,
				demo: settings.demo,
				social: settings.social,
				domain: settings.domain,
			});
		} catch (e) {
			openGameError = e instanceof Error ? e.message : 'Failed to open game';
		} finally {
			openGameLoading = false;
		}
	}

	function handleReplay() {
		replayLoading = true;
		try {
			const settings = loadGameSettings();
			openReplay({
				mode: mode,
				eventId: simId,
				gameUUID: settings.gameUUID,
				gameVersion: settings.gameVersion,
				currency: settings.currency,
				amount: settings.balance, // use balance as bet amount for display
				language: settings.language,
				device: settings.device,
				social: settings.social,
				domain: settings.domain,
			});
		} finally {
			replayLoading = false;
		}
	}

	function formatNumber(n: number): string {
		return n.toLocaleString('en-US', { maximumFractionDigits: 6 });
	}

	function formatProbability(p: number): string {
		if (p >= 0.01) {
			return (p * 100).toFixed(2) + '%';
		}
		return p.toExponential(4);
	}
</script>

<div
	class="fixed inset-0 z-50 flex items-center justify-center bg-black/70"
	onclick={onClose}
	onkeydown={(e) => e.key === 'Escape' && onClose()}
	role="dialog"
	tabindex="-1"
>
	<!-- svelte-ignore a11y_click_events_have_key_events a11y_no_noninteractive_element_interactions -->
	<div
		class="relative max-h-[90vh] w-full max-w-4xl overflow-hidden rounded-lg bg-gray-800 shadow-xl"
		onclick={(e) => e.stopPropagation()}
		role="document"
	>
		<!-- Header -->
		<div class="flex items-center justify-between border-b border-gray-700 px-6 py-4">
			<div>
				<h2 class="text-lg font-semibold text-white">{$_('events.eventDetails')}</h2>
				<p class="text-sm text-gray-400">
					Mode: {mode} | SimID: {simId}
				</p>
			</div>
			<button onclick={onClose} class="text-gray-400 hover:text-white" aria-label="Close">
				<svg class="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
					<path
						stroke-linecap="round"
						stroke-linejoin="round"
						stroke-width="2"
						d="M6 18L18 6M6 6l12 12"
					/>
				</svg>
			</button>
		</div>

		<!-- Content -->
		<div class="max-h-[calc(90vh-120px)] overflow-y-auto p-6">
			{#if loading}
				<div class="flex items-center justify-center py-12">
					<div
						class="h-10 w-10 animate-spin rounded-full border-4 border-gray-600 border-t-blue-500"
					></div>
				</div>
			{:else if error}
				<div class="rounded-lg bg-red-900/30 p-4 text-red-400">
					{error}
				</div>
			{:else if eventInfo}
				<!-- Statistics -->
				<div class="mb-6 grid grid-cols-2 gap-4 sm:grid-cols-3">
					<div class="rounded-lg bg-gray-700/50 p-4">
						<div class="text-xs text-gray-400">{$_('events.payout')}</div>
						<div class="mt-1 text-xl font-bold text-green-400">
							{formatNumber(eventInfo.payout)}x
						</div>
					</div>
					<div class="rounded-lg bg-gray-700/50 p-4">
						<div class="text-xs text-gray-400">{$_('events.weight')}</div>
						<div class="mt-1 text-xl font-bold text-white">
							{formatNumber(eventInfo.weight)}
						</div>
					</div>
					<div class="rounded-lg bg-gray-700/50 p-4">
						<div class="text-xs text-gray-400">{$_('events.odds')}</div>
						<div class="mt-1 text-xl font-bold text-purple-400">
							{eventInfo.odds}
						</div>
					</div>
				</div>

				<!-- Replay Panel -->
				<div class="mb-6 rounded-lg bg-blue-900/20 border border-blue-500/30 p-4">
					<div class="flex items-center justify-between">
						<div class="flex items-center gap-3">
							<svg class="w-5 h-5 text-blue-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
								<path stroke-linecap="round" stroke-linejoin="round" d="M14.752 11.168l-3.197-2.132A1 1 0 0010 9.87v4.263a1 1 0 001.555.832l3.197-2.132a1 1 0 000-1.664z" />
								<path stroke-linecap="round" stroke-linejoin="round" d="M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
							</svg>
							<div>
								<span class="text-sm font-semibold text-blue-400">{$_('events.replayEvent')}</span>
								<p class="text-xs text-gray-400">{$_('events.replayDesc')}</p>
							</div>
						</div>
						<button
							onclick={handleReplay}
							disabled={replayLoading}
							class="px-4 py-2 rounded bg-blue-500 text-white font-semibold text-sm hover:bg-blue-400 transition-colors disabled:opacity-50 disabled:cursor-not-allowed flex items-center gap-2"
						>
							{#if replayLoading}
								<svg class="w-4 h-4 animate-spin" fill="none" viewBox="0 0 24 24">
									<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
									<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
								</svg>
								{$_('common.opening')}
							{:else}
								<svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
									<path stroke-linecap="round" stroke-linejoin="round" d="M14.752 11.168l-3.197-2.132A1 1 0 0010 9.87v4.263a1 1 0 001.555.832l3.197-2.132a1 1 0 000-1.664z" />
									<path stroke-linecap="round" stroke-linejoin="round" d="M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
								</svg>
								{$_('events.replay')}
							{/if}
						</button>
					</div>
				</div>

				<!-- Force Outcome Panel -->
				<div class="mb-6 rounded-lg bg-amber-900/20 border border-amber-500/30 p-4">
					<div class="flex items-center gap-3 mb-3">
						<svg class="w-5 h-5 text-amber-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
							<path stroke-linecap="round" stroke-linejoin="round" d="M13 10V3L4 14h7v7l9-11h-7z" />
						</svg>
						<span class="text-sm font-semibold text-amber-400">{$_('lgs.forceNextOutcome')}</span>
					</div>
					{#if sessions.length > 0}
						<div class="flex items-center gap-3 flex-wrap">
							<select
								bind:value={selectedSession}
								class="bg-gray-700 border border-gray-600 rounded px-3 py-2 text-sm text-white focus:outline-none focus:border-amber-500"
							>
								{#each sessions as session}
									<option value={session.sessionID}>{session.sessionID}</option>
								{/each}
							</select>
							<button
								onclick={forceOutcome}
								disabled={forceLoading || openGameLoading || !selectedSession}
								class="px-4 py-2 rounded bg-amber-500 text-gray-900 font-semibold text-sm hover:bg-amber-400 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
							>
								{#if forceLoading}
									{$_('common.setting')}
								{:else}
									{$_('events.forceOutcome')}
								{/if}
							</button>
							<button
								onclick={forceAndOpenGame}
								disabled={forceLoading || openGameLoading || !selectedSession}
								class="px-4 py-2 rounded bg-coral-500 text-white font-semibold text-sm hover:bg-coral-400 transition-colors disabled:opacity-50 disabled:cursor-not-allowed flex items-center gap-2"
								style="background-color: #f97066;"
							>
								{#if openGameLoading}
									<svg class="w-4 h-4 animate-spin" fill="none" viewBox="0 0 24 24">
										<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
										<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
									</svg>
									{$_('common.opening')}
								{:else}
									<svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
										<path stroke-linecap="round" stroke-linejoin="round" d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14" />
									</svg>
									{$_('events.forceAndOpenGame')}
								{/if}
							</button>
						</div>
						{#if forceMessage}
							<div class="mt-3 text-sm text-emerald-400 flex items-center gap-2">
								<svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
									<path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7" />
								</svg>
								{forceMessage}
							</div>
						{/if}
						{#if forceError}
							<div class="mt-3 text-sm text-red-400">{forceError}</div>
						{/if}
						{#if openGameError}
							<div class="mt-3 text-sm text-red-400">{openGameError}</div>
						{/if}
					{:else}
						<div class="text-sm text-gray-400">
							{$_('events.noLgsSessions')}
						</div>
					{/if}
				</div>

				<!-- Event JSON -->
				{#if eventInfo.event}
					<div>
						<div class="mb-2 flex items-center gap-2">
							<h3 class="text-sm font-medium text-gray-300">{$_('events.eventData')}</h3>
							{#if eventInfo.lazy_load}
								<span class="px-2 py-0.5 text-xs rounded bg-emerald-500/20 text-emerald-400 border border-emerald-500/30">
									{$_('events.lazyLoaded')}
								</span>
							{:else if eventInfo.events_loaded}
								<span class="px-2 py-0.5 text-xs rounded bg-blue-500/20 text-blue-400 border border-blue-500/30">
									{$_('events.fromCache')}
								</span>
							{/if}
						</div>
						<pre
							class="max-h-96 overflow-auto rounded-lg bg-gray-900 p-4 text-sm text-gray-300">{JSON.stringify(eventInfo.event, null, 2)}</pre>
					</div>
				{:else if eventInfo.no_events_file}
					<div class="rounded-lg bg-gray-700/30 p-6 text-center">
						<p class="text-gray-400">{$_('events.noEventsFile')}</p>
						<p class="mt-2 text-sm text-gray-500">
							{$_('events.noEventsFileDesc')}
						</p>
					</div>
				{:else if eventInfo.event_missing}
					<div class="rounded-lg bg-yellow-900/30 p-6 text-center">
						<p class="text-yellow-400">{$_('events.notAvailable')}</p>
						<p class="mt-2 text-sm text-gray-400">
							{$_('events.notFoundDesc')}
						</p>
					</div>
				{:else if eventInfo.error}
					<div class="rounded-lg bg-red-900/30 p-6 text-center">
						<p class="text-red-400">{$_('errors.loadFailed')}</p>
						<p class="mt-2 text-sm text-gray-400">{eventInfo.error}</p>
					</div>
				{:else}
					<div class="rounded-lg bg-gray-700/30 p-6 text-center">
						<p class="text-gray-400">{$_('events.notAvailable')}</p>
					</div>
				{/if}
			{/if}
		</div>
	</div>
</div>
