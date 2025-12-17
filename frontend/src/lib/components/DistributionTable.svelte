<script lang="ts">
	import { api, type DistributionItem, type LGSSessionSummary } from '$lib/api';

	interface Props {
		distribution: DistributionItem[];
		mode: string;
		maxRows?: number;
		onLook?: (simId: number) => void;
	}

	let { distribution, mode, maxRows = 50, onLook }: Props = $props();

	let showAll = $state(false);
	let sessions = $state<LGSSessionSummary[]>([]);
	let selectedSession = $state<string>('');
	let forceLoading = $state<number | null>(null);
	let forceMessage = $state<{ simId: number; message: string } | null>(null);

	let displayedItems = $derived(showAll ? distribution : distribution.slice(0, maxRows));
	let hasMore = $derived(distribution.length > maxRows);

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

		{#if hasMore}
			<button
				class="text-xs text-blue-400 hover:text-blue-300 transition-colors flex items-center gap-1"
				onclick={() => (showAll = !showAll)}
			>
				{#if showAll}
					<svg class="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
						<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 15l7-7 7 7" />
					</svg>
					Less
				{:else}
					<svg class="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
						<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
					</svg>
					All
				{/if}
			</button>
		{/if}
	</div>

	{#if distribution.length === 0}
		<div class="py-8 text-center text-slate-500">No data</div>
	{:else}
		<div class="max-h-[600px] overflow-auto rounded-xl border border-slate-700/50">
			<table class="w-full">
				<thead class="sticky top-0 bg-slate-800/95 backdrop-blur-sm">
					<tr class="text-left text-xs uppercase text-slate-500 tracking-wider">
						<th class="px-4 py-3 font-medium">Payout</th>
						<th class="px-4 py-3 text-right font-medium">Books</th>
						<th class="px-4 py-3 text-right font-medium">Weight</th>
						<th class="px-4 py-3 text-right font-medium">Odds</th>
						<th class="px-4 py-3 text-center font-medium">Actions</th>
					</tr>
				</thead>
				<tbody class="text-sm">
					{#each displayedItems as item, i}
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
		</div>
	{/if}
</div>
