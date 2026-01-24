<script lang="ts">
	import { api, type LGSSessionSummary, type LGSAggregateStats, type LGSSessionsResponse, type ModeSummary, type WSMessage, type LoaderModeStatus, type WSLoadingProgress } from '$lib/api';
	import { CURRENCIES, LANGUAGES, loadGameSettings, saveGameSettings, getCurrencyDisplay, openGame as openGameHelper } from '$lib/openGame';
	import BatchSimModal from './BatchSimModal.svelte';
	import { _ } from '$lib/i18n';

	const LGS_HTTPS_URL = 'https://localhost:7755';

	interface Props {
		modes: ModeSummary[];
	}

	let { modes }: Props = $props();

	// Certificate status
	let certStatus = $state<'checking' | 'ok' | 'error'>('checking');
	let certBannerDismissed = $state(false);

	// Check if HTTPS server is accessible
	async function checkCertificate(): Promise<boolean> {
		try {
			const controller = new AbortController();
			const timeout = setTimeout(() => controller.abort(), 5000);
			await fetch(`${LGS_HTTPS_URL}/lgs/health`, {
				method: 'GET',
				signal: controller.signal
			});
			clearTimeout(timeout);
			return true;
		} catch {
			return false;
		}
	}

	function openLgsUrl() {
		window.open(LGS_HTTPS_URL, '_blank');
	}

	async function recheckCertificate() {
		certStatus = 'checking';
		const ok = await checkCertificate();
		certStatus = ok ? 'ok' : 'error';
	}

	let sessions = $state<LGSSessionSummary[]>([]);
	let aggregate = $state<LGSAggregateStats | null>(null);
	let loading = $state(false);
	let error = $state<string | null>(null);
	let wsConnected = $state(false);
	let ws: WebSocket | null = null;

	// Force outcome state
	let selectedSession = $state<string>('');
	let selectedMode = $state<string>('');
	let simIdInput = $state<string>('');
	let forceLoading = $state(false);
	let forceMessage = $state<string | null>(null);
	let forceError = $state<string | null>(null);
	let forcedOutcomes = $state<Record<string, number>>({});

	// RTP Bias state
	let biasInput = $state<number>(0);
	let biasLoading = $state(false);
	let biasMessage = $state<string | null>(null);
	let biasError = $state<string | null>(null);

	// Create session state
	let newSessionID = $state<string>('');
	let createSessionLoading = $state(false);

	// Batch simulation modal
	let showBatchModal = $state(false);
	let sessionToSimulate = $state<string>('');
	let sessionToSimulateBalance = $derived(() => {
		const session = sessions.find(s => s.sessionID === sessionToSimulate);
		return session?.balance ?? 0;
	});

	// Open Game panel state
	const initialSettings = loadGameSettings();
	let gameDomain = $state(initialSettings.domain);
	let gameSession = $state(initialSettings.session);
	let gameCurrency = $state(initialSettings.currency);
	let gameBalance = $state(initialSettings.balance);
	let gameLanguage = $state(initialSettings.language);
	let gameDevice = $state<'desktop' | 'mobile'>(initialSettings.device);
	let gameDemo = $state(initialSettings.demo);
	let gameSocial = $state(initialSettings.social);
	let gameUUID = $state(initialSettings.gameUUID);
	let gameVersion = $state(initialSettings.gameVersion);
	let openGameLoading = $state(false);
	let openGameError = $state<string | null>(null);

	// Save settings when they change
	$effect(() => {
		saveGameSettings({
			domain: gameDomain,
			session: gameSession,
			currency: gameCurrency,
			balance: gameBalance,
			language: gameLanguage,
			device: gameDevice,
			demo: gameDemo,
			social: gameSocial,
			gameUUID: gameUUID,
			gameVersion: gameVersion,
		});
	});

	async function handleOpenGame() {
		openGameLoading = true;
		openGameError = null;

		try {
			await openGameHelper({
				sessionID: gameSession,
				balance: gameBalance,
				currency: gameCurrency,
				language: gameLanguage,
				device: gameDevice,
				demo: gameDemo,
				social: gameSocial,
				domain: gameDomain,
			});
		} catch (e) {
			openGameError = e instanceof Error ? e.message : 'Failed to open game';
		} finally {
			openGameLoading = false;
		}
	}

	function openSimulateModal(sessionID: string) {
		sessionToSimulate = sessionID;
		showBatchModal = true;
	}

	// Loader status
	let loaderModes = $state<Record<string, LoaderModeStatus>>({});
	let loaderPriority = $state<'low' | 'high'>('low');
	let turboLoading = $state(false);

	// Note: Events are now lazy-loaded on demand, no need to wait for full loading

	// Connect WebSocket for real-time updates
	function connectWebSocket() {
		const wsUrl = api.getWebSocketUrl();
		ws = new WebSocket(wsUrl);

		ws.onopen = () => {
			wsConnected = true;
		};

		ws.onclose = () => {
			wsConnected = false;
			// Reconnect after delay
			setTimeout(connectWebSocket, 3000);
		};

		ws.onerror = (err) => {
			console.error('[LGS] WebSocket error:', err);
		};

		ws.onmessage = (event) => {
			try {
				const msg: WSMessage = JSON.parse(event.data);
				handleMessage(msg);
			} catch (e) {
				console.error('[LGS] Failed to parse WebSocket message:', e);
			}
		};
	}

	// Handle WebSocket messages
	function handleMessage(msg: WSMessage) {
		if (msg.type === 'lgs_sessions_update') {
			const data = msg.payload as LGSSessionsResponse;
			sessions = data.sessions;
			aggregate = data.aggregate;
			// Auto-select first session if none selected
			if (sessions.length > 0 && !selectedSession) {
				selectedSession = sessions[0].sessionID;
			}
			// Update forcedOutcomes from selected session
			if (selectedSession) {
				const session = sessions.find(s => s.sessionID === selectedSession);
				if (session) {
					forcedOutcomes = session.forcedOutcomes || {};
				}
			}
		} else if (msg.type === 'loading_progress') {
			const progress = msg.payload as WSLoadingProgress;
			if (progress.mode && loaderModes[progress.mode]) {
				loaderModes[progress.mode] = {
					...loaderModes[progress.mode],
					current_line: progress.current_line,
					bytes_read: progress.bytes_read,
					percent_bytes: progress.percent_bytes,
					status: 'loading'
				};
				loaderModes = { ...loaderModes };
			}
		} else if (msg.type === 'loading_complete') {
			const data = msg.payload as { mode: string; total_lines: number };
			if (data.mode && loaderModes[data.mode]) {
				loaderModes[data.mode] = {
					...loaderModes[data.mode],
					status: 'complete',
					percent_bytes: 100,
					total_lines: data.total_lines,
					current_line: data.total_lines
				};
				loaderModes = { ...loaderModes };
			}
		} else if (msg.type === 'loading_started') {
			const data = msg.payload as { mode: string; events_file: string; total_bytes: number };
			if (data.mode) {
				loaderModes[data.mode] = {
					...loaderModes[data.mode],
					mode: data.mode,
					events_file: data.events_file,
					status: 'loading',
					total_bytes: data.total_bytes,
					bytes_read: 0,
					percent_bytes: 0,
					current_line: 0
				};
				loaderModes = { ...loaderModes };
			}
		} else if (msg.type === 'priority_changed') {
			const data = msg.payload as { new_priority: string };
			loaderPriority = data.new_priority as 'low' | 'high';
		} else if (msg.type === 'reload_started') {
			// Reset loader modes to trigger loading overlay
			loaderModes = {};
			// Reload loader status to get fresh mode list
			loadLoaderStatus();
		}
	}

	async function loadLoaderStatus() {
		try {
			const status = await api.loaderStatus();
			loaderModes = status.modes;
			loaderPriority = status.priority;
		} catch {
			// Ignore errors
		}
	}

	async function toggleTurbo() {
		turboLoading = true;
		try {
			if (loaderPriority === 'high') {
				await api.loaderUnboost();
				loaderPriority = 'low';
			} else {
				await api.loaderBoost();
				loaderPriority = 'high';
			}
		} catch (e) {
			console.error('Failed to toggle turbo:', e);
		} finally {
			turboLoading = false;
		}
	}

	async function loadSessions() {
		loading = true;
		error = null;
		try {
			const response = await api.lgsSessions();
			sessions = response.sessions;
			aggregate = response.aggregate;
			if (sessions.length > 0 && !selectedSession) {
				selectedSession = sessions[0].sessionID;
			}
			// Update forcedOutcomes from selected session
			if (selectedSession) {
				const session = sessions.find(s => s.sessionID === selectedSession);
				if (session) {
					forcedOutcomes = session.forcedOutcomes || {};
				}
			}
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load sessions';
		} finally {
			loading = false;
		}
	}

	async function loadForcedOutcomes() {
		if (!selectedSession) return;
		try {
			const response = await api.lgsGetForcedOutcomes(selectedSession);
			forcedOutcomes = response.forcedOutcomes;
		} catch {
			forcedOutcomes = {};
		}
	}

	async function forceOutcome() {
		if (!selectedSession || !selectedMode || !simIdInput) return;

		const simID = parseInt(simIdInput, 10);
		if (isNaN(simID)) {
			forceError = 'Invalid SimID';
			return;
		}

		forceLoading = true;
		forceError = null;
		forceMessage = null;

		try {
			const response = await api.lgsForceOutcome(selectedSession, selectedMode, simID);
			forceMessage = `Set: ${response.mode} -> SimID ${response.simID} (${response.payout.toFixed(2)}x)`;
			simIdInput = '';
			await loadForcedOutcomes();
		} catch (e) {
			forceError = e instanceof Error ? e.message : 'Failed to set forced outcome';
		} finally {
			forceLoading = false;
		}
	}

	async function clearForcedOutcome(mode: string) {
		if (!selectedSession) return;
		try {
			await api.lgsClearForcedOutcome(selectedSession, mode);
			await loadForcedOutcomes();
		} catch (e) {
			forceError = e instanceof Error ? e.message : 'Failed to clear';
		}
	}

	async function setRTPBias() {
		if (!selectedSession) return;

		biasLoading = true;
		biasError = null;
		biasMessage = null;

		try {
			const response = await api.lgsSetRTPBias(selectedSession, biasInput);
			biasMessage = `RTP Bias set to ${response.bias.toFixed(2)}`;
		} catch (e) {
			biasError = e instanceof Error ? e.message : 'Failed to set RTP bias';
		} finally {
			biasLoading = false;
		}
	}

	function getBiasColor(bias: number | undefined): string {
		if (!bias || bias === 0) return 'text-[var(--color-mist)]';
		if (bias > 0) return 'text-emerald-400';
		return 'text-red-400';
	}

	function formatBias(bias: number | undefined): string {
		if (!bias || bias === 0) return 'OFF';
		return bias > 0 ? `+${bias.toFixed(2)}` : bias.toFixed(2);
	}

	async function resetBalance(sessionID: string) {
		try {
			await api.lgsResetBalance(sessionID);
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to reset balance';
		}
	}

	async function clearStats(sessionID: string) {
		try {
			await api.lgsClearStats(sessionID);
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to clear stats';
		}
	}

	async function createSession() {
		const sessionID = newSessionID.trim() || `session-${Date.now()}`;
		createSessionLoading = true;
		try {
			await api.lgsAuthenticate(sessionID);
			newSessionID = '';
			selectedSession = sessionID;
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to create session';
		} finally {
			createSessionLoading = false;
		}
	}

	function formatBalance(amount: number): string {
		// API uses 1,000,000 = $1
		return (amount / 1000000).toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 });
	}

	function formatRTP(rtp: number): string {
		return (rtp * 100).toFixed(2) + '%';
	}

	function formatHitRate(hr: number): string {
		return (hr * 100).toFixed(1) + '%';
	}

	function getRTPColor(rtp: number): string {
		if (rtp >= 0.97) return 'text-emerald-400';
		if (rtp >= 0.94) return 'text-yellow-400';
		return 'text-red-400';
	}

	function getStatusColor(status: string): string {
		if (status === 'complete') return 'text-emerald-400';
		if (status === 'loading') return 'text-[var(--color-cyan)]';
		if (status === 'error') return 'text-red-400';
		return 'text-[var(--color-mist)]';
	}

	// Initialize
	$effect(() => {
		loadSessions();
		loadLoaderStatus();
		connectWebSocket();

		// Check certificate status
		(async () => {
			const ok = await checkCertificate();
			certStatus = ok ? 'ok' : 'error';
		})();

		return () => {
			if (ws) {
				ws.close();
			}
		};
	});

	$effect(() => {
		if (selectedSession) {
			// Update forcedOutcomes and biasInput from local session data
			const session = sessions.find(s => s.sessionID === selectedSession);
			if (session) {
				forcedOutcomes = session.forcedOutcomes || {};
				biasInput = session.rtpBias || 0;
			}
		}
	});
</script>

<div class="relative">
	<!-- Certificate Warning Banner -->
	{#if certStatus === 'error' && !certBannerDismissed}
		<div class="mb-4 p-4 rounded-xl bg-amber-500/10 border border-amber-500/30 flex items-start gap-4">
			<div class="w-10 h-10 rounded-lg bg-amber-500/20 flex items-center justify-center shrink-0">
				<svg class="w-5 h-5 text-amber-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
					<path stroke-linecap="round" stroke-linejoin="round" d="M12 9v3.75m-9.303 3.376c-.866 1.5.217 3.374 1.948 3.374h14.71c1.73 0 2.813-1.874 1.948-3.374L13.949 3.378c-.866-1.5-3.032-1.5-3.898 0L2.697 16.126zM12 15.75h.007v.008H12v-.008z" />
				</svg>
			</div>
			<div class="flex-1 min-w-0">
				<h4 class="font-display text-sm text-amber-400 tracking-wider mb-1">{$_('lgs.certWarningTitle')}</h4>
				<p class="text-sm text-[var(--color-light)]/70 mb-3">
					{$_('lgs.certWarningDesc')}
				</p>
				<div class="flex flex-wrap items-center gap-3">
					<button
						onclick={openLgsUrl}
						class="px-4 py-2 rounded-lg bg-amber-500 text-[var(--color-void)] font-mono font-semibold text-sm hover:bg-amber-400 transition-colors flex items-center gap-2"
					>
						<svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
							<path stroke-linecap="round" stroke-linejoin="round" d="M13.5 6H5.25A2.25 2.25 0 003 8.25v10.5A2.25 2.25 0 005.25 21h10.5A2.25 2.25 0 0018 18.75V10.5m-10.5 6L21 3m0 0h-5.25M21 3v5.25" />
						</svg>
						{$_('lgs.open')} {LGS_HTTPS_URL}
					</button>
					<button
						onclick={recheckCertificate}
						class="px-4 py-2 rounded-lg bg-[var(--color-slate)] text-[var(--color-light)] font-mono text-sm hover:bg-[var(--color-graphite)] transition-colors"
					>
						{$_('buttons.recheck')}
					</button>
					<button
						onclick={() => certBannerDismissed = true}
						class="px-4 py-2 rounded-lg text-[var(--color-mist)] font-mono text-sm hover:text-[var(--color-light)] transition-colors"
					>
						{$_('buttons.dismiss')}
					</button>
				</div>
			</div>
		</div>
	{/if}

	<div class="space-y-6">
		<!-- Aggregate Stats -->
		{#if aggregate}
			<div class="glass-panel rounded-2xl p-6">
				<div class="flex items-center gap-3 mb-6">
					<div class="w-1 h-5 bg-[var(--color-cyan)] rounded-full"></div>
					<h3 class="font-display text-lg text-[var(--color-light)] tracking-wider">{$_('lgs.aggregateStats')}</h3>
				</div>

				<div class="grid grid-cols-2 md:grid-cols-4 gap-4">
					<div class="bg-[var(--color-graphite)]/50 rounded-xl p-4">
						<div class="text-sm font-mono text-[var(--color-light)] mb-1">{$_('lgs.overallRtp')}</div>
						<div class="text-2xl font-mono font-bold {getRTPColor(aggregate.overallRTP)}">
							{formatRTP(aggregate.overallRTP)}
						</div>
					</div>
					<div class="bg-[var(--color-graphite)]/50 rounded-xl p-4">
						<div class="text-sm font-mono text-[var(--color-light)] mb-1">{$_('metrics.hitRate')}</div>
						<div class="text-2xl font-mono font-bold text-[var(--color-light)]">
							{formatHitRate(aggregate.overallHitRate)}
						</div>
					</div>
					<div class="bg-[var(--color-graphite)]/50 rounded-xl p-4">
						<div class="text-sm font-mono text-[var(--color-light)] mb-1">{$_('lgs.totalBets')}</div>
						<div class="text-2xl font-mono font-bold text-[var(--color-light)]">
							{aggregate.totalBets.toLocaleString()}
						</div>
					</div>
					<div class="bg-[var(--color-graphite)]/50 rounded-xl p-4">
						<div class="text-sm font-mono text-[var(--color-light)] mb-1">{$_('lgs.playerPL')}</div>
						<div class="text-2xl font-mono font-bold {-aggregate.totalProfit >= 0 ? 'text-emerald-400' : 'text-red-400'}">
							{-aggregate.totalProfit >= 0 ? '+' : ''}{formatBalance(-aggregate.totalProfit)}
						</div>
					</div>
				</div>
			</div>
		{/if}

		<!-- Force Outcome Panel -->
		<div class="glass-panel rounded-2xl p-6">
			<div class="flex items-center gap-3 mb-6">
				<div class="w-1 h-5 bg-[var(--color-gold)] rounded-full"></div>
				<h3 class="font-display text-lg text-[var(--color-light)] tracking-wider">{$_('lgs.forceNextOutcome')}</h3>
			</div>

			<div class="grid grid-cols-1 md:grid-cols-4 gap-4 mb-4">
				<div>
					<label class="block text-sm font-mono text-[var(--color-light)] mb-2">{$_('lgs.session')}</label>
					<select
						bind:value={selectedSession}
						class="w-full bg-[var(--color-graphite)] border border-white/10 rounded-lg px-3 py-2.5 text-sm font-mono text-[var(--color-light)] focus:outline-none focus:border-[var(--color-cyan)]"
					>
						{#each sessions as session}
							<option value={session.sessionID}>{session.sessionID}</option>
						{/each}
					</select>
				</div>

				<div>
					<label class="block text-sm font-mono text-[var(--color-light)] mb-2">{$_('lgs.mode')}</label>
					<select
						bind:value={selectedMode}
						class="w-full bg-[var(--color-graphite)] border border-white/10 rounded-lg px-3 py-2.5 text-sm font-mono text-[var(--color-light)] focus:outline-none focus:border-[var(--color-cyan)]"
					>
						<option value="">{$_('common.selectMode')}</option>
						{#each modes as mode}
							<option value={mode.mode}>{mode.mode}</option>
						{/each}
					</select>
				</div>

				<div>
					<label class="block text-sm font-mono text-[var(--color-light)] mb-2">{$_('lgs.simId')}</label>
					<input
						type="number"
						bind:value={simIdInput}
						placeholder={$_('lgs.enterSimId')}
						class="w-full bg-[var(--color-graphite)] border border-white/10 rounded-lg px-3 py-2.5 text-sm font-mono text-[var(--color-light)] focus:outline-none focus:border-[var(--color-cyan)]"
					/>
				</div>

				<div class="flex items-end">
					<button
						onclick={forceOutcome}
						disabled={forceLoading || !selectedSession || !selectedMode || !simIdInput}
						class="w-full px-4 py-2 rounded-lg bg-[var(--color-gold)] text-[var(--color-void)] font-mono font-semibold text-sm hover:bg-[var(--color-gold)]/90 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
					>
						{forceLoading ? $_('common.setting') : $_('buttons.set')}
					</button>
				</div>
			</div>

			{#if forceMessage}
				<div class="text-sm font-mono text-emerald-400 mb-2">{forceMessage}</div>
			{/if}
			{#if forceError}
				<div class="text-sm font-mono text-red-400 mb-2">{forceError}</div>
			{/if}

			<!-- Active Forced Outcomes -->
			{#if Object.keys(forcedOutcomes).length > 0}
				<div class="mt-4 pt-4 border-t border-white/10">
					<div class="text-sm font-mono text-[var(--color-light)] mb-2">{$_('lgs.pendingForcedOutcomes')}</div>
					<div class="flex flex-wrap gap-2">
						{#each Object.entries(forcedOutcomes) as [mode, simID]}
							<div class="inline-flex items-center gap-2 px-3 py-1.5 rounded-lg bg-[var(--color-gold)]/20 text-[var(--color-gold)] text-sm font-mono">
								<span>{mode}: #{simID}</span>
								<button
									onclick={() => clearForcedOutcome(mode)}
									class="hover:text-red-400 transition-colors"
								>
									<svg class="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
										<path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" />
									</svg>
								</button>
							</div>
						{/each}
					</div>
				</div>
			{/if}
		</div>

		<!-- RTP Bias Panel -->
		<div class="glass-panel rounded-2xl p-6">
			<div class="flex items-center gap-3 mb-6">
				<div class="w-1 h-5 bg-emerald-500 rounded-full"></div>
				<h3 class="font-display text-lg text-[var(--color-light)] tracking-wider">{$_('lgs.rtpBias')}</h3>
				<span class="text-sm font-mono text-[var(--color-mist)]">{$_('lgs.rtpBiasHint')}</span>
			</div>

			<div class="grid grid-cols-1 md:grid-cols-3 gap-4 mb-4">
				<div>
					<label class="block text-sm font-mono text-[var(--color-light)] mb-2">{$_('lgs.session')}</label>
					<select
						bind:value={selectedSession}
						class="w-full bg-[var(--color-graphite)] border border-white/10 rounded-lg px-3 py-2.5 text-sm font-mono text-[var(--color-light)] focus:outline-none focus:border-emerald-500"
					>
						{#each sessions as session}
							<option value={session.sessionID}>{session.sessionID}</option>
						{/each}
					</select>
				</div>

				<div>
					<label class="block text-sm font-mono text-[var(--color-light)] mb-2">{$_('lgs.biasValue')}</label>
					<div class="flex items-center gap-3">
						<div class="flex-1">
							<input
								type="range"
								bind:value={biasInput}
								min="-2"
								max="2"
								step="0.25"
								class="w-full h-2 rounded-full appearance-none cursor-pointer
									[&::-webkit-slider-thumb]:appearance-none [&::-webkit-slider-thumb]:w-4 [&::-webkit-slider-thumb]:h-4 [&::-webkit-slider-thumb]:rounded-full [&::-webkit-slider-thumb]:bg-emerald-500 [&::-webkit-slider-thumb]:cursor-pointer
									[&::-moz-range-thumb]:w-4 [&::-moz-range-thumb]:h-4 [&::-moz-range-thumb]:rounded-full [&::-moz-range-thumb]:bg-emerald-500 [&::-moz-range-thumb]:cursor-pointer [&::-moz-range-thumb]:border-0
									{biasInput > 0 ? 'bg-gradient-to-r from-[var(--color-slate)] to-emerald-500/50' : biasInput < 0 ? 'bg-gradient-to-r from-red-500/50 to-[var(--color-slate)]' : 'bg-[var(--color-slate)]'}"
							/>
							<div class="flex justify-between text-sm font-mono text-[var(--color-mist)] mt-1">
								<span>-2</span>
								<span>0</span>
								<span>+2</span>
							</div>
						</div>
						<input
							type="number"
							bind:value={biasInput}
							min="-2"
							max="2"
							step="0.25"
							class="w-24 shrink-0 bg-[var(--color-graphite)] border border-white/10 rounded-lg px-3 py-2 text-sm font-mono text-center {getBiasColor(biasInput)} focus:outline-none focus:border-emerald-500"
						/>
					</div>
				</div>

				<div class="flex items-end">
					<button
						onclick={setRTPBias}
						disabled={biasLoading || !selectedSession}
						class="w-full px-4 py-2 rounded-lg bg-emerald-600 text-white font-mono font-semibold text-sm hover:bg-emerald-500 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
					>
						{biasLoading ? $_('common.setting') : $_('buttons.applyBias')}
					</button>
				</div>
			</div>

			{#if biasMessage}
				<div class="text-sm font-mono text-emerald-400 mb-2">{biasMessage}</div>
			{/if}
			{#if biasError}
				<div class="text-sm font-mono text-red-400 mb-2">{biasError}</div>
			{/if}

			<div class="mt-4 pt-4 border-t border-white/10 text-sm font-mono text-[var(--color-mist)]">
				{$_('lgs.biasFormula')}<br/>
				{$_('lgs.biasExample')}
			</div>
		</div>

		<!-- Open Game Panel -->
		<div class="glass-panel rounded-2xl p-6">
			<div class="flex items-center gap-3 mb-6">
				<div class="w-1 h-5 bg-[var(--color-coral)] rounded-full"></div>
				<h3 class="font-display text-lg text-[var(--color-light)] tracking-wider">{$_('lgs.openGame')}</h3>
				<span class="text-sm font-mono text-[var(--color-mist)]">{$_('lgs.openGameHint')}</span>
			</div>

			<div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4 mb-4">
				<!-- Game Server -->
				<div>
					<label class="block text-sm font-mono text-[var(--color-light)] mb-2">{$_('lgs.gameServer')}</label>
					<input
						type="text"
						bind:value={gameDomain}
						placeholder="localhost:4234"
						class="w-full bg-[var(--color-graphite)] border border-white/10 rounded-lg px-3 py-2.5 text-sm font-mono text-[var(--color-light)] focus:outline-none focus:border-[var(--color-coral)] placeholder:text-[var(--color-mist)]/50"
					/>
				</div>

				<!-- Session -->
				<div>
					<label class="block text-sm font-mono text-[var(--color-light)] mb-2">{$_('lgs.sessionIdLabel')}</label>
					<div class="flex gap-2">
						<input
							type="text"
							bind:value={gameSession}
							placeholder="auto-generate"
							class="flex-1 bg-[var(--color-graphite)] border border-white/10 rounded-lg px-3 py-2.5 text-sm font-mono text-[var(--color-light)] focus:outline-none focus:border-[var(--color-coral)] placeholder:text-[var(--color-mist)]/50"
						/>
						{#if sessions.length > 0}
							<select
								onchange={(e) => {
									const target = e.target as HTMLSelectElement;
									if (target.value) gameSession = target.value;
								}}
								class="bg-[var(--color-graphite)] border border-white/10 rounded-lg px-2 py-2.5 text-sm font-mono text-[var(--color-light)] focus:outline-none focus:border-[var(--color-coral)]"
							>
								<option value="">{$_('common.pick')}</option>
								{#each sessions as session}
									<option value={session.sessionID}>{session.sessionID}</option>
								{/each}
							</select>
						{/if}
					</div>
				</div>

				<!-- Balance -->
				<div>
					<label class="block text-sm font-mono text-[var(--color-light)] mb-2">{$_('lgs.balanceLabel')} ({getCurrencyDisplay(gameCurrency)})</label>
					<input
						type="number"
						bind:value={gameBalance}
						min="0"
						step="100"
						placeholder="1000"
						class="w-full bg-[var(--color-graphite)] border border-white/10 rounded-lg px-3 py-2.5 text-sm font-mono text-[var(--color-light)] focus:outline-none focus:border-[var(--color-coral)]"
					/>
				</div>

				<!-- Currency -->
				<div>
					<label class="block text-sm font-mono text-[var(--color-light)] mb-2">{$_('lgs.currency')}</label>
					<select
						bind:value={gameCurrency}
						class="w-full bg-[var(--color-graphite)] border border-white/10 rounded-lg px-3 py-2.5 text-sm font-mono text-[var(--color-light)] focus:outline-none focus:border-[var(--color-coral)]"
					>
						{#each CURRENCIES as currency}
							<option value={currency.code}>{currency.display} {currency.code} - {currency.name}</option>
						{/each}
					</select>
				</div>

				<!-- Language -->
				<div>
					<label class="block text-sm font-mono text-[var(--color-light)] mb-2">{$_('lgs.language')}</label>
					<select
						bind:value={gameLanguage}
						class="w-full bg-[var(--color-graphite)] border border-white/10 rounded-lg px-3 py-2.5 text-sm font-mono text-[var(--color-light)] focus:outline-none focus:border-[var(--color-coral)]"
					>
						{#each LANGUAGES as lang}
							<option value={lang.code}>{lang.code.toUpperCase()} - {lang.name}</option>
						{/each}
					</select>
				</div>

				<!-- Device -->
				<div>
					<label class="block text-sm font-mono text-[var(--color-light)] mb-2">{$_('lgs.device')}</label>
					<select
						bind:value={gameDevice}
						class="w-full bg-[var(--color-graphite)] border border-white/10 rounded-lg px-3 py-2.5 text-sm font-mono text-[var(--color-light)] focus:outline-none focus:border-[var(--color-coral)]"
					>
						<option value="desktop">{$_('lgs.desktop')}</option>
						<option value="mobile">{$_('lgs.mobile')}</option>
					</select>
				</div>

				<!-- Toggles -->
				<div class="flex items-end gap-4">
					<label class="flex items-center gap-2 cursor-pointer">
						<input
							type="checkbox"
							bind:checked={gameDemo}
							class="w-4 h-4 rounded border-white/20 bg-[var(--color-graphite)] text-[var(--color-coral)] focus:ring-[var(--color-coral)] focus:ring-offset-0"
						/>
						<span class="text-sm font-mono text-[var(--color-light)]">{$_('lgs.demo')}</span>
					</label>
					<label class="flex items-center gap-2 cursor-pointer">
						<input
							type="checkbox"
							bind:checked={gameSocial}
							class="w-4 h-4 rounded border-white/20 bg-[var(--color-graphite)] text-[var(--color-coral)] focus:ring-[var(--color-coral)] focus:ring-offset-0"
						/>
						<span class="text-sm font-mono text-[var(--color-light)]">{$_('lgs.social')}</span>
					</label>
				</div>
			</div>

			{#if openGameError}
				<div class="text-sm font-mono text-red-400 mb-4">{openGameError}</div>
			{/if}

			<div class="flex items-center gap-4">
				<button
					onclick={handleOpenGame}
					disabled={!gameDomain.trim() || openGameLoading}
					class="px-6 py-2.5 rounded-lg bg-[var(--color-coral)] text-white font-mono font-semibold text-sm hover:bg-[var(--color-coral)]/90 transition-colors disabled:opacity-50 disabled:cursor-not-allowed flex items-center justify-center gap-2"
				>
					{#if openGameLoading}
						<svg class="w-4 h-4 animate-spin" fill="none" viewBox="0 0 24 24">
							<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
							<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
						</svg>
						{$_('buttons.opening')}
					{:else}
						<svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
							<path stroke-linecap="round" stroke-linejoin="round" d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14" />
						</svg>
						{$_('buttons.openGame')}
					{/if}
				</button>
				<span class="text-sm font-mono text-[var(--color-mist)]">
					{$_('lgs.settingsSaved')}
				</span>
			</div>
		</div>

		<!-- Sessions List -->
		<div class="glass-panel rounded-2xl p-6">
			<div class="flex items-center gap-3 mb-6">
				<div class="w-1 h-5 bg-[var(--color-violet)] rounded-full"></div>
				<h3 class="font-display text-lg text-[var(--color-light)] tracking-wider">{$_('lgs.sessions')}</h3>
				<span class="text-sm font-mono text-[var(--color-mist)]">({sessions.length})</span>
				<div class="ml-auto flex items-center gap-3">
					<!-- Create Session -->
					<div class="flex items-center gap-2">
						<input
							type="text"
							bind:value={newSessionID}
							placeholder={$_('lgs.sessionId')}
							class="w-36 bg-[var(--color-graphite)] border border-white/10 rounded-lg px-3 py-1.5 text-sm font-mono text-[var(--color-light)] focus:outline-none focus:border-[var(--color-violet)] placeholder:text-[var(--color-mist)]/50"
						/>
						<button
							onclick={createSession}
							disabled={createSessionLoading}
							class="px-4 py-1.5 rounded-lg bg-[var(--color-violet)] text-white text-sm font-mono font-semibold hover:bg-[var(--color-violet)]/80 transition-colors disabled:opacity-50"
						>
							{createSessionLoading ? '...' : $_('buttons.create')}
						</button>
					</div>
					<div class="w-px h-4 bg-white/10"></div>
					{#if wsConnected}
						<span class="flex items-center gap-1.5 text-sm font-mono text-emerald-400">
							<span class="w-2 h-2 bg-emerald-400 rounded-full animate-pulse"></span>
							{$_('status.live')}
						</span>
					{:else}
						<span class="flex items-center gap-1.5 text-sm font-mono text-[var(--color-mist)]">
							<span class="w-2 h-2 bg-[var(--color-mist)] rounded-full"></span>
							{$_('status.offline')}
						</span>
					{/if}
					<button
						onclick={loadSessions}
						class="text-sm font-mono text-[var(--color-cyan)] hover:text-[var(--color-cyan)]/80 transition-colors"
					>
						{$_('buttons.refresh')}
					</button>
				</div>
			</div>

			{#if loading && sessions.length === 0}
				<div class="py-8 text-center text-[var(--color-mist)]">{$_('status.loadingSessions')}</div>
			{:else if error}
				<div class="py-8 text-center text-red-400">{error}</div>
			{:else if sessions.length === 0}
				<div class="py-8 text-center text-[var(--color-mist)]">{$_('status.noActiveSessions')}</div>
			{:else}
				<div class="overflow-x-auto">
					<table class="w-full">
						<thead>
							<tr class="text-left text-sm uppercase text-[var(--color-light)] tracking-wider">
								<th class="pb-3 font-medium">{$_('table.session')}</th>
								<th class="pb-3 text-right font-medium">{$_('table.balance')}</th>
								<th class="pb-3 text-right font-medium">{$_('table.bets')}</th>
								<th class="pb-3 text-right font-medium">{$_('metrics.rtp')}</th>
								<th class="pb-3 text-right font-medium">{$_('table.hitRate')}</th>
								<th class="pb-3 text-right font-medium">{$_('table.bias')}</th>
								<th class="pb-3 text-right font-medium">{$_('table.pl')}</th>
								<th class="pb-3 text-right font-medium">{$_('table.actions')}</th>
							</tr>
						</thead>
						<tbody class="text-sm">
							{#each sessions as session}
								<tr class="border-t border-white/5 hover:bg-white/5 transition-colors">
									<td class="py-3 font-mono text-[var(--color-light)]">
										{session.sessionID}
									</td>
									<td class="py-3 text-right font-mono text-[var(--color-light)]">
										{formatBalance(session.balance)}
									</td>
									<td class="py-3 text-right font-mono text-[var(--color-mist)]">
										{session.totalBets.toLocaleString()}
									</td>
									<td class="py-3 text-right font-mono font-semibold {getRTPColor(session.rtp)}">
										{formatRTP(session.rtp)}
									</td>
									<td class="py-3 text-right font-mono text-[var(--color-mist)]">
										{formatHitRate(session.hitRate)}
									</td>

								<td class="py-3 text-right font-mono {getBiasColor(session.rtpBias)}">
									{formatBias(session.rtpBias)}
									</td>
									<td class="py-3 text-right font-mono {-session.profit >= 0 ? 'text-emerald-400' : 'text-red-400'}">
										{-session.profit >= 0 ? '+' : ''}{formatBalance(-session.profit)}
									</td>
									<td class="py-3 text-right">
										<div class="flex items-center justify-end gap-2">
											<button
												onclick={() => openSimulateModal(session.sessionID)}
												class="px-3 py-1.5 rounded text-sm font-mono font-semibold bg-[var(--color-cyan)]/20 text-[var(--color-cyan)] hover:bg-[var(--color-cyan)]/30 transition-colors"
												title={$_('buttons.simulate')}
											>
												{$_('buttons.simulate')}
											</button>
											<button
												onclick={() => resetBalance(session.sessionID)}
												class="px-3 py-1.5 rounded text-sm font-mono text-[var(--color-mist)] hover:bg-white/10 transition-colors"
												title={$_('lgs.resetBalance')}
											>
												{$_('buttons.reset')}
											</button>
											<button
												onclick={() => clearStats(session.sessionID)}
												class="px-3 py-1.5 rounded text-sm font-mono text-[var(--color-mist)] hover:bg-white/10 transition-colors"
												title={$_('lgs.clearStats')}
											>
												{$_('buttons.clear')}
											</button>
										</div>
									</td>
								</tr>
							{/each}
						</tbody>
					</table>
				</div>
			{/if}
		</div>
	</div>
</div>

<!-- Batch Simulation Modal -->
<BatchSimModal
	open={showBatchModal}
	{modes}
	sessionID={sessionToSimulate}
	balance={sessionToSimulateBalance()}
	onClose={() => showBatchModal = false}
/>
