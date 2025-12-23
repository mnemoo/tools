// API Client for LUT Explorer Backend

import type {
	ApiResponse,
	IndexInfo,
	ModeSummary,
	Statistics,
	DistributionItem,
	Outcome,
	CompareResponse,
	EventLoadResult,
	EventInfo,
	LGSAuthResponse,
	LGSPlayResponse,
	LGSSessionsResponse,
	LGSStatsResponse,
	LGSRound,
	LGSBatchPlayResponse,
	LoaderStatusResponse,
	LoaderPriorityResponse,
	LoaderBoostResponse,
	ComplianceResult,
	AllModesComplianceResult,
	CrowdSimConfig,
	CrowdSimResult,
	CrowdSimCompareResult,
	CrowdSimPresetInfo,
	OptimizerConfig,
	OptimizerResult,
	BucketDistributionResponse
} from './types';

const DEFAULT_BASE_URL = 'http://localhost:7754';
const DEFAULT_LGS_URL = 'http://localhost:7754';

class LutApiClient {
	private baseUrl: string;

	constructor(baseUrl: string = DEFAULT_BASE_URL) {
		this.baseUrl = baseUrl;
	}

	private async fetch<T>(endpoint: string): Promise<T> {
		const response = await fetch(`${this.baseUrl}${endpoint}`);
		const data: ApiResponse<T> = await response.json();

		if (!data.success) {
			throw new Error(data.error || 'Unknown error');
		}

		return data.data as T;
	}

	async health(): Promise<{ status: string }> {
		return this.fetch('/api/health');
	}

	async getIndex(): Promise<IndexInfo> {
		return this.fetch('/api/index');
	}

	async getModes(): Promise<ModeSummary[]> {
		return this.fetch('/api/modes');
	}

	async getMode(mode: string): Promise<ModeSummary> {
		return this.fetch(`/api/mode/${encodeURIComponent(mode)}`);
	}

	async getModeStats(mode: string): Promise<Statistics> {
		return this.fetch(`/api/mode/${encodeURIComponent(mode)}/stats`);
	}

	async getModeDistribution(mode: string): Promise<DistributionItem[]> {
		return this.fetch(`/api/mode/${encodeURIComponent(mode)}/distribution`);
	}

	async getModeBucketDistribution(
		mode: string,
		rangeStart: number,
		rangeEnd: number,
		offset: number = 0,
		limit: number = 100
	): Promise<BucketDistributionResponse> {
		const params = new URLSearchParams({
			range_start: rangeStart.toString(),
			range_end: rangeEnd.toString(),
			offset: offset.toString(),
			limit: limit.toString()
		});
		return this.fetch(`/api/mode/${encodeURIComponent(mode)}/distribution/bucket?${params}`);
	}

	async getModeOutcomes(mode: string): Promise<Outcome[]> {
		return this.fetch(`/api/mode/${encodeURIComponent(mode)}/outcomes`);
	}

	async compare(modes?: string[]): Promise<CompareResponse> {
		const params = modes?.map((m) => `mode=${encodeURIComponent(m)}`).join('&');
		const endpoint = params ? `/api/compare?${params}` : '/api/compare';
		return this.fetch(endpoint);
	}

	async loadEvents(mode: string): Promise<EventLoadResult> {
		return this.post(`/api/mode/${encodeURIComponent(mode)}/events/load`);
	}

	async getEvent(mode: string, simId: number): Promise<EventInfo> {
		return this.fetch(`/api/mode/${encodeURIComponent(mode)}/event/${simId}`);
	}

	private async post<T>(endpoint: string): Promise<T> {
		const response = await fetch(`${this.baseUrl}${endpoint}`, {
			method: 'POST'
		});
		const data: ApiResponse<T> = await response.json();

		if (!data.success) {
			throw new Error(data.error || 'Unknown error');
		}

		return data.data as T;
	}

	private async postJson<T>(endpoint: string, body: unknown): Promise<T> {
		const response = await fetch(`${this.baseUrl}${endpoint}`, {
			method: 'POST',
			headers: {
				'Content-Type': 'application/json'
			},
			body: JSON.stringify(body)
		});
		const data: ApiResponse<T> = await response.json();

		if (!data.success) {
			throw new Error(data.error || 'Unknown error');
		}

		return data.data as T;
	}

	setBaseUrl(url: string) {
		this.baseUrl = url;
	}

	getBaseUrl(): string {
		return this.baseUrl;
	}

	// ============ LGS (Local Game Server) Methods ============

	// LGS responses don't use the ApiResponse wrapper
	private async lgsPost<T>(endpoint: string, body?: unknown): Promise<T> {
		const response = await fetch(`${this.baseUrl}${endpoint}`, {
			method: 'POST',
			headers: {
				'Content-Type': 'application/json'
			},
			body: body ? JSON.stringify(body) : undefined
		});
		return response.json();
	}

	private async lgsGet<T>(endpoint: string): Promise<T> {
		const response = await fetch(`${this.baseUrl}${endpoint}`);
		return response.json();
	}

	private async lgsDelete<T>(endpoint: string): Promise<T> {
		const response = await fetch(`${this.baseUrl}${endpoint}`, {
			method: 'DELETE'
		});
		return response.json();
	}

	// Wallet endpoints (RGS-compatible)
	async lgsAuthenticate(sessionID: string, language: string = 'en'): Promise<LGSAuthResponse> {
		return this.lgsPost('/wallet/authenticate', { sessionID, language });
	}

	async lgsPlay(options: {
		sessionID: string;
		mode: string;
		amount: number;
		currency?: string;
	}): Promise<LGSPlayResponse> {
		return this.lgsPost('/wallet/play', {
			sessionID: options.sessionID,
			mode: options.mode,
			amount: options.amount,
			currency: options.currency || 'USD'
		});
	}

	async lgsEndRound(sessionID: string): Promise<{ balance: { amount: number; currency: string }; round: LGSRound | null }> {
		return this.lgsPost('/wallet/end-round', { sessionID });
	}

	// LGS utility endpoints
	async lgsSessions(): Promise<LGSSessionsResponse> {
		return this.lgsGet('/lgs/sessions');
	}

	async lgsStats(sessionID: string): Promise<LGSStatsResponse> {
		return this.lgsGet(`/lgs/stats?sessionID=${encodeURIComponent(sessionID)}`);
	}

	async lgsHistory(sessionID: string, limit: number = 50): Promise<{ rounds: LGSRound[]; balance: { amount: number; currency: string } }> {
		return this.lgsPost('/lgs/history', { sessionID, limit });
	}

	async lgsResetBalance(sessionID: string): Promise<{ success: boolean; balance: { amount: number; currency: string } }> {
		return this.lgsPost('/lgs/reset-balance', { sessionID });
	}

	async lgsSetBalance(sessionID: string, balance: number, currency?: string): Promise<{ success: boolean; balance: { amount: number; currency: string } }> {
		return this.lgsPost('/lgs/set-balance', { sessionID, balance, currency });
	}

	async lgsClearHistory(sessionID: string): Promise<{ success: boolean; message: string }> {
		return this.lgsDelete(`/lgs/history?sessionID=${encodeURIComponent(sessionID)}`);
	}

	async lgsClearStats(sessionID: string): Promise<{ success: boolean; message: string }> {
		return this.lgsDelete(`/lgs/stats?sessionID=${encodeURIComponent(sessionID)}`);
	}

	async lgsBatchPlay(options: {
		sessionID: string;
		mode: string;
		amount: number;
		spins: number;
		currency?: string;
	}): Promise<LGSBatchPlayResponse> {
		return this.lgsPost('/lgs/batchplay', {
			sessionID: options.sessionID,
			mode: options.mode,
			amount: options.amount,
			spins: options.spins,
			currency: options.currency || 'USD'
		});
	}

	async lgsForceOutcome(sessionID: string, mode: string, simID: number): Promise<{
		success: boolean;
		message: string;
		mode: string;
		simID: number;
		payout: number;
	}> {
		return this.lgsPost('/lgs/force-outcome', { sessionID, mode, simID });
	}

	async lgsGetForcedOutcomes(sessionID: string): Promise<{
		sessionID: string;
		forcedOutcomes: Record<string, number>;
	}> {
		return this.lgsGet(`/lgs/force-outcome?sessionID=${encodeURIComponent(sessionID)}`);
	}

	async lgsClearForcedOutcome(sessionID: string, mode?: string): Promise<{ success: boolean; message: string }> {
		const params = new URLSearchParams({ sessionID });
		if (mode) params.set('mode', mode);
		return this.lgsDelete(`/lgs/force-outcome?${params.toString()}`);
	}

	async lgsSetRTPBias(sessionID: string, bias: number): Promise<{
		success: boolean;
		message: string;
		sessionID: string;
		bias: number;
	}> {
		return this.lgsPost('/lgs/rtp-bias', { sessionID, bias });
	}

	async lgsGetRTPBias(sessionID: string): Promise<{
		sessionID: string;
		bias: number;
	}> {
		return this.lgsGet(`/lgs/rtp-bias?sessionID=${encodeURIComponent(sessionID)}`);
	}

	// ============ Background Loader Methods ============

	async loaderStatus(): Promise<LoaderStatusResponse> {
		return this.fetch('/api/loader/status');
	}

	async loaderPriority(): Promise<LoaderPriorityResponse> {
		return this.fetch('/api/loader/priority');
	}

	async loaderBoost(): Promise<LoaderBoostResponse> {
		return this.post('/api/loader/boost');
	}

	async loaderUnboost(): Promise<LoaderBoostResponse> {
		const response = await fetch(`${this.baseUrl}/api/loader/boost`, {
			method: 'DELETE'
		});
		const data: ApiResponse<LoaderBoostResponse> = await response.json();
		if (!data.success) {
			throw new Error(data.error || 'Unknown error');
		}
		return data.data as LoaderBoostResponse;
	}

	async reload(): Promise<{ message: string }> {
		return this.post('/api/reload');
	}

	// WebSocket URL
	getWebSocketUrl(): string {
		const url = new URL(this.baseUrl);
		const wsProtocol = url.protocol === 'https:' ? 'wss:' : 'ws:';
		return `${wsProtocol}//${url.host}/ws`;
	}

	// ============ Compliance Methods ============

	async getModeCompliance(mode: string): Promise<ComplianceResult> {
		return this.fetch(`/api/mode/${encodeURIComponent(mode)}/compliance`);
	}

	async getAllCompliance(): Promise<AllModesComplianceResult> {
		return this.fetch('/api/compliance');
	}

	// ============ CrowdSim Methods ============

	async crowdsimSimulate(mode: string, config?: Partial<CrowdSimConfig>): Promise<CrowdSimResult> {
		return this.postJson(`/api/crowdsim/${encodeURIComponent(mode)}/simulate`, config || {});
	}

	async crowdsimCompare(modes: string[], config?: Partial<CrowdSimConfig>): Promise<CrowdSimCompareResult> {
		return this.postJson('/api/crowdsim/compare', { modes, config: config || {} });
	}

	async crowdsimPresets(): Promise<CrowdSimPresetInfo[]> {
		return this.fetch('/api/crowdsim/presets');
	}

	async crowdsimValidate(mode: string, config?: Partial<CrowdSimConfig>): Promise<{
		result: CrowdSimResult;
		validation: {
			mode: string;
			theoretical_rtp: number;
			actual_rtp: number;
			deviation: number;
			deviation_pct: number;
			is_valid: boolean;
			tolerance: number;
		};
	}> {
		return this.postJson(`/api/crowdsim/${encodeURIComponent(mode)}/validate`, config || {});
	}

	async crowdsimVolatilityCheck(mode: string, config?: Partial<CrowdSimConfig>, profile?: string): Promise<{
		result: CrowdSimResult;
		target_profile: string;
		thresholds: unknown;
		checks: Record<string, boolean>;
		passed: number;
		total: number;
		compliant: boolean;
	}> {
		return this.postJson(`/api/crowdsim/${encodeURIComponent(mode)}/volatility-check`, {
			config: config || {},
			profile: profile || 'medium'
		});
	}

	// ============ Optimizer Methods (Simplified) ============

	/**
	 * Run optimization on a mode
	 * Recalculates all weights from scratch using formula: weight = BaseWeight / payout^exponent
	 */
	async optimizerOptimize(mode: string, config?: Partial<OptimizerConfig>): Promise<OptimizerResult> {
		return this.postJson(`/api/optimizer/${encodeURIComponent(mode)}/optimize`, config || {});
	}

	/**
	 * Get available volatility presets
	 */
	async optimizerPresets(): Promise<Array<{ name: string; description: string; exponent: number }>> {
		return this.fetch('/api/optimizer/presets');
	}

	/**
	 * Apply weights to a mode
	 */
	async optimizerApply(mode: string, weights: number[], createBackup?: boolean): Promise<{ saved: boolean; backup_path?: string }> {
		return this.postJson(`/api/optimizer/${encodeURIComponent(mode)}/apply`, {
			weights,
			create_backup: createBackup ?? true
		});
	}

	/**
	 * Get list of backups for a mode
	 */
	async optimizerBackups(mode: string): Promise<Array<{ filename: string; timestamp: string; path: string }>> {
		return this.fetch(`/api/optimizer/${encodeURIComponent(mode)}/backups`);
	}

	/**
	 * Restore weights from a backup
	 */
	async optimizerRestore(mode: string, backupFile: string, createBackup?: boolean): Promise<{ restored: boolean; message: string }> {
		return this.postJson(`/api/optimizer/${encodeURIComponent(mode)}/restore`, {
			backup_file: backupFile,
			create_backup: createBackup ?? true
		});
	}

	// ============ Bucket Optimizer Methods ============

	/**
	 * Run bucket-based optimization on a mode
	 * Allows specifying frequency (1 in N) or RTP% for each payout range
	 */
	async bucketOptimize(mode: string, config: {
		target_rtp?: number;
		rtp_tolerance?: number;
		buckets?: Array<{
			name: string;
			min_payout: number;
			max_payout: number;
			type: 'frequency' | 'rtp_percent' | 'auto';
			frequency?: number;
			rtp_percent?: number;
			auto_exponent?: number;
		}>;
		save_to_file?: boolean;
		create_backup?: boolean;
	}): Promise<{
		original_rtp: number;
		final_rtp: number;
		target_rtp: number;
		converged: boolean;
		total_weight: number;
		bucket_results: Array<{
			name: string;
			min_payout: number;
			max_payout: number;
			outcome_count: number;
			actual_probability: number;
			actual_frequency: number;
			rtp_contribution: number;
		}>;
		loss_result: {
			name: string;
			min_payout: number;
			max_payout: number;
			outcome_count: number;
			actual_probability: number;
			actual_frequency: number;
			rtp_contribution: number;
		} | null;
		warnings?: string[];
		config: {
			target_rtp: number;
			buckets: Array<{
				name: string;
				min_payout: number;
				max_payout: number;
				type: string;
				frequency?: number;
				rtp_percent?: number;
			}>;
		};
		save_result?: { saved: boolean; backup_path?: string };
	}> {
		return this.postJson(`/api/optimizer/${encodeURIComponent(mode)}/bucket-optimize`, config);
	}

	/**
	 * Get suggested bucket configuration for a mode
	 */
	async suggestBuckets(mode: string, targetRtp?: number): Promise<{
		suggested_buckets: Array<{
			name: string;
			min_payout: number;
			max_payout: number;
			type: 'frequency' | 'rtp_percent';
			frequency?: number;
			rtp_percent?: number;
		}>;
		table_stats: {
			outcome_count: number;
			max_payout: number;
			min_payout: number;
			payout_counts: Record<string, number>;
			current_rtp: number;
		};
	}> {
		const params = targetRtp ? `?target_rtp=${targetRtp}` : '';
		return this.fetch(`/api/optimizer/${encodeURIComponent(mode)}/suggest-buckets${params}`);
	}

	/**
	 * Get bucket optimizer presets
	 */
	async bucketPresets(): Promise<Record<string, Array<{
		name: string;
		min_payout: number;
		max_payout: number;
		type: 'frequency' | 'rtp_percent';
		frequency?: number;
		rtp_percent?: number;
	}>>> {
		return this.fetch('/api/optimizer/bucket-presets');
	}
}

export const api = new LutApiClient();
export { LutApiClient };
