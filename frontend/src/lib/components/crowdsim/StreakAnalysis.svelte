<script lang="ts">
	import type { CrowdSimStreakStats } from '$lib/api/types';
	import { _ } from '$lib/i18n';

	let { stats }: { stats: CrowdSimStreakStats } = $props();

	// Calculate ratio for visualization
	let winRatio = $derived(stats.avg_win_streak / (stats.avg_win_streak + stats.avg_lose_streak));
</script>

<div class="rounded-2xl bg-[var(--color-graphite)]/50 border border-white/[0.03] p-5">
	<div class="flex items-center gap-3 mb-5">
		<div class="w-1 h-5 bg-[var(--color-violet)] rounded-full"></div>
		<h3 class="font-mono text-base text-[var(--color-light)]">{$_('streaks.title')}</h3>
	</div>

	<div class="grid grid-cols-2 gap-4">
		<!-- Winning Streaks -->
		<div class="rounded-xl bg-[var(--color-emerald)]/5 border border-[var(--color-emerald)]/10 p-4">
			<div class="flex items-center gap-2 mb-3">
				<svg class="w-5 h-5 text-[var(--color-emerald)]" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
					<path stroke-linecap="round" stroke-linejoin="round" d="M5 10l7-7m0 0l7 7m-7-7v18" />
				</svg>
				<span class="text-sm font-mono text-[var(--color-emerald)]">{$_('streaks.winStreaks')}</span>
			</div>
			<div class="space-y-3">
				<div class="flex items-center justify-between">
					<span class="text-sm font-mono text-[var(--color-mist)]">{$_('streaks.average')}</span>
					<span class="text-xl font-mono font-semibold text-[var(--color-emerald)]">{stats.avg_win_streak.toFixed(1)}</span>
				</div>
				<div class="flex items-center justify-between">
					<span class="text-sm font-mono text-[var(--color-mist)]">{$_('streaks.maximum')}</span>
					<span class="text-xl font-mono font-bold text-[var(--color-emerald)]">{stats.max_win_streak}</span>
				</div>
			</div>
		</div>

		<!-- Losing Streaks -->
		<div class="rounded-xl bg-[var(--color-coral)]/5 border border-[var(--color-coral)]/10 p-4">
			<div class="flex items-center gap-2 mb-3">
				<svg class="w-5 h-5 text-[var(--color-coral)]" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
					<path stroke-linecap="round" stroke-linejoin="round" d="M19 14l-7 7m0 0l-7-7m7 7V3" />
				</svg>
				<span class="text-sm font-mono text-[var(--color-coral)]">{$_('streaks.loseStreaks')}</span>
			</div>
			<div class="space-y-3">
				<div class="flex items-center justify-between">
					<span class="text-sm font-mono text-[var(--color-mist)]">{$_('streaks.average')}</span>
					<span class="text-xl font-mono font-semibold text-[var(--color-coral)]">{stats.avg_lose_streak.toFixed(1)}</span>
				</div>
				<div class="flex items-center justify-between">
					<span class="text-sm font-mono text-[var(--color-mist)]">{$_('streaks.maximum')}</span>
					<span class="text-xl font-mono font-bold text-[var(--color-coral)]">{stats.max_lose_streak}</span>
				</div>
			</div>
		</div>
	</div>

	<!-- Visual Streak Ratio -->
	<div class="mt-5 pt-4 border-t border-white/[0.03]">
		<div class="text-sm font-mono text-[var(--color-mist)] mb-3">{$_('streaks.ratio')}</div>
		<div class="h-4 rounded-full overflow-hidden flex">
			<div
				class="bg-[var(--color-emerald)] transition-all"
				style="width: {winRatio * 100}%; box-shadow: 0 0 10px var(--color-emerald)40;"
			></div>
			<div
				class="bg-[var(--color-coral)] transition-all"
				style="width: {(1 - winRatio) * 100}%; box-shadow: 0 0 10px var(--color-coral)40;"
			></div>
		</div>
		<div class="mt-2 flex justify-between text-sm font-mono">
			<span class="text-[var(--color-emerald)]">{$_('streaks.win')}: {stats.avg_win_streak.toFixed(1)}</span>
			<span class="text-[var(--color-coral)]">{$_('streaks.lose')}: {stats.avg_lose_streak.toFixed(1)}</span>
		</div>
	</div>
</div>
