<script lang="ts">
	import type { CompareItem } from '$lib/api';

	interface Props {
		items: CompareItem[];
	}

	let { items }: Props = $props();

	function formatPercent(value: number): string {
		return (value * 100).toFixed(2) + '%';
	}

	function formatMultiplier(value: number): string {
		return value.toFixed(2) + 'x';
	}

	function formatDecimal(value: number): string {
		return value.toFixed(2);
	}

	function getVolatilityLabel(volatility: number): { label: string; color: string } {
		if (volatility < 3) return { label: 'Low', color: 'text-green-400' };
		if (volatility < 7) return { label: 'Medium', color: 'text-yellow-400' };
		if (volatility < 15) return { label: 'High', color: 'text-orange-400' };
		return { label: 'Very High', color: 'text-red-400' };
	}
</script>

<div>
	<div class="flex items-center gap-3 mb-6">
		<div class="w-1 h-5 bg-[var(--color-violet)] rounded-full"></div>
		<h3 class="font-display text-lg text-[var(--color-light)] tracking-wider">MODE COMPARISON</h3>
	</div>

	{#if items.length === 0}
		<div class="py-8 text-center text-slate-500">No modes to compare</div>
	{:else}
		<div class="overflow-x-auto">
			<table class="w-full">
				<thead>
					<tr class="text-left text-xs uppercase text-slate-500 tracking-wider">
						<th class="pb-3 font-medium">Mode</th>
						<th class="pb-3 text-right font-medium">RTP</th>
						<th class="pb-3 text-right font-medium">Hit Rate</th>
						<th class="pb-3 text-right font-medium">Max Payout</th>
						<th class="pb-3 text-right font-medium">Mean</th>
						<th class="pb-3 text-right font-medium">Median</th>
						<th class="pb-3 text-right font-medium">Volatility</th>
					</tr>
				</thead>
				<tbody class="text-sm">
					{#each items as item}
						{@const vol = getVolatilityLabel(item.volatility)}
						<tr class="border-t border-slate-700/50 hover:bg-slate-700/20 transition-colors">
							<td class="py-3">
								<span class="font-semibold text-white capitalize">{item.mode}</span>
							</td>
							<td class="py-3 text-right">
								<span class="inline-flex items-center px-2 py-0.5 rounded bg-emerald-500/10 text-emerald-400 font-medium font-mono text-xs">
									{formatPercent(item.rtp)}
								</span>
							</td>
							<td class="py-3 text-right">
								<span class="text-blue-400 font-mono">{formatPercent(item.hit_rate)}</span>
							</td>
							<td class="py-3 text-right">
								<span class="text-amber-400 font-bold">{formatMultiplier(item.max_payout)}</span>
							</td>
							<td class="py-3 text-right text-slate-300 font-mono">{formatMultiplier(item.mean_payout)}</td>
							<td class="py-3 text-right text-slate-300 font-mono">{formatMultiplier(item.median_payout)}</td>
							<td class="py-3 text-right">
								<span class="inline-flex items-center gap-1.5">
									<span class="{vol.color} font-mono font-medium">{formatDecimal(item.volatility)}</span>
									<span class="text-[10px] px-1.5 py-0.5 rounded-full bg-slate-700 text-slate-400">{vol.label}</span>
								</span>
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
	{/if}
</div>
