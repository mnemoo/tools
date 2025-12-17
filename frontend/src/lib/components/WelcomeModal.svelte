<script lang="ts">
	const LGS_HTTPS_URL = 'https://localhost:7755';
	const STORAGE_KEY = 'lut_explorer_cert_accepted';
	const CHECK_INTERVAL = 30000; // Check every 30 seconds

	interface Props {
		onReady: () => void;
	}

	let { onReady }: Props = $props();

	let visible = $state(false);
	let checking = $state(false);
	let certStatus = $state<'unknown' | 'ok' | 'error'>('unknown');
	let errorMessage = $state<string | null>(null);
	let isFirstVisit = $state(true);

	// Check if user has visited before
	function hasVisitedBefore(): boolean {
		if (typeof window === 'undefined') return false;
		return localStorage.getItem(STORAGE_KEY) === 'true';
	}

	// Mark as visited
	function markVisited() {
		if (typeof window === 'undefined') return;
		localStorage.setItem(STORAGE_KEY, 'true');
	}

	// Clear visited status (for re-showing modal on cert issues)
	function clearVisited() {
		if (typeof window === 'undefined') return;
		localStorage.removeItem(STORAGE_KEY);
	}

	// Check if HTTPS server is accessible via /lgs/health endpoint
	async function checkConnection(): Promise<boolean> {
		try {
			const controller = new AbortController();
			const timeout = setTimeout(() => controller.abort(), 5000);

			// Try to fetch LGS health endpoint
			const response = await fetch(`${LGS_HTTPS_URL}/lgs/health`, {
				method: 'GET',
				signal: controller.signal
			});

			clearTimeout(timeout);

			// Any response (even error) means certificate is trusted
			return true;
		} catch (e) {
			// Network error = certificate not trusted or server down
			return false;
		}
	}

	// Run verification check (user clicked button)
	async function runVerification() {
		checking = true;
		errorMessage = null;

		try {
			const ok = await checkConnection();
			if (ok) {
				certStatus = 'ok';
				markVisited();
				// Small delay to show success state
				setTimeout(() => {
					visible = false;
					onReady();
				}, 500);
			} else {
				certStatus = 'error';
				errorMessage = 'Cannot connect to LGS server. Please trust the certificate first.';
			}
		} catch (e) {
			certStatus = 'error';
			errorMessage = e instanceof Error ? e.message : 'Connection check failed';
		} finally {
			checking = false;
		}
	}

	// Open LGS HTTPS URL in new tab for user to trust certificate
	function openLgsUrl() {
		window.open(LGS_HTTPS_URL, '_blank');
	}

	// Skip welcome (for users who don't need LGS)
	function skip() {
		markVisited();
		visible = false;
		onReady();
	}

	// Show modal (called externally or when cert fails)
	export function show() {
		visible = true;
		certStatus = 'unknown';
		errorMessage = null;
	}

	// Background check function (exported for parent to call periodically)
	export async function backgroundCheck(): Promise<boolean> {
		// Only check if user has visited before (not first time)
		if (!hasVisitedBefore()) return true;

		const ok = await checkConnection();
		if (!ok) {
			// Certificate issue detected - show modal
			clearVisited();
			show();
			return false;
		}
		return true;
	}

	// Initialize on mount
	$effect(() => {
		isFirstVisit = !hasVisitedBefore();

		if (isFirstVisit) {
			// First visit - always show welcome modal
			visible = true;
		} else {
			// Returning user - check in background
			(async () => {
				const ok = await checkConnection();
				if (ok) {
					certStatus = 'ok';
					onReady();
				} else {
					// Cert issue - show modal
					show();
				}
			})();
		}
	});
</script>

{#if visible}
	<div class="fixed inset-0 z-[100] flex items-center justify-center">
		<!-- Backdrop -->
		<div class="absolute inset-0 bg-black/80 backdrop-blur-md"></div>

		<!-- Modal -->
		<div class="relative bg-[var(--color-graphite)] rounded-2xl shadow-2xl border border-white/10 w-full max-w-xl mx-4 overflow-hidden">
			<!-- Header with gradient -->
			<div class="relative px-8 pt-8 pb-6 bg-gradient-to-br from-[var(--color-cyan)]/20 via-transparent to-[var(--color-violet)]/10">
				<div class="flex items-center gap-4">
					<div class="w-14 h-14 rounded-2xl bg-[var(--color-cyan)]/20 flex items-center justify-center border border-[var(--color-cyan)]/30">
						<svg class="w-7 h-7 text-[var(--color-cyan)]" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
							<path stroke-linecap="round" stroke-linejoin="round" d="M9 12.75L11.25 15 15 9.75m-3-7.036A11.959 11.959 0 013.598 6 11.99 11.99 0 003 9.749c0 5.592 3.824 10.29 9 11.623 5.176-1.332 9-6.03 9-11.622 0-1.31-.21-2.571-.598-3.751h-.152c-3.196 0-6.1-1.248-8.25-3.285z" />
						</svg>
					</div>
					<div>
						<h2 class="font-display text-2xl text-[var(--color-light)] tracking-wider">
							{#if isFirstVisit}
								WELCOME TO LUT EXPLORER
							{:else}
								CONNECTION ISSUE
							{/if}
						</h2>
						<p class="text-sm font-mono text-[var(--color-mist)] mt-1">
							{#if isFirstVisit}
								Setup LGS Certificate
							{:else}
								LGS Certificate needs re-authorization
							{/if}
						</p>
					</div>
				</div>
			</div>

			<!-- Content -->
			<div class="px-8 py-6 space-y-6">
				<!-- Explanation -->
				<div class="text-sm text-[var(--color-light)]/80 leading-relaxed">
					{#if isFirstVisit}
						<p>
							LUT Explorer includes a <strong class="text-[var(--color-cyan)]">Local Game Server (LGS)</strong> that runs on HTTPS for secure testing.
							Your browser needs to trust the self-signed certificate to use LGS features.
						</p>
					{:else}
						<p>
							The LGS server certificate may have been updated or your browser's trust has expired.
							Please re-authorize the certificate to continue using LGS features.
						</p>
					{/if}
				</div>

				<!-- Status Card -->
				<div class="bg-[var(--color-slate)]/50 rounded-xl p-5 border border-white/5">
					<div class="flex items-center justify-between mb-4">
						<div class="flex items-center gap-3">
							<div class="w-3 h-3 rounded-full {certStatus === 'ok' ? 'bg-emerald-400' : certStatus === 'error' ? 'bg-red-400 animate-pulse' : 'bg-[var(--color-mist)]'}"></div>
							<span class="font-mono text-sm text-[var(--color-light)]">
								{#if certStatus === 'ok'}
									Connection Verified
								{:else if certStatus === 'error'}
									Certificate Not Trusted
								{:else}
									Awaiting Verification
								{/if}
							</span>
						</div>
						<span class="text-xs font-mono text-[var(--color-mist)]">{LGS_HTTPS_URL}</span>
					</div>

					{#if certStatus !== 'ok'}
						<!-- Instructions -->
						<div class="space-y-3">
							<div class="flex items-start gap-3 text-sm">
								<span class="w-6 h-6 rounded-lg bg-[var(--color-cyan)]/20 text-[var(--color-cyan)] flex items-center justify-center text-xs font-bold shrink-0">1</span>
								<p class="text-[var(--color-light)]/70">Click the button below to open the LGS server in a new tab</p>
							</div>
							<div class="flex items-start gap-3 text-sm">
								<span class="w-6 h-6 rounded-lg bg-[var(--color-cyan)]/20 text-[var(--color-cyan)] flex items-center justify-center text-xs font-bold shrink-0">2</span>
								<p class="text-[var(--color-light)]/70">In the browser warning, click <strong class="text-[var(--color-light)]">"Advanced"</strong> then <strong class="text-[var(--color-light)]">"Proceed to localhost"</strong></p>
							</div>
							<div class="flex items-start gap-3 text-sm">
								<span class="w-6 h-6 rounded-lg bg-[var(--color-cyan)]/20 text-[var(--color-cyan)] flex items-center justify-center text-xs font-bold shrink-0">3</span>
								<p class="text-[var(--color-light)]/70">Return here and click <strong class="text-[var(--color-light)]">"Verify Connection"</strong></p>
							</div>
						</div>

						<!-- Open URL Button -->
						<button
							onclick={openLgsUrl}
							class="mt-4 w-full px-4 py-3 rounded-xl bg-[var(--color-gold)] text-[var(--color-void)] font-mono font-semibold text-sm hover:bg-[var(--color-gold)]/90 transition-colors flex items-center justify-center gap-2"
						>
							<svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
								<path stroke-linecap="round" stroke-linejoin="round" d="M13.5 6H5.25A2.25 2.25 0 003 8.25v10.5A2.25 2.25 0 005.25 21h10.5A2.25 2.25 0 0018 18.75V10.5m-10.5 6L21 3m0 0h-5.25M21 3v5.25" />
							</svg>
							OPEN LGS SERVER
						</button>
					{:else}
						<div class="flex items-center gap-2 text-emerald-400 text-sm font-mono">
							<svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
								<path stroke-linecap="round" stroke-linejoin="round" d="M9 12.75L11.25 15 15 9.75M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
							</svg>
							Connection verified successfully!
						</div>
					{/if}
				</div>

				{#if errorMessage}
					<div class="p-4 rounded-xl bg-red-500/10 border border-red-500/20 text-red-400 text-sm font-mono">
						{errorMessage}
					</div>
				{/if}
			</div>

			<!-- Footer -->
			<div class="px-8 py-5 border-t border-white/10 flex items-center justify-between gap-3">
				<button
					onclick={skip}
					class="px-4 py-2.5 rounded-xl font-mono text-sm text-[var(--color-mist)] hover:text-[var(--color-light)] hover:bg-white/5 transition-colors"
				>
					Skip for now
				</button>
				<button
					onclick={runVerification}
					disabled={checking || certStatus === 'ok'}
					class="px-6 py-2.5 rounded-xl font-mono font-semibold text-sm transition-colors disabled:opacity-50 disabled:cursor-not-allowed bg-[var(--color-cyan)] text-[var(--color-void)] hover:bg-[var(--color-cyan)]/90 flex items-center gap-2"
				>
					{#if checking}
						<svg class="w-4 h-4 animate-spin" fill="none" viewBox="0 0 24 24">
							<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
							<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
						</svg>
						VERIFYING...
					{:else if certStatus === 'ok'}
						<svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
							<path stroke-linecap="round" stroke-linejoin="round" d="M4.5 12.75l6 6 9-13.5" />
						</svg>
						CONNECTED
					{:else}
						<svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
							<path stroke-linecap="round" stroke-linejoin="round" d="M9 12.75L11.25 15 15 9.75M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
						</svg>
						VERIFY CONNECTION
					{/if}
				</button>
			</div>
		</div>
	</div>
{/if}
