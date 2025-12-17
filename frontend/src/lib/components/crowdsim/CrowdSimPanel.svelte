<script lang="ts">
	import { onMount } from 'svelte';
	import { api } from '$lib/api';
	import type { CrowdSimConfig, CrowdSimResult, CrowdSimPresetInfo } from '$lib/api/types';
	import ConfigPanel from './ConfigPanel.svelte';
	import MetricsDashboard from './MetricsDashboard.svelte';
	import PoPCurveChart from './PoPCurveChart.svelte';
	import BalanceCurveChart from './BalanceCurveChart.svelte';
	import BalanceDistribution from './BalanceDistribution.svelte';
	import DrawdownAnalysis from './DrawdownAnalysis.svelte';
	import StreakAnalysis from './StreakAnalysis.svelte';

	let { mode }: { mode: string } = $props();

	// Default config
	let config = $state<CrowdSimConfig>({
		player_count: 1000,
		spins_per_session: 200,
		initial_balance: 100,
		bet_amount: 1,
		big_win_threshold: 10,
		danger_threshold: 0.1,
		use_crypto_rng: false,
		streaming_mode: false,
		parallel_workers: 4
	});

	let presets = $state<CrowdSimPresetInfo[]>([]);
	let result = $state<CrowdSimResult | null>(null);
	let loading = $state(false);
	let error = $state<string | null>(null);
	let progress = $state({ players: 0, total: 0, percent: 0 });

	onMount(async () => {
		try {
			presets = await api.crowdsimPresets();
		} catch (e) {
			console.error('Failed to load presets:', e);
		}
	});

	async function runSimulation() {
		loading = true;
		error = null;
		result = null;
		progress = { players: 0, total: config.player_count, percent: 0 };

		try {
			result = await api.crowdsimSimulate(mode, config);
		} catch (e) {
			error = e instanceof Error ? e.message : 'Simulation failed';
		} finally {
			loading = false;
		}
	}

	function getVolatilityInfo(profile: string): { label: string; color: string; textClass: string } {
		switch (profile) {
			case 'low':
				return { label: 'LOW', color: 'emerald', textClass: 'text-[var(--color-emerald)]' };
			case 'medium':
				return { label: 'MEDIUM', color: 'gold', textClass: 'text-[var(--color-gold)]' };
			case 'high':
				return { label: 'HIGH', color: 'coral', textClass: 'text-[var(--color-coral)]' };
			default:
				return { label: 'UNKNOWN', color: 'mist', textClass: 'text-[var(--color-mist)]' };
		}
	}

	function getPoPColor(pop: number): string {
		if (pop >= 0.4) return 'text-[var(--color-emerald)]';
		if (pop >= 0.25) return 'text-[var(--color-gold)]';
		return 'text-[var(--color-coral)]';
	}
</script>

<div class="space-y-6">
	<!-- Header -->
	<div class="flex items-center gap-3 mb-2">
		<div class="w-1 h-6 bg-[var(--color-violet)] rounded-full"></div>
		<h2 class="font-display text-xl text-[var(--color-light)] tracking-wider">CROWD SIMULATION</h2>
		<span class="text-xs font-mono text-[var(--color-mist)] uppercase">{mode}</span>
	</div>

	<!-- Configuration Panel -->
	<ConfigPanel bind:config {presets} onRun={runSimulation} disabled={loading} />

	<!-- Loading State -->
	{#if loading}
		<div class="relative rounded-2xl bg-gradient-to-br from-[var(--color-graphite)]/80 to-[var(--color-onyx)] border border-[var(--color-violet)]/20 p-10 overflow-hidden">
			<!-- Background glow -->
			<div class="absolute inset-0 bg-[var(--color-violet)] opacity-[0.03] animate-pulse"></div>

			<div class="relative flex flex-col items-center">
				<!-- Animated spinner -->
				<div class="relative w-20 h-20 mb-6">
					<div class="absolute inset-0 rounded-full border-2 border-[var(--color-slate)]"></div>
					<div class="absolute inset-0 rounded-full border-2 border-transparent border-t-[var(--color-violet)] animate-spin"></div>
					<div class="absolute inset-2 rounded-full border border-[var(--color-slate)]"></div>
					<div class="absolute inset-2 rounded-full border border-transparent border-b-[var(--color-cyan)] animate-spin" style="animation-direction: reverse; animation-duration: 1.5s;"></div>
					<div class="absolute inset-4 rounded-full bg-[var(--color-violet)]/20 flex items-center justify-center">
						<svg class="w-6 h-6 text-[var(--color-violet)]" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
							<path stroke-linecap="round" stroke-linejoin="round" d="M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0zm6 3a2 2 0 11-4 0 2 2 0 014 0zM7 10a2 2 0 11-4 0 2 2 0 014 0z" />
						</svg>
					</div>
				</div>

				<h3 class="font-display text-xl text-[var(--color-light)] tracking-wider mb-2">SIMULATING</h3>
				<p class="text-sm font-mono text-[var(--color-mist)] mb-6">
					{config.player_count.toLocaleString()} players × {config.spins_per_session} spins
				</p>

				<!-- Progress bar -->
				{#if progress.percent > 0}
					<div class="w-64">
						<div class="h-1.5 rounded-full bg-[var(--color-slate)] overflow-hidden">
							<div
								class="h-full rounded-full bg-gradient-to-r from-[var(--color-violet)] to-[var(--color-cyan)] transition-all duration-300"
								style="width: {progress.percent}%"
							></div>
						</div>
						<p class="text-xs font-mono text-[var(--color-mist)] text-center mt-2">{progress.percent}%</p>
					</div>
				{/if}
			</div>
		</div>
	{/if}

	<!-- Error State -->
	{#if error}
		<div class="rounded-2xl bg-[var(--color-coral)]/10 border border-[var(--color-coral)]/30 p-6">
			<div class="flex items-center gap-4">
				<div class="w-12 h-12 rounded-xl bg-[var(--color-coral)]/20 flex items-center justify-center flex-shrink-0">
					<svg class="w-6 h-6 text-[var(--color-coral)]" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
						<path stroke-linecap="round" stroke-linejoin="round" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
					</svg>
				</div>
				<div>
					<h3 class="font-display text-lg text-[var(--color-coral)] tracking-wider">SIMULATION FAILED</h3>
					<p class="text-sm text-[var(--color-mist)] mt-1">{error}</p>
				</div>
			</div>
		</div>
	{/if}

	<!-- Results -->
	{#if result}
		{@const volInfo = getVolatilityInfo(result.volatility_profile)}

		<!-- Bonus Mode Info Banner -->
		{#if result.mode_info?.is_bonus_mode}
			<div class="px-4 py-3 rounded-xl bg-violet-500/10 border border-violet-500/30 flex items-start gap-3">
				<svg class="w-5 h-5 text-violet-400 shrink-0 mt-0.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
					<path stroke-linecap="round" stroke-linejoin="round" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
				</svg>
				<div class="text-sm text-violet-200/90">
					<strong class="text-violet-400">Bonus Mode (Cost: {result.mode_info.cost}x)</strong>
					<p class="mt-1 text-xs text-violet-200/70">
						Payouts are normalized by cost. Big win threshold of {config.big_win_threshold}x = {Math.round(config.big_win_threshold * result.mode_info.cost)}x absolute.
					</p>
				</div>
			</div>
		{/if}

		<!-- Hero Results Banner -->
		<div class="relative rounded-2xl bg-gradient-to-br from-[var(--color-graphite)]/80 to-[var(--color-onyx)] border border-white/[0.05] p-6 overflow-hidden">
			<!-- Decorative background -->
			<div class="absolute top-0 right-0 w-64 h-64 bg-[var(--color-{volInfo.color})] rounded-full blur-[120px] opacity-[0.07]"></div>

			<div class="relative grid grid-cols-2 lg:grid-cols-4 gap-6">
				<!-- PoP -->
				<div class="text-center lg:text-left">
					<p class="text-xs font-mono text-[var(--color-mist)] tracking-widest mb-1">PROBABILITY OF PROFIT</p>
					<p class="font-display text-5xl {getPoPColor(result.final_pop)}">{(result.final_pop * 100).toFixed(1)}<span class="text-xl">%</span></p>
					<p class="text-sm font-mono text-[var(--color-mist)] mt-1">
						{Math.round(result.final_pop * result.config.player_count)} / {result.config.player_count} profitable
					</p>
				</div>

				<!-- RTP -->
				<div class="text-center lg:text-left">
					<p class="text-xs font-mono text-[var(--color-mist)] tracking-widest mb-1">ACTUAL RTP</p>
					<p class="font-display text-5xl text-[var(--color-cyan)]">{(result.actual_rtp * 100).toFixed(2)}<span class="text-xl">%</span></p>
					<p class="text-sm font-mono mt-1 {Math.abs(result.rtp_deviation) <= 0.005 ? 'text-[var(--color-emerald)]' : Math.abs(result.rtp_deviation) <= 0.01 ? 'text-[var(--color-gold)]' : 'text-[var(--color-coral)]'}">
						{result.rtp_deviation >= 0 ? '+' : ''}{(result.rtp_deviation * 100).toFixed(3)}% deviation
					</p>
				</div>

				<!-- Volatility -->
				<div class="text-center lg:text-left">
					<p class="text-xs font-mono text-[var(--color-mist)] tracking-widest mb-1">VOLATILITY PROFILE</p>
					<p class="font-display text-5xl {volInfo.textClass}">{volInfo.label}</p>
					<p class="text-sm font-mono text-[var(--color-mist)] mt-1">Score: {result.composite_score.toFixed(3)}</p>
				</div>

				<!-- Duration -->
				<div class="text-center lg:text-left">
					<p class="text-xs font-mono text-[var(--color-mist)] tracking-widest mb-1">SIMULATION TIME</p>
					<p class="font-display text-5xl text-[var(--color-violet)]">
						{result.duration_ms < 1000 ? result.duration_ms : (result.duration_ms / 1000).toFixed(2)}<span class="text-xl">{result.duration_ms < 1000 ? 'ms' : 's'}</span>
					</p>
					<p class="text-sm font-mono text-[var(--color-mist)] mt-1">
						{(result.config.player_count * result.config.spins_per_session).toLocaleString()} total spins
					</p>
				</div>
			</div>
		</div>

		<!-- Metrics Dashboard -->
		<MetricsDashboard {result} />

		<!-- Charts Grid -->
		<div class="grid gap-6 lg:grid-cols-2">
			<!-- PoP Curve -->
			{#if result.pop_curve && result.pop_curve.length > 0}
				<PoPCurveChart popCurve={result.pop_curve} initialBalance={config.initial_balance} />
			{/if}

			<!-- Balance Curve -->
			{#if result.balance_curve && result.balance_curve.length > 0}
				<BalanceCurveChart balanceCurve={result.balance_curve} initialBalance={config.initial_balance} />
			{/if}
		</div>

		<!-- Balance Distribution -->
		<BalanceDistribution stats={result.balance_stats} initialBalance={config.initial_balance} playerCount={result.config.player_count} />

		<!-- Secondary Analysis -->
		<div class="grid gap-6 lg:grid-cols-2">
			<DrawdownAnalysis stats={result.drawdown_stats} />
			<StreakAnalysis stats={result.streak_stats} />
		</div>
	{/if}
</div>
