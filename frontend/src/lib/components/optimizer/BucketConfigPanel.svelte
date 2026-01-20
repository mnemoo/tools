<script lang="ts">
	import { api } from '$lib/api/client';
	import { onMount } from 'svelte';
	import type { ModeInfo } from '$lib/api/types';

	type BucketConfig = {
		name: string;
		min_payout: number;
		max_payout: number;
		type: 'frequency' | 'rtp_percent' | 'auto';
		frequency?: number;
		rtp_percent?: number;
		auto_exponent?: number;
	};

	type ShortBucket = [number, number, number, number]; // [min, max, type(0/1/2), value]
	type ShortConfig = { r: number; f: 'absolute' | 'normalized'; b: ShortBucket[] };

	type BucketResult = {
		name: string;
		min_payout: number;
		max_payout: number;
		outcome_count: number;
		actual_probability: number;
		actual_frequency: number;
		rtp_contribution: number;
	};

	type ProfileConfig = {
		profile: string;
		profile_name: string;
		description: string;
		target_rtp: number;
		max_win: number;
		b64_config: string;
		stats: {
			total_buckets: number;
			avg_hit_rate: number;
			max_win_freq: number;
		};
	};

	let {
		mode,
		onOptimize,
		disabled = false
	}: {
		mode: string;
		onOptimize?: (result: unknown) => void;
		disabled?: boolean;
	} = $props();

	// Mode info for cost-aware display
	let modeInfo = $state<ModeInfo | null>(null);

	let targetRtp = $state(0.97);
	let buckets = $state<BucketConfig[]>([]);

	// Input format: whether the UI fields are absolute multipliers (abs) or normalized (units where 1.0 == mode cost)
	let inputFormat = $state<'absolute' | 'normalized'>('absolute');

	let isLoading = $state(false);
	let result = $state<{
		original_rtp: number;
		final_rtp: number;
		converged: boolean;
		bucket_results: BucketResult[];
		loss_result: BucketResult | null;
		warnings?: string[];
	} | null>(null);
	let error = $state<string | null>(null);
	let saveToFile = $state(false);
	let createBackup = $state(true);
	let configCode = $state('');
	let showConfigInput = $state(false);
	let initialized = $state(false);

	// Profile generator state
	let showProfiles = $state(false);
	let profileConfigs = $state<ProfileConfig[]>([]);
	let loadingProfiles = $state(false);
	let selectedProfile = $state<string | null>(null);

	const STORAGE_KEY = $derived(`lut_bucket_${mode}`);

	// Type mapping for short config
	const TYPE_MAP: Record<string, number> = { frequency: 0, rtp_percent: 1, auto: 2 };
	const TYPE_REVERSE: ('frequency' | 'rtp_percent' | 'auto')[] = ['frequency', 'rtp_percent', 'auto'];

	// Convert config to short b64 format
	function toShortConfig(): string {
		const short: ShortConfig = {
			r: Math.round(targetRtp * 100),
			f: inputFormat,
			b: buckets.map((b) => {
				const t = TYPE_MAP[b.type];
				const v = b.type === 'frequency' ? (b.frequency ?? 10) : b.type === 'rtp_percent' ? (b.rtp_percent ?? 1) : (b.auto_exponent ?? 1);
				return [b.min_payout, b.max_payout, t, v];
			})
		};
		return btoa(JSON.stringify(short));
	}

	// Parse short b64 config
	function fromShortConfig(code: string): boolean {
		try {
			const json = atob(code.trim());
			const short: ShortConfig = JSON.parse(json);
			if (!short.r || !Array.isArray(short.b)) return false;

			// Restore input format if present (backwards compatible)
			inputFormat = (short as any).f ?? 'absolute';

			targetRtp = short.r / 100;
			buckets = short.b.map(([min, max, t, v], i) => {
				const type = TYPE_REVERSE[t] ?? 'frequency';
				return {
					name: `b${i}`,
					min_payout: min,
					max_payout: max,
					type,
					frequency: type === 'frequency' ? v : undefined,
					rtp_percent: type === 'rtp_percent' ? v : undefined,
					auto_exponent: type === 'auto' ? v : undefined
				};
			});
			return true;
		} catch {
			return false;
		}
	}

	// Save to localStorage
	function saveToStorage() {
		if (!initialized) return;
		try {
			localStorage.setItem(STORAGE_KEY, toShortConfig());
		} catch {}
	}

	// Load from localStorage
	function loadFromStorage(): boolean {
		try {
			const saved = localStorage.getItem(STORAGE_KEY);
			if (saved) return fromShortConfig(saved);
		} catch {}
		return false;
	}

	// Watch for changes and save
	$effect(() => {
		if (initialized && buckets.length > 0) {
			// Access reactive values to track them
			const _ = [targetRtp, JSON.stringify(buckets)];
			saveToStorage();
		}
	});

	// Load mode info (for cost/bonus display)
	async function loadModeInfo() {
		try {
			const response = await api.suggestBuckets(mode, targetRtp);
			if (response.mode_info) {
				modeInfo = response.mode_info;
			}
		} catch {
			// Ignore errors for mode info
		}
	}

	// Initialize on mount
	onMount(() => {
		loadModeInfo(); // Always load mode info
		if (!loadFromStorage()) {
			loadSuggestedBuckets();
		}
		initialized = true;
	});

	// Convert buckets between absolute and normalized using mode cost
	function canConvert(): boolean {
		return !!modeInfo?.cost && modeInfo.cost > 0;
	}

	function convertBucketsFormat() {
		if (!canConvert()) {
			// Try to refresh mode info, then re-check
			loadModeInfo();
			if (!canConvert()) {
				error = 'Mode cost unknown — cannot convert formats';
				return;
			}
		}

		const cost = modeInfo!.cost;
		if (inputFormat === 'absolute') {
			// Convert absolute -> normalized (divide by cost)
			buckets = buckets.map(b => ({ ...b, min_payout: +(b.min_payout / cost), max_payout: +(b.max_payout / cost) }));
			inputFormat = 'normalized';
		} else {
			// Convert normalized -> absolute (multiply by cost)
			buckets = buckets.map(b => ({ ...b, min_payout: +(b.min_payout * cost), max_payout: +(b.max_payout * cost) }));
			inputFormat = 'absolute';
		}
		// Clear any prior errors
		error = null;
	}

	// Reload mode info when mode changes
	$effect(() => {
		const _ = mode; // Track mode changes
		if (initialized) {
			loadModeInfo();
		}
	});

	async function loadSuggestedBuckets() {
		try {
			const response = await api.suggestBuckets(mode, targetRtp);
			buckets = response.suggested_buckets;
			// Store mode info for cost-aware display
			if (response.mode_info) {
				modeInfo = response.mode_info;
			}
		} catch {
			buckets = getDefaultBuckets();
		}
	}

	function getDefaultBuckets(): BucketConfig[] {
		return [
			{ name: 'b0', min_payout: 0.01, max_payout: 1, type: 'frequency', frequency: 3 },
			{ name: 'b1', min_payout: 1, max_payout: 5, type: 'frequency', frequency: 6 },
			{ name: 'b2', min_payout: 5, max_payout: 20, type: 'frequency', frequency: 25 },
			{ name: 'b3', min_payout: 20, max_payout: 100, type: 'frequency', frequency: 100 },
			{ name: 'b4', min_payout: 100, max_payout: 1000, type: 'rtp_percent', rtp_percent: 5 },
			{ name: 'b5', min_payout: 1000, max_payout: 10000, type: 'auto', auto_exponent: 1 }
		];
	}

	function addBucket() {
		const last = buckets[buckets.length - 1];
		const min = last ? last.max_payout : 0;
		buckets = [...buckets, { name: `b${buckets.length}`, min_payout: min, max_payout: min * 10 || 100, type: 'frequency', frequency: 50 }];
	}

	function removeBucket(index: number) {
		buckets = buckets.filter((_, i) => i !== index);
	}

	function moveBucket(index: number, direction: -1 | 1) {
		const newIndex = index + direction;
		if (newIndex < 0 || newIndex >= buckets.length) return;
		const newBuckets = [...buckets];
		[newBuckets[index], newBuckets[newIndex]] = [newBuckets[newIndex], newBuckets[index]];
		buckets = newBuckets;
	}

	function setType(index: number, type: 'frequency' | 'rtp_percent' | 'auto') {
		buckets = buckets.map((b, i) => {
			if (i !== index) return b;
			return {
				...b,
				type,
				frequency: type === 'frequency' ? (b.frequency ?? 10) : undefined,
				rtp_percent: type === 'rtp_percent' ? (b.rtp_percent ?? 1) : undefined,
				auto_exponent: type === 'auto' ? (b.auto_exponent ?? 1) : undefined
			};
		});
	}

	async function runOptimization() {
		isLoading = true;
		error = null;
		result = null;

		try {
				// Generate names and ensure buckets are normalized for the backend.
				// Backend expects normalized ranges (units where 1.0 == mode cost).
				const bucketsWithNames = buckets.map((b, i) => {
					let min = b.min_payout;
					let max = b.max_payout;
					// If user entered absolute values, convert to normalized using mode cost.
					if (inputFormat === 'absolute' && modeInfo?.cost && modeInfo.cost > 0) {
						const cost = modeInfo.cost;
						min = +(min / cost);
						max = +(max / cost);
					}
					// If user selected 'normalized', we trust the inputs and send as-is.
					return { ...b, name: `bucket_${i}`, min_payout: min, max_payout: max };
				});

				const response = await api.bucketOptimize(mode, {
					target_rtp: targetRtp,
					buckets: bucketsWithNames,
					save_to_file: saveToFile,
					create_backup: createBackup
				});
			result = response;
			onOptimize?.(response);
		} catch (e) {
			error = e instanceof Error ? e.message : 'Optimization failed';
		} finally {
			isLoading = false;
		}
	}

	function copyConfig() {
		const code = toShortConfig();
		navigator.clipboard.writeText(code);
		configCode = code;
	}

	function pasteConfig() {
		if (configCode && fromShortConfig(configCode)) {
			showConfigInput = false;
			error = null;
		} else {
			error = 'Invalid config code';
		}
	}

	function formatFreq(f: number): string {
		if (f < 10) return f.toFixed(1);
		if (f < 1000) return Math.round(f).toString();
		if (f < 1e6) return (f / 1000).toFixed(1) + 'K';
		return (f / 1e6).toFixed(1) + 'M';
	}

	function formatPct(v: number): string {
		return v < 0.01 ? v.toFixed(4) + '%' : v < 1 ? v.toFixed(2) + '%' : v.toFixed(1) + '%';
	}

	// Profile generator functions
	async function loadProfiles() {
		if (loadingProfiles) return;
		loadingProfiles = true;
		error = null;
		try {
			const res = await fetch(`http://localhost:7754/api/optimizer/${encodeURIComponent(mode)}/generate-configs?target_rtp=${targetRtp}`);
			const data = await res.json();
			if (data.success && data.data?.configs) {
				profileConfigs = data.data.configs;
				showProfiles = true;
			} else {
				error = data.error || 'Failed to load profiles';
			}
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load profiles';
		} finally {
			loadingProfiles = false;
		}
	}

	function applyProfile(config: ProfileConfig) {
		if (fromShortConfig(config.b64_config)) {
			selectedProfile = config.profile;
			showProfiles = false;
			// Profiles are normalized by default; ensure the UI reflects that
			inputFormat = 'normalized';
			error = null;
		} else {
			error = 'Failed to apply profile config';
		}
	}

	function getProfileIcon(profile: string): string {
		switch (profile) {
			case 'low_volatility': return '🎯';
			case 'medium_volatility': return '⚖️';
			case 'high_volatility': return '🎢';
			default: return '⚙️';
		}
	}

	function getProfileColor(profile: string): string {
		switch (profile) {
			case 'low_volatility': return 'var(--color-emerald)';
			case 'medium_volatility': return 'var(--color-cyan)';
			case 'high_volatility': return 'var(--color-coral)';
			default: return 'var(--color-mist)';
		}
	}
</script>

<div class="space-y-4">
	<!-- Bonus Mode / Units Banner -->
{#if modeInfo?.is_bonus_mode}
	<div class="px-4 py-3 rounded-xl bg-violet-500/10 border border-violet-500/30">
		<div class="flex items-center gap-2 mb-2">
			<svg class="w-4 h-4 text-violet-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
				<path stroke-linecap="round" stroke-linejoin="round" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
			</svg>
			<span class="text-sm font-mono text-violet-400">BONUS MODE</span>
			<span class="px-2 py-0.5 text-xs font-mono bg-violet-500/20 text-violet-300 rounded">Cost: {modeInfo.cost}x</span>

			<!-- Input format selector + convert button -->
			<span class="ml-auto flex items-center gap-2">
				<span class="text-xs font-mono text-violet-300/80">Bucket ranges:</span>
				<select
					bind:value={inputFormat}
					class="bg-[var(--color-graphite)] border border-white/10 rounded px-2 py-1 text-xs font-mono text-[var(--color-light)] focus:outline-none"
					aria-label="Bucket range units"
					title="Choose how you want to type bucket Min/Max values"
				>
					<option value="absolute">Absolute (x)</option>
					<option value="normalized">Normalized (x / cost)</option>
				</select>

				<button
					class="p-1 rounded text-xs bg-[var(--color-slate)]/20 text-[var(--color-mist)] hover:bg-[var(--color-slate)]/30 transition-colors"
					onclick={convertBucketsFormat}
					title="Convert all current bucket Min/Max values to the other unit using the mode cost"
					aria-label="Convert bucket values between absolute and normalized"
				>
					<svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
						<path stroke-linecap="round" stroke-linejoin="round" d="M7 7h10M7 7l3 3M7 7l3-3M17 17H7M17 17l-3-3M17 17l-3 3" />
					</svg>
				</button>
			</span>
		</div>

		<div class="text-xs font-mono text-violet-200/70 space-y-2">
			<p>
				In bonus modes, the optimizer uses <span class="text-violet-300">normalized</span> payouts where
				<span class="text-emerald-400">1.0x</span> means “one mode cost”.
				If you select <span class="text-violet-300">Absolute</span> ranges, they are converted automatically before optimizing.
			</p>

			<div class="space-y-1">
				<p><span class="text-violet-300">Absolute</span>: type the multipliers exactly as shown in your distribution/payout table (e.g. 500x = 500x).</p>
				<p><span class="text-violet-300">Normalized</span>: type optimizer units (absolute ÷ cost).</p>
			</div>

			<p class="text-violet-200/60">
				The swap button converts the current bucket values to quickly view and edit buckets in the other unit and switches the selected range unit (Absolute ↔ Normalized) so everything stays consistent.
			</p>
		</div>
	</div>
{/if}


	<!-- Header: RTP + Config buttons -->
	<div class="flex items-center gap-3 p-3 rounded-xl bg-[var(--color-graphite)]/50 border border-white/[0.03]">
		<span class="text-sm font-mono text-[var(--color-light)]">RTP</span>
		<input
			type="range"
			min="0.90"
			max="0.99"
			step="0.01"
			bind:value={targetRtp}
			class="flex-1 h-1.5 bg-[var(--color-slate)] rounded-full appearance-none cursor-pointer
				   [&::-webkit-slider-thumb]:appearance-none [&::-webkit-slider-thumb]:w-4 [&::-webkit-slider-thumb]:h-4
				   [&::-webkit-slider-thumb]:rounded-full [&::-webkit-slider-thumb]:bg-[var(--color-emerald)]
				   [&::-webkit-slider-thumb]:cursor-pointer"
			{disabled}
		/>
		<span class="font-mono text-lg text-[var(--color-emerald)] min-w-[4rem] text-right">{(targetRtp * 100).toFixed(1)}%</span>
		<div class="flex gap-1 ml-2 border-l border-white/[0.05] pl-3">
			<!-- AI Generate button -->
			<button
				class="p-1.5 rounded-lg transition-colors {showProfiles ? 'text-[var(--color-gold)] bg-[var(--color-gold)]/10' : 'text-[var(--color-mist)] hover:text-[var(--color-gold)] hover:bg-[var(--color-gold)]/10'}"
				onclick={loadProfiles}
				title="Generate optimal configs"
				{disabled}
			>
				{#if loadingProfiles}
					<svg class="w-4 h-4 animate-spin" fill="none" viewBox="0 0 24 24">
						<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
						<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"></path>
					</svg>
				{:else}
					<svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
						<path stroke-linecap="round" stroke-linejoin="round" d="M9.663 17h4.673M12 3v1m6.364 1.636l-.707.707M21 12h-1M4 12H3m3.343-5.657l-.707-.707m2.828 9.9a5 5 0 117.072 0l-.548.547A3.374 3.374 0 0014 18.469V19a2 2 0 11-4 0v-.531c0-.895-.356-1.754-.988-2.386l-.548-.547z" />
					</svg>
				{/if}
			</button>
			<button
				class="p-1.5 rounded-lg text-[var(--color-mist)] hover:text-[var(--color-cyan)] hover:bg-[var(--color-cyan)]/10 transition-colors"
				onclick={copyConfig}
				title="Copy config"
			>
				<svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
					<path stroke-linecap="round" stroke-linejoin="round" d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z" />
				</svg>
			</button>
			<button
				class="p-1.5 rounded-lg text-[var(--color-mist)] hover:text-[var(--color-violet)] hover:bg-[var(--color-violet)]/10 transition-colors"
				onclick={() => (showConfigInput = !showConfigInput)}
				title="Paste config"
			>
				<svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
					<path stroke-linecap="round" stroke-linejoin="round" d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2" />
				</svg>
			</button>
		</div>
	</div>

	<!-- Profile Selector Panel -->
	{#if showProfiles && profileConfigs.length > 0}
		<div class="rounded-xl bg-gradient-to-br from-[var(--color-graphite)]/80 to-[var(--color-onyx)] border border-[var(--color-gold)]/20 overflow-hidden">
			<div class="px-4 py-3 border-b border-white/[0.03] flex items-center justify-between">
				<div class="flex items-center gap-2">
					<svg class="w-4 h-4 text-[var(--color-gold)]" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
						<path stroke-linecap="round" stroke-linejoin="round" d="M9.663 17h4.673M12 3v1m6.364 1.636l-.707.707M21 12h-1M4 12H3m3.343-5.657l-.707-.707m2.828 9.9a5 5 0 117.072 0l-.548.547A3.374 3.374 0 0014 18.469V19a2 2 0 11-4 0v-.531c0-.895-.356-1.754-.988-2.386l-.548-.547z" />
					</svg>
					<span class="text-sm font-mono text-[var(--color-gold)]">VOLATILITY PROFILES</span>
				</div>
				<button
					class="p-1 text-[var(--color-mist)]/50 hover:text-[var(--color-light)] transition-colors"
					onclick={() => (showProfiles = false)}
				>
					<svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
						<path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" />
					</svg>
				</button>
			</div>
			<div class="p-3 grid grid-cols-3 gap-2">
				{#each profileConfigs as config}
					<button
						class="p-3 rounded-lg text-left transition-all border
							   {selectedProfile === config.profile
								? 'bg-[var(--color-gold)]/10 border-[var(--color-gold)]/30'
								: 'bg-[var(--color-onyx)]/50 border-white/[0.03] hover:border-white/[0.1] hover:bg-white/[0.02]'}"
						onclick={() => applyProfile(config)}
					>
						<div class="flex items-center gap-2 mb-1">
							<span class="text-base">{getProfileIcon(config.profile)}</span>
							<span class="text-sm font-mono" style="color: {getProfileColor(config.profile)}">{config.profile_name}</span>
						</div>
						<p class="text-xs font-mono text-[var(--color-mist)] line-clamp-2 mb-2">{config.description}</p>
						<div class="flex gap-3 text-xs font-mono text-[var(--color-mist)]/80">
							<span>{config.stats.total_buckets} buckets</span>
							<span>~1:{Math.round(config.stats.avg_hit_rate)} hit</span>
						</div>
					</button>
				{/each}
			</div>
		</div>
	{/if}

	<!-- Config paste input -->
	{#if showConfigInput}
		<div class="flex gap-2 p-3 rounded-xl bg-[var(--color-onyx)]/50 border border-white/[0.03]">
			<input
				type="text"
				bind:value={configCode}
				placeholder="Paste config code..."
				class="flex-1 px-3 py-2 text-sm font-mono bg-[var(--color-slate)]/50 border border-white/[0.05] rounded-lg text-[var(--color-light)] focus:outline-none focus:border-[var(--color-violet)]/30"
			/>
			<button
				class="px-4 py-2 text-sm font-mono bg-[var(--color-violet)]/20 text-[var(--color-violet)] rounded-lg hover:bg-[var(--color-violet)]/30 transition-colors"
				onclick={pasteConfig}
			>
				APPLY
			</button>
		</div>
	{/if}

	<!-- Buckets -->
	<div class="space-y-1.5">
		{#each buckets as bucket, index}
			<div class="flex items-center gap-2 p-2 rounded-lg bg-[var(--color-graphite)]/30 border border-white/[0.02] group">
				<!-- Move buttons -->
				<div class="flex flex-col gap-0.5 opacity-0 group-hover:opacity-100 transition-opacity">
					<button
						class="p-0.5 text-[var(--color-mist)]/40 hover:text-[var(--color-gold)] disabled:opacity-20 disabled:cursor-not-allowed transition-colors"
						onclick={() => moveBucket(index, -1)}
						disabled={disabled || index === 0}
						title="Move up"
					>
						<svg class="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
							<path stroke-linecap="round" stroke-linejoin="round" d="M5 15l7-7 7 7" />
						</svg>
					</button>
					<button
						class="p-0.5 text-[var(--color-mist)]/40 hover:text-[var(--color-gold)] disabled:opacity-20 disabled:cursor-not-allowed transition-colors"
						onclick={() => moveBucket(index, 1)}
						disabled={disabled || index === buckets.length - 1}
						title="Move down"
					>
						<svg class="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
							<path stroke-linecap="round" stroke-linejoin="round" d="M19 9l-7 7-7-7" />
						</svg>
					</button>
				</div>

				<!-- Range -->
				<input
					type="number"
					bind:value={bucket.min_payout}
					class="w-20 px-2 py-1.5 text-sm font-mono bg-[var(--color-slate)]/30 border border-white/[0.05] rounded text-[var(--color-light)] text-right focus:outline-none focus:border-[var(--color-gold)]/30 [appearance:textfield] [&::-webkit-outer-spin-button]:appearance-none [&::-webkit-inner-spin-button]:appearance-none"
					step="0.1"
					{disabled}
				/>
				<span class="text-xs text-[var(--color-mist)]">–</span>
				<input
					type="number"
					bind:value={bucket.max_payout}
					class="w-20 px-2 py-1.5 text-sm font-mono bg-[var(--color-slate)]/30 border border-white/[0.05] rounded text-[var(--color-light)] text-right focus:outline-none focus:border-[var(--color-gold)]/30 [appearance:textfield] [&::-webkit-outer-spin-button]:appearance-none [&::-webkit-inner-spin-button]:appearance-none"
					step="0.1"
					{disabled}
				/>
				<span class="text-xs text-[var(--color-mist)]">x</span>

				<!-- Type buttons -->
				<div class="flex gap-0.5 ml-1">
					<button
						class="px-2 py-1 text-xs font-mono rounded transition-all {bucket.type === 'frequency' ? 'bg-[var(--color-cyan)]/20 text-[var(--color-cyan)]' : 'text-[var(--color-mist)]/70 hover:text-[var(--color-mist)]'}"
						onclick={() => setType(index, 'frequency')}
						title="Frequency: 1 in N spins"
						{disabled}
					>FREQ</button>
					<button
						class="px-2 py-1 text-xs font-mono rounded transition-all {bucket.type === 'rtp_percent' ? 'bg-[var(--color-violet)]/20 text-[var(--color-violet)]' : 'text-[var(--color-mist)]/70 hover:text-[var(--color-mist)]'}"
						onclick={() => setType(index, 'rtp_percent')}
						title="RTP %: % of target RTP"
						{disabled}
					>RTP</button>
					<button
						class="px-2 py-1 text-xs font-mono rounded transition-all {bucket.type === 'auto' ? 'bg-[var(--color-emerald)]/20 text-[var(--color-emerald)]' : 'text-[var(--color-mist)]/70 hover:text-[var(--color-mist)]'}"
						onclick={() => setType(index, 'auto')}
						title="Auto: remaining RTP, inverse weight"
						{disabled}
					>AUTO</button>
				</div>

				<!-- Value input -->
				<div class="flex items-center gap-1.5 ml-1">
					{#if bucket.type === 'frequency'}
						<span class="text-sm text-[var(--color-cyan)]">1:</span>
						<input
							type="number"
							bind:value={bucket.frequency}
							class="w-20 px-2 py-1.5 text-sm font-mono bg-[var(--color-cyan)]/10 border border-[var(--color-cyan)]/20 rounded text-[var(--color-cyan)] text-right focus:outline-none [appearance:textfield] [&::-webkit-outer-spin-button]:appearance-none [&::-webkit-inner-spin-button]:appearance-none"
							step="1"
							min="1"
							{disabled}
						/>
					{:else if bucket.type === 'rtp_percent'}
						<input
							type="number"
							bind:value={bucket.rtp_percent}
							class="w-20 px-2 py-1.5 text-sm font-mono bg-[var(--color-violet)]/10 border border-[var(--color-violet)]/20 rounded text-[var(--color-violet)] text-right focus:outline-none [appearance:textfield] [&::-webkit-outer-spin-button]:appearance-none [&::-webkit-inner-spin-button]:appearance-none"
							step="0.1"
							min="0.01"
							{disabled}
						/>
						<span class="text-sm text-[var(--color-violet)]">%</span>
					{:else}
						<span class="text-sm text-[var(--color-emerald)]">^</span>
						<input
							type="number"
							bind:value={bucket.auto_exponent}
							class="w-16 px-2 py-1.5 text-sm font-mono bg-[var(--color-emerald)]/10 border border-[var(--color-emerald)]/20 rounded text-[var(--color-emerald)] text-right focus:outline-none [appearance:textfield] [&::-webkit-outer-spin-button]:appearance-none [&::-webkit-inner-spin-button]:appearance-none"
							step="0.1"
							min="0"
							max="3"
							placeholder="1"
							{disabled}
						/>
					{/if}
				</div>

				<!-- Delete -->
				<button
					class="ml-auto p-1 text-[var(--color-mist)]/30 hover:text-[var(--color-coral)] opacity-0 group-hover:opacity-100 transition-all"
					onclick={() => removeBucket(index)}
					aria-label="Remove bucket"
					{disabled}
				>
					<svg class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
						<path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" />
					</svg>
				</button>
			</div>
		{/each}

		<!-- Add button -->
		<button
			class="w-full py-2 text-sm font-mono text-[var(--color-mist)]/70 hover:text-[var(--color-gold)] hover:bg-[var(--color-gold)]/5 rounded-lg border border-dashed border-white/[0.05] hover:border-[var(--color-gold)]/30 transition-all"
			onclick={addBucket}
			{disabled}
		>
			+ ADD BUCKET
		</button>
	</div>

	<!-- Save toggle -->
	<div class="flex items-center justify-between p-3 rounded-xl bg-[var(--color-graphite)]/30 border border-white/[0.02]">
		<div class="flex items-center gap-3">
			<span class="text-sm font-mono text-[var(--color-light)]">SAVE</span>
			<button
				class="relative w-10 h-5 rounded-full transition-all {saveToFile ? 'bg-[var(--color-cyan)]' : 'bg-[var(--color-slate)]'}"
				onclick={() => (saveToFile = !saveToFile)}
				aria-label="Toggle save to file"
				{disabled}
			>
				<div class="absolute top-0.5 w-4 h-4 rounded-full bg-white shadow-sm transition-all {saveToFile ? 'left-5' : 'left-0.5'}"></div>
			</button>
		</div>
		{#if saveToFile}
			<div class="flex items-center gap-2">
				<span class="text-xs font-mono text-[var(--color-mist)]">backup</span>
				<button
					class="relative w-8 h-4 rounded-full transition-all {createBackup ? 'bg-[var(--color-emerald)]' : 'bg-[var(--color-slate)]'}"
					onclick={() => (createBackup = !createBackup)}
					aria-label="Toggle create backup"
					{disabled}
				>
					<div class="absolute top-0.5 w-3 h-3 rounded-full bg-white shadow-sm transition-all {createBackup ? 'left-4' : 'left-0.5'}"></div>
				</button>
			</div>
		{/if}
	</div>

	<!-- Optimize Button -->
	<button
		class="w-full flex items-center justify-center gap-2 px-4 py-3 rounded-xl font-mono text-sm
			   bg-gradient-to-r from-[var(--color-gold)] to-[var(--color-coral)] text-[var(--color-void)]
			   hover:shadow-[0_0_20px_rgba(250,204,21,0.3)] transition-all
			   disabled:opacity-40 disabled:cursor-not-allowed"
		onclick={runOptimization}
		disabled={disabled || isLoading}
	>
		{#if isLoading}
			<svg class="w-4 h-4 animate-spin" fill="none" viewBox="0 0 24 24">
				<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
				<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"></path>
			</svg>
		{/if}
		{isLoading ? 'OPTIMIZING...' : 'OPTIMIZE'}
	</button>

	<!-- Error -->
	{#if error}
		<div class="px-4 py-3 rounded-lg bg-[var(--color-coral)]/10 border border-[var(--color-coral)]/20">
			<p class="text-sm font-mono text-[var(--color-coral)]">{error}</p>
		</div>
	{/if}

	<!-- Results -->
	{#if result}
		<div class="rounded-xl bg-[var(--color-graphite)]/50 border border-white/[0.03] overflow-hidden">
			<div class="px-4 py-3 border-b border-white/[0.03] flex items-center gap-2">
				<span class="font-mono text-sm text-[var(--color-light)]">RESULT</span>
				{#if result.converged}
					<span class="px-2 py-0.5 text-xs font-mono bg-[var(--color-emerald)]/20 text-[var(--color-emerald)] rounded">OK</span>
				{:else}
					<span class="px-2 py-0.5 text-xs font-mono bg-[var(--color-coral)]/20 text-[var(--color-coral)] rounded">!</span>
				{/if}
				<span class="ml-auto text-sm font-mono text-[var(--color-mist)]">{(result.original_rtp * 100).toFixed(2)}%</span>
				<svg class="w-4 h-4 text-[var(--color-gold)]" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
					<path stroke-linecap="round" stroke-linejoin="round" d="M13 7l5 5m0 0l-5 5m5-5H6" />
				</svg>
				<span class="text-sm font-mono text-[var(--color-emerald)]">{(result.final_rtp * 100).toFixed(2)}%</span>
			</div>

			<div class="p-3">
				<table class="w-full text-sm font-mono">
					<thead>
						<tr class="text-[var(--color-mist)]">
							<th class="py-1.5 text-left">RANGE</th>
							<th class="py-1.5 text-right">FREQ</th>
							<th class="py-1.5 text-right">RTP</th>
						</tr>
					</thead>
					<tbody>
						{#each result.bucket_results as br, i}
							<tr class="border-t border-white/[0.02]">
								<td class="py-1.5 text-[var(--color-light)]">{br.min_payout}–{br.max_payout}x</td>
								<td class="py-1.5 text-right text-[var(--color-cyan)]">1:{formatFreq(br.actual_frequency)}</td>
								<td class="py-1.5 text-right text-[var(--color-violet)]">{formatPct(br.rtp_contribution)}</td>
							</tr>
						{/each}
						{#if result.loss_result}
							<tr class="border-t border-white/[0.02] text-[var(--color-mist)]/70">
								<td class="py-1.5">loss</td>
								<td class="py-1.5 text-right">1:{formatFreq(result.loss_result.actual_frequency)}</td>
								<td class="py-1.5 text-right">0%</td>
							</tr>
						{/if}
					</tbody>
				</table>
			</div>

			{#if result.warnings?.length}
				<div class="px-3 pb-3">
					<div class="px-2 py-1.5 rounded bg-[var(--color-gold)]/10 text-xs font-mono text-[var(--color-gold)]">
						{#each result.warnings as w}
							<div>{w}</div>
						{/each}
					</div>
				</div>
			{/if}
		</div>
	{/if}

	<!-- Legend -->
	<div class="text-xs font-mono text-[var(--color-mist)]/80 text-center">
		FREQ = 1 in N spins | RTP = % of target | AUTO = remaining RTP (^exp)
	</div>
</div>
