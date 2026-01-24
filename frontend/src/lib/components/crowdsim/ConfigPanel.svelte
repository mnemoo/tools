<script lang="ts">
	import type { CrowdSimConfig, CrowdSimPresetInfo } from '$lib/api/types';
	import { _ } from '$lib/i18n';

	let {
		config = $bindable<CrowdSimConfig>(),
		presets = [] as CrowdSimPresetInfo[],
		onRun,
		disabled = false
	}: {
		config: CrowdSimConfig;
		presets?: CrowdSimPresetInfo[];
		onRun: () => void;
		disabled?: boolean;
	} = $props();

	function applyPreset(preset: CrowdSimPresetInfo) {
		config = { ...preset.config };
	}
</script>

<div class="rounded-2xl bg-[var(--color-graphite)]/50 border border-white/[0.03] overflow-hidden">
	<!-- Header -->
	<div class="px-5 py-4 border-b border-white/[0.03] flex items-center justify-between">
		<div class="flex items-center gap-3">
			<div class="w-8 h-8 rounded-lg bg-[var(--color-cyan)]/10 flex items-center justify-center">
				<svg class="w-5 h-5 text-[var(--color-cyan)]" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
					<path stroke-linecap="round" stroke-linejoin="round" d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
					<path stroke-linecap="round" stroke-linejoin="round" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
				</svg>
			</div>
			<span class="font-mono text-base text-[var(--color-light)]">{$_('crowdsim.configuration')}</span>
		</div>

		<!-- Presets -->
		{#if presets.length > 0}
			<div class="flex items-center gap-2">
				<span class="text-sm font-mono text-[var(--color-mist)]">{$_('crowdsim.presets')}</span>
				{#each presets as preset}
					<button
						class="px-3 py-1.5 rounded-lg text-sm font-mono bg-[var(--color-slate)]/30 text-[var(--color-mist)] hover:bg-[var(--color-cyan)]/20 hover:text-[var(--color-cyan)] transition-all disabled:opacity-40 disabled:cursor-not-allowed"
						onclick={() => applyPreset(preset)}
						{disabled}
						title={preset.description}
					>
						{preset.name.toUpperCase()}
					</button>
				{/each}
			</div>
		{/if}
	</div>

	<!-- Configuration Grid -->
	<div class="p-5">
		<div class="grid gap-5 sm:grid-cols-2 lg:grid-cols-3">
			<!-- Player Count -->
			<div class="space-y-2">
				<div class="flex items-center justify-between">
					<label class="text-sm font-mono text-[var(--color-mist)]" for="player_count">{$_('crowdsim.players')}</label>
					<span class="text-base font-mono text-[var(--color-cyan)]">{config.player_count.toLocaleString()}</span>
				</div>
				<div class="relative">
					<input
						type="range"
						id="player_count"
						min="100"
						max="10000"
						step="100"
						bind:value={config.player_count}
						class="w-full h-2 bg-[var(--color-slate)] rounded-full appearance-none cursor-pointer
							   [&::-webkit-slider-thumb]:appearance-none [&::-webkit-slider-thumb]:w-5 [&::-webkit-slider-thumb]:h-5
							   [&::-webkit-slider-thumb]:rounded-full [&::-webkit-slider-thumb]:bg-[var(--color-cyan)]
							   [&::-webkit-slider-thumb]:cursor-pointer [&::-webkit-slider-thumb]:shadow-[0_0_10px_var(--color-cyan)]
							   disabled:opacity-40"
						{disabled}
					/>
				</div>
			</div>

			<!-- Spins Per Session -->
			<div class="space-y-2">
				<label class="text-sm font-mono text-[var(--color-mist)]" for="spins_per_session">{$_('crowdsim.spinsPerSession')}</label>
				<input
					type="number"
					id="spins_per_session"
					min="1"
					max="1000"
					step="1"
					bind:value={config.spins_per_session}
					onchange={(e) => {
						const val = Math.max(1, Math.min(1000, Number(e.currentTarget.value) || 1));
						config.spins_per_session = val;
						e.currentTarget.value = String(val);
					}}
					class="w-full h-10 px-3 rounded-lg bg-[var(--color-onyx)] border border-white/[0.05] text-[var(--color-light)] font-mono text-base
						   focus:outline-none focus:border-[var(--color-cyan)]/50 focus:ring-1 focus:ring-[var(--color-cyan)]/20
						   disabled:opacity-40"
					{disabled}
				/>
			</div>

			<!-- Initial Balance -->
			<div class="space-y-2">
				<label class="text-sm font-mono text-[var(--color-mist)]" for="initial_balance">{$_('crowdsim.initialBalance')}</label>
				<input
					type="number"
					id="initial_balance"
					min="10"
					max="10000"
					step="10"
					bind:value={config.initial_balance}
					class="w-full h-10 px-3 rounded-lg bg-[var(--color-onyx)] border border-white/[0.05] text-[var(--color-light)] font-mono text-base
						   focus:outline-none focus:border-[var(--color-cyan)]/50 focus:ring-1 focus:ring-[var(--color-cyan)]/20
						   disabled:opacity-40"
					{disabled}
				/>
			</div>

			<!-- Big Win Threshold -->
			<div class="space-y-2">
				<label class="text-sm font-mono text-[var(--color-mist)]" for="big_win_threshold">{$_('crowdsim.bigWinThreshold')}</label>
				<div class="relative">
					<input
						type="number"
						id="big_win_threshold"
						min="5"
						max="100"
						step="1"
						bind:value={config.big_win_threshold}
						class="w-full h-10 px-3 pr-8 rounded-lg bg-[var(--color-onyx)] border border-white/[0.05] text-[var(--color-light)] font-mono text-base
							   focus:outline-none focus:border-[var(--color-cyan)]/50 focus:ring-1 focus:ring-[var(--color-cyan)]/20
							   disabled:opacity-40"
						{disabled}
					/>
					<span class="absolute right-3 top-1/2 -translate-y-1/2 text-sm font-mono text-[var(--color-mist)]">x</span>
				</div>
			</div>

			<!-- Danger Threshold -->
			<div class="space-y-2">
				<label class="text-sm font-mono text-[var(--color-mist)]" for="danger_threshold">{$_('crowdsim.dangerThreshold')}</label>
				<div class="relative">
					<input
						type="number"
						id="danger_threshold"
						min="1"
						max="50"
						step="1"
						value={config.danger_threshold * 100}
						oninput={(e) => (config.danger_threshold = Number(e.currentTarget.value) / 100)}
						class="w-full h-10 px-3 pr-8 rounded-lg bg-[var(--color-onyx)] border border-white/[0.05] text-[var(--color-light)] font-mono text-base
							   focus:outline-none focus:border-[var(--color-cyan)]/50 focus:ring-1 focus:ring-[var(--color-cyan)]/20
							   disabled:opacity-40"
						{disabled}
					/>
					<span class="absolute right-3 top-1/2 -translate-y-1/2 text-sm font-mono text-[var(--color-mist)]">%</span>
				</div>
			</div>

			<!-- Workers -->
			<div class="space-y-2">
				<label class="text-sm font-mono text-[var(--color-mist)]" for="parallel_workers">{$_('crowdsim.parallelWorkers')}</label>
				<input
					type="number"
					id="parallel_workers"
					min="1"
					max="32"
					bind:value={config.parallel_workers}
					class="w-full h-10 px-3 rounded-lg bg-[var(--color-onyx)] border border-white/[0.05] text-[var(--color-light)] font-mono text-base
						   focus:outline-none focus:border-[var(--color-cyan)]/50 focus:ring-1 focus:ring-[var(--color-cyan)]/20
						   disabled:opacity-40"
					{disabled}
				/>
			</div>
		</div>

		<!-- Advanced Options & Run Button -->
		<div class="mt-5 pt-5 border-t border-white/[0.03] flex flex-wrap items-center gap-6">
			<!-- Checkboxes -->
			<label class="flex items-center gap-2 cursor-pointer group">
				<div class="relative w-5 h-5">
					<input
						type="checkbox"
						bind:checked={config.use_crypto_rng}
						class="peer sr-only"
						{disabled}
					/>
					<div class="w-5 h-5 rounded bg-[var(--color-onyx)] border border-white/[0.1] peer-checked:bg-[var(--color-cyan)]/20 peer-checked:border-[var(--color-cyan)]/50 transition-all"></div>
					<svg class="absolute inset-0 w-5 h-5 text-[var(--color-cyan)] opacity-0 peer-checked:opacity-100 transition-opacity p-1" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="3">
						<path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7" />
					</svg>
				</div>
				<span class="text-sm font-mono text-[var(--color-mist)] group-hover:text-[var(--color-light)] transition-colors">{$_('crowdsim.cryptoRng')}</span>
			</label>

			<label class="flex items-center gap-2 cursor-pointer group">
				<div class="relative w-5 h-5">
					<input
						type="checkbox"
						bind:checked={config.streaming_mode}
						class="peer sr-only"
						{disabled}
					/>
					<div class="w-5 h-5 rounded bg-[var(--color-onyx)] border border-white/[0.1] peer-checked:bg-[var(--color-cyan)]/20 peer-checked:border-[var(--color-cyan)]/50 transition-all"></div>
					<svg class="absolute inset-0 w-5 h-5 text-[var(--color-cyan)] opacity-0 peer-checked:opacity-100 transition-opacity p-1" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="3">
						<path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7" />
					</svg>
				</div>
				<span class="text-sm font-mono text-[var(--color-mist)] group-hover:text-[var(--color-light)] transition-colors">{$_('crowdsim.ecoMode')}</span>
			</label>

			<!-- Run Button -->
			<button
				class="ml-auto flex items-center gap-2 px-6 py-3 rounded-xl font-mono text-base font-semibold
					   bg-gradient-to-r from-[var(--color-violet)] to-[var(--color-cyan)] text-[var(--color-void)]
					   hover:shadow-[0_0_20px_rgba(139,92,246,0.3)] transition-all
					   disabled:opacity-40 disabled:cursor-not-allowed disabled:hover:shadow-none"
				onclick={onRun}
				{disabled}
			>
				<svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
					<path stroke-linecap="round" stroke-linejoin="round" d="M14.752 11.168l-3.197-2.132A1 1 0 0010 9.87v4.263a1 1 0 001.555.832l3.197-2.132a1 1 0 000-1.664z" />
					<path stroke-linecap="round" stroke-linejoin="round" d="M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
				</svg>
				{$_('crowdsim.runSimulation')}
			</button>
		</div>
	</div>
</div>
