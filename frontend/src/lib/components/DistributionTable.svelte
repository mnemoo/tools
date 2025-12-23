<script lang="ts">
	import { api, type DistributionItem, type PayoutBucket, type LGSSessionSummary } from '$lib/api';

	interface Props {
		distribution: DistributionItem[];
		buckets: PayoutBucket[];
		mode: string;
		onLook?: (simId: number) => void;
	}

	let { distribution, buckets, mode, onLook }: Props = $props();

	const ITEMS_PER_PAGE = 100;

	let sessions = $state<LGSSessionSummary[]>([]);
	let selectedSession = $state<string>('');
	let forceLoading = $state<number | null>(null);
	let forceMessage = $state<{ simId: number; message: string } | null>(null);

	// Track which bucket is expanded (only one at a time)
	let expandedBucket = $state<string | null>(null);

	// Track visible items count per bucket for lazy loading
	let visibleCounts = $state<Record<string, number>>({});

	// Group distribution items by bucket and sort buckets by range_end descending (biggest first)
	let groupedByBucket = $derived(() => {
		// Sort buckets by range_end descending (wincap first)
		const sortedBuckets = [...buckets].sort((a, b) => b.range_end - a.range_end);

		const groups: { bucket: PayoutBucket; items: DistributionItem[]; key: string }[] = [];

		for (const bucket of sortedBuckets) {
			const key = `${bucket.range_start}-${bucket.range_end}`;
			const items = distribution.filter(item => {
				// Special case for 0x (loss) bucket
				if (bucket.range_start === 0 && bucket.range_end === 0) {
					return item.payout === 0;
				}
				// For the highest bucket, include items >= range_start
				if (bucket.range_end >= Math.max(...buckets.map(b => b.range_end)) * 0.99) {
					return item.payout >= bucket.range_start;
				}
				// Normal range: range_start <= payout < range_end
				return item.payout >= bucket.range_start && item.payout < bucket.range_end;
			}).sort((a, b) => b.payout - a.payout); // Sort items by payout descending within bucket

			if (items.length > 0) {
				groups.push({ bucket, items, key });
			}
		}

		return groups;
	});

	// Initialize visible counts when groups change
	$effect(() => {
		const groups = groupedByBucket();
		for (const { key } of groups) {
			if (!(key in visibleCounts)) {
				visibleCounts[key] = ITEMS_PER_PAGE;
			}
		}
	});

	// Set first bucket as expanded by default (wincap)
	$effect(() => {
		const groups = groupedByBucket();
		if (groups.length > 0 && expandedBucket === null) {
			expandedBucket = groups[0].key;
		}
	});

	function toggleBucket(key: string) {
		// Accordion behavior: close current if clicking same, otherwise switch
		if (expandedBucket === key) {
			expandedBucket = null;
		} else {
			expandedBucket = key;
			// Reset visible count when opening a new bucket
			visibleCounts[key] = ITEMS_PER_PAGE;
		}
	}

	// Load more items for a bucket
	function loadMore(key: string, totalItems: number) {
		const current = visibleCounts[key] ?? ITEMS_PER_PAGE;
		visibleCounts[key] = Math.min(current + ITEMS_PER_PAGE, totalItems);
	}

	// Intersection observer for lazy loading
	function observeSentinel(node: HTMLElement, params: { key: string; totalItems: number }) {
		const observer = new IntersectionObserver(
			(entries) => {
				if (entries[0].isIntersecting) {
					loadMore(params.key, params.totalItems);
				}
			},
			{ rootMargin: '100px' }
		);

		observer.observe(node);

		return {
			destroy() {
				observer.disconnect();
			},
			update(newParams: { key: string; totalItems: number }) {
				params = newParams;
			}
		};
	}

	// Load sessions for force outcome
	async function loadSessions() {
		try {
			const response = await api.lgsSessions();
			sessions = response.sessions;
			if (sessions.length > 0 && !selectedSession) {
				selectedSession = sessions[0].sessionID;
			}
		} catch {
			// Ignore errors
		}
	}

	// Load sessions on mount
	$effect(() => {
		loadSessions();
	});

	function formatMultiplier(value: number): string {
		return value.toFixed(2) + 'x';
	}

	function formatRange(bucket: PayoutBucket): string {
		if (bucket.range_start === 0 && bucket.range_end === 0) {
			return '0x (loss)';
		}
		const maxEnd = Math.max(...buckets.map(b => b.range_end));
		// Last bucket (extends to max)
		if (bucket.range_end >= maxEnd * 0.99) {
			return `${formatNumber(bucket.range_start)}x+`;
		}
		return `${formatNumber(bucket.range_start)}x - ${formatNumber(bucket.range_end)}x`;
	}

	function formatNumber(v: number): string {
		if (v >= 1_000_000) return (v / 1_000_000).toFixed(v % 1_000_000 === 0 ? 0 : 1) + 'M';
		if (v >= 1_000) return (v / 1_000).toFixed(v % 1_000 === 0 ? 0 : 1) + 'K';
		if (v >= 10) return v.toFixed(0);
		if (v >= 1) return v.toFixed(v % 1 === 0 ? 0 : 1);
		if (v > 0) return v.toFixed(2);
		return '0';
	}

	function formatOdds(probability: number): string {
		if (probability === 0) return '-';
		const odds = 1 / probability;
		if (odds >= 1_000_000) return '1 in ' + (odds / 1_000_000).toFixed(1) + 'M';
		if (odds >= 1_000) return '1 in ' + (odds / 1_000).toFixed(1) + 'K';
		if (odds >= 10) return '1 in ' + odds.toFixed(0);
		return '1 in ' + odds.toFixed(2);
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

	function handleLook(item: DistributionItem) {
		if (item.sim_ids && item.sim_ids.length > 0 && onLook) {
			onLook(item.sim_ids[0]);
		}
	}

	async function handleForce(item: DistributionItem) {
		if (!selectedSession || !item.sim_ids || item.sim_ids.length === 0) return;

		const simId = item.sim_ids[0];
		forceLoading = simId;
		forceMessage = null;

		try {
			const response = await api.lgsForceOutcome(selectedSession, mode, simId);
			forceMessage = { simId, message: `Set ${response.payout.toFixed(2)}x for next spin` };
			setTimeout(() => {
				if (forceMessage?.simId === simId) {
					forceMessage = null;
				}
			}, 3000);
		} catch (e) {
			forceMessage = { simId, message: e instanceof Error ? e.message : 'Failed to force' };
		} finally {
			forceLoading = null;
		}
	}
</script>

<div>
	<div class="flex items-center gap-3 mb-6 flex-wrap">
		<div class="w-1 h-5 bg-[var(--color-white)] rounded-full"></div>
		<h3 class="font-display text-lg text-[var(--color-light)] tracking-wider">DISTRIBUTION</h3>
		<span class="text-xs font-mono text-[var(--color-mist)]">({distribution.length.toLocaleString()} unique payouts)</span>

		<!-- Session selector for force -->
		{#if sessions.length > 0}
			<div class="ml-auto flex items-center gap-2">
				<span class="text-xs font-mono text-[var(--color-mist)]">Session:</span>
				<select
					bind:value={selectedSession}
					class="bg-[var(--color-graphite)] border border-white/10 rounded px-2 py-1 text-xs font-mono text-[var(--color-light)] focus:outline-none focus:border-[var(--color-cyan)]"
				>
					{#each sessions as session}
						<option value={session.sessionID}>{session.sessionID}</option>
					{/each}
				</select>
			</div>
		{/if}
	</div>

	{#if distribution.length === 0}
		<div class="py-8 text-center text-slate-500">No data</div>
	{:else}
		<div class="space-y-2">
			{#each groupedByBucket() as { bucket, items, key }}
				{@const isExpanded = expandedBucket === key}
				{@const visibleCount = visibleCounts[key] ?? ITEMS_PER_PAGE}
				{@const visibleItems = items.slice(0, visibleCount)}
				{@const hasMore = visibleCount < items.length}
				<div class="rounded-xl border border-slate-700/50 overflow-hidden bg-slate-800/30">
					<!-- Accordion Header -->
					<button
						class="w-full flex items-center gap-4 px-4 py-3 hover:bg-slate-700/30 transition-colors"
						onclick={() => toggleBucket(key)}
					>
						<!-- Expand/Collapse Icon -->
						<svg
							class="w-4 h-4 text-[var(--color-mist)] transition-transform duration-200 {isExpanded ? 'rotate-90' : ''}"
							fill="none"
							viewBox="0 0 24 24"
							stroke="currentColor"
							stroke-width="2"
						>
							<path stroke-linecap="round" stroke-linejoin="round" d="M9 5l7 7-7 7" />
						</svg>

						<!-- Color indicator -->
						<div class="w-3 h-3 rounded-full {getBarColor(bucket.range_start)}"></div>

						<!-- Range label -->
						<span class="font-mono text-sm text-[var(--color-light)] font-medium min-w-[120px] text-left">
							{formatRange(bucket)}
						</span>

						<!-- Stats -->
						<div class="flex items-center gap-6 ml-auto text-xs font-mono">
							<div class="flex items-center gap-2">
								<span class="text-[var(--color-mist)]">Payouts:</span>
								<span class="text-[var(--color-cyan)]">{items.length}</span>
							</div>
							<div class="flex items-center gap-2">
								<span class="text-[var(--color-mist)]">Books:</span>
								<span class="text-[var(--color-cyan)]">{bucket.count.toLocaleString()}</span>
							</div>
							<div class="flex items-center gap-2">
								<span class="text-[var(--color-mist)]">Odds:</span>
								<span class="text-white">{formatOdds(bucket.probability)}</span>
							</div>
						</div>
					</button>

					<!-- Accordion Content -->
					{#if isExpanded}
						<div class="border-t border-slate-700/50">
							<div class="max-h-[400px] overflow-auto">
								<table class="w-full">
									<thead class="sticky top-0 bg-slate-800/95 backdrop-blur-sm">
										<tr class="text-left text-xs uppercase text-slate-500 tracking-wider">
											<th class="px-4 py-2 font-medium">Payout</th>
											<th class="px-4 py-2 text-right font-medium">Books</th>
											<th class="px-4 py-2 text-right font-medium">Weight</th>
											<th class="px-4 py-2 text-right font-medium">Odds</th>
											<th class="px-4 py-2 text-center font-medium">Actions</th>
										</tr>
									</thead>
									<tbody class="text-sm">
										{#each visibleItems as item, i}
											<tr class="border-t border-slate-700/30 hover:bg-slate-700/30 transition-colors {i % 2 === 0 ? 'bg-slate-800/20' : ''}">
												<td class="px-4 py-2">
													<span class="font-medium text-white font-mono">{formatMultiplier(item.payout)}</span>
												</td>
												<td class="px-4 py-2 text-right">
													<span class="font-mono text-[var(--color-cyan)]">{item.count.toLocaleString()}</span>
												</td>
												<td class="px-4 py-2 text-right text-slate-400 font-mono">{item.weight.toLocaleString()}</td>
												<td class="px-4 py-2 text-right">
													<span class="text-blue-400 font-mono text-xs">{item.odds}</span>
												</td>
												<td class="px-4 py-2 text-center">
													<div class="flex items-center justify-center gap-1">
														{#if item.sim_ids && item.sim_ids.length > 0}
															<!-- LOOK button -->
															<button
																onclick={() => handleLook(item)}
																class="px-2 py-1 rounded text-xs font-mono bg-[var(--color-cyan)]/20 text-[var(--color-cyan)] hover:bg-[var(--color-cyan)]/30 transition-colors"
																title="View event #{item.sim_ids[0]}"
															>
																LOOK
															</button>

															<!-- FORCE button -->
															{#if sessions.length > 0}
																<button
																	onclick={() => handleForce(item)}
																	disabled={forceLoading === item.sim_ids[0]}
																	class="px-2 py-1 rounded text-xs font-mono bg-[var(--color-gold)]/20 text-[var(--color-gold)] hover:bg-[var(--color-gold)]/30 transition-colors disabled:opacity-50"
																	title="Force next spin to #{item.sim_ids[0]}"
																>
																	{#if forceLoading === item.sim_ids[0]}
																		...
																	{:else}
																		FORCE
																	{/if}
																</button>
															{/if}

															<!-- Force message -->
															{#if forceMessage?.simId === item.sim_ids[0]}
																<span class="text-xs font-mono text-emerald-400 ml-1">{forceMessage.message}</span>
															{/if}
														{:else}
															<span class="text-xs text-slate-500">-</span>
														{/if}
													</div>
												</td>
											</tr>
										{/each}
									</tbody>
								</table>

								<!-- Sentinel for lazy loading -->
								{#if hasMore}
									<div
										use:observeSentinel={{ key, totalItems: items.length }}
										class="py-3 text-center text-xs font-mono text-[var(--color-mist)]"
									>
										Loading more... ({visibleCount} / {items.length})
									</div>
								{/if}
							</div>
						</div>
					{/if}
				</div>
			{/each}
		</div>
	{/if}
</div>
