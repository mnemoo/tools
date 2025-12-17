<script lang="ts">
	import { api, type ModeSummary, type LGSBatchPlayResponse } from '$lib/api';

	interface Props {
		open: boolean;
		modes: ModeSummary[];
		sessionID: string;
		balance: number; // Current session balance in API units
		onClose: () => void;
	}

	let { open, modes, sessionID, balance, onClose }: Props = $props();

	// Form state
	let selectedMode = $state<string>('');
	let spins = $state<number>(1000);
	let betAmount = $state<number>(1000000); // $1 in API units

	// Calculate required balance (must be after state declarations)
	let requiredBalance = $derived(spins * betAmount);
	let hasEnoughBalance = $derived(balance >= requiredBalance);
	let maxAffordableSpins = $derived(Math.floor(balance / betAmount));

	// Results state
	let loading = $state(false);
	let error = $state<string | null>(null);
	let result = $state<LGSBatchPlayResponse | null>(null);

	// Preset spin counts
	const spinPresets = [100, 500, 1000, 5000, 10000];

	async function runSimulation() {
		if (!selectedMode || !sessionID) return;

		loading = true;
		error = null;
		result = null;

		try {
			const response = await api.lgsBatchPlay({
				sessionID,
				mode: selectedMode,
				amount: betAmount,
				spins,
				currency: 'USD'
			});
			result = response;
		} catch (e) {
			error = e instanceof Error ? e.message : 'Simulation failed';
		} finally {
			loading = false;
		}
	}

	function formatMoney(amount: number): string {
		return (amount / 1000000).toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 });
	}

	function formatPercent(value: number): string {
		return (value * 100).toFixed(2) + '%';
	}

	function getRTPColor(rtp: number): string {
		if (rtp >= 0.97) return 'text-emerald-400';
		if (rtp >= 0.94) return 'text-yellow-400';
		return 'text-red-400';
	}

	function close() {
		result = null;
		error = null;
		onClose();
	}

	// Auto-select first mode
	$effect(() => {
		if (modes.length > 0 && !selectedMode) {
			selectedMode = modes[0].mode;
		}
	});
</script>

{#if open}
	<div class="fixed inset-0 z-50 flex items-center justify-center">
		<!-- Backdrop -->
		<div
			class="absolute inset-0 bg-black/70 backdrop-blur-sm"
			onclick={close}
			onkeydown={(e) => e.key === 'Escape' && close()}
			role="button"
			tabindex="-1"
		></div>

		<!-- Modal -->
		<div class="relative bg-[var(--color-graphite)] rounded-2xl shadow-2xl border border-white/10 w-full max-w-2xl mx-4 max-h-[90vh] overflow-hidden flex flex-col">
			<!-- Header -->
			<div class="flex items-center justify-between p-6 border-b border-white/10">
				<div class="flex items-center gap-3">
					<div class="w-10 h-10 rounded-xl bg-[var(--color-cyan)]/20 flex items-center justify-center">
						<svg class="w-5 h-5 text-[var(--color-cyan)]" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
							<path stroke-linecap="round" stroke-linejoin="round" d="M13 10V3L4 14h7v7l9-11h-7z" />
						</svg>
					</div>
					<div>
						<h2 class="font-display text-xl text-[var(--color-light)] tracking-wider">BATCH SIMULATION</h2>
						<p class="text-xs font-mono text-[var(--color-mist)]">Run multiple spins at once</p>
					</div>
				</div>
				<button
					onclick={close}
					class="p-2 rounded-lg hover:bg-white/10 transition-colors text-[var(--color-mist)] hover:text-[var(--color-light)]"
				>
					<svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
						<path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" />
					</svg>
				</button>
			</div>

			<!-- Content -->
			<div class="p-6 overflow-y-auto flex-1">
				{#if !result}
					<!-- Configuration Form -->
					<div class="space-y-6">
						<!-- Mode Selection -->
						<div>
							<label class="block text-xs font-mono text-[var(--color-mist)] mb-2 uppercase tracking-wider">Game Mode</label>
							<div class="grid grid-cols-2 gap-2">
								{#each modes as mode}
									<button
										onclick={() => selectedMode = mode.mode}
										class="px-4 py-3 rounded-xl text-left transition-all border {selectedMode === mode.mode
											? 'bg-[var(--color-cyan)]/20 border-[var(--color-cyan)] text-[var(--color-light)]'
											: 'bg-[var(--color-slate)] border-transparent text-[var(--color-mist)] hover:bg-[var(--color-slate)]/80'}"
									>
										<div class="font-mono font-semibold text-sm">{mode.mode}</div>
										<div class="text-xs opacity-70">RTP: {(mode.rtp * 100).toFixed(2)}%</div>
									</button>
								{/each}
							</div>
						</div>

						<!-- Spin Count -->
						<div>
							<label class="block text-xs font-mono text-[var(--color-mist)] mb-2 uppercase tracking-wider">Number of Spins</label>
							<div class="flex flex-wrap gap-2 mb-3">
								{#each spinPresets as preset}
									<button
										onclick={() => spins = preset}
										class="px-4 py-2 rounded-lg font-mono text-sm transition-colors {spins === preset
											? 'bg-[var(--color-cyan)] text-[var(--color-void)]'
											: 'bg-[var(--color-slate)] text-[var(--color-mist)] hover:bg-[var(--color-slate)]/80'}"
									>
										{preset.toLocaleString()}
									</button>
								{/each}
							</div>
							<input
								type="number"
								bind:value={spins}
								min="1"
								max="100000"
								class="w-full bg-[var(--color-slate)] border border-white/10 rounded-lg px-4 py-3 text-sm font-mono text-[var(--color-light)] focus:outline-none focus:border-[var(--color-cyan)]"
							/>
						</div>

						<!-- Bet Amount -->
						<div>
							<label class="block text-xs font-mono text-[var(--color-mist)] mb-2 uppercase tracking-wider">Bet Amount (per spin)</label>
							<div class="flex items-center gap-2">
								<span class="text-[var(--color-mist)] font-mono">$</span>
								<input
									type="number"
									value={betAmount / 1000000}
									onchange={(e) => betAmount = parseFloat(e.currentTarget.value) * 1000000}
									min="0.01"
									step="0.01"
									class="flex-1 bg-[var(--color-slate)] border border-white/10 rounded-lg px-4 py-3 text-sm font-mono text-[var(--color-light)] focus:outline-none focus:border-[var(--color-cyan)]"
								/>
							</div>
						</div>

						<!-- Session Info & Balance Check -->
						<div class="grid grid-cols-2 gap-4">
							<div class="bg-[var(--color-slate)]/50 rounded-xl p-4">
								<div class="text-xs font-mono text-[var(--color-mist)] mb-1">SESSION</div>
								<div class="font-mono text-[var(--color-light)]">{sessionID}</div>
							</div>
							<div class="bg-[var(--color-slate)]/50 rounded-xl p-4">
								<div class="text-xs font-mono text-[var(--color-mist)] mb-1">AVAILABLE BALANCE</div>
								<div class="font-mono text-[var(--color-light)]">${formatMoney(balance)}</div>
							</div>
						</div>

						<!-- Cost Estimate -->
						<div class="bg-[var(--color-slate)]/50 rounded-xl p-4">
							<div class="flex justify-between items-center">
								<div>
									<div class="text-xs font-mono text-[var(--color-mist)] mb-1">REQUIRED BALANCE</div>
									<div class="font-mono {hasEnoughBalance ? 'text-[var(--color-light)]' : 'text-red-400'}">
										${formatMoney(requiredBalance)}
									</div>
								</div>
								{#if !hasEnoughBalance}
									<div class="text-right">
										<div class="text-xs font-mono text-red-400 mb-1">INSUFFICIENT FUNDS</div>
										<button
											onclick={() => spins = maxAffordableSpins}
											class="text-xs font-mono text-[var(--color-cyan)] hover:underline"
										>
											Use max: {maxAffordableSpins.toLocaleString()} spins
										</button>
									</div>
								{/if}
							</div>
						</div>
					</div>
				{:else}
					<!-- Results -->
					<div class="space-y-6">
						<!-- Summary Stats -->
						<div class="grid grid-cols-2 md:grid-cols-4 gap-4">
							<div class="bg-[var(--color-slate)]/50 rounded-xl p-4">
								<div class="text-xs font-mono text-[var(--color-mist)] mb-1">SPINS</div>
								<div class="text-2xl font-mono font-bold text-[var(--color-light)]">
									{result.spins.toLocaleString()}
								</div>
							</div>
							<div class="bg-[var(--color-slate)]/50 rounded-xl p-4">
								<div class="text-xs font-mono text-[var(--color-mist)] mb-1">RTP</div>
								<div class="text-2xl font-mono font-bold {getRTPColor(result.rtp)}">
									{formatPercent(result.rtp)}
								</div>
							</div>
							<div class="bg-[var(--color-slate)]/50 rounded-xl p-4">
								<div class="text-xs font-mono text-[var(--color-mist)] mb-1">HIT RATE</div>
								<div class="text-2xl font-mono font-bold text-[var(--color-light)]">
									{formatPercent(result.hitRate)}
								</div>
							</div>
							<div class="bg-[var(--color-slate)]/50 rounded-xl p-4">
								<div class="text-xs font-mono text-[var(--color-mist)] mb-1">MAX WIN</div>
								<div class="text-2xl font-mono font-bold text-[var(--color-gold)]">
									{result.maxWin.toFixed(1)}x
								</div>
							</div>
						</div>

						<!-- Financial Summary -->
						<div class="grid grid-cols-2 gap-4">
							<div class="bg-[var(--color-slate)]/50 rounded-xl p-4">
								<div class="text-xs font-mono text-[var(--color-mist)] mb-1">TOTAL WAGERED</div>
								<div class="text-xl font-mono font-bold text-[var(--color-light)]">
									${formatMoney(result.totalWagered)}
								</div>
							</div>
							<div class="bg-[var(--color-slate)]/50 rounded-xl p-4">
								<div class="text-xs font-mono text-[var(--color-mist)] mb-1">TOTAL WON</div>
								<div class="text-xl font-mono font-bold {result.totalWon >= result.totalWagered ? 'text-emerald-400' : 'text-red-400'}">
									${formatMoney(result.totalWon)}
								</div>
							</div>
						</div>

						<!-- Win Stats -->
						<div class="grid grid-cols-3 gap-4">
							<div class="bg-[var(--color-slate)]/50 rounded-xl p-4 text-center">
								<div class="text-xs font-mono text-[var(--color-mist)] mb-1">HITS</div>
								<div class="text-xl font-mono font-bold text-[var(--color-light)]">
									{result.hitCount.toLocaleString()}
								</div>
							</div>
							<div class="bg-[var(--color-slate)]/50 rounded-xl p-4 text-center">
								<div class="text-xs font-mono text-[var(--color-mist)] mb-1">BIG WINS (10x+)</div>
								<div class="text-xl font-mono font-bold text-[var(--color-cyan)]">
									{result.bigWins}
								</div>
							</div>
							<div class="bg-[var(--color-slate)]/50 rounded-xl p-4 text-center">
								<div class="text-xs font-mono text-[var(--color-mist)] mb-1">MEGA WINS (50x+)</div>
								<div class="text-xl font-mono font-bold text-[var(--color-gold)]">
									{result.megaWins}
								</div>
							</div>
						</div>

						<!-- Profit/Loss -->
						<div class="bg-gradient-to-r {result.totalWon >= result.totalWagered ? 'from-emerald-500/20 to-emerald-500/5' : 'from-red-500/20 to-red-500/5'} rounded-xl p-6 text-center">
							<div class="text-xs font-mono text-[var(--color-mist)] mb-2">NET RESULT</div>
							<div class="text-3xl font-mono font-bold {result.totalWon >= result.totalWagered ? 'text-emerald-400' : 'text-red-400'}">
								{result.totalWon >= result.totalWagered ? '+' : ''}${formatMoney(result.totalWon - result.totalWagered)}
							</div>
						</div>

						<!-- Duration -->
						<div class="text-center text-xs font-mono text-[var(--color-mist)]">
							Completed in {result.durationMs}ms ({(result.spins / (result.durationMs / 1000)).toFixed(0)} spins/sec)
						</div>
					</div>
				{/if}

				{#if error}
					<div class="mt-4 p-4 rounded-xl bg-red-500/20 border border-red-500/30 text-red-400 text-sm font-mono">
						{error}
					</div>
				{/if}
			</div>

			<!-- Footer -->
			<div class="p-6 border-t border-white/10 flex justify-end gap-3">
				{#if result}
					<button
						onclick={() => result = null}
						class="px-6 py-3 rounded-xl font-mono font-semibold text-sm bg-[var(--color-slate)] text-[var(--color-light)] hover:bg-[var(--color-slate)]/80 transition-colors"
					>
						RUN AGAIN
					</button>
				{/if}
				<button
					onclick={result ? close : runSimulation}
					disabled={loading || (!result && (!selectedMode || !hasEnoughBalance || spins < 1))}
					class="px-6 py-3 rounded-xl font-mono font-semibold text-sm transition-colors disabled:opacity-50 disabled:cursor-not-allowed
						{result
							? 'bg-[var(--color-slate)] text-[var(--color-light)] hover:bg-[var(--color-slate)]/80'
							: 'bg-[var(--color-cyan)] text-[var(--color-void)] hover:bg-[var(--color-cyan)]/90'}"
				>
					{#if loading}
						<span class="flex items-center gap-2">
							<svg class="w-4 h-4 animate-spin" fill="none" viewBox="0 0 24 24">
								<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
								<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
							</svg>
							SIMULATING...
						</span>
					{:else if result}
						CLOSE
					{:else}
						RUN SIMULATION
					{/if}
				</button>
			</div>
		</div>
	</div>
{/if}
