<script lang="ts">
	import type { CrowdSimDrawdownStats } from '$lib/api/types';

	let { stats }: { stats: CrowdSimDrawdownStats } = $props();

	function formatPercent(value: number): string {
		return (value * 100).toFixed(1) + '%';
	}
</script>

<div class="rounded-2xl bg-[var(--color-graphite)]/50 border border-white/[0.03] p-5">
	<div class="flex items-center gap-3 mb-5">
		<div class="w-1 h-5 bg-[var(--color-coral)] rounded-full"></div>
		<h3 class="font-mono text-base text-[var(--color-light)]">DRAWDOWN ANALYSIS</h3>
	</div>

	<!-- Main Stats -->
	<div class="grid grid-cols-2 gap-3 mb-5">
		<div class="rounded-xl bg-[var(--color-onyx)]/50 p-4">
			<div class="text-xs font-mono text-[var(--color-mist)] tracking-wider mb-1">AVG MAX DRAWDOWN</div>
			<div class="font-display text-3xl text-[var(--color-coral)]">{formatPercent(stats.avg_max_drawdown)}</div>
		</div>
		<div class="rounded-xl bg-[var(--color-onyx)]/50 p-4">
			<div class="text-xs font-mono text-[var(--color-mist)] tracking-wider mb-1">MEDIAN MAX DD</div>
			<div class="font-display text-3xl text-[var(--color-gold)]">{formatPercent(stats.median_max_drawdown)}</div>
		</div>
	</div>

	<!-- Severity Breakdown -->
	<div class="space-y-3">
		<div class="text-sm font-mono text-[var(--color-mist)] mb-2">PLAYERS BY DRAWDOWN SEVERITY</div>

		<div class="space-y-2">
			<div class="flex items-center gap-3">
				<span class="w-24 text-right text-sm font-mono text-[var(--color-mist)]">Below 50%</span>
				<div class="flex-1 h-6 rounded-lg bg-[var(--color-onyx)] overflow-hidden relative">
					<div
						class="h-full rounded-lg bg-[var(--color-gold)]"
						style="width: {Math.min(stats.percent_below_50, 100)}%; box-shadow: 0 0 10px var(--color-gold)40;"
					></div>
					<span class="absolute inset-0 flex items-center justify-center text-sm font-mono text-white/90">
						{stats.players_below_50pct.toLocaleString()} <span class="text-white/50 ml-1">({stats.percent_below_50.toFixed(1)}%)</span>
					</span>
				</div>
			</div>

			<div class="flex items-center gap-3">
				<span class="w-24 text-right text-sm font-mono text-[var(--color-mist)]">Below 90%</span>
				<div class="flex-1 h-6 rounded-lg bg-[var(--color-onyx)] overflow-hidden relative">
					<div
						class="h-full rounded-lg bg-[var(--color-coral)]"
						style="width: {Math.min(stats.percent_below_90, 100)}%; box-shadow: 0 0 10px var(--color-coral)40;"
					></div>
					<span class="absolute inset-0 flex items-center justify-center text-sm font-mono text-white/90">
						{stats.players_below_90pct.toLocaleString()} <span class="text-white/50 ml-1">({stats.percent_below_90.toFixed(1)}%)</span>
					</span>
				</div>
			</div>
		</div>
	</div>

	<!-- Max Observed -->
	<div class="mt-5 pt-4 border-t border-white/[0.03]">
		<div class="flex items-center justify-between">
			<span class="text-sm font-mono text-[var(--color-mist)]">MAXIMUM OBSERVED DRAWDOWN</span>
			<span class="text-base font-mono font-semibold text-[var(--color-coral)]">{formatPercent(stats.max_drawdown_observed)}</span>
		</div>
	</div>
</div>
