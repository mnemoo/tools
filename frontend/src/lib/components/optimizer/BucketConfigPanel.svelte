<script lang="ts">
	import { api } from '$lib/api/client';
	import { onMount, onDestroy } from 'svelte';
	import type { WSOptimizerProgress, WSOptimizerMessage, ModeAnalysis, GenerateConfigsAnalysis, VoidedBucketInfo, VoidedOutcomeInfo } from '$lib/api/types';
	import { _ } from '$lib/i18n';

	// Simple mode info type for optimizer context (subset of full ModeInfo)
	type SimpleModeInfo = {
		cost: number;
		is_bonus_mode: boolean;
		note?: string;
		max_payout?: number;
	};

	type BucketConfig = {
		name: string;
		min_payout: number;
		max_payout: number;
		type: 'frequency' | 'rtp_percent' | 'auto' | 'max_win_freq';
		frequency?: number;
		rtp_percent?: number;
		auto_exponent?: number;
		max_win_frequency?: number;
		priority?: 1 | 2; // 1=hard, 2=soft
		is_maxwin_bucket?: boolean; // True if this bucket contains max payout
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
	let modeInfo = $state<SimpleModeInfo | null>(null);

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
		brute_force_info?: {
			iterations: number;
			search_duration: number;
			final_error: number;
		};
		voided_buckets?: VoidedBucketInfo[];
		// Auto-voiding results
		voided_outcomes?: VoidedOutcomeInfo[];
		total_voided?: number;
		voided_rtp?: number;
	} | null>(null);
	let error = $state<string | null>(null);
	let saveToFile = $state(false);
	let createBackup = $state(true);
	let configCode = $state('');
	let showConfigInput = $state(false);
	let initialized = $state(false);

	// UI Mode: presets vs manual
	type UIMode = 'presets' | 'manual';
	let uiMode = $state<UIMode>('presets');

	// Selected preset in presets mode
	let selectedPreset = $state<'low' | 'medium' | 'high'>('medium');

	// Display mode for bonus modes (manual mode only)
	type DisplayMode = 'abs' | 'norm';
	let displayMode = $state<DisplayMode>('norm');

	// Brute force optimization state (enabled by default now)
	let enableBruteForce = $state(true);
	// Note: optimization mode removed - now runs until converged or stopped
	let globalMaxWinFreq = $state<number | null>(null);
	let showAdvancedOptions = $state(false);

	// WebSocket progress state
	let ws: WebSocket | null = $state(null);
	let progress = $state<WSOptimizerProgress | null>(null);

	// Profile/Presets state
	let showProfiles = $state(false);
	let profileConfigs = $state<ProfileConfig[]>([]);
	let presetConfigsMap = $state<Record<string, ProfileConfig>>({});
	let loadingProfiles = $state(false);
	let selectedProfile = $state<string | null>(null);
	let presetsLoaded = $state(false);

	// Mode analysis state
	let modeAnalysis = $state<ModeAnalysis | null>(null);
	let analysisInfo = $state<GenerateConfigsAnalysis | null>(null);
	let rtpWarning = $state<string | null>(null);

	// Auto-voiding state (simple toggle - system selects outcomes automatically)
	let enableAutoVoiding = $state(false);

	// Optimizer state indicator
	type OptimizerState = 'idle' | 'running' | 'complete' | 'error';
	let optimizerState = $state<OptimizerState>('idle');

	// Format RTP for display with smart capping
	function formatRTPDisplay(rtp: number): string {
		const rtpPercent = rtp * 100;
		if (rtpPercent > 1000) return '>1000%';
		if (rtpPercent < 0.01) return '<0.01%';
		return rtpPercent.toFixed(2) + '%';
	}

	// Debounce timer for preset loading
	let loadPresetsDebounceTimer: ReturnType<typeof setTimeout> | null = null;

	// MaxWin linked controls state
	let maxWinFreq = $state<number>(50000); // 1:N frequency
	let maxWinRtpContrib = $state<number>(1.0); // % of target RTP

	const STORAGE_KEY = $derived(`lut_bucket_${mode}`);

	// Type mapping for short config
	const TYPE_MAP: Record<string, number> = { frequency: 0, rtp_percent: 1, auto: 2, max_win_freq: 3 };
	const TYPE_REVERSE: ('frequency' | 'rtp_percent' | 'auto' | 'max_win_freq')[] = ['frequency', 'rtp_percent', 'auto', 'max_win_freq'];

	// Convert config to short b64 format
	function toShortConfig(): string {
		const short: ShortConfig = {
			r: Math.round(targetRtp * 100),
			f: inputFormat,
			b: buckets.map((b) => {
				const t = TYPE_MAP[b.type];
				let v: number;
				switch (b.type) {
					case 'frequency': v = b.frequency ?? 10; break;
					case 'rtp_percent': v = b.rtp_percent ?? 1; break;
					case 'max_win_freq': v = b.max_win_frequency ?? 50000; break;
					default: v = b.auto_exponent ?? 1;
				}
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

			// Clamp RTP to valid range (0.01 - 0.99) to prevent validation errors
			const parsedRtp = short.r / 100;
			targetRtp = Math.max(0.01, Math.min(0.99, parsedRtp));
			buckets = short.b.map(([min, max, t, v], i) => {
				const type = TYPE_REVERSE[t] ?? 'frequency';
				return {
					name: `b${i}`,
					min_payout: min,
					max_payout: max,
					type,
					frequency: type === 'frequency' ? v : undefined,
					rtp_percent: type === 'rtp_percent' ? v : undefined,
					auto_exponent: type === 'auto' ? v : undefined,
					max_win_frequency: type === 'max_win_freq' ? v : undefined
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
	onMount(async () => {
		await loadModeInfo(); // Always load mode info
		await loadPresetsForUI(); // Always load presets for presets mode

		// If saved config exists, switch to manual mode
		if (loadFromStorage()) {
			uiMode = 'manual';
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
				error = 'Mode cost unknown - cannot convert formats';
				return;
			}
		}

		const cost = modeInfo!.cost;
		// Swap based on the current ABS/NORM display toggle (displayMode).
		// If currently showing normalized values, convert them to absolute (multiply by cost),
		// otherwise convert absolute -> normalized (divide by cost).
		if (displayMode === 'norm') {
			// Convert normalized -> absolute (multiply by cost)
			buckets = buckets.map(b => ({ ...b, min_payout: +(b.min_payout * cost), max_payout: +(b.max_payout * cost) }));
			displayMode = 'abs';
			inputFormat = 'absolute';
		} else {
			// Convert absolute -> normalized (divide by cost)
			buckets = buckets.map(b => ({ ...b, min_payout: +(b.min_payout / cost), max_payout: +(b.max_payout / cost) }));
			displayMode = 'norm';
			inputFormat = 'normalized';
		}
		// Clear any prior errors
		error = null;
	}

	// Reload mode info and presets when mode changes
	$effect(() => {
		const _ = mode; // Track mode changes
		if (initialized) {
			// IMPORTANT: Reset state BEFORE loading new data to prevent stale UI
			presetsLoaded = false;
			presetConfigsMap = {};
			analysisInfo = null;
			rtpWarning = null;
			result = null;

			loadModeInfo();
			if (uiMode === 'presets') {
				loadPresetsForUI();
			}
		}
	});

	// Reload presets when RTP changes (in presets mode)
	$effect(() => {
		const _ = targetRtp; // Track RTP changes
		if (initialized && uiMode === 'presets') {
			loadPresetsForUI();
		}
	});

	// Load mode analysis when in manual mode
	$effect(() => {
		const _ = [targetRtp, uiMode]; // Track RTP and UI mode changes
		if (initialized && uiMode === 'manual') {
			analyzeModeForRTP();
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
		// Prevent deletion of maxwin bucket
		if (buckets[index]?.is_maxwin_bucket) return;
		buckets = buckets.filter((_, i) => i !== index);
	}

	function moveBucket(index: number, direction: -1 | 1) {
		const newIndex = index + direction;
		if (newIndex < 0 || newIndex >= buckets.length) return;
		const newBuckets = [...buckets];
		[newBuckets[index], newBuckets[newIndex]] = [newBuckets[newIndex], newBuckets[index]];
		buckets = newBuckets;
	}

	function setType(index: number, type: 'frequency' | 'rtp_percent' | 'auto' | 'max_win_freq') {
		buckets = buckets.map((b, i) => {
			if (i !== index) return b;
			return {
				...b,
				type,
				frequency: type === 'frequency' ? (b.frequency ?? 10) : undefined,
				rtp_percent: type === 'rtp_percent' ? (b.rtp_percent ?? 1) : undefined,
				auto_exponent: type === 'auto' ? (b.auto_exponent ?? 1) : undefined,
				max_win_frequency: type === 'max_win_freq' ? (b.max_win_frequency ?? 50000) : undefined
			};
		});
	}

	async function runOptimization() {
		isLoading = true;
		error = null;
		result = null;
		progress = null;
		optimizerState = 'running';

		// In presets mode, apply selected preset first
		if (uiMode === 'presets') {
			const presetConfig = presetConfigsMap[selectedPreset];
			if (presetConfig) {
				fromShortConfig(presetConfig.b64_config);
			} else {
				error = 'Please select a preset first';
				isLoading = false;
				return;
			}
		}

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

		const config = {
			target_rtp: targetRtp,
			buckets: bucketsWithNames,
			save_to_file: saveToFile,
			create_backup: createBackup,
			enable_brute_force: enableBruteForce,
			global_max_win_freq: globalMaxWinFreq ?? undefined,
			// Auto-voiding: system automatically selects outcomes to void
			enable_auto_voiding: enableAutoVoiding
		};

		// Use WebSocket for brute force (real-time progress)
		if (enableBruteForce) {
			runBruteForceOptimizeWS(config);
		} else {
			// Use regular HTTP for standard optimization
			try {
				const response = await api.bucketOptimize(mode, config);
				result = response;
				onOptimize?.(response);
				optimizerState = 'complete';
			} catch (e) {
				error = e instanceof Error ? e.message : 'Optimization failed';
				optimizerState = 'error';
			} finally {
				isLoading = false;
			}
		}
	}

	function runBruteForceOptimizeWS(config: Record<string, unknown>) {
		// Close existing WebSocket if any
		if (ws) {
			ws.close();
			ws = null;
		}

		const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
		const wsUrl = `${protocol}//${window.location.hostname}:7754/api/optimizer/${encodeURIComponent(mode)}/optimize-stream`;

		ws = new WebSocket(wsUrl);

		ws.onopen = () => {
			ws?.send(JSON.stringify(config));
		};

		ws.onmessage = (event) => {
			try {
				const msg = JSON.parse(event.data) as WSOptimizerMessage;

				if (msg.type === 'progress') {
					progress = msg as WSOptimizerProgress;
				} else if (msg.type === 'result') {
					result = (msg as { type: 'result'; result: typeof result }).result;
					onOptimize?.(result);
					isLoading = false;
					progress = null;
					optimizerState = 'complete';
					ws?.close();
					ws = null;
				} else if (msg.type === 'error') {
					error = (msg as { type: 'error'; message: string }).message;
					isLoading = false;
					progress = null;
					optimizerState = 'error';
					ws?.close();
					ws = null;
				}
			} catch (e) {
				error = 'Failed to parse WebSocket message';
				optimizerState = 'error';
			}
		};

		ws.onerror = () => {
			// Only show generic message if no specific error was set
			if (!error) {
				error = 'WebSocket connection failed';
			}
			isLoading = false;
			progress = null;
			optimizerState = 'error';
			ws = null;
		};

		ws.onclose = () => {
			if (isLoading && !error) {
				// Unexpected close - only show generic message if no specific error was set
				error = 'WebSocket connection closed unexpectedly';
				isLoading = false;
				progress = null;
				optimizerState = 'error';
			}
			ws = null;
		};
	}

	// Stop running optimization
	function stopOptimization() {
		if (ws && ws.readyState === WebSocket.OPEN) {
			ws.send(JSON.stringify({ type: 'stop' }));
		}
	}

	// Cleanup WebSocket and timers on component destroy
	onDestroy(() => {
		if (ws) {
			ws.close();
			ws = null;
		}
		if (loadPresetsDebounceTimer) {
			clearTimeout(loadPresetsDebounceTimer);
			loadPresetsDebounceTimer = null;
		}
	});

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

	// MaxWin Frequency ↔ RTP Contribution conversion functions
	// MaxWin payout from modeInfo (e.g., 5000x)
	const maxPayout = $derived(modeInfo?.max_payout ?? 5000);

	// Frequency → RTP% conversion
	// RTP contribution = (1/freq) * maxPayout
	// As % of target RTP = (contrib / targetRtp) * 100
	function freqToRtp(freq: number): number {
		if (freq <= 0) return 0;
		const prob = 1 / freq;
		const rtpContrib = prob * maxPayout;
		return (rtpContrib / targetRtp) * 100; // as % of target RTP
	}

	// RTP% → Frequency conversion
	// rtpContrib (absolute) = (rtpPercent / 100) * targetRtp
	// prob = rtpContrib / maxPayout
	// freq = 1 / prob
	function rtpToFreq(rtpPercent: number): number {
		if (rtpPercent <= 0 || maxPayout <= 0) return 100000;
		const rtpContrib = (rtpPercent / 100) * targetRtp;
		const prob = rtpContrib / maxPayout;
		if (prob <= 0) return 100000;
		return Math.round(1 / prob);
	}

	// MaxWin validation functions
	function validateMaxWinFreq(freq: number): { value: number; warning: string | null } {
		if (freq < 100) return { value: 100, warning: 'Min frequency: 1:100' };
		if (freq > 10000000) return { value: 10000000, warning: 'Max frequency: 1:10M' };
		return { value: Math.round(freq), warning: null };
	}

	function validateMaxWinRtp(rtp: number): { value: number; warning: string | null } {
		if (rtp < 0.01) return { value: 0.01, warning: 'Min RTP: 0.01%' };
		if (rtp > 10) return { value: 10, warning: 'Max RTP: 10% of target' };
		return { value: Math.round(rtp * 100) / 100, warning: null };
	}

	// MaxWin validation state
	let maxWinWarning = $state<string | null>(null);

	// Handle frequency change - update RTP%
	function onMaxWinFreqChange() {
		const validated = validateMaxWinFreq(maxWinFreq);
		maxWinFreq = validated.value;
		maxWinWarning = validated.warning;
		maxWinRtpContrib = Math.round(freqToRtp(maxWinFreq) * 100) / 100;
		applyMaxWinToPreset();
	}

	// Handle RTP% change - update frequency
	function onMaxWinRtpChange() {
		const validated = validateMaxWinRtp(maxWinRtpContrib);
		maxWinRtpContrib = validated.value;
		maxWinWarning = validated.warning;
		maxWinFreq = rtpToFreq(maxWinRtpContrib);
		applyMaxWinToPreset();
	}

	// Auto-fill optimal maxwin values
	function autoFillMaxWin() {
		// Recommended: 0.5-2% of RTP for jackpot tier, 1% is a good default
		const suggestedRtpPercent = 1.0;
		maxWinRtpContrib = suggestedRtpPercent;
		maxWinFreq = rtpToFreq(suggestedRtpPercent);
		applyMaxWinToPreset();
	}

	// Apply maxwin settings to currently selected preset config
	function applyMaxWinToPreset() {
		const presetConfig = presetConfigsMap[selectedPreset];
		if (!presetConfig) return;

		// Parse current config
		if (!fromShortConfig(presetConfig.b64_config)) return;

		// Find or create maxwin bucket
		const lastBucket = buckets[buckets.length - 1];
		if (lastBucket && (lastBucket.is_maxwin_bucket || lastBucket.name === 'maxwin' || lastBucket.type === 'max_win_freq')) {
			// Update existing maxwin bucket
			lastBucket.type = 'rtp_percent';
			lastBucket.rtp_percent = maxWinRtpContrib;
			lastBucket.max_win_frequency = undefined;
			buckets = [...buckets]; // trigger reactivity
		} else {
			// This shouldn't happen if presets are properly generated
			// but add a maxwin bucket just in case
			buckets = [...buckets, {
				name: 'maxwin',
				min_payout: maxPayout * 0.9,
				max_payout: maxPayout + 1,
				type: 'rtp_percent',
				rtp_percent: maxWinRtpContrib,
				is_maxwin_bucket: true
			}];
		}
	}

	// ABS/NORM display conversion for bonus modes
	function toDisplayPayout(value: number): number {
		if (!modeInfo?.is_bonus_mode || displayMode === 'norm') {
			return value;
		}
		// ABS mode: multiply by cost
		return value * modeInfo.cost;
	}

	function fromDisplayPayout(displayValue: number): number {
		if (!modeInfo?.is_bonus_mode || displayMode === 'norm') {
			return displayValue;
		}
		// ABS mode: divide by cost
		return displayValue / modeInfo.cost;
	}

	// For preset selection in presets mode
	function applyPreset(preset: 'low' | 'medium' | 'high') {
		const config = presetConfigsMap[preset];
		if (config && fromShortConfig(config.b64_config)) {
			selectedPreset = preset;
		}
	}

	// Load presets for UI (silent, for presets mode) with debounce
	async function loadPresetsForUI() {
		// Cancel previous pending request
		if (loadPresetsDebounceTimer) {
			clearTimeout(loadPresetsDebounceTimer);
			loadPresetsDebounceTimer = null;
		}

		// Debounce to prevent race conditions
		loadPresetsDebounceTimer = setTimeout(async () => {
			if (loadingProfiles) return;
			loadingProfiles = true;
			rtpWarning = null;

			// Capture current mode for staleness check
			const requestMode = mode;
			const requestRtp = targetRtp;

			try {
				const res = await fetch(`http://localhost:7754/api/optimizer/${encodeURIComponent(requestMode)}/generate-configs?target_rtp=${requestRtp}`);
				const data = await res.json();

				// Check if mode/rtp changed during the request (stale response)
				if (mode !== requestMode || targetRtp !== requestRtp) {
					loadingProfiles = false;
					return; // Discard stale response
				}

				if (data.success && data.data?.configs) {
					profileConfigs = data.data.configs;
					// Build map for easy access
					const map: Record<string, ProfileConfig> = {};
					for (const c of data.data.configs) {
						if (c.profile === 'low_volatility') map['low'] = c;
						else if (c.profile === 'medium_volatility') map['medium'] = c;
						else if (c.profile === 'high_volatility') map['high'] = c;
					}
					presetConfigsMap = map;
					presetsLoaded = true;

					// Extract analysis info if available
					if (data.data.analysis) {
						analysisInfo = data.data.analysis as GenerateConfigsAnalysis;

						// Check feasibility and set warning
						if (!analysisInfo.feasible && analysisInfo.feasibility_note) {
							rtpWarning = analysisInfo.feasibility_note;
						} else if (analysisInfo.mode_type === 'extreme' || analysisInfo.mode_type === 'high_rtp') {
							// Cap extreme max RTP display to avoid confusing values like 10000%
							const minDisplay = (analysisInfo.min_achievable_rtp * 100).toFixed(1);
							const maxDisplay = Math.min(analysisInfo.max_achievable_rtp * 100, 1000).toFixed(1);
							const maxSuffix = analysisInfo.max_achievable_rtp * 100 > 1000 ? '+' : '';
							rtpWarning = `${analysisInfo.mode_type.toUpperCase()}: RTP ${minDisplay}% - ${maxDisplay}%${maxSuffix}`;
						}
					}
				}
			} catch {
				// Silent fail for presets auto-load
			} finally {
				loadingProfiles = false;
			}
		}, 100); // 100ms debounce
	}

	// Analyze mode for RTP feasibility
	async function analyzeModeForRTP() {
		try {
			modeAnalysis = await api.analyzeMode(mode, targetRtp);

			// Update warning based on analysis
			if (!modeAnalysis.feasible && modeAnalysis.feasibility_note) {
				rtpWarning = modeAnalysis.feasibility_note;
			} else {
				rtpWarning = null;
			}
		} catch {
			// Silent fail
			modeAnalysis = null;
		}
	}

	// Adjust RTP to feasible value
	function adjustToFeasibleRTP() {
		if (analysisInfo?.suggested_rtp && analysisInfo.suggested_rtp > 0) {
			targetRtp = analysisInfo.suggested_rtp;
			rtpWarning = null;
			loadPresetsForUI();
		} else if (modeAnalysis?.suggested_rtp && modeAnalysis.suggested_rtp > 0) {
			targetRtp = modeAnalysis.suggested_rtp;
			rtpWarning = null;
			loadPresetsForUI();
		}
	}

	// Profile generator functions (for manual mode AI button)
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
	<!-- Mode Tabs -->
	<div class="flex gap-1 p-1 rounded-xl bg-[var(--color-slate)]/30">
		<button
			class="flex-1 px-4 py-2.5 rounded-lg font-mono text-sm transition-all
				   {uiMode === 'presets' ? 'bg-[var(--color-gold)]/20 text-[var(--color-gold)]' : 'text-[var(--color-mist)] hover:text-[var(--color-light)]'}"
			onclick={() => uiMode = 'presets'}
		>{$_('optimizer.presets')}</button>
		<button
			class="flex-1 px-4 py-2.5 rounded-lg font-mono text-sm transition-all
				   {uiMode === 'manual' ? 'bg-[var(--color-cyan)]/20 text-[var(--color-cyan)]' : 'text-[var(--color-mist)] hover:text-[var(--color-light)]'}"
			onclick={() => uiMode = 'manual'}
		>{$_('optimizer.manual')}</button>
	</div>

	<!-- NOTE: Top explanatory bonus-mode banner removed per UX request. -->

	<!-- ═══════ PRESETS MODE ═══════ -->
	{#if uiMode === 'presets'}
		<!-- RTP Slider -->
		<div class="flex items-center gap-3 p-3 rounded-xl bg-[var(--color-graphite)]/50 border border-white/[0.03]">
			<span class="text-sm font-mono text-[var(--color-light)]">RTP</span>
			<input
				type="range"
				min="0.90"
				max="0.99"
				step="0.005"
				bind:value={targetRtp}
				class="flex-1 h-1.5 bg-[var(--color-slate)] rounded-full appearance-none cursor-pointer
					   [&::-webkit-slider-thumb]:appearance-none [&::-webkit-slider-thumb]:w-4 [&::-webkit-slider-thumb]:h-4
					   [&::-webkit-slider-thumb]:rounded-full [&::-webkit-slider-thumb]:bg-[var(--color-emerald)]
					   [&::-webkit-slider-thumb]:cursor-pointer"
				{disabled}
			/>
			<span class="font-mono text-lg text-[var(--color-emerald)] min-w-[4rem] text-right">{(targetRtp * 100).toFixed(1)}%</span>
		</div>

		<!-- RTP Feasibility Warning -->
		{#if rtpWarning}
			<div class="flex items-center justify-between gap-3 p-3 rounded-xl bg-amber-900/30 border border-amber-500/30">
				<div class="flex items-center gap-2">
					<svg class="w-5 h-5 text-amber-400 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor">
						<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
					</svg>
					<span class="text-sm font-mono text-amber-400">{rtpWarning}</span>
				</div>
				{#if (analysisInfo?.suggested_rtp && analysisInfo.suggested_rtp > 0) || (modeAnalysis?.suggested_rtp && modeAnalysis.suggested_rtp > 0)}
					<button
						class="px-3 py-1.5 rounded-lg bg-amber-500/20 text-amber-400 text-xs font-mono hover:bg-amber-500/30 transition-colors shrink-0"
						onclick={adjustToFeasibleRTP}
						{disabled}
					>
						Auto-adjust
					</button>
				{/if}
			</div>
		{/if}

		<!-- Mode Analysis Info (for all modes) -->
		{#if analysisInfo}
			<div class="p-3 rounded-xl bg-[var(--color-slate)]/30 border border-white/[0.03]">
				<div class="flex items-center gap-2 mb-2">
					<span class="text-sm font-mono text-[var(--color-mist)]">{$_('optimizer.modeAnalysis')}</span>
					<span class="px-2 py-0.5 rounded text-xs font-mono
						{analysisInfo.mode_type === 'extreme' ? 'bg-red-500/20 text-red-400' :
						 analysisInfo.mode_type === 'high_rtp' ? 'bg-orange-500/20 text-orange-400' :
						 analysisInfo.is_bonus_mode ? 'bg-purple-500/20 text-purple-400' :
						 analysisInfo.mode_type === 'standard' ? 'bg-emerald-500/20 text-emerald-400' :
						 'bg-[var(--color-slate)] text-[var(--color-mist)]'}">
						{analysisInfo.mode_type.replace('_', ' ').toUpperCase()}
					</span>
				</div>
				<div class="grid grid-cols-2 gap-2 text-xs font-mono text-[var(--color-mist)]/80">
					<span>{$_('optimizer.minRtp')} {formatRTPDisplay(analysisInfo.min_achievable_rtp)}</span>
					<span>{$_('optimizer.maxRtp')} {formatRTPDisplay(analysisInfo.max_achievable_rtp)}</span>
				</div>
			</div>
		{/if}

		<!-- Preset Cards -->
		{#if loadingProfiles && !presetsLoaded}
			<div class="flex items-center justify-center py-8">
				<svg class="w-6 h-6 animate-spin text-[var(--color-gold)]" fill="none" viewBox="0 0 24 24">
					<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
					<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"></path>
				</svg>
			</div>
		{:else if presetsLoaded}
			<div class="grid grid-cols-3 gap-3">
				<!-- Low Volatility -->
				<button
					class="p-4 rounded-xl text-left transition-all border-2
						   {selectedPreset === 'low'
							? 'border-[var(--color-emerald)] bg-[var(--color-emerald)]/10'
							: 'border-white/[0.05] hover:border-white/[0.1] hover:bg-white/[0.02]'}"
					onclick={() => selectedPreset = 'low'}
					{disabled}
				>
					<div class="flex items-center gap-2 mb-2">
						<span class="text-2xl">🎯</span>
						<span class="font-mono text-lg text-[var(--color-emerald)]">{$_('optimizer.low')}</span>
					</div>
					<p class="text-xs font-mono text-[var(--color-mist)] mb-3">{$_('optimizer.volatility')}</p>
					{#if presetConfigsMap['low']}
						<div class="flex flex-col gap-1 text-xs font-mono text-[var(--color-mist)]/80">
							<span>{presetConfigsMap['low'].stats.total_buckets} {$_('optimizer.buckets')}</span>
							<span>~1:{Math.round(presetConfigsMap['low'].stats.avg_hit_rate)} {$_('optimizer.hit')}</span>
						</div>
					{/if}
					{#if selectedPreset === 'low'}
						<div class="mt-2 text-xs font-mono text-[var(--color-emerald)]">✓ {$_('optimizer.selected')}</div>
					{/if}
				</button>

				<!-- Medium Volatility -->
				<button
					class="p-4 rounded-xl text-left transition-all border-2
						   {selectedPreset === 'medium'
							? 'border-[var(--color-cyan)] bg-[var(--color-cyan)]/10'
							: 'border-white/[0.05] hover:border-white/[0.1] hover:bg-white/[0.02]'}"
					onclick={() => selectedPreset = 'medium'}
					{disabled}
				>
					<div class="flex items-center gap-2 mb-2">
						<span class="text-2xl">⚖️</span>
						<span class="font-mono text-lg text-[var(--color-cyan)]">{$_('optimizer.medium')}</span>
					</div>
					<p class="text-xs font-mono text-[var(--color-mist)] mb-3">{$_('optimizer.volatility')}</p>
					{#if presetConfigsMap['medium']}
						<div class="flex flex-col gap-1 text-xs font-mono text-[var(--color-mist)]/80">
							<span>{presetConfigsMap['medium'].stats.total_buckets} {$_('optimizer.buckets')}</span>
							<span>~1:{Math.round(presetConfigsMap['medium'].stats.avg_hit_rate)} {$_('optimizer.hit')}</span>
						</div>
					{/if}
					{#if selectedPreset === 'medium'}
						<div class="mt-2 text-xs font-mono text-[var(--color-cyan)]">✓ {$_('optimizer.selected')}</div>
					{/if}
				</button>

				<!-- High Volatility -->
				<button
					class="p-4 rounded-xl text-left transition-all border-2
						   {selectedPreset === 'high'
							? 'border-[var(--color-coral)] bg-[var(--color-coral)]/10'
							: 'border-white/[0.05] hover:border-white/[0.1] hover:bg-white/[0.02]'}"
					onclick={() => selectedPreset = 'high'}
					{disabled}
				>
					<div class="flex items-center gap-2 mb-2">
						<span class="text-2xl">🎢</span>
						<span class="font-mono text-lg text-[var(--color-coral)]">{$_('optimizer.high')}</span>
					</div>
					<p class="text-xs font-mono text-[var(--color-mist)] mb-3">{$_('optimizer.volatility')}</p>
					{#if presetConfigsMap['high']}
						<div class="flex flex-col gap-1 text-xs font-mono text-[var(--color-mist)]/80">
							<span>{presetConfigsMap['high'].stats.total_buckets} {$_('optimizer.buckets')}</span>
							<span>~1:{Math.round(presetConfigsMap['high'].stats.avg_hit_rate)} {$_('optimizer.hit')}</span>
						</div>
					{/if}
					{#if selectedPreset === 'high'}
						<div class="mt-2 text-xs font-mono text-[var(--color-coral)]">✓ {$_('optimizer.selected')}</div>
					{/if}
				</button>
			</div>
		{:else}
			<div class="text-center py-8 text-sm font-mono text-[var(--color-mist)]">
				{$_('optimizer.failedToLoadPresets')} <button class="text-[var(--color-gold)] hover:underline" onclick={loadPresetsForUI}>{$_('optimizer.retry')}</button>
			</div>
		{/if}

		<!-- MaxWin Controls Panel -->
		<div class="p-3 rounded-xl bg-gradient-to-r from-[var(--color-gold)]/10 to-[var(--color-coral)]/10 border border-[var(--color-gold)]/20">
			<div class="flex items-center justify-between mb-3">
				<div class="flex items-center gap-2">
					<span class="font-mono text-sm text-[var(--color-gold)]">{$_('optimizer.maxwinControl')}</span>
					<span class="px-2 py-0.5 text-xs font-mono bg-[var(--color-emerald)]/20 text-[var(--color-emerald)] rounded">NEW</span>
				</div>
				<button
					class="px-3 py-1.5 text-xs font-mono rounded bg-[var(--color-gold)]/20 text-[var(--color-gold)] hover:bg-[var(--color-gold)]/30 transition-colors"
					onclick={autoFillMaxWin}
					title="Auto-fill optimal maxwin values (1% RTP)"
					{disabled}
				>{$_('optimizer.autoFill')}</button>
			</div>

			<div class="grid grid-cols-2 gap-3">
				<!-- MaxWin Frequency -->
				<div class="flex flex-col gap-1">
					<label class="text-xs font-mono text-[var(--color-mist)]">{$_('optimizer.frequency')}</label>
					<div class="flex items-center gap-1.5">
						<span class="text-sm text-[var(--color-coral)]">1:</span>
						<input
							type="number"
							bind:value={maxWinFreq}
							onchange={onMaxWinFreqChange}
							class="flex-1 px-2 py-1.5 text-sm font-mono bg-[var(--color-coral)]/10 border border-[var(--color-coral)]/20 rounded text-[var(--color-coral)] text-right focus:outline-none [appearance:textfield] [&::-webkit-outer-spin-button]:appearance-none [&::-webkit-inner-spin-button]:appearance-none"
							step="1000"
							min="100"
							placeholder="50000"
							{disabled}
						/>
					</div>
					<span class="text-xs font-mono text-[var(--color-mist)]/60">{$_('optimizer.oneInSpins', { values: { value: formatFreq(maxWinFreq) } })}</span>
				</div>

				<!-- RTP Contribution -->
				<div class="flex flex-col gap-1">
					<label class="text-xs font-mono text-[var(--color-mist)]">{$_('optimizer.rtpContribution')}</label>
					<div class="flex items-center gap-1.5">
						<input
							type="number"
							bind:value={maxWinRtpContrib}
							onchange={onMaxWinRtpChange}
							class="flex-1 px-2 py-1.5 text-sm font-mono bg-[var(--color-violet)]/10 border border-[var(--color-violet)]/20 rounded text-[var(--color-violet)] text-right focus:outline-none [appearance:textfield] [&::-webkit-outer-spin-button]:appearance-none [&::-webkit-inner-spin-button]:appearance-none"
							step="0.1"
							min="0.01"
							max="10"
							placeholder="1.0"
							{disabled}
						/>
						<span class="text-sm text-[var(--color-violet)]">%</span>
					</div>
					<span class="text-xs font-mono text-[var(--color-mist)]/60">{maxWinRtpContrib}{$_('optimizer.ofTargetRtp')}</span>
				</div>
			</div>

			{#if modeInfo?.max_payout}
				<div class="mt-2 text-xs font-mono text-[var(--color-mist)]/60">
					{$_('optimizer.maxPayout')} {modeInfo.max_payout}x
				</div>
			{/if}
			{#if maxWinWarning}
				<div class="mt-2 px-2 py-1 rounded bg-amber-500/20 text-xs font-mono text-amber-400">
					{maxWinWarning}
				</div>
			{/if}
		</div>

		<!-- Brute Force Panel (inline) -->
		<div class="flex items-center justify-between p-3 rounded-xl bg-[var(--color-violet)]/10 border border-[var(--color-violet)]/20">
			<div class="flex items-center gap-2">
				<span class="font-mono text-sm text-[var(--color-light)]">{$_('optimizer.bruteForce')}</span>
				<span class="text-xs font-mono text-[var(--color-mist)]">{$_('optimizer.runsUntilConverged')}</span>
			</div>
			{#if isLoading}
				<button
					class="px-4 py-1.5 text-xs font-mono rounded bg-red-500/20 text-red-400 hover:bg-red-500/30 transition-all"
					onclick={stopOptimization}
					title="Stop optimization and use best result so far"
				>{$_('optimizer.stop')}</button>
			{/if}
		</div>

		<!-- Auto-Voiding Panel (PRESETS mode) -->
		<div class="flex items-center justify-between p-3 rounded-xl bg-[var(--color-coral)]/5 border border-[var(--color-coral)]/20">
			<div class="flex items-center gap-2">
				<span class="font-mono text-sm text-[var(--color-coral)]">{$_('optimizer.autoVoid')}</span>
				<span class="text-xs font-mono text-[var(--color-mist)]">{$_('optimizer.autoRemoveOutcomes')}</span>
			</div>
			<button
				class="relative w-10 h-5 rounded-full transition-all {enableAutoVoiding ? 'bg-[var(--color-coral)]' : 'bg-[var(--color-slate)]'}"
				onclick={() => enableAutoVoiding = !enableAutoVoiding}
				aria-label="Toggle auto-voiding"
				{disabled}
			>
				<div class="absolute top-0.5 w-4 h-4 rounded-full bg-white shadow-sm transition-all {enableAutoVoiding ? 'left-5' : 'left-0.5'}"></div>
			</button>
		</div>

	<!-- ═══════ MANUAL MODE ═══════ -->
	{:else}
		<!-- RTP + Display Mode Toggle -->
		<div class="flex items-center gap-3 p-3 rounded-xl bg-[var(--color-graphite)]/50 border border-white/[0.03]">
			<span class="text-sm font-mono text-[var(--color-light)]">RTP</span>
			<input
				type="range"
				min="0.90"
				max="0.99"
				step="0.005"
				bind:value={targetRtp}
				class="flex-1 h-1.5 bg-[var(--color-slate)] rounded-full appearance-none cursor-pointer
					   [&::-webkit-slider-thumb]:appearance-none [&::-webkit-slider-thumb]:w-4 [&::-webkit-slider-thumb]:h-4
					   [&::-webkit-slider-thumb]:rounded-full [&::-webkit-slider-thumb]:bg-[var(--color-emerald)]
					   [&::-webkit-slider-thumb]:cursor-pointer"
				{disabled}
			/>
			<span class="font-mono text-lg text-[var(--color-emerald)] min-w-[4rem] text-right">{(targetRtp * 100).toFixed(1)}%</span>

			<!-- ABS/NORM Toggle (only for bonus modes) -->
			{#if modeInfo?.is_bonus_mode}
				<div class="flex items-center gap-2 ml-2 pl-3 border-l border-white/[0.05]">
					<div class="flex gap-0.5 p-0.5 bg-[var(--color-slate)]/30 rounded">
						<button
							class="px-2 py-1 text-xs font-mono rounded transition-all
								{displayMode === 'abs' ? 'bg-[var(--color-violet)]/20 text-[var(--color-violet)]' : 'text-[var(--color-mist)]/70 hover:text-[var(--color-mist)]'}"
							onclick={() => displayMode = 'abs'}
							title="Absolute values (multiplied by cost)"
						>ABS</button>
						<button
							class="px-2 py-1 text-xs font-mono rounded transition-all
								{displayMode === 'norm' ? 'bg-[var(--color-emerald)]/20 text-[var(--color-emerald)]' : 'text-[var(--color-mist)]/70 hover:text-[var(--color-mist)]'}"
							onclick={() => displayMode = 'norm'}
							title="Normalized values (divided by cost)"
						>NORM</button>
					</div>

					<!-- Convert/Switch button moved from the removed banner -->
					<button
						class="p-1 rounded text-xs bg-[var(--color-slate)]/20 text-[var(--color-mist)] hover:bg-[var(--color-slate)]/30 transition-colors"
						onclick={convertBucketsFormat}
						title="Convert current bucket Min/Max values between absolute and normalized units"
						aria-label="Convert bucket values between absolute and normalized"
					>
						<svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
							<path stroke-linecap="round" stroke-linejoin="round" d="M7 7h10M7 7l3 3M7 7l3-3M17 17H7M17 17l-3-3M17 17l-3 3" />
						</svg>
					</button>

					<span class="px-2 py-0.5 text-xs font-mono bg-[var(--color-cyan)]/20 text-[var(--color-cyan)] rounded">NEW</span>
				</div>
			{/if}

			<!-- Config buttons -->
			<div class="flex gap-1 ml-2 border-l border-white/[0.05] pl-3">
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

		<!-- Mode Analysis Info (Manual mode) -->
		{#if modeAnalysis || analysisInfo}
			{@const info = modeAnalysis || analysisInfo}
			<div class="p-3 rounded-xl bg-[var(--color-slate)]/30 border border-white/[0.03]">
				<div class="flex items-center gap-2 mb-2">
					<span class="text-sm font-mono text-[var(--color-mist)]">{$_('optimizer.modeAnalysis')}</span>
					{#if info && 'mode_type' in info}
						<span class="px-2 py-0.5 rounded text-xs font-mono
							{info.mode_type === 'extreme' ? 'bg-red-500/20 text-red-400' :
							 info.mode_type === 'high_rtp' ? 'bg-orange-500/20 text-orange-400' :
							 info.mode_type === 'standard' ? 'bg-emerald-500/20 text-emerald-400' :
							 'bg-[var(--color-slate)] text-[var(--color-mist)]'}">
							{info.mode_type?.replace('_', ' ').toUpperCase() ?? 'UNKNOWN'}
						</span>
					{/if}
				</div>
				{#if info}
					<div class="grid grid-cols-2 gap-2 text-xs font-mono text-[var(--color-mist)]/80">
						<span>{$_('optimizer.minRtp')} {formatRTPDisplay(info.min_achievable_rtp)}</span>
						<span>{$_('optimizer.maxRtp')} {formatRTPDisplay(info.max_achievable_rtp)}</span>
					</div>
				{/if}
			</div>
		{/if}

				<!-- Bonus Mode Info (simplified) -->
				{#if modeInfo?.is_bonus_mode}
					<div class="px-3 py-2 rounded-lg bg-violet-500/10 border border-violet-500/20">
						<div class="flex items-center gap-2 text-xs font-mono">
							<span class="text-violet-400">BONUS MODE</span>
							<span class="px-1.5 py-0.5 bg-violet-500/20 text-violet-300 rounded">Cost: {modeInfo.cost}x</span>
							<span class="text-violet-200/70 ml-2">
								{displayMode === 'abs' ? 'Showing absolute values' : 'Showing normalized values'}
							</span>
						</div>
					</div>
				{/if}

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
						aria-label="Close profiles"
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
								<span>{config.stats.total_buckets} {$_('optimizer.buckets')}</span>
								<span>~1:{Math.round(config.stats.avg_hit_rate)} {$_('optimizer.hit')}</span>
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
				<div class="flex items-center gap-2 p-2 rounded-lg group transition-all
					{bucket.is_maxwin_bucket
						? 'bg-gradient-to-r from-[var(--color-gold)]/10 to-[var(--color-coral)]/10 border border-[var(--color-gold)]/30'
						: 'bg-[var(--color-graphite)]/30 border border-white/[0.02]'}">

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
						<button
							class="px-2 py-1 text-xs font-mono rounded transition-all {bucket.type === 'max_win_freq' ? 'bg-[var(--color-coral)]/20 text-[var(--color-coral)]' : 'text-[var(--color-mist)]/70 hover:text-[var(--color-mist)]'}"
							onclick={() => setType(index, 'max_win_freq')}
							title="Max Win Freq: frequency of the max payout outcome"
							{disabled}
						>MAX</button>
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
						{:else if bucket.type === 'max_win_freq'}
							<span class="text-sm text-[var(--color-coral)]">1:</span>
							<input
								type="number"
								bind:value={bucket.max_win_frequency}
								class="w-24 px-2 py-1.5 text-sm font-mono bg-[var(--color-coral)]/10 border border-[var(--color-coral)]/20 rounded text-[var(--color-coral)] text-right focus:outline-none [appearance:textfield] [&::-webkit-outer-spin-button]:appearance-none [&::-webkit-inner-spin-button]:appearance-none"
								step="1000"
								min="100"
								placeholder="50000"
								{disabled}
							/>
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

					<!-- MaxWin badge or Delete -->
					{#if bucket.is_maxwin_bucket}
						<span class="ml-auto px-2 py-0.5 text-xs font-mono bg-[var(--color-gold)]/20 text-[var(--color-gold)] rounded-full">
							MAXWIN
						</span>
					{:else}
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
					{/if}
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

		<!-- Options Panel -->
		<div class="space-y-3 p-4 rounded-xl bg-[var(--color-onyx)]/50 border border-white/[0.05]">
			<div class="flex items-center gap-2 mb-3">
				<span class="text-sm font-mono text-[var(--color-light)]">{$_('optimizer.options')}</span>
			</div>

			<!-- Global Max Win Frequency -->
			<div class="flex items-center justify-between">
				<span class="text-sm font-mono text-[var(--color-mist)]">{$_('optimizer.maxWinFreq')}</span>
				<div class="flex items-center gap-1.5">
					<span class="text-sm text-[var(--color-coral)]">1:</span>
					<input
						type="number"
						bind:value={globalMaxWinFreq}
						placeholder="e.g. 50000"
						class="w-28 px-2 py-1.5 text-sm font-mono bg-[var(--color-coral)]/10 border border-[var(--color-coral)]/20 rounded text-[var(--color-coral)] text-right focus:outline-none placeholder:text-[var(--color-coral)]/30 [appearance:textfield] [&::-webkit-outer-spin-button]:appearance-none [&::-webkit-inner-spin-button]:appearance-none"
						step="1000"
						min="100"
						{disabled}
					/>
				</div>
			</div>

			<!-- Brute Force Mode -->
			<div class="flex items-center justify-between">
				<div class="flex items-center gap-2">
					<span class="text-sm font-mono text-[var(--color-mist)]">{$_('optimizer.bruteForce')}</span>
					<span class="text-xs font-mono text-[var(--color-mist)]/50">{$_('optimizer.runsUntilConverged')}</span>
				</div>
				{#if isLoading}
					<button
						class="px-4 py-1.5 text-xs font-mono rounded bg-red-500/20 text-red-400 hover:bg-red-500/30 transition-all"
						onclick={stopOptimization}
						title="Stop optimization and use best result so far"
					>{$_('optimizer.stop')}</button>
				{/if}
			</div>

			<!-- Auto-Voiding Toggle -->
			<div class="flex items-center justify-between">
				<div class="flex items-center gap-2">
					<span class="text-sm font-mono text-[var(--color-mist)]">{$_('optimizer.autoVoid')}</span>
					<span class="text-xs font-mono text-[var(--color-mist)]/50">{$_('optimizer.autoRemoveOutcomes')}</span>
				</div>
				<button
					class="relative w-10 h-5 rounded-full transition-all {enableAutoVoiding ? 'bg-[var(--color-coral)]' : 'bg-[var(--color-slate)]'}"
					onclick={() => enableAutoVoiding = !enableAutoVoiding}
					aria-label="Toggle auto-voiding"
					{disabled}
				>
					<div class="absolute top-0.5 w-4 h-4 rounded-full bg-white shadow-sm transition-all {enableAutoVoiding ? 'left-5' : 'left-0.5'}"></div>
				</button>
			</div>
		</div>
	{/if}

	<!-- ═══════ COMMON ELEMENTS ═══════ -->

	<!-- Progress Bar (during brute force optimization) -->
	{#if progress}
		{@const isUnlimited = progress.max_iter >= 1000000}
		{@const errorProgress = isUnlimited ? Math.max(0, Math.min(100, 100 - (Math.log10(progress.error + 0.00001) + 5) * 20)) : (progress.iteration / progress.max_iter) * 100}
		<div class="p-4 rounded-xl bg-[var(--color-graphite)]/50 border border-[var(--color-gold)]/20">
			<div class="flex items-center justify-between mb-2">
				<div class="flex items-center gap-2">
					<span class="text-sm font-mono text-[var(--color-gold)]">{progress.phase.toUpperCase()}</span>
					<span class="w-2 h-2 rounded-full bg-[var(--color-gold)] animate-pulse"></span>
				</div>
				{#if isUnlimited}
					<span class="text-xs font-mono text-[var(--color-mist)]">iter {progress.iteration.toLocaleString()}</span>
				{:else}
					<span class="text-xs font-mono text-[var(--color-mist)]">{progress.iteration}/{progress.max_iter}</span>
				{/if}
			</div>
			<div class="w-full h-2 bg-[var(--color-slate)] rounded-full overflow-hidden mb-2">
				<div
					class="h-full bg-gradient-to-r from-[var(--color-gold)] to-[var(--color-coral)] transition-all duration-200"
					style="width: {errorProgress}%"
				></div>
			</div>
			<div class="flex justify-between text-xs font-mono">
				<span class="text-[var(--color-mist)]">RTP: <span class="text-[var(--color-cyan)]">{(progress.current_rtp * 100).toFixed(3)}%</span></span>
				<span class="text-[var(--color-mist)]">Target: <span class="text-[var(--color-emerald)]">{(progress.target_rtp * 100).toFixed(2)}%</span></span>
				<span class="text-[var(--color-mist)]">Error: <span class="{progress.error < 0.0001 ? 'text-[var(--color-emerald)]' : 'text-[var(--color-coral)]'}">{(progress.error * 100).toFixed(4)}%</span></span>
			</div>
		</div>
	{/if}

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

	<!-- State Indicator -->
	{#if !progress}
		<div class="flex items-center justify-center gap-2 py-2">
			{#if optimizerState === 'idle'}
				<span class="w-2 h-2 rounded-full bg-[var(--color-mist)]/50"></span>
				<span class="text-xs font-mono text-[var(--color-mist)]">Ready</span>
			{:else if optimizerState === 'running'}
				<span class="w-2 h-2 rounded-full bg-[var(--color-gold)] animate-pulse"></span>
				<span class="text-xs font-mono text-[var(--color-gold)]">Running...</span>
			{:else if optimizerState === 'complete'}
				<span class="w-2 h-2 rounded-full bg-[var(--color-emerald)]"></span>
				<span class="text-xs font-mono text-[var(--color-emerald)]">Complete</span>
			{:else if optimizerState === 'error'}
				<span class="w-2 h-2 rounded-full bg-[var(--color-coral)]"></span>
				<span class="text-xs font-mono text-[var(--color-coral)]">Error</span>
			{/if}
		</div>
	{/if}

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
		{isLoading ? $_('optimizer.optimizing') : $_('optimizer.optimize')}
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
				<span class="font-mono text-sm text-[var(--color-light)]">{$_('optimizer.result')}</span>
				{#if result.converged}
					<span class="px-2 py-0.5 text-xs font-mono bg-[var(--color-emerald)]/20 text-[var(--color-emerald)] rounded">{$_('optimizer.ok')}</span>
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
							<th class="py-1.5 text-left">{$_('optimizer.range')}</th>
							<th class="py-1.5 text-right">{$_('optimizer.freq')}</th>
							<th class="py-1.5 text-right">RTP</th>
						</tr>
					</thead>
					<tbody>
						{#each result.bucket_results as br, i}
							<tr class="border-t border-white/[0.02]">
								<td class="py-1.5 text-[var(--color-light)]">{toDisplayPayout(br.min_payout).toFixed(2)}–{toDisplayPayout(br.max_payout).toFixed(2)}x</td>
								<td class="py-1.5 text-right text-[var(--color-cyan)]">1:{formatFreq(br.actual_frequency)}</td>
								<td class="py-1.5 text-right text-[var(--color-violet)]">{formatPct(br.rtp_contribution)}</td>
							</tr>
						{/each}
						{#if result.loss_result}
							<tr class="border-t border-white/[0.02] text-[var(--color-mist)]/70">
								<td class="py-1.5">{$_('optimizer.loss')}</td>
								<td class="py-1.5 text-right">1:{formatFreq(result.loss_result.actual_frequency)}</td>
								<td class="py-1.5 text-right">{result.loss_result.rtp_contribution ? formatPct(result.loss_result.rtp_contribution) : '0%'}</td>
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

			{#if result.brute_force_info}
				<div class="px-3 pb-3">
					<div class="flex items-center justify-between px-2 py-1.5 rounded bg-[var(--color-violet)]/10 text-xs font-mono">
						<span class="text-[var(--color-violet)]">{$_('optimizer.bruteForce')}</span>
						<div class="flex gap-4 text-[var(--color-mist)]">
							<span>{result.brute_force_info.iterations} {$_('optimizer.iter')}</span>
							<span>{result.brute_force_info.search_duration}ms</span>
							<span class="{result.brute_force_info.final_error < 0.0001 ? 'text-[var(--color-emerald)]' : 'text-[var(--color-coral)]'}">
								{(result.brute_force_info.final_error * 100).toFixed(4)}% {$_('optimizer.err')}
							</span>
						</div>
					</div>
				</div>
			{/if}

			<!-- Voided Buckets Notification (deprecated - for backwards compatibility) -->
			{#if result.voided_buckets?.length}
				<div class="px-3 pb-3">
					<div class="px-3 py-2 rounded-lg bg-[var(--color-coral)]/10 border border-[var(--color-coral)]/20">
						<div class="flex items-center gap-2 text-sm font-mono text-[var(--color-coral)]">
							<svg class="w-4 h-4 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
								<path stroke-linecap="round" stroke-linejoin="round" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
							</svg>
							<span>Voided {result.voided_buckets.length} bucket(s) to achieve target RTP</span>
						</div>
						<div class="mt-2 space-y-1">
							{#each result.voided_buckets as vb}
								<div class="flex items-center justify-between text-xs font-mono text-[var(--color-coral)]/70">
									<span class="line-through">{vb.name}</span>
									<span>{vb.outcome_count} outcomes removed ({vb.rtp_contribution.toFixed(2)}% RTP)</span>
								</div>
							{/each}
						</div>
					</div>
				</div>
			{/if}

			<!-- Auto-Voided Outcomes Notification -->
			{#if result.voided_outcomes?.length}
				<div class="px-3 pb-3">
					<div class="px-3 py-2 rounded-lg bg-[var(--color-coral)]/10 border border-[var(--color-coral)]/20">
						<div class="flex items-center justify-between text-sm font-mono text-[var(--color-coral)]">
							<div class="flex items-center gap-2">
								<svg class="w-4 h-4 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
									<path stroke-linecap="round" stroke-linejoin="round" d="M13 10V3L4 14h7v7l9-11h-7z" />
								</svg>
								<span>Auto-voided {result.total_voided || result.voided_outcomes.length} outcome(s)</span>
							</div>
							{#if result.voided_rtp}
								<span class="text-xs font-mono text-[var(--color-coral)]/70">
									-{(result.voided_rtp * 100).toFixed(2)}% RTP
								</span>
							{/if}
						</div>
						{#if result.voided_outcomes.length <= 5}
							<div class="mt-2 space-y-1">
								{#each result.voided_outcomes as vo}
									<div class="flex items-center justify-between text-xs font-mono text-[var(--color-coral)]/70">
										<span class="line-through">{vo.payout.toFixed(2)}x</span>
										<span class="flex items-center gap-2">
											<span class="px-1.5 py-0.5 rounded bg-[var(--color-coral)]/20 text-[var(--color-coral)]">
												{vo.reason === 'duplicate' ? 'dup' : 'high'}
											</span>
											<span>-{(vo.rtp_loss * 100).toFixed(3)}%</span>
										</span>
									</div>
								{/each}
							</div>
						{:else}
							<div class="mt-2 text-xs font-mono text-[var(--color-coral)]/70">
								<div class="flex flex-wrap gap-1.5">
									{#each result.voided_outcomes.slice(0, 3) as vo}
										<span class="px-1.5 py-0.5 rounded bg-[var(--color-coral)]/20 line-through">{vo.payout.toFixed(2)}x</span>
									{/each}
									<span class="px-1.5 py-0.5 text-[var(--color-coral)]/50">+{result.voided_outcomes.length - 3} more</span>
								</div>
							</div>
						{/if}
					</div>
				</div>
			{/if}
		</div>
	{/if}

	<!-- Legend -->
	<div class="text-xs font-mono text-[var(--color-mist)]/80 text-center">
		{$_('optimizer.legendHelp')}
	</div>
</div>
