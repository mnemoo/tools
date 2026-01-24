<script lang="ts">
	import type { CrowdSimResult } from '$lib/api/types';
	import { _ } from '$lib/i18n';

	let { result }: { result: CrowdSimResult } = $props();

	function formatNumber(value: number): string {
		return value.toFixed(2);
	}
</script>

<div class="space-y-4">
	<!-- Balance Statistics -->
	<div class="rounded-2xl bg-[var(--color-graphite)]/50 border border-white/[0.03] p-5">
		<div class="flex items-center gap-3 mb-4">
			<div class="w-1 h-5 bg-[var(--color-cyan)] rounded-full"></div>
			<h3 class="font-mono text-base text-[var(--color-light)]">{$_('crowdsim.balanceStats')}</h3>
		</div>
		<div class="grid grid-cols-3 lg:grid-cols-6 gap-3">
			<div class="rounded-xl bg-[var(--color-onyx)]/50 p-3">
				<div class="text-xs font-mono text-[var(--color-mist)] tracking-wider">{$_('metrics.mean')}</div>
				<div class="text-xl font-mono text-[var(--color-light)]">{formatNumber(result.balance_stats.mean)}</div>
			</div>
			<div class="rounded-xl bg-[var(--color-onyx)]/50 p-3">
				<div class="text-xs font-mono text-[var(--color-mist)] tracking-wider">{$_('metrics.median')}</div>
				<div class="text-xl font-mono text-[var(--color-light)]">{formatNumber(result.balance_stats.median)}</div>
			</div>
			<div class="rounded-xl bg-[var(--color-onyx)]/50 p-3">
				<div class="text-xs font-mono text-[var(--color-mist)] tracking-wider">{$_('metrics.stdDev')}</div>
				<div class="text-xl font-mono text-[var(--color-gold)]">{formatNumber(result.balance_stats.std_dev)}</div>
			</div>
			<div class="rounded-xl bg-[var(--color-onyx)]/50 p-3">
				<div class="text-xs font-mono text-[var(--color-mist)] tracking-wider">{$_('metrics.min')}</div>
				<div class="text-xl font-mono text-[var(--color-coral)]">{formatNumber(result.balance_stats.min)}</div>
			</div>
			<div class="rounded-xl bg-[var(--color-onyx)]/50 p-3">
				<div class="text-xs font-mono text-[var(--color-mist)] tracking-wider">{$_('metrics.maxWin')}</div>
				<div class="text-xl font-mono text-[var(--color-emerald)]">{formatNumber(result.balance_stats.max)}</div>
			</div>
			<div class="rounded-xl bg-[var(--color-onyx)]/50 p-3">
				<div class="text-xs font-mono text-[var(--color-mist)] tracking-wider">{$_('crowdsim.initial')}</div>
				<div class="text-xl font-mono text-[var(--color-mist)]">{formatNumber(result.config.initial_balance)}</div>
			</div>
		</div>
	</div>

	<!-- Secondary Metrics Grid -->
	<div class="grid gap-4 lg:grid-cols-4">
		<!-- Peak Stats -->
		<div class="rounded-2xl bg-[var(--color-graphite)]/50 border border-white/[0.03] p-5">
			<div class="flex items-center gap-2 mb-4">
				<svg class="w-5 h-5 text-[var(--color-emerald)]" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
					<path stroke-linecap="round" stroke-linejoin="round" d="M13 7h8m0 0v8m0-8l-8 8-4-4-6 6" />
				</svg>
				<span class="text-sm font-mono text-[var(--color-mist)]">{$_('crowdsim.peakBalance')}</span>
			</div>
			<div class="space-y-3">
				<div class="flex items-center justify-between">
					<span class="text-sm font-mono text-[var(--color-mist)]">{$_('crowdsim.avgPeak')}</span>
					<span class="text-base font-mono text-[var(--color-emerald)]">{formatNumber(result.peak_stats.avg_peak)}</span>
				</div>
				<div class="flex items-center justify-between">
					<span class="text-sm font-mono text-[var(--color-mist)]">{$_('metrics.median')}</span>
					<span class="text-base font-mono text-[var(--color-emerald)]">{formatNumber(result.peak_stats.median_peak)}</span>
				</div>
				<div class="flex items-center justify-between">
					<span class="text-sm font-mono text-[var(--color-mist)]">{$_('metrics.maxWin')}</span>
					<span class="text-base font-mono font-bold text-[var(--color-emerald)]">{formatNumber(result.peak_stats.max_peak)}</span>
				</div>
			</div>
		</div>

		<!-- Drawdown Stats -->
		<div class="rounded-2xl bg-[var(--color-graphite)]/50 border border-white/[0.03] p-5">
			<div class="flex items-center gap-2 mb-4">
				<svg class="w-5 h-5 text-[var(--color-coral)]" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
					<path stroke-linecap="round" stroke-linejoin="round" d="M13 17h8m0 0V9m0 8l-8-8-4 4-6-6" />
				</svg>
				<span class="text-sm font-mono text-[var(--color-mist)]">{$_('crowdsim.drawdown')}</span>
			</div>
			<div class="space-y-3">
				<div class="flex items-center justify-between">
					<span class="text-sm font-mono text-[var(--color-mist)]">{$_('crowdsim.avgMaxDd')}</span>
					<span class="text-base font-mono text-[var(--color-coral)]">{(result.drawdown_stats.avg_max_drawdown * 100).toFixed(1)}%</span>
				</div>
				<div class="flex items-center justify-between">
					<span class="text-sm font-mono text-[var(--color-mist)]">{$_('crowdsim.below50')}</span>
					<span class="text-base font-mono text-[var(--color-gold)]">{result.drawdown_stats.percent_below_50.toFixed(1)}%</span>
				</div>
				<div class="flex items-center justify-between">
					<span class="text-sm font-mono text-[var(--color-mist)]">{$_('crowdsim.below90')}</span>
					<span class="text-base font-mono text-[var(--color-coral)]">{result.drawdown_stats.percent_below_90.toFixed(1)}%</span>
				</div>
			</div>
		</div>

		<!-- Streak Stats -->
		<div class="rounded-2xl bg-[var(--color-graphite)]/50 border border-white/[0.03] p-5">
			<div class="flex items-center gap-2 mb-4">
				<svg class="w-5 h-5 text-[var(--color-violet)]" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
					<path stroke-linecap="round" stroke-linejoin="round" d="M17 8h2a2 2 0 012 2v6a2 2 0 01-2 2h-2v4l-4-4H9a1.994 1.994 0 01-1.414-.586m0 0L11 14h4a2 2 0 002-2V6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2v4l.586-.586z" />
				</svg>
				<span class="text-sm font-mono text-[var(--color-mist)]">{$_('crowdsim.streaks')}</span>
			</div>
			<div class="space-y-3">
				<div class="flex items-center justify-between">
					<span class="text-sm font-mono text-[var(--color-mist)]">{$_('crowdsim.avgWin')}</span>
					<span class="text-base font-mono text-[var(--color-emerald)]">{result.streak_stats.avg_win_streak.toFixed(1)}</span>
				</div>
				<div class="flex items-center justify-between">
					<span class="text-sm font-mono text-[var(--color-mist)]">{$_('crowdsim.maxWin')}</span>
					<span class="text-base font-mono font-bold text-[var(--color-emerald)]">{result.streak_stats.max_win_streak}</span>
				</div>
				<div class="flex items-center justify-between">
					<span class="text-sm font-mono text-[var(--color-mist)]">{$_('crowdsim.maxLose')}</span>
					<span class="text-base font-mono font-bold text-[var(--color-coral)]">{result.streak_stats.max_lose_streak}</span>
				</div>
			</div>
		</div>

		<!-- Big Win Stats -->
		<div class="rounded-2xl bg-[var(--color-graphite)]/50 border border-white/[0.03] p-5">
			<div class="flex items-center gap-2 mb-4">
				<svg class="w-5 h-5 text-[var(--color-gold)]" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
					<path stroke-linecap="round" stroke-linejoin="round" d="M11.049 2.927c.3-.921 1.603-.921 1.902 0l1.519 4.674a1 1 0 00.95.69h4.915c.969 0 1.371 1.24.588 1.81l-3.976 2.888a1 1 0 00-.363 1.118l1.518 4.674c.3.922-.755 1.688-1.538 1.118l-3.976-2.888a1 1 0 00-1.176 0l-3.976 2.888c-.783.57-1.838-.197-1.538-1.118l1.518-4.674a1 1 0 00-.363-1.118l-3.976-2.888c-.784-.57-.38-1.81.588-1.81h4.914a1 1 0 00.951-.69l1.519-4.674z" />
				</svg>
				<span class="text-sm font-mono text-[var(--color-mist)]">{$_('crowdsim.bigWins')} ({result.config.big_win_threshold}x+)</span>
			</div>
			<div class="space-y-3">
				<div class="flex items-center justify-between">
					<span class="text-sm font-mono text-[var(--color-mist)]">{$_('metrics.hitRate')}</span>
					<span class="text-base font-mono text-[var(--color-emerald)]">{result.big_win_stats.percent_hit.toFixed(1)}%</span>
				</div>
				<div class="flex items-center justify-between">
					<span class="text-sm font-mono text-[var(--color-mist)]">{$_('crowdsim.avgSpins')}</span>
					<span class="text-base font-mono text-[var(--color-cyan)]">
						{result.big_win_stats.avg_spins_to_first > 0 ? result.big_win_stats.avg_spins_to_first.toFixed(0) : 'N/A'}
					</span>
				</div>
				<div class="flex items-center justify-between">
					<span class="text-sm font-mono text-[var(--color-mist)]">{$_('crowdsim.neverHit')}</span>
					<span class="text-base font-mono text-[var(--color-coral)]">{result.big_win_stats.percent_never_hit.toFixed(1)}%</span>
				</div>
			</div>
		</div>
	</div>

	<!-- Danger Stats (only if danger events occurred) -->
	{#if result.danger_stats.total_danger_events > 0}
		<div class="rounded-2xl bg-[var(--color-coral)]/5 border border-[var(--color-coral)]/20 p-5">
			<div class="flex items-center gap-3 mb-4">
				<div class="w-8 h-8 rounded-lg bg-[var(--color-coral)]/20 flex items-center justify-center">
					<svg class="w-5 h-5 text-[var(--color-coral)]" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
						<path stroke-linecap="round" stroke-linejoin="round" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
					</svg>
				</div>
				<div>
					<span class="font-mono text-base text-[var(--color-coral)]">{$_('crowdsim.dangerZone')}</span>
					<span class="text-sm font-mono text-[var(--color-mist)] ml-2">{$_('crowdsim.balanceBelow', { values: { value: (result.config.danger_threshold * 100).toFixed(0) } })}</span>
				</div>
			</div>
			<div class="grid grid-cols-3 gap-4">
				<div>
					<div class="text-xs font-mono text-[var(--color-mist)] tracking-wider">{$_('crowdsim.totalEvents')}</div>
					<div class="text-2xl font-mono text-[var(--color-coral)]">{result.danger_stats.total_danger_events.toLocaleString()}</div>
				</div>
				<div>
					<div class="text-xs font-mono text-[var(--color-mist)] tracking-wider">{$_('crowdsim.playersAffected')}</div>
					<div class="text-2xl font-mono text-[var(--color-coral)]">{result.danger_stats.percent_with_danger.toFixed(1)}%</div>
				</div>
				<div>
					<div class="text-xs font-mono text-[var(--color-mist)] tracking-wider">{$_('crowdsim.avgPerPlayer')}</div>
					<div class="text-2xl font-mono text-[var(--color-coral)]">{result.danger_stats.avg_danger_events.toFixed(1)}</div>
				</div>
			</div>
		</div>
	{/if}
</div>
