<script lang="ts">
	import { onMount } from 'svelte';
	import { api, type IndexInfo, type Statistics, type CompareItem } from '$lib/api';
	import {
		PayoutBuckets,
		TopPayouts,
		DistributionTable,
		ModeComparison,
		EventModal,
		ComplianceChecklist,
		CrowdSimPanel,
		BooksLoadingPanel,
		LGSPanel,
		OptimizerPanel,
		WelcomeModal
	} from '$lib/components';

	// Certificate check state
	let welcomeModal: WelcomeModal;
	let certCheckInterval: ReturnType<typeof setInterval> | null = null;
	const CERT_CHECK_INTERVAL = 30000; // Check every 30 seconds

	let indexInfo = $state<IndexInfo | null>(null);
	let selectedMode = $state<string | null>(null);
	let stats = $state<Statistics | null>(null);
	let compareItems = $state<CompareItem[]>([]);

	let loading = $state(true);
	let statsLoading = $state(false);
	let error = $state<string | null>(null);
	let reloadLoading = $state(false);

	// Key to force re-mount of components on reload
	let reloadKey = $state(0);

	// Active panel for detailed view
	type PanelType = 'overview' | 'distribution' | 'compliance' | 'crowdsim' | 'optimizer' | 'lgs';
	const validPanels: PanelType[] = ['overview', 'distribution', 'compliance', 'crowdsim', 'optimizer', 'lgs'];
	let activePanel = $state<PanelType>('overview');

	// Hash-based navigation
	function getHashPanel(): PanelType {
		if (typeof window === 'undefined') return 'overview';
		const hash = window.location.hash.slice(1); // remove #
		return validPanels.includes(hash as PanelType) ? (hash as PanelType) : 'overview';
	}

	function setPanel(panel: PanelType) {
		activePanel = panel;
		if (typeof window !== 'undefined') {
			window.location.hash = panel;
		}
	}

	// Event modal state
	let showEventModal = $state(false);
	let selectedSimId = $state<number | null>(null);

	function openEventModal(simId: number) {
		selectedSimId = simId;
		showEventModal = true;
	}

	function closeEventModal() {
		showEventModal = false;
		selectedSimId = null;
	}

	onMount(() => {
		// Set initial panel from URL hash
		activePanel = getHashPanel();

		// Listen for hash changes (browser back/forward)
		const handleHashChange = () => {
			activePanel = getHashPanel();
		};
		window.addEventListener('hashchange', handleHashChange);

		// Load data
		(async () => {
			try {
				indexInfo = await api.getIndex();
				const comparison = await api.compare();
				compareItems = comparison.modes;

				if (indexInfo.modes.length > 0) {
					await selectMode(indexInfo.modes[0].mode);
				}
			} catch (e) {
				error = e instanceof Error ? e.message : 'Failed to connect to backend';
			} finally {
				loading = false;
			}
		})();

		// Start background certificate check
		certCheckInterval = setInterval(() => {
			if (welcomeModal) {
				welcomeModal.backgroundCheck();
			}
		}, CERT_CHECK_INTERVAL);

		return () => {
			window.removeEventListener('hashchange', handleHashChange);
			if (certCheckInterval) {
				clearInterval(certCheckInterval);
			}
		};
	});

	async function selectMode(mode: string) {
		selectedMode = mode;
		statsLoading = true;
		try {
			stats = await api.getModeStats(mode);
		} catch (e) {
			console.error('Failed to load stats:', e);
		} finally {
			statsLoading = false;
		}
	}

	async function reloadBooks() {
		reloadLoading = true;
		try {
			await api.reload();
			// Reload all data
			indexInfo = await api.getIndex();
			const comparison = await api.compare();
			compareItems = comparison.modes;

			// Reload stats for current mode
			if (selectedMode) {
				stats = await api.getModeStats(selectedMode);
			}

			// Increment reloadKey to force re-mount components that load their own data
			reloadKey++;
		} catch (e) {
			console.error('Failed to reload:', e);
		} finally {
			reloadLoading = false;
		}
	}

	// Derived data for dashboard
	let currentModeInfo = $derived(indexInfo?.modes.find((m) => m.mode === selectedMode));
	let currentCompareItem = $derived(compareItems.find((c) => c.mode === selectedMode));
	let volatilityInfo = $derived(stats ? getVolatilityInfo(stats.volatility) : null);

	function formatPercent(value: number): string {
		return (value * 100).toFixed(2) + '%';
	}

	function formatMultiplier(value: number): string {
		return value.toFixed(0) + 'x';
	}

	function getVolatilityInfo(vol: number): { label: string; color: string; textClass: string; glowClass: string } {
		if (vol < 3) return { label: 'LOW', color: 'emerald', textClass: 'text-[var(--color-emerald)]', glowClass: 'glow-emerald' };
		if (vol < 7) return { label: 'MEDIUM', color: 'gold', textClass: 'text-[var(--color-gold)]', glowClass: 'glow-gold' };
		if (vol < 15) return { label: 'HIGH', color: 'coral', textClass: 'text-[var(--color-coral)]', glowClass: 'glow-coral' };
		return { label: 'EXTREME', color: 'coral', textClass: 'text-[var(--color-coral)]', glowClass: 'glow-coral' };
	}

	// Tab configuration
	const tabs = [
		{ id: 'overview', label: 'OVERVIEW', icon: 'grid', badge: null },
		{ id: 'distribution', label: 'DISTRIBUTION', icon: 'chart', badge: null },
		{ id: 'compliance', label: 'COMPLIANCE', icon: 'shield', badge: null },
		{ id: 'crowdsim', label: 'CROWD SIM', icon: 'users', badge: null },
		{ id: 'optimizer', label: 'OPTIMIZER', icon: 'bolt', badge: 'beta' },
		{ id: 'lgs', label: 'LGS', icon: 'server', badge: null }
	] as const;
</script>

<svelte:head>
	<title>Mnemoo Tools</title>
</svelte:head>

<!-- Main Container -->
<div class="min-h-screen bg-[var(--color-void)] relative overflow-hidden">
	<!-- Background Effects -->
	<div class="fixed inset-0 pointer-events-none">
		<!-- Grid Pattern -->
		<div class="absolute inset-0 bg-grid opacity-50"></div>

		<!-- Ambient Glow Orbs -->
		<div class="absolute top-0 left-1/4 w-[600px] h-[600px] bg-[var(--color-cyan)] rounded-full blur-[200px] opacity-[0.03]"></div>
		<div class="absolute bottom-0 right-1/4 w-[500px] h-[500px] bg-[var(--color-violet)] rounded-full blur-[180px] opacity-[0.03]"></div>

		<!-- Noise Overlay -->
		<div class="absolute inset-0 noise"></div>
	</div>

	<div class="relative z-10">
		<!-- Header -->
		<header class="border-b border-white/[0.04] sticky top-0 z-50 glass-panel">
			<div class="max-w-[1800px] mx-auto px-8 py-5">
				<div class="flex items-center justify-between">
					<!-- Logo & Title -->
					<div class="flex items-center gap-5">
						<div class="relative">
							<div class="w-12 h-12 rounded-lg bg-[var(--color-cyan)] flex items-center justify-center glow-cyan">
								<svg class="w-6 h-6 text-[var(--color-void)]" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
									<path stroke-linecap="round" stroke-linejoin="round" d="M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z" />
								</svg>
							</div>

						</div>
						<div>
							<h1 class="font-display text-2xl text-[var(--color-light)] tracking-wider">Mnemoo Tools</h1>
							<p class="text-xs text-[var(--color-mist)] font-mono tracking-widest">LOOKUP TABLE EXPLORER</p>
						</div>
					</div>

					<!-- Status Indicators -->
					{#if indexInfo}
						<div class="flex items-center gap-3">
							<div class="px-3 py-1.5 rounded-lg data-cell">
								<span class="text-xs font-mono text-[var(--color-mist)]">
									<span class="text-[var(--color-cyan)]">{indexInfo.modes.length}</span> MODES
								</span>
							</div>
							<button
								onclick={reloadBooks}
								disabled={reloadLoading}
								class="px-3 py-1.5 rounded-lg data-cell text-xs font-mono text-[var(--color-gold)] hover:bg-[var(--color-gold)]/10 transition-colors disabled:opacity-50 flex items-center gap-2"
								title="Reload index.json and all books"
							>
								{#if reloadLoading}
									<svg class="w-3.5 h-3.5 animate-spin" fill="none" viewBox="0 0 24 24">
										<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
										<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
									</svg>
								{:else}
									<svg class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
										<path stroke-linecap="round" stroke-linejoin="round" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
									</svg>
								{/if}
								RELOAD
							</button>
						</div>
					{/if}
				</div>
			</div>
		</header>

		<!-- Main Content -->
		<main class="max-w-[1800px] mx-auto px-8 py-10">
			{#if loading}
				<!-- Loading State -->
				<div class="flex flex-col items-center justify-center py-40 gap-6">
					<div class="relative w-20 h-20">
						<div class="absolute inset-0 rounded-full border-2 border-[var(--color-slate)]"></div>
						<div class="absolute inset-0 rounded-full border-2 border-transparent border-t-[var(--color-cyan)] animate-spin"></div>
						<div class="absolute inset-2 rounded-full border border-[var(--color-slate)]"></div>
						<div class="absolute inset-2 rounded-full border border-transparent border-b-[var(--color-violet)] animate-spin" style="animation-direction: reverse; animation-duration: 1.5s;"></div>
					</div>
					<div class="text-center">
						<p class="font-display text-xl text-[var(--color-light)] tracking-wider">INITIALIZING</p>
						<p class="text-xs font-mono text-[var(--color-mist)] mt-2">Loading analysis data...</p>
					</div>
				</div>
			{:else if error}
				<!-- Error State -->
				<div class="max-w-lg mx-auto mt-32">
					<div class="metric-card glow-coral text-center p-10">
						<div class="w-16 h-16 mx-auto mb-6 rounded-full bg-[var(--color-coral-glow)] flex items-center justify-center">
							<svg class="w-8 h-8 text-[var(--color-coral)]" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
								<path stroke-linecap="round" stroke-linejoin="round" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
							</svg>
						</div>
						<h2 class="font-display text-2xl text-[var(--color-coral)] tracking-wider mb-3">CONNECTION FAILED</h2>
						<p class="text-[var(--color-mist)] mb-6">{error}</p>
						<div class="inline-block px-4 py-2 rounded-lg data-cell">
							<code class="font-mono text-sm text-[var(--color-light)]">localhost:7755</code>
						</div>
					</div>
				</div>
			{:else if indexInfo}
				<!-- Mode Selection Grid -->
				<section class="mb-10 opacity-0 animate-fade-up" style="animation-fill-mode: forwards;">
					<div class="flex items-center justify-between mb-5">
						<div class="flex items-center gap-3">
							<div class="w-1 h-6 bg-[var(--color-cyan)] rounded-full"></div>
							<h2 class="font-display text-xl text-[var(--color-light)] tracking-wider">GAME MODES</h2>
						</div>
						<span class="text-xs font-mono text-[var(--color-mist)]">SELECT MODE TO ANALYZE</span>
					</div>

					<div class="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5 2xl:grid-cols-6 gap-3">
						{#each indexInfo.modes as mode, i}
							{@const isSelected = selectedMode === mode.mode}
							{@const modeVol = compareItems.find(c => c.mode === mode.mode)?.volatility ?? 5}
							{@const volInfo = getVolatilityInfo(modeVol)}
							<button
								class="mode-btn text-left group {isSelected ? 'active' : ''}"
								onclick={() => selectMode(mode.mode)}
								style="animation-delay: {i * 50}ms"
							>
								<div class="flex items-center justify-between mb-3">
									<span class="font-display text-lg text-[var(--color-light)] tracking-wide uppercase">{mode.mode}</span>
									<span class="badge badge-{volInfo.color} text-[10px]">{volInfo.label}</span>
								</div>

								<div class="grid grid-cols-2 gap-x-4 gap-y-2 text-xs font-mono">
									<div class="flex justify-between">
										<span class="text-[var(--color-mist)]">RTP</span>
										<span class="text-[var(--color-emerald)]">{formatPercent(mode.rtp)}</span>
									</div>
									<div class="flex justify-between">
										<span class="text-[var(--color-mist)]">HIT</span>
										<span class="text-[var(--color-cyan)]">{formatPercent(mode.hit_rate)}</span>
									</div>
									<div class="flex justify-between">
										<span class="text-[var(--color-mist)]">MAX</span>
										<span class="text-[var(--color-gold)]">{formatMultiplier(mode.max_payout)}</span>
									</div>
									<div class="flex justify-between">
										<span class="text-[var(--color-mist)]">COST</span>
										<span class="text-[var(--color-light)]">{mode.cost}x</span>
									</div>
								</div>

								<!-- Selected indicator line -->
								{#if isSelected}
									<div class="absolute bottom-0 left-4 right-4 h-0.5 bg-gradient-to-r from-transparent via-[var(--color-cyan)] to-transparent"></div>
								{/if}
							</button>
						{/each}
					</div>
				</section>

				{#if currentModeInfo && stats}
					<!-- Hero Metrics - Compact Dashboard -->
					<section class="mb-8 opacity-0 animate-fade-up delay-1" style="animation-fill-mode: forwards;">
						<div class="grid grid-cols-2 lg:grid-cols-4 gap-3">
							<!-- RTP -->
							<div class="relative p-4 rounded-xl bg-gradient-to-br from-[var(--color-graphite)]/60 to-[var(--color-onyx)]/80 border border-[var(--color-emerald)]/10 group hover:border-[var(--color-emerald)]/30 transition-all overflow-hidden">
								<div class="absolute -top-8 -right-8 w-24 h-24 bg-[var(--color-emerald)] rounded-full blur-[50px] opacity-10 group-hover:opacity-20 transition-opacity"></div>
								<div class="relative flex items-center gap-3">
									<!-- Mini gauge -->
									<div class="relative w-12 h-12 flex-shrink-0">
										<svg class="w-full h-full -rotate-90" viewBox="0 0 48 48">
											<circle cx="24" cy="24" r="20" fill="none" stroke="var(--color-slate)" stroke-width="3"/>
											<circle cx="24" cy="24" r="20" fill="none" stroke="var(--color-emerald)" stroke-width="3" stroke-linecap="round" stroke-dasharray="{Math.max(0, Math.min(1, (stats.rtp - 0.90) / 0.08)) * 125.66} 125.66" class="drop-shadow-[0_0_4px_var(--color-emerald)]"/>
										</svg>
									</div>
									<div class="min-w-0">
										<p class="text-[10px] font-mono text-[var(--color-emerald)] tracking-widest">RTP</p>
										<p class="font-display text-2xl text-[var(--color-light)] leading-tight">{(stats.rtp * 100).toFixed(2)}<span class="text-sm text-[var(--color-mist)]">%</span></p>
									</div>
								</div>
							</div>

							<!-- Hit Rate -->
							<div class="relative p-4 rounded-xl bg-gradient-to-br from-[var(--color-graphite)]/60 to-[var(--color-onyx)]/80 border border-[var(--color-cyan)]/10 group hover:border-[var(--color-cyan)]/30 transition-all overflow-hidden">
								<div class="absolute -top-8 -right-8 w-24 h-24 bg-[var(--color-cyan)] rounded-full blur-[50px] opacity-10 group-hover:opacity-20 transition-opacity"></div>
								<div class="relative flex items-center gap-3">
									<div class="relative w-12 h-12 flex-shrink-0">
										<svg class="w-full h-full -rotate-90" viewBox="0 0 48 48">
											<circle cx="24" cy="24" r="20" fill="none" stroke="var(--color-slate)" stroke-width="3"/>
											<circle cx="24" cy="24" r="20" fill="none" stroke="var(--color-cyan)" stroke-width="3" stroke-linecap="round" stroke-dasharray="{stats.hit_rate * 125.66} 125.66" class="drop-shadow-[0_0_4px_var(--color-cyan)]"/>
										</svg>
									</div>
									<div class="min-w-0">
										<p class="text-[10px] font-mono text-[var(--color-cyan)] tracking-widest">HIT RATE</p>
										<p class="font-display text-2xl text-[var(--color-light)] leading-tight">{(stats.hit_rate * 100).toFixed(2)}<span class="text-sm text-[var(--color-mist)]">%</span></p>
									</div>
								</div>
							</div>

							<!-- Max Payout -->
							<div class="relative p-4 rounded-xl bg-gradient-to-br from-[var(--color-graphite)]/60 to-[var(--color-onyx)]/80 border border-[var(--color-gold)]/10 group hover:border-[var(--color-gold)]/30 transition-all overflow-hidden">
								<div class="absolute -top-8 -right-8 w-24 h-24 bg-[var(--color-gold)] rounded-full blur-[50px] opacity-10 group-hover:opacity-20 transition-opacity"></div>
								<div class="relative flex items-center gap-3">
									<div class="w-12 h-12 flex-shrink-0 rounded-lg bg-[var(--color-gold)]/10 border border-[var(--color-gold)]/20 flex items-center justify-center">
										<svg class="w-5 h-5 text-[var(--color-gold)]" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
											<path stroke-linecap="round" stroke-linejoin="round" d="M11.48 3.499a.562.562 0 011.04 0l2.125 5.111a.563.563 0 00.475.345l5.518.442c.499.04.701.663.321.988l-4.204 3.602a.563.563 0 00-.182.557l1.285 5.385a.562.562 0 01-.84.61l-4.725-2.885a.563.563 0 00-.586 0L6.982 20.54a.562.562 0 01-.84-.61l1.285-5.386a.562.562 0 00-.182-.557l-4.204-3.602a.563.563 0 01.321-.988l5.518-.442a.563.563 0 00.475-.345L11.48 3.5z" />
										</svg>
									</div>
									<div class="min-w-0">
										<p class="text-[10px] font-mono text-[var(--color-gold)] tracking-widest">MAX WIN</p>
										<p class="font-display text-2xl text-[var(--color-light)] leading-tight">{stats.max_payout.toFixed(0)}<span class="text-sm text-[var(--color-mist)]">x</span></p>
									</div>
								</div>
							</div>

							<!-- Volatility -->
							{#if volatilityInfo}
								<div class="relative p-4 rounded-xl bg-gradient-to-br from-[var(--color-graphite)]/60 to-[var(--color-onyx)]/80 border border-[var(--color-{volatilityInfo.color})]/10 group hover:border-[var(--color-{volatilityInfo.color})]/30 transition-all overflow-hidden">
									<div class="absolute -top-8 -right-8 w-24 h-24 bg-[var(--color-{volatilityInfo.color})] rounded-full blur-[50px] opacity-10 group-hover:opacity-20 transition-opacity"></div>
									<div class="relative">
										<div class="flex items-center justify-between mb-2">
											<div>
												<p class="text-[10px] font-mono {volatilityInfo.textClass} tracking-widest">VOLATILITY</p>
												<p class="font-display text-2xl text-[var(--color-light)] leading-tight">{stats.volatility.toFixed(2)}</p>
											</div>
											<span class="text-[9px] font-mono font-bold {volatilityInfo.textClass} px-1.5 py-0.5 rounded bg-[var(--color-{volatilityInfo.color})]/10">{volatilityInfo.label}</span>
										</div>
										<!-- Mini meter -->
										<div class="h-1 rounded-full bg-[var(--color-slate)] overflow-hidden">
											<div class="h-full rounded-full" style="width: {Math.min(stats.volatility / 20 * 100, 100)}%; background: linear-gradient(90deg, var(--color-emerald), var(--color-gold), var(--color-coral));"></div>
										</div>
									</div>
								</div>
							{/if}
						</div>
					</section>

					<!-- Navigation Tabs -->
					<section class="mb-8 opacity-0 animate-fade-up delay-2" style="animation-fill-mode: forwards;">
						<div class="flex gap-1 p-1.5 rounded-xl glass-panel w-fit">
							{#each tabs as tab}
								{@const isActive = activePanel === tab.id}
								<button
									class="tab-btn {isActive ? 'active' : ''}"
									onclick={() => setPanel(tab.id)}
								>
									<span class="font-mono text-xs tracking-wider">{tab.label}</span>
									{#if tab.badge}
										<span class="ml-1.5 px-1.5 py-0.5 text-[9px] font-mono font-bold uppercase rounded bg-amber-500/20 text-amber-400 border border-amber-500/30">{tab.badge}</span>
									{/if}
								</button>
							{/each}
						</div>
					</section>

					<!-- Panel Content -->
					<section class="opacity-0 animate-fade-up delay-3" style="animation-fill-mode: forwards;">
						{#if activePanel === 'overview'}
							<div class="grid gap-6 lg:grid-cols-2 items-stretch">
								<!-- Detailed Metrics Grid -->
								<div class="glass-panel rounded-2xl p-6">
									<div class="flex items-center gap-3 mb-6">
										<div class="w-1 h-5 bg-[var(--color-violet)] rounded-full"></div>
										<h3 class="font-display text-lg text-[var(--color-light)] tracking-wider">DETAILED METRICS</h3>
									</div>
									<div class="grid grid-cols-3 gap-3">
										<div class="data-cell rounded-xl p-4">
											<div class="text-xs font-mono text-[var(--color-mist)] mb-1">OUTCOMES</div>
											<div class="font-mono text-xl text-[var(--color-light)]">{stats.total_outcomes.toLocaleString()}</div>
										</div>
										<div class="data-cell rounded-xl p-4">
											<div class="text-xs font-mono text-[var(--color-mist)] mb-1">ZERO RATE</div>
											<div class="font-mono text-xl text-[var(--color-coral)]">{(stats.zero_payout_rate * 100).toFixed(2)}%</div>
										</div>
										<div class="data-cell rounded-xl p-4">
											<div class="text-xs font-mono text-[var(--color-mist)] mb-1">MEAN</div>
											<div class="font-mono text-xl text-[var(--color-violet)]">{stats.mean_payout.toFixed(2)}x</div>
										</div>
										<div class="data-cell rounded-xl p-4">
											<div class="text-xs font-mono text-[var(--color-mist)] mb-1">MEDIAN</div>
											<div class="font-mono text-xl text-[var(--color-violet)]">{stats.median_payout.toFixed(2)}x</div>
										</div>
										<div class="data-cell rounded-xl p-4">
											<div class="text-xs font-mono text-[var(--color-mist)] mb-1">STD DEV</div>
											<div class="font-mono text-xl text-[var(--color-gold)]">{stats.std_dev.toFixed(4)}</div>
										</div>
										<div class="data-cell rounded-xl p-4">
											<div class="text-xs font-mono text-[var(--color-mist)] mb-1">MIN</div>
											<div class="font-mono text-xl text-[var(--color-mist)]">{stats.min_payout.toFixed(2)}x</div>
										</div>
									</div>
								</div>

								<!-- Books Loading Status -->
								<div class="glass-panel rounded-2xl p-6 flex flex-col">
									{#key reloadKey}
										<BooksLoadingPanel />
									{/key}
								</div>

								<!-- Payout Buckets -->
								<div class="glass-panel rounded-2xl p-6 flex flex-col">
									<PayoutBuckets buckets={stats.payout_buckets} />
								</div>

								<!-- Top Payouts -->
								<div class="glass-panel rounded-2xl p-6 flex flex-col">
									<TopPayouts payouts={stats.top_payouts} onLook={openEventModal} />
								</div>

								<!-- Mode Comparison -->
								<div class="lg:col-span-2 glass-panel rounded-2xl p-6">
									<ModeComparison items={compareItems} />
								</div>
							</div>
						{:else if activePanel === 'distribution'}
							<div class="glass-panel rounded-2xl p-6">
								<DistributionTable
									distribution={stats.distribution}
									buckets={stats.payout_buckets}
									mode={selectedMode ?? ''}
									onLook={openEventModal}
								/>
							</div>
						{:else if activePanel === 'compliance'}
							<div class="glass-panel rounded-2xl p-6">
								{#key reloadKey}
									<ComplianceChecklist />
								{/key}
							</div>
						{:else if activePanel === 'crowdsim'}
							<div class="glass-panel rounded-2xl p-6">
								{#if selectedMode}
									{#key reloadKey}
										<CrowdSimPanel mode={selectedMode} />
									{/key}
								{:else}
									<div class="py-20 text-center">
										<div class="w-16 h-16 mx-auto mb-4 rounded-full bg-[var(--color-slate)] flex items-center justify-center">
											<svg class="w-8 h-8 text-[var(--color-mist)]" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
												<path stroke-linecap="round" stroke-linejoin="round" d="M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0zm6 3a2 2 0 11-4 0 2 2 0 014 0zM7 10a2 2 0 11-4 0 2 2 0 014 0z" />
											</svg>
										</div>
										<p class="font-display text-lg text-[var(--color-mist)]">SELECT A MODE</p>
										<p class="text-xs font-mono text-[var(--color-mist)] mt-2">Choose a game mode to run CrowdSim</p>
									</div>
								{/if}
							</div>
						{:else if activePanel === 'optimizer'}
							<div class="glass-panel rounded-2xl p-6">
								{#if selectedMode}
									{#key reloadKey}
										<OptimizerPanel mode={selectedMode} />
									{/key}
								{:else}
									<div class="py-20 text-center">
										<div class="w-16 h-16 mx-auto mb-4 rounded-full bg-[var(--color-slate)] flex items-center justify-center">
											<svg class="w-8 h-8 text-[var(--color-mist)]" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
												<path stroke-linecap="round" stroke-linejoin="round" d="M13 10V3L4 14h7v7l9-11h-7z" />
											</svg>
										</div>
										<p class="font-display text-lg text-[var(--color-mist)]">SELECT A MODE</p>
										<p class="text-xs font-mono text-[var(--color-mist)] mt-2">Choose a game mode to optimize distribution</p>
									</div>
								{/if}
							</div>
						{:else if activePanel === 'lgs'}
							{#key reloadKey}
								<LGSPanel modes={indexInfo?.modes ?? []} />
							{/key}
						{/if}
					</section>
				{:else}
					<!-- No mode selected -->
					<div class="py-32 text-center">
						<div class="w-24 h-24 mx-auto mb-8 rounded-2xl bg-[var(--color-graphite)] flex items-center justify-center">
							<svg class="w-12 h-12 text-[var(--color-slate)]" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
								<path stroke-linecap="round" stroke-linejoin="round" d="M15 15l-2 5L9 9l11 4-5 2zm0 0l5 5M7.188 2.239l.777 2.897M5.136 7.965l-2.898-.777M13.95 4.05l-2.122 2.122m-5.657 5.656l-2.12 2.122" />
							</svg>
						</div>
						<h3 class="font-display text-2xl text-[var(--color-mist)] tracking-wider">SELECT A MODE</h3>
						<p class="text-sm font-mono text-[var(--color-mist)] mt-3">Choose a game mode above to view detailed analytics</p>
					</div>
				{/if}
			{/if}
		</main>

		<!-- Footer -->
		<footer class="border-t border-white/[0.03] py-8 mt-16">
			<div class="max-w-[1800px] mx-auto px-8 flex items-center justify-between">
				<div class="flex items-center gap-4">
					<div class="w-1 h-4 bg-[var(--color-cyan)]/50 rounded-full"></div>
					<p class="text-xs font-mono text-[var(--color-mist)]">0.1.0</p>
				</div>
				<div class="flex items-center gap-6">
					<span class="text-xs font-mono text-[var(--color-mist)]">MNEMOO TOOLS</span>
				</div>
			</div>
		</footer>
	</div>
</div>

<!-- Welcome Modal for LGS Certificate -->
<WelcomeModal bind:this={welcomeModal} onReady={() => {}} />

<!-- Event Modal -->
{#if showEventModal && selectedMode && selectedSimId !== null}
	<EventModal mode={selectedMode} simId={selectedSimId} onClose={closeEventModal} />
{/if}
