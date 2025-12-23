<script lang="ts">
	import { api, type DistributionItem, type PayoutBucket, type LGSSessionSummary, type BucketDistributionResponse } from '$lib/api';

	interface Props {
		buckets: PayoutBucket[];
		mode: string;
		onLook?: (simId: number) => void;
	}

	let { buckets, mode, onLook }: Props = $props();

	const ITEMS_PER_PAGE = 100;

	let sessions = $state<LGSSessionSummary[]>([]);
	let selectedSession = $state<string>('');
	let forceLoading = $state<number | null>(null);
	let forceMessage = $state<{ simId: number; message: string } | null>(null);

	// Track which bucket is expanded (only one at a time)
	let expandedBucket = $state<string | null>(null);

	// Track bucket data loaded from API
	let bucketData = $state<Record<string, {
		items: DistributionItem[];
		total: number;
		loading: boolean;
		hasMore: boolean;
		error: string | null;
	}>>({});

	// Sort buckets by range_end descending (biggest first)
	let sortedBuckets = $derived(() => {
		return [...buckets].sort((a, b) => b.range_end - a.range_end);
	});

	// Get bucket key
	function getBucketKey(bucket: PayoutBucket): string {
		return `${bucket.range_start}-${bucket.range_end}`;
	}

	// Set first bucket as expanded by default (wincap)
	$effect(() => {
		const sorted = sortedBuckets();
		if (sorted.length > 0 && expandedBucket === null) {
			const key = getBucketKey(sorted[0]);
			expandedBucket = key;
			loadBucketData(sorted[0], 0);
		}
	});

	// Load bucket data from API
	async function loadBucketData(bucket: PayoutBucket, offset: number) {
		const key = getBucketKey(bucket);

		// Initialize if needed
		if (!bucketData[key]) {
			bucketData[key] = {
				items: [],
				total: 0,
				loading: true,
				hasMore: false,
				error: null
			};
		} else if (offset === 0) {
			// Reset for fresh load
			bucketData[key].items = [];
			bucketData[key].loading = true;
			bucketData[key].error = null;
		} else {
			bucketData[key].loading = true;
		}

		try {
			const response = await api.getModeBucketDistribution(
				mode,
				bucket.range_start,
				bucket.range_end,
				offset,
				ITEMS_PER_PAGE
			);

			bucketData[key] = {
				items: offset === 0 ? response.items : [...bucketData[key].items, ...response.items],
				total: response.total,
				loading: false,
				hasMore: response.has_more,
				error: null
			};
		} catch (e) {
			bucketData[key] = {
				...bucketData[key],
				loading: false,
				error: e instanceof Error ? e.message : 'Failed to load'
			};
		}
	}

	function toggleBucket(bucket: PayoutBucket) {
		const key = getBucketKey(bucket);

		if (expandedBucket === key) {
			expandedBucket = null;
		} else {
			expandedBucket = key;
			// Load data if not already loaded
			if (!bucketData[key] || bucketData[key].items.length === 0) {
				loadBucketData(bucket, 0);
			}
		}
	}

	// Load more items for a bucket
	function loadMore(bucket: PayoutBucket) {
		const key = getBucketKey(bucket);
		const data = bucketData[key];
		if (data && !data.loading && data.hasMore) {
			loadBucketData(bucket, data.items.length);
		}
	}

	// Intersection observer for lazy loading
	function observeSentinel(node: HTMLElement, bucket: PayoutBucket) {
		const observer = new IntersectionObserver(
			(entries) => {
				if (entries[0].isIntersecting) {
					loadMore(bucket);
				}
			},
			{ rootMargin: '100px' }
		);

		observer.observe(node);

		return {
			destroy() {
				observer.disconnect();
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

	// Calculate total unique payouts from buckets
	let totalPayouts = $derived(() => {
		return buckets.reduce((sum, b) => sum + b.count, 0);
	});
</script>

<div>
	<div class="flex items-center gap-3 mb-6 flex-wrap">
		<div class="w-1 h-5 bg-[var(--color-white)] rounded-full"></div>
		<h3 class="font-display text-lg text-[var(--color-light)] tracking-wider">DISTRIBUTION</h3>
		<span class="text-xs font-mono text-[var(--color-mist)]">({totalPayouts().toLocaleString()} books)</span>

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

	{#if buckets.length === 0}
		<div class="py-8 text-center text-slate-500">No data</div>
	{:else}
		<div class="space-y-2">
			{#each sortedBuckets() as bucket}
				{@const key = getBucketKey(bucket)}
				{@const isExpanded = expandedBucket === key}
				{@const data = bucketData[key]}
				<div class="rounded-xl border border-slate-700/50 overflow-hidden bg-slate-800/30">
					<!-- Accordion Header -->
					<button
						class="w-full flex items-center gap-4 px-4 py-3 hover:bg-slate-700/30 transition-colors"
						onclick={() => toggleBucket(bucket)}
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
							{#if data?.loading && data.items.length === 0}
								<!-- Initial loading -->
								<div class="py-8 text-center">
									<div class="inline-block w-6 h-6 border-2 border-[var(--color-cyan)] border-t-transparent rounded-full animate-spin"></div>
									<p class="text-xs font-mono text-[var(--color-mist)] mt-2">Loading...</p>
								</div>
							{:else if data?.error}
								<!-- Error -->
								<div class="py-8 text-center text-red-400 text-sm font-mono">{data.error}</div>
							{:else if data?.items.length === 0}
								<!-- No items -->
								<div class="py-8 text-center text-slate-500 text-sm">No payouts in this range</div>
							{:else if data}
								<!-- Items table -->
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
											{#each data.items as item, i}
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
									{#if data.hasMore}
										<div
											use:observeSentinel={bucket}
											class="py-3 text-center text-xs font-mono text-[var(--color-mist)]"
										>
											{#if data.loading}
												<span class="inline-block w-4 h-4 border-2 border-[var(--color-cyan)] border-t-transparent rounded-full animate-spin mr-2"></span>
											{/if}
											Loading more... ({data.items.length} / {data.total})
										</div>
									{/if}
								</div>
							{/if}
						</div>
					{/if}
				</div>
			{/each}
		</div>
	{/if}
</div>
