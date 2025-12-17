<script lang="ts">
	import type { PayoutInfo } from '$lib/api';

	interface Props {
		payouts: PayoutInfo[];
		onLook?: (simId: number) => void;
	}

	let { payouts, onLook }: Props = $props();

	function formatMultiplier(value: number): string {
		return value.toFixed(2) + 'x';
	}
</script>

<div class="h-full flex flex-col">
	<div class="flex items-center gap-3 mb-6 shrink-0">
		<div class="w-1 h-5 bg-[var(--color-gold)] rounded-full"></div>
		<h3 class="font-display text-lg text-[var(--color-light)] tracking-wider">TOP PAYOUTS</h3>
	</div>

	{#if payouts.length === 0}
		<div class="py-8 text-center text-slate-500">No data</div>
	{:else}
		<div class="flex-1 overflow-x-auto">
			<table class="w-full">
				<thead>
					<tr class="text-left text-xs uppercase text-slate-500 tracking-wider">
						<th class="pb-3 font-medium">#</th>
						<th class="pb-3 font-medium">Payout</th>
						<th class="pb-3 text-right font-medium">Books</th>
						<th class="pb-3 text-right font-medium">Weight</th>
						<th class="pb-3 text-right font-medium">Odds</th>
						<th class="pb-3 text-right font-medium">Action</th>
					</tr>
				</thead>
				<tbody class="text-sm">
					{#each payouts as payout, i}
						<tr class="border-t border-slate-700/50 hover:bg-slate-700/20 transition-colors">
							<td class="py-3 font-mono">
								<span class="text-slate-500">#{i + 1}</span>
							</td>
							<td class="py-3">
								<span class="inline-flex items-center gap-1.5 px-2 py-1 rounded-lg bg-amber-500/10 text-amber-400 font-bold">
									{formatMultiplier(payout.payout)}
								</span>
							</td>
							<td class="py-3 text-right font-mono text-[var(--color-cyan)]">
								{payout.count.toLocaleString()}
							</td>
							<td class="py-3 text-right text-slate-400 font-mono">
								{payout.weight.toLocaleString()}
							</td>
							<td class="py-3 text-right text-slate-500 text-xs">{payout.odds}</td>
							<td class="py-3 text-right">
								<button
									onclick={() => onLook?.(payout.sim_id)}
									class="inline-flex items-center gap-1 px-3 py-1.5 rounded-lg bg-blue-600/20 text-blue-400 text-xs font-medium hover:bg-blue-600/30 transition-colors border border-blue-500/20"
								>
									<svg class="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
										<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
										<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M2.458 12C3.732 7.943 7.523 5 12 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7z" />
									</svg>
									View
								</button>
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
	{/if}
</div>
