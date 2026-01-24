// API Types for LUT Explorer

export interface ApiResponse<T> {
	success: boolean;
	data?: T;
	error?: string;
}

export interface ModeSummary {
	mode: string;
	cost: number;
	outcomes: number;
	rtp: number;
	hit_rate: number;
	max_payout: number;
}

export interface IndexInfo {
	modes: ModeSummary[];
}

export interface PayoutBucket {
	range_start: number;
	range_end: number;
	count: number;
	weight: number;
	probability: number;
}

export interface DistributionItem {
	payout: number;
	weight: number;
	odds: string;
	count: number;      // number of sim_ids with this payout
	sim_ids: number[];  // first few sim_ids for quick lookup
}

export interface PayoutInfo {
	sim_id: number;
	payout: number;
	weight: number;
	odds: string;
	count: number;
}

export interface Statistics {
	mode: string;
	cost: number; // Cost to trigger this mode (1.0 for base)
	total_outcomes: number;
	total_weight: number;
	rtp: number;
	hit_rate: number;
	max_payout: number;
	min_payout: number;
	mean_payout: number;
	median_payout: number;
	variance: number;
	std_dev: number;
	volatility: number;
	mean_median_ratio: number;
	payout_buckets: PayoutBucket[];
	distribution: DistributionItem[];
	top_payouts: PayoutInfo[];
	zero_payout_rate: number;
	// Cost-adjusted metrics (for bonus modes)
	breakeven_rate: number; // P(payout >= cost)
	cost_adj_volatility: number; // StdDev / Cost
}

export interface Outcome {
	sim_id: number;
	weight: number;
	payout: number;
	probability: number;
}

export interface CompareItem {
	mode: string;
	cost: number;
	rtp: number;
	hit_rate: number;
	max_payout: number;
	volatility: number;
	mean_payout: number;
	median_payout: number;
	breakeven_rate: number;
	cost_adj_volatility: number;
}

export interface CompareResponse {
	modes: CompareItem[];
}

export interface EventLoadResult {
	mode: string;
	loaded: boolean;
	count: number;
}

export interface EventInfo {
	sim_id: number;
	weight: number;
	payout: number;
	probability: number;
	odds: string;
	event: unknown;
	events_loaded: boolean;
	event_missing?: boolean;
	lazy_load?: boolean;      // true if event was loaded on-demand (not from full cache)
	no_events_file?: boolean; // true if mode has no events file configured
	error?: string;           // error message if lazy loading failed
}

// LGS (Local Game Server) types
export interface LGSBalance {
	amount: number;
	currency: string;
}

export interface LGSConfig {
	gameID: string;
	minBet: number;
	maxBet: number;
	stepBet: number;
	defaultBetLevel: number;
	betLevels: number[];
}

export interface LGSModeInfo {
	name: string;
	cost: number;
	rtp: number;
	hitRate: number;
	maxWin: number;
	outcomes: number;
}

export interface LGSRound {
	betID: number;
	simID: number;
	amount: number;
	payout: number;
	payoutMultiplier: number;
	active: boolean;
	mode: string;
	event?: unknown;
}

export interface LGSAuthResponse {
	balance: LGSBalance;
	round: LGSRound | null;
	config: LGSConfig;
	modes: LGSModeInfo[];
}

export interface LGSPlayResponse {
	balance: LGSBalance;
	round: LGSRound;
}

export interface LGSSessionSummary {
	sessionID: string;
	balance: number;
	currency: string;
	totalBets: number;
	totalWins: number;
	totalWagered: number;
	totalWon: number;
	rtp: number;
	hitRate: number;
	profit: number;
	historySize: number;
	createdAt: string;
	lastActivity: string;
	forcedOutcomes: Record<string, number>;
	rtpBias: number;
}

export interface LGSAggregateStats {
	totalBets: number;
	totalWins: number;
	totalWagered: number;
	totalWon: number;
	overallRTP: number;
	overallHitRate: number;
	totalProfit: number;
}

export interface LGSSessionsResponse {
	sessions: LGSSessionSummary[];
	totalSessions: number;
	totalCreated: number;
	aggregate: LGSAggregateStats;
}

export interface LGSStatsResponse {
	totalBets: number;
	totalWins: number;
	totalWagered: number;
	totalWon: number;
	hitRate: number;
	rtp: number;
	balance: number;
	currency: string;
}

// Batch Play types
export interface LGSBatchPlayRound {
	spinNum: number;
	simID: number;
	payout: number;
	payoutMultiplier: number;
}

export interface LGSBatchPlayResponse {
	sessionID: string;
	mode: string;
	spins: number;
	totalWagered: number;
	totalWon: number;
	hitCount: number;
	hitRate: number;
	rtp: number;
	maxWin: number;
	bigWins: number;
	megaWins: number;
	balance: LGSBalance;
	rounds?: LGSBatchPlayRound[];
	durationMs: number;
}

// Background Loader types
export interface LoaderModeStatus {
	mode: string;
	events_file: string;
	status: 'pending' | 'loading' | 'complete' | 'error';
	current_line: number;
	total_lines?: number;
	bytes_read: number;
	total_bytes: number;
	percent_bytes: number;
	error?: string;
	started_at?: number;
	completed_at?: number;
}

export interface MemoryEstimate {
	compressed_bytes: number;
	estimated_bytes: number;
	estimated_mb: number;
	mode_count: number;
	decompression_ratio: number;
}

export interface LoaderStatusResponse {
	priority: 'low' | 'high';
	started: boolean;
	modes: Record<string, LoaderModeStatus>;
	ws_clients: number;
	memory_estimate: MemoryEstimate;
}

export interface LoaderPriorityResponse {
	priority: 'low' | 'high';
	description: string;
}

export interface LoaderBoostResponse {
	priority: string;
	message: string;
}

// WebSocket message types
export type WSMessageType =
	| 'loading_started'
	| 'loading_progress'
	| 'loading_complete'
	| 'loading_error'
	| 'priority_changed'
	| 'reload_started'
	| 'lgs_session_update'
	| 'lgs_sessions_update'
	| 'crowdsim_progress'
	| 'optimizer_progress';

export interface WSMessage {
	type: WSMessageType;
	mode?: string;
	payload?: unknown;
}

export interface WSLoadingProgress {
	mode: string;
	events_file: string;
	current_line: number;
	total_lines?: number;
	bytes_read: number;
	total_bytes: number;
	percent_bytes: number;
	percent_lines?: number;
	priority: 'low' | 'high';
	elapsed_ms: number;
	estimated_ms?: number;
	lines_per_second: number;
}

export interface WSLoadingStarted {
	mode: string;
	events_file: string;
	total_bytes: number;
}

export interface WSLoadingComplete {
	mode: string;
	total_lines: number;
	total_bytes: number;
	elapsed_ms: number;
	lines_per_sec: number;
}

export interface WSLoadingError {
	mode: string;
	error: string;
}

export interface WSPriorityChanged {
	old_priority: string;
	new_priority: string;
}

// Compliance types
export type ComplianceCheckID =
	| 'rtp_range'
	| 'rtp_variation'
	| 'max_win_achievable'
	| 'hit_rate_reasonable'
	| 'payout_gaps'
	| 'unique_payouts'
	| 'simulation_diversity'
	| 'zero_payout_rate'
	| 'volatility';

export type ComplianceSeverity = 'error' | 'warning' | 'info';

export interface ComplianceCheck {
	id: ComplianceCheckID;
	nameKey: string;
	descriptionKey: string;
	passed: boolean;
	value: string;
	expected: string;
	reasonKey?: string;
	severity: ComplianceSeverity;
	details?: unknown;
}

export interface ComplianceSummary {
	rtp: number;
	hit_rate: number;
	max_payout: number;
	max_payout_hit_rate: number;
	total_outcomes: number;
	unique_payouts: number;
	zero_payout_rate: number;
	volatility: number;
	most_frequent_probability: number;
}

export interface ComplianceResult {
	mode: string;
	passed: boolean;
	passed_count: number;
	failed_count: number;
	warning_count: number;
	checks: ComplianceCheck[];
	summary: ComplianceSummary;
}

export interface AllModesComplianceResult {
	all_passed: boolean;
	mode_results: Record<string, ComplianceResult>;
	global_checks: ComplianceCheck[];
}

// Mode info for cost-aware display
export interface ModeInfo {
	cost: number;
	is_bonus_mode: boolean;
	breakeven_rate: number; // Theoretical P(payout >= cost) from LUT
	simulated_breakeven_rate: number; // Empirical breakeven rate from simulation
	breakeven_rate_deviation: number; // Simulated - Theoretical
	note: string;
	max_payout?: number; // Maximum payout in the mode (normalized by cost)
}

// CrowdSim types
export interface CrowdSimConfig {
	player_count: number;
	spins_per_session: number;
	initial_balance: number;
	bet_amount: number;
	big_win_threshold: number;
	danger_threshold: number;
	use_crypto_rng: boolean;
	streaming_mode: boolean;
	parallel_workers: number;
}

export interface CrowdSimBalanceBucket {
	range_start: number;
	range_end: number;
	count: number;
	percent: number;
}

export interface CrowdSimBalanceStats {
	mean: number;
	median: number;
	std_dev: number;
	min: number;
	max: number;
	percentiles: Record<string, number>;
	distribution?: CrowdSimBalanceBucket[];
}

export interface CrowdSimPeakStats {
	avg_peak: number;
	median_peak: number;
	max_peak: number;
	min_peak: number;
}

export interface CrowdSimDrawdownStats {
	avg_max_drawdown: number;
	median_max_drawdown: number;
	players_below_50pct: number;
	players_below_90pct: number;
	percent_below_50: number;
	percent_below_90: number;
	max_drawdown_observed: number;
}

export interface CrowdSimDangerStats {
	total_danger_events: number;
	players_with_danger: number;
	avg_danger_events: number;
	percent_with_danger: number;
}

export interface CrowdSimStreakStats {
	avg_win_streak: number;
	max_win_streak: number;
	avg_lose_streak: number;
	max_lose_streak: number;
}

export interface CrowdSimBigWinStats {
	avg_spins_to_first: number;
	median_spins_to_first: number;
	players_never_hit: number;
	percent_never_hit: number;
	players_hit: number;
	percent_hit: number;
}

export interface CrowdSimPlayerSummary {
	id: number;
	final_balance: number;
	peak_balance: number;
	min_balance: number;
	max_drawdown: number;
	max_win_streak: number;
	max_lose_streak: number;
	is_profitable: boolean;
	hit_big_win: boolean;
	actual_rtp: number;
}

export type CrowdSimVolatilityProfile = 'low' | 'medium' | 'high';

export interface CrowdSimBalanceCurvePoint {
	spin: number;
	avg: number;
	median: number;
	p5: number;  // 5th percentile (worst players)
	p95: number; // 95th percentile (best players)
}

export interface CrowdSimResult {
	mode: string;
	mode_info?: ModeInfo;
	config: CrowdSimConfig;
	duration_ms: number;

	// RTP Validation
	theoretical_rtp: number;
	actual_rtp: number;
	rtp_deviation: number;

	// Primary Metrics
	final_pop: number;
	pop_curve?: number[];
	balance_curve?: CrowdSimBalanceCurvePoint[];
	balance_stats: CrowdSimBalanceStats;

	// Secondary Metrics
	peak_stats: CrowdSimPeakStats;
	drawdown_stats: CrowdSimDrawdownStats;
	danger_stats: CrowdSimDangerStats;
	streak_stats: CrowdSimStreakStats;
	big_win_stats: CrowdSimBigWinStats;

	// Classification
	volatility_profile: CrowdSimVolatilityProfile;
	composite_score: number;

	// Detailed Data
	player_summaries?: CrowdSimPlayerSummary[];
}

export interface CrowdSimRankedResult {
	mode: string;
	score: number;
	rank: number;
}

export interface CrowdSimCompareResult {
	results: CrowdSimResult[];
	ranking: CrowdSimRankedResult[];
}

export interface CrowdSimPresetInfo {
	name: string;
	description: string;
	config: CrowdSimConfig;
}

export interface CrowdSimProgress {
	mode: string;
	players_complete: number;
	total_players: number;
	percent_complete: number;
	elapsed_ms: number;
}

// ============ Optimizer Types (Simplified) ============

// Volatility presets
export type OptimizerVolatility = 'low' | 'medium' | 'high' | 'very_high';

// Volatility preset info from backend
export interface OptimizerVolatilityPreset {
	name: string;
	description: string;
	exponent: number;
}

// Optimization configuration
export interface OptimizerConfig {
	target_rtp: number;
	rtp_tolerance: number;
	volatility: OptimizerVolatility;
	payout_exponent: number;
	recalculate_weights?: boolean;  // true: recalculate from formula, false: preserve original distribution
	min_payout_for_weight?: number; // Floor for payout in formula (default 1.0)
	save_to_file: boolean;
	create_backup: boolean;
}

// Weight change for a single payout
export interface OptimizerPayoutChange {
	payout: number;
	old_weight: number;
	new_weight: number;
	old_prob: number;
	new_prob: number;
	change_pct: number;
}

// Optimization result
export interface OptimizerResult {
	original_rtp: number;
	final_rtp: number;
	scale_factor: number;
	iterations: number;
	converged: boolean;
	changes_count: number;
	payout_changes: OptimizerPayoutChange[];
	config: {
		target_rtp: number;
		payout_exponent: number;
		volatility: string;
		recalculate_weights: boolean;
		min_payout_for_weight: number;
	};
	save_result?: {
		saved: boolean;
		backup_path?: string;
	};
}

// Backup info
export interface OptimizerBackupInfo {
	filename: string;
	timestamp: string;
	path: string;
}

// ============================================================================
// Bucket Optimizer Types
// ============================================================================

// Constraint type for bucket configuration
export type BucketConstraintType = 'frequency' | 'rtp_percent' | 'auto' | 'max_win_freq' | 'outcome_freq';

// Constraint priority (1=hard, 2=soft)
export type ConstraintPriority = 1 | 2;

// Optimization mode
export type OptimizationMode = 'fast' | 'balanced' | 'precise';

// Configuration for a single payout bucket
export interface BucketConfig {
	name: string;              // Human-readable name (e.g., "small_wins")
	min_payout: number;        // Minimum payout in range (inclusive)
	max_payout: number;        // Maximum payout in range (exclusive)
	type: BucketConstraintType; // "frequency", "rtp_percent", "auto", "max_win_freq", "outcome_freq"
	frequency?: number;        // 1 in N spins (e.g., 20 = 1 in 20 spins)
	rtp_percent?: number;      // % of total RTP (e.g., 0.5 = 0.5% of RTP)
	auto_exponent?: number;    // For auto: weight ∝ 1/payout^exponent (default 1.0)
	max_win_frequency?: number; // For max_win_freq: frequency of the max payout in this bucket (1 in N)
	priority?: ConstraintPriority; // 1=hard, 2=soft constraint
}

// Request for bucket-based optimization
export interface BucketOptimizeRequest {
	target_rtp: number;
	rtp_tolerance?: number;
	buckets: BucketConfig[];
	save_to_file?: boolean;
	create_backup?: boolean;
	enable_brute_force?: boolean;   // Enable iterative brute force search
	max_iterations?: number;        // Max iterations for brute force
	optimization_mode?: OptimizationMode; // "fast"/"balanced"/"precise"
	global_max_win_freq?: number;   // Global max win frequency (1 in N)
	// Voiding options (deprecated - use enable_auto_voiding)
	enable_voiding?: boolean;       // DEPRECATED: Enable voiding mode to exclude buckets
	voided_bucket_indices?: number[]; // DEPRECATED: Indices of buckets to void
	// Auto-voiding (recommended)
	enable_auto_voiding?: boolean;  // Enable automatic outcome voiding to reach target RTP
}

// Result for a single bucket after optimization
export interface BucketResult {
	name: string;
	min_payout: number;
	max_payout: number;
	outcome_count: number;
	target_probability: number;
	actual_probability: number;
	target_frequency: number;   // 1 in N (derived)
	actual_frequency: number;   // 1 in N (achieved)
	rtp_contribution: number;   // % of RTP this bucket contributes
	total_weight: number;
	avg_payout: number;
}

// Outcome detail showing bucket assignment
export interface BucketOutcomeDetail {
	sim_id: number;
	payout: number;
	old_weight: number;
	new_weight: number;
	bucket_name: string;
	probability: number;
}

// Information about a voided bucket
export interface VoidedBucketInfo {
	index: number;           // Bucket index
	name: string;            // Bucket name
	outcome_count: number;   // Number of outcomes voided
	rtp_contribution: number; // RTP contribution that was removed
}

// Information about a single auto-voided outcome
export interface VoidedOutcomeInfo {
	sim_id: number;          // Simulation ID of the voided outcome
	payout: number;          // Payout value of the voided outcome
	reason: string;          // Why it was voided: "duplicate" or "high_payout"
	rtp_loss: number;        // RTP contribution lost by voiding this outcome
}

// Full result from bucket optimization
export interface BucketOptimizeResult {
	original_rtp: number;
	final_rtp: number;
	target_rtp: number;
	converged: boolean;
	total_weight: number;
	bucket_results: BucketResult[];
	loss_result: BucketResult | null;
	warnings?: string[];
	outcome_details?: BucketOutcomeDetail[];
	mode_info?: ModeInfo;
	voided_buckets?: VoidedBucketInfo[]; // Buckets that were voided (deprecated)
	// Auto-voiding results
	voided_outcomes?: VoidedOutcomeInfo[]; // Individual outcomes that were auto-voided
	total_voided?: number;                 // Total count of voided outcomes
	voided_rtp?: number;                   // Total RTP removed by voiding
	config: {
		target_rtp: number;
		buckets: BucketConfig[];
	};
	save_result?: {
		saved: boolean;
		backup_path?: string;
	};
}

// Suggested buckets response
export interface SuggestBucketsResponse {
	suggested_buckets: BucketConfig[];
	table_stats: {
		outcome_count: number;
		max_payout: number;
		min_payout: number;
		payout_counts: Record<string, number>;
		current_rtp: number;
	};
	mode_info?: ModeInfo;
}

// Bucket presets response
export interface BucketPresetsResponse {
	default: BucketConfig[];
	conservative: BucketConfig[];
	aggressive: BucketConfig[];
}

// Bucket Distribution API types
export interface BucketDistributionResponse {
	range_start: number;
	range_end: number;
	items: DistributionItem[];
	total: number;
	offset: number;
	limit: number;
	has_more: boolean;
}

// Brute force optimization info
export interface BruteForceInfo {
	iterations: number;      // Total iterations performed
	search_duration: number; // Search duration in ms
	final_error: number;     // Final RTP error
}

// Extended bucket optimize result with brute force info
export interface BucketOptimizeResultExtended extends BucketOptimizeResult {
	brute_force_info?: BruteForceInfo;
}

// WebSocket progress message for brute force optimization
export interface WSOptimizerProgress {
	type: 'progress';
	phase: 'init' | 'search' | 'refine' | 'complete';
	iteration: number;
	max_iter: number;
	current_rtp: number;
	target_rtp: number;
	error: number;
	converged: boolean;
	elapsed_ms: number;
}

// WebSocket result message
export interface WSOptimizerResult {
	type: 'result';
	result: BucketOptimizeResultExtended;
}

// WebSocket error message
export interface WSOptimizerError {
	type: 'error';
	message: string;
}

// Union type for all WebSocket optimizer messages
export type WSOptimizerMessage = WSOptimizerProgress | WSOptimizerResult | WSOptimizerError;

// ============================================================================
// Convex Optimizer Types (CVXPY-based optimization)
// ============================================================================

export type ConvexDistributionType = 'log_normal' | 'gaussian' | 'exponential';

export interface ConvexDistributionParams {
	type: ConvexDistributionType;
	mode?: number;
	std?: number;
	mean?: number;
	power?: number;
	scale: number;
}

export interface ConvexCriteriaConfig {
	name: string;
	rtp: number;
	hit_rate: number;
	average_win?: number;
	distribution: ConvexDistributionParams;
	mix_distribution?: ConvexDistributionParams;
	mix_weight: number;
}

export interface ConvexOptimizerSettings {
	kl_divergence_weight: number;
	smoothness_weight: number;
}

export interface ConvexOptimizeRequest {
	mode: string;
	cost: number;
	criteria: ConvexCriteriaConfig[];
	optimizer_settings: ConvexOptimizerSettings[];
	weight_scale: number;
	lookup_file: string;
	segmented_file: string;
	win_step_size: number;
	excluded_payouts: number[];
	save_to_file: boolean;
	create_backup: boolean;
}

export interface ConvexHitRateRange {
	range_start: number;
	range_end: number;
	hit_rate: number;
}

export interface ConvexPlotPoint {
	x: number;
	y: number;
}

export interface ConvexPlotData {
	actual_points: ConvexPlotPoint[];
	theoretical_curve: ConvexPlotPoint[];
	solution_curve: ConvexPlotPoint[];
	x_label: string;
	y_label: string;
	x_min: number;
	x_max: number;
	y_min: number;
	y_max: number;
}

export interface ConvexCriteriaSolution {
	name: string;
	target_rtp: number;
	achieved_rtp: number;
	target_hit_rate: number;
	achieved_hit_rate: number;
	solved_weights: number[];
	unique_payout_count: number;
	distribution_type: string;
	hit_rate_ranges: ConvexHitRateRange[];
	solution_metrics: Record<string, number>;
	plot_data?: ConvexPlotData;
}

export interface ConvexLookupEntry {
	sim_id: number;
	weight: number;
	payout: number;
}

export interface ConvexSaveResult {
	saved: boolean;
	lookup_path?: string;
	hitrate_path?: string;
	backup_path?: string;
}

export interface ConvexOptimizeResponse {
	success: boolean;
	mode: string;
	original_rtp: number;
	final_rtp: number;
	criteria_solutions: ConvexCriteriaSolution[];
	final_lookup: ConvexLookupEntry[];
	hit_rate_summary: ConvexHitRateRange[];
	zero_weight_probability: number;
	total_lookup_length: number;
	warnings: string[];
	save_result?: ConvexSaveResult;
}

export interface ConvexHealthResponse {
	status: string;
	service: string;
	version: string;
}

export interface ConvexModeInfoResponse {
	mode: string;
	cost: number;
	criteria_names: string[];
	lookup_file: string;
	segmented_file: string;
	is_bonus_mode: boolean;
}

// ============================================================================
// Mode Analyzer Types
// ============================================================================

export type ModeTypeClassification =
	| 'standard'      // Standard slots ~96% RTP
	| 'bonus_narrow'  // Bonus with narrow payout range
	| 'bonus_wide'    // Bonus with wide payout range
	| 'high_rtp'      // RTP > 200%
	| 'extreme';      // RTP > 1000%

export interface BucketRecommendation {
	min_payout: number;
	max_payout: number;
	outcome_count: number;
	rtp_capacity: number;
	avg_payout: number;
	suggested_rtp: number;
	description: string;
}

export interface ModeAnalysis {
	mode: string;
	mode_type: ModeTypeClassification;

	// LUT statistics
	total_outcomes: number;
	min_payout: number;
	max_payout: number;
	avg_payout: number;
	payout_variance: number;
	payout_std_dev: number;

	// Payout distribution percentiles
	percentiles: Record<string, number>;

	// RTP boundaries
	min_achievable_rtp: number;
	max_achievable_rtp: number;

	// Mode info
	cost: number;
	is_bonus_mode: boolean;

	// Recommendations
	recommended_buckets: BucketRecommendation[];
	feasible: boolean;
	feasibility_note?: string;
	suggested_rtp?: number;
}

export interface FeasibilityInfo {
	original: number;
	effective: number;
	was_adjusted: boolean;
	min_possible: number;
	max_possible: number;
}

// Void suggestion for when RTP is unreachable
export interface VoidSuggestion {
	index: number;           // Bucket index
	name: string;            // Bucket name
	min_payout: number;      // Min payout of bucket
	max_payout: number;      // Max payout of bucket
	outcome_count: number;   // Number of outcomes in bucket
	rtp_contribution: number; // RTP contribution of this bucket
	priority: number;        // Voiding priority (1 = void first)
}

export interface GenerateConfigsAnalysis {
	mode_type: ModeTypeClassification;
	feasible: boolean;
	feasibility_note?: string;
	min_achievable_rtp: number;
	max_achievable_rtp: number;
	suggested_rtp?: number;
	is_bonus_mode: boolean;
	suggested_void_buckets?: VoidSuggestion[]; // Suggestions for voiding when RTP unreachable
}
