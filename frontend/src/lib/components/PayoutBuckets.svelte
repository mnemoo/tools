<script lang="ts">
	import type { PayoutBucket } from '$lib/api';
	import { _ } from '$lib/i18n';

	interface Props {
		buckets: PayoutBucket[];
	}

	let { buckets }: Props = $props();

	let maxProbability = $derived(Math.max(...buckets.map((b) => b.probability)));
	let maxRangeEnd = $derived(Math.max(...buckets.map((b) => b.range_end)));

	function formatNumber(v: number): string {
		if (v >= 1_000_000) return (v / 1_000_000).toFixed(v % 1_000_000 === 0 ? 0 : 1) + 'M';
		if (v >= 1_000) return (v / 1_000).toFixed(v % 1_000 === 0 ? 0 : 1) + 'K';
		if (v >= 10) return v.toFixed(0);
		if (v >= 1) return v.toFixed(v % 1 === 0 ? 0 : 1);
		if (v > 0) return v.toFixed(2);
		return '0';
	}

	function formatRange(bucket: PayoutBucket, lossLabel: string): string {
		if (bucket.range_start === 0 && bucket.range_end === 0) {
			return lossLabel;
		}
		// Last bucket (extends to max)
		if (bucket.range_end >= maxRangeEnd * 0.99) {
			return `${formatNumber(bucket.range_start)}x+`;
		}
		return `${formatNumber(bucket.range_start)}x - ${formatNumber(bucket.range_end)}x`;
	}

	function formatOdds(probability: number): string {
		if (probability === 0) return '-';
		const odds = 1 / probability;
		if (odds >= 1_000_000) {
			return '1 in ' + (odds / 1_000_000).toFixed(1) + 'M';
		}
		if (odds >= 1_000) {
			return '1 in ' + (odds / 1_000).toFixed(1) + 'K';
		}
		if (odds >= 10) {
			return '1 in ' + odds.toFixed(0);
		}
		return '1 in ' + odds.toFixed(2);
	}

	function getBarWidth(probability: number): number {
		return (probability / maxProbability) * 100;
	}

	function getBarColor(rangeStart: number): string {
		if (rangeStart === 0) return 'bg-gray-500';
		if (rangeStart < 1) return 'bg-blue-400';
		if (rangeStart < 5) return 'bg-green-400';
		if (rangeStart < 20) return 'bg-yellow-400';
		if (rangeStart < 100) return 'bg-orange-400';
		if (rangeStart < 1000) return 'bg-red-400';
		if (rangeStart < 10000) return 'bg-pink-500';
		return 'bg-purple-500';
	}
</script>

<div class="h-full flex flex-col">
	<div class="flex items-center gap-3 mb-6 shrink-0">
		<div class="w-1 h-5 bg-[var(--color-cyan)] rounded-full"></div>
		<h3 class="font-display text-lg text-[var(--color-light)] tracking-wider">{$_('distribution.title')}</h3>
	</div>

	{#if buckets.length === 0}
		<div class="py-8 text-center text-slate-500">{$_('status.noData')}</div>
	{:else}
		<div class="flex-1 overflow-y-auto pr-1 space-y-2 scrollbar-thin min-h-0">
			{#each buckets as bucket}
				<div class="flex items-center gap-3 group">
					<div class="w-32 shrink-0 text-right text-sm text-slate-400 font-mono">
						{formatRange(bucket, $_('distribution.loss'))}
					</div>
					<div class="relative h-7 flex-1 overflow-hidden rounded-lg bg-slate-700/50">
						<div
							class="h-full transition-all duration-500 ease-out {getBarColor(bucket.range_start)} group-hover:opacity-90"
							style="width: {getBarWidth(bucket.probability)}%"
						></div>
						<div class="absolute inset-0 bg-gradient-to-r from-transparent to-white/5"></div>
					</div>
					<div class="w-24 shrink-0 text-right text-sm">
						<span class="text-white font-medium">{formatOdds(bucket.probability)}</span>
					</div>
					<div class="w-16 shrink-0 text-right text-xs text-slate-500 font-mono">
						{bucket.count.toLocaleString()}
					</div>
				</div>
			{/each}
		</div>

		<div class="mt-4 flex items-center justify-end gap-6 text-xs text-slate-500 shrink-0">
			<span>{$_('table.odds')}</span>
			<span class="w-16 text-right">{$_('table.count')}</span>
		</div>
	{/if}
</div>
