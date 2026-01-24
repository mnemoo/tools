<script lang="ts">
	import type { CompareItem } from '$lib/api';
	import { _ } from '$lib/i18n';

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

	function getVolatilityLabel(item: CompareItem): { label: string; color: string; displayValue: number } {
		// For bonus modes (cost > 1), use breakeven rate for classification
		if (item.cost > 1) {
			const br = item.breakeven_rate;
			if (br >= 0.50) return { label: 'Low', color: 'text-green-400', displayValue: item.cost_adj_volatility };
			if (br >= 0.30) return { label: 'Medium', color: 'text-yellow-400', displayValue: item.cost_adj_volatility };
			if (br >= 0.15) return { label: 'High', color: 'text-orange-400', displayValue: item.cost_adj_volatility };
			return { label: 'Extreme', color: 'text-red-400', displayValue: item.cost_adj_volatility };
		}
		// For base mode, use traditional CV-based classification
		const vol = item.volatility;
		if (vol < 3) return { label: 'Low', color: 'text-green-400', displayValue: vol };
		if (vol < 7) return { label: 'Medium', color: 'text-yellow-400', displayValue: vol };
		if (vol < 15) return { label: 'High', color: 'text-orange-400', displayValue: vol };
		return { label: 'Very High', color: 'text-red-400', displayValue: vol };
	}
</script>

<div>
	<div class="flex items-center gap-3 mb-6">
		<div class="w-1 h-5 bg-[var(--color-violet)] rounded-full"></div>
		<h3 class="font-display text-lg text-[var(--color-light)] tracking-wider">{$_('modeComparison.title')}</h3>
	</div>

	{#if items.length === 0}
		<div class="py-8 text-center text-slate-500">{$_('status.noData')}</div>
	{:else}
		<div class="overflow-x-auto">
			<table class="w-full">
				<thead>
					<tr class="text-left text-xs uppercase text-slate-500 tracking-wider">
						<th class="pb-3 font-medium">{$_('table.mode')}</th>
						<th class="pb-3 text-right font-medium">{$_('metrics.rtp')}</th>
						<th class="pb-3 text-right font-medium">{$_('metrics.hitRate')}</th>
						<th class="pb-3 text-right font-medium">{$_('metrics.maxWin')}</th>
						<th class="pb-3 text-right font-medium">{$_('metrics.mean')}</th>
						<th class="pb-3 text-right font-medium">{$_('metrics.median')}</th>
						<th class="pb-3 text-right font-medium">{$_('metrics.volatility')}</th>
					</tr>
				</thead>
				<tbody class="text-sm">
					{#each items as item}
						{@const vol = getVolatilityLabel(item)}
						<tr class="border-t border-slate-700/50 hover:bg-slate-700/20 transition-colors">
							<td class="py-3">
								<span class="font-semibold text-white capitalize">{item.mode}</span>
								{#if item.cost > 1}
									<span class="ml-1.5 text-[10px] px-1.5 py-0.5 rounded bg-violet-500/20 text-violet-400">{item.cost}x</span>
								{/if}
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
									<span class="{vol.color} font-mono font-medium">{formatDecimal(vol.displayValue)}</span>
									<span class="text-[10px] px-1.5 py-0.5 rounded-full bg-slate-700 text-slate-400">{vol.label}</span>
									{#if item.cost > 1}
										<span class="text-[9px] text-slate-500">({(item.breakeven_rate * 100).toFixed(0)}% BE)</span>
									{/if}
								</span>
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
	{/if}
</div>
