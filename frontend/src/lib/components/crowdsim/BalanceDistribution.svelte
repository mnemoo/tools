<script lang="ts">
	import type { CrowdSimBalanceStats } from '$lib/api/types';

	let {
		stats,
		initialBalance = 100,
		playerCount = 0
	}: {
		stats: CrowdSimBalanceStats;
		initialBalance?: number;
		playerCount?: number;
	} = $props();

	// Calculate total players in distribution buckets
	let playersInBuckets = $derived(
		stats.distribution
			? stats.distribution.reduce((sum, b) => sum + b.count, 0)
			: 0
	);

	// Calculate busted players (balance < 0, not in any bucket)
	let bustedPlayers = $derived(
		playerCount > 0 ? Math.max(0, playerCount - playersInBuckets) : 0
	);

	let bustedPercent = $derived(
		playerCount > 0 ? (bustedPlayers / playerCount) * 100 : 0
	);

	// Total players for display (including busted)
	let totalPlayers = $derived(playerCount > 0 ? playerCount : playersInBuckets);

	// Calculate max count for scaling (include busted in max calculation)
	let maxCount = $derived(() => {
		const bucketMax = stats.distribution
			? Math.max(...stats.distribution.map((b) => b.count))
			: 0;
		return Math.max(bucketMax, bustedPlayers);
	});

	let profitablePlayers = $derived(
		stats.distribution
			? stats.distribution
				.filter(b => b.range_start >= initialBalance)
				.reduce((sum, b) => sum + b.count, 0)
			: 0
	);

	let profitPercent = $derived(totalPlayers > 0 ? (profitablePlayers / totalPlayers) * 100 : 0);

	function getBarGradient(rangeStart: number): string {
		if (rangeStart >= initialBalance) return 'from-[var(--color-emerald)] to-[var(--color-emerald)]/70';
		if (rangeStart >= initialBalance * 0.75) return 'from-[var(--color-gold)] to-[var(--color-gold)]/70';
		if (rangeStart >= initialBalance * 0.5) return 'from-[var(--color-coral)]/90 to-[var(--color-coral)]/60';
		return 'from-[var(--color-coral)]/80 to-[var(--color-coral)]/50';
	}

	function getBarGlow(rangeStart: number): string {
		if (rangeStart >= initialBalance) return '0 0 15px var(--color-emerald)';
		if (rangeStart >= initialBalance * 0.75) return '0 0 15px var(--color-gold)';
		return '0 0 15px var(--color-coral)';
	}

	function formatRange(start: number, end: number): string {
		if (end >= 1000) return `${start}+`;
		return `${start}-${end}`;
	}

	function getPercentileColor(value: number): string {
		if (value < initialBalance * 0.5) return 'text-[var(--color-coral)]';
		if (value < initialBalance) return 'text-[var(--color-gold)]';
		return 'text-[var(--color-emerald)]';
	}

	function getPercentileBg(value: number): string {
		if (value < initialBalance * 0.5) return 'bg-[var(--color-coral)]/10 border-[var(--color-coral)]/20';
		if (value < initialBalance) return 'bg-[var(--color-gold)]/10 border-[var(--color-gold)]/20';
		return 'bg-[var(--color-emerald)]/10 border-[var(--color-emerald)]/20';
	}

</script>

<div class="rounded-2xl bg-[var(--color-graphite)]/50 border border-white/[0.03] overflow-hidden">
	<!-- Header with Summary Stats -->
	<div class="px-5 py-4 border-b border-white/[0.03] bg-gradient-to-r from-[var(--color-graphite)]/80 to-transparent">
		<div class="flex items-center justify-between">
			<div class="flex items-center gap-3">
				<div class="w-1 h-5 bg-[var(--color-gold)] rounded-full"></div>
				<div>
					<h3 class="font-mono text-sm text-[var(--color-light)]">FINAL BALANCE DISTRIBUTION</h3>
					<p class="text-xs font-mono text-[var(--color-mist)]">{totalPlayers.toLocaleString()} players analyzed</p>
				</div>
			</div>

			<!-- Quick Stats -->
			<div class="flex items-center gap-4">
				<div class="text-right">
					<div class="text-[10px] font-mono text-[var(--color-mist)]">PROFITABLE</div>
					<div class="flex items-baseline gap-1">
						<span class="font-display text-xl text-[var(--color-emerald)]">{profitPercent.toFixed(1)}</span>
						<span class="text-xs text-[var(--color-emerald)]">%</span>
					</div>
				</div>
				<div class="w-px h-8 bg-white/[0.05]"></div>
				<div class="text-right">
					<div class="text-[10px] font-mono text-[var(--color-mist)]">AT LOSS</div>
					<div class="flex items-baseline gap-1">
						<span class="font-display text-xl text-[var(--color-coral)]">{(100 - profitPercent).toFixed(1)}</span>
						<span class="text-xs text-[var(--color-coral)]">%</span>
					</div>
				</div>
				{#if bustedPlayers > 0}
					<div class="w-px h-8 bg-white/[0.05]"></div>
					<div class="text-right">
						<div class="text-[10px] font-mono text-[var(--color-mist)]">BUSTED</div>
						<div class="flex items-baseline gap-1">
							<span class="font-display text-xl text-red-500">{bustedPercent.toFixed(1)}</span>
							<span class="text-xs text-red-500">%</span>
						</div>
					</div>
				{/if}
			</div>
		</div>
	</div>

	<!-- Distribution Chart -->
	<div class="p-5">
		{#if stats.distribution && stats.distribution.length > 0}
			<div class="space-y-1.5">
				<!-- BUSTED category (balance < 0) -->
				{#if bustedPlayers > 0}
					{@const widthPercent = (bustedPlayers / maxCount()) * 100}
					<div class="group relative flex items-center gap-3 py-0.5">
						<!-- Range Label -->
						<div class="w-14 text-right text-xs font-mono text-red-400 shrink-0 font-semibold">
							&lt; 0
						</div>

						<!-- Bar Container -->
						<div class="flex-1 relative">
							<!-- Background track -->
							<div class="h-7 rounded-lg bg-[var(--color-onyx)]/80 overflow-hidden">
								<!-- Filled bar with special "busted" styling -->
								<div
									class="h-full rounded-lg bg-gradient-to-r from-red-600 to-red-500/70 transition-all duration-500 ease-out relative"
									style="width: {widthPercent}%; box-shadow: 0 0 20px rgba(239, 68, 68, 0.5);"
								>
									<!-- Shine effect -->
									<div class="absolute inset-0 bg-gradient-to-b from-white/10 to-transparent rounded-lg"></div>
									<!-- Skull icon for busted -->
									<div class="absolute right-2 top-1/2 -translate-y-1/2">
										<svg class="w-4 h-4 text-white/60" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
											<path stroke-linecap="round" stroke-linejoin="round" d="M12 9v3.75m-9.303 3.376c-.866 1.5.217 3.374 1.948 3.374h14.71c1.73 0 2.813-1.874 1.948-3.374L13.949 3.378c-.866-1.5-3.032-1.5-3.898 0L2.697 16.126zM12 15.75h.007v.008H12v-.008z" />
										</svg>
									</div>
								</div>
							</div>

							<!-- Value label -->
							<div
								class="absolute top-0 h-7 flex items-center text-xs font-mono transition-all"
								style="left: {widthPercent > 40 ? '8px' : `calc(${widthPercent}% + 8px)`};"
							>
								<span class="{widthPercent > 40 ? 'text-white' : 'text-red-400'} font-semibold">
									{bustedPlayers.toLocaleString()}
								</span>
								<span class="{widthPercent > 40 ? 'text-white/60' : 'text-red-400/70'} ml-1.5">
									{bustedPercent.toFixed(1)}%
								</span>
								<span class="{widthPercent > 40 ? 'text-white/40' : 'text-red-400/50'} ml-1.5 text-[10px]">
									BUSTED
								</span>
							</div>
						</div>
					</div>

					<!-- Separator -->
					<div class="h-px bg-gradient-to-r from-transparent via-white/10 to-transparent my-2"></div>
				{/if}

				<!-- Regular distribution buckets -->
				{#each stats.distribution as bucket, i}
					{#if bucket.count > 0}
						{@const widthPercent = (bucket.count / maxCount()) * 100}
						<div class="group relative flex items-center gap-3 py-0.5">
							<!-- Range Label -->
							<div class="w-14 text-right text-xs font-mono text-[var(--color-mist)] shrink-0">
								{formatRange(bucket.range_start, bucket.range_end)}
							</div>

							<!-- Bar Container -->
							<div class="flex-1 relative">
								<!-- Background track -->
								<div class="h-7 rounded-lg bg-[var(--color-onyx)]/80 overflow-hidden">
									<!-- Filled bar -->
									<div
										class="h-full rounded-lg bg-gradient-to-r {getBarGradient(bucket.range_start)} transition-all duration-500 ease-out relative"
										style="width: {widthPercent}%; box-shadow: {getBarGlow(bucket.range_start)};"
									>
										<!-- Shine effect -->
										<div class="absolute inset-0 bg-gradient-to-b from-white/10 to-transparent rounded-lg"></div>
									</div>
								</div>

								<!-- Value label (positioned based on bar width) -->
								<div
									class="absolute top-0 h-7 flex items-center text-xs font-mono transition-all"
									style="left: {widthPercent > 40 ? '8px' : `calc(${widthPercent}% + 8px)`};"
								>
									<span class="{widthPercent > 40 ? 'text-white' : 'text-[var(--color-mist)]'}">
										{bucket.count.toLocaleString()}
									</span>
									<span class="{widthPercent > 40 ? 'text-white/60' : 'text-[var(--color-mist)]'} ml-1.5">
										{bucket.percent.toFixed(1)}%
									</span>
								</div>
							</div>
						</div>
					{/if}
				{/each}
			</div>

			<!-- Legend -->
			<div class="mt-4 flex items-center justify-center gap-4 text-xs font-mono flex-wrap">
				<div class="flex items-center gap-1.5">
					<div class="w-3 h-3 rounded bg-[var(--color-emerald)]/60"></div>
					<span class="text-[var(--color-mist)]">Profit</span>
				</div>
				<div class="flex items-center gap-1.5">
					<div class="w-3 h-3 rounded bg-[var(--color-coral)]/60"></div>
					<span class="text-[var(--color-mist)]">Loss</span>
				</div>
				{#if bustedPlayers > 0}
					<div class="flex items-center gap-1.5">
						<div class="w-3 h-3 rounded bg-red-500/60"></div>
						<span class="text-[var(--color-mist)]">Busted</span>
					</div>
				{/if}
			</div>
		{:else}
			<div class="text-center py-12">
				<div class="w-12 h-12 rounded-xl bg-[var(--color-mist)]/20 flex items-center justify-center mx-auto mb-3">
					<svg class="w-6 h-6 text-[var(--color-mist)]" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
						<path stroke-linecap="round" stroke-linejoin="round" d="M3 13.125C3 12.504 3.504 12 4.125 12h2.25c.621 0 1.125.504 1.125 1.125v6.75C7.5 20.496 6.996 21 6.375 21h-2.25A1.125 1.125 0 013 19.875v-6.75z" />
					</svg>
				</div>
				<p class="text-[var(--color-mist)]">No distribution data available</p>
			</div>
		{/if}
	</div>

	<!-- Percentiles Section -->
	{#if stats.percentiles}
		<div class="px-5 pb-5">
			<div class="rounded-xl bg-[var(--color-onyx)]/30 border border-white/[0.02] p-4">
				<div class="flex items-center gap-2 mb-4">
					<svg class="w-4 h-4 text-[var(--color-violet)]" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
						<path stroke-linecap="round" stroke-linejoin="round" d="M7 12l3-3 3 3 4-4M8 21l4-4 4 4M3 4h18M4 4h16v12a1 1 0 01-1 1H5a1 1 0 01-1-1V4z" />
					</svg>
					<span class="text-xs font-mono text-[var(--color-mist)]">BALANCE PERCENTILES</span>
				</div>

				<!-- Percentile Cards -->
				<div class="grid grid-cols-7 gap-2">
					{#each ['5', '10', '25', '50', '75', '90', '95'] as p}
						{#if stats.percentiles[p] !== undefined}
							{@const value = stats.percentiles[p]}
							{@const isMedian = p === '50'}
							{@const isNegative = value < 0}
							<div class="relative rounded-lg border {isNegative ? 'bg-red-500/10 border-red-500/20' : getPercentileBg(value)} p-2.5 text-center transition-all hover:scale-105 {isMedian ? 'ring-1 ring-[var(--color-violet)]/30' : ''}">
								{#if isMedian}
									<div class="absolute -top-1.5 left-1/2 -translate-x-1/2 px-1.5 py-0.5 rounded text-[8px] font-mono bg-[var(--color-violet)] text-white">MEDIAN</div>
								{/if}
								<div class="text-[10px] font-mono text-[var(--color-mist)] {isMedian ? 'mt-1' : ''}">P{p}</div>
								<div class="text-base font-mono font-bold {isNegative ? 'text-red-400' : getPercentileColor(value)}">
									{value.toFixed(0)}
								</div>
								<!-- Mini indicator -->
								<div class="mt-1.5 h-0.5 rounded-full bg-[var(--color-mist)]/20 overflow-hidden">
									{#if isNegative}
										<div class="h-full rounded-full bg-red-500" style="width: 5%"></div>
									{:else}
										<div
											class="h-full rounded-full {value >= initialBalance ? 'bg-[var(--color-emerald)]' : value >= initialBalance * 0.5 ? 'bg-[var(--color-gold)]' : 'bg-[var(--color-coral)]'}"
											style="width: {Math.min((value / (initialBalance * 2)) * 100, 100)}%"
										></div>
									{/if}
								</div>
							</div>
						{/if}
					{/each}
				</div>

				<!-- Percentile Legend -->
				<div class="mt-4 pt-3 border-t border-white/[0.03] flex items-center justify-center gap-6 text-[10px] font-mono text-[var(--color-mist)]">
					<div class="flex items-center gap-1.5">
						<div class="w-2 h-2 rounded-sm bg-[var(--color-emerald)]"></div>
						<span>≥ Initial ({initialBalance})</span>
					</div>
					<div class="flex items-center gap-1.5">
						<div class="w-2 h-2 rounded-sm bg-[var(--color-gold)]"></div>
						<span>50-100%</span>
					</div>
					<div class="flex items-center gap-1.5">
						<div class="w-2 h-2 rounded-sm bg-[var(--color-coral)]"></div>
						<span>&lt; 50%</span>
					</div>
					<div class="flex items-center gap-1.5">
						<div class="w-2 h-2 rounded-sm bg-red-500"></div>
						<span>&lt; 0 (Busted)</span>
					</div>
				</div>
			</div>
		</div>
	{/if}
</div>
