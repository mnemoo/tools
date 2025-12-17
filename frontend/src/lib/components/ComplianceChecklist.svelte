<script lang="ts">
	import { onMount } from 'svelte';
	import { api, type AllModesComplianceResult } from '$lib/api';

	let loading = $state(true);
	let error = $state<string | null>(null);
	let compliance = $state<AllModesComplianceResult | null>(null);

	onMount(() => {
		loadCompliance();
	});

	async function loadCompliance() {
		loading = true;
		error = null;
		try {
			compliance = await api.getAllCompliance();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load compliance data';
		} finally {
			loading = false;
		}
	}

	function getIcon(passed: boolean, severity: string) {
		if (passed) return { color: 'text-emerald-400', bg: 'bg-emerald-500/20' };
		if (severity === 'error') return { color: 'text-red-400', bg: 'bg-red-500/20' };
		if (severity === 'warning') return { color: 'text-amber-400', bg: 'bg-amber-500/20' };
		return { color: 'text-blue-400', bg: 'bg-blue-500/20' };
	}
</script>

<div class="space-y-6">
	{#if loading}
		<div class="flex items-center justify-center py-16">
			<div class="h-12 w-12 animate-spin rounded-full border-4 border-slate-700 border-t-blue-500"></div>
		</div>
	{:else if error}
		<div class="rounded-xl bg-red-900/30 border border-red-700 p-5">
			<p class="text-red-400 text-base">{error}</p>
		</div>
	{:else if compliance}
		<!-- Global Status Banner -->
		<div class="flex items-center justify-between rounded-xl p-6 {compliance.all_passed ? 'bg-emerald-900/30 border border-emerald-700' : 'bg-red-900/30 border border-red-700'}">
			<div class="flex items-center gap-4">
				<div class="flex h-14 w-14 items-center justify-center rounded-xl {compliance.all_passed ? 'bg-emerald-500/30' : 'bg-red-500/30'}">
					{#if compliance.all_passed}
						<svg class="h-8 w-8 text-emerald-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
							<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
						</svg>
					{:else}
						<svg class="h-8 w-8 text-red-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
							<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
						</svg>
					{/if}
				</div>
				<div>
					<h2 class="text-xl font-bold {compliance.all_passed ? 'text-emerald-400' : 'text-red-400'}">
						{compliance.all_passed ? 'All Checks Passed' : 'Compliance Issues Found'}
					</h2>
					<p class="text-base text-slate-400">{Object.keys(compliance.mode_results).length} modes analyzed</p>
				</div>
			</div>
		</div>

		<!-- Cross-Mode Checks -->
		{#if compliance.global_checks.length > 0}
			<div class="rounded-xl bg-slate-800/50 border border-slate-700/50 p-6">
				<h3 class="mb-4 text-base font-semibold text-slate-300 uppercase tracking-wider">Cross-Mode Checks</h3>
				<div class="space-y-3">
					{#each compliance.global_checks as check}
						{@const icon = getIcon(check.passed, check.severity)}
						<div class="rounded-xl bg-slate-900/70 border border-slate-700/30 p-5">
							<div class="flex items-start gap-4">
								<div class="flex h-10 w-10 flex-shrink-0 items-center justify-center rounded-xl {icon.bg}">
									{#if check.passed}
										<svg class="h-5 w-5 {icon.color}" fill="none" stroke="currentColor" viewBox="0 0 24 24">
											<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
										</svg>
									{:else}
										<svg class="h-5 w-5 {icon.color}" fill="none" stroke="currentColor" viewBox="0 0 24 24">
											<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
										</svg>
									{/if}
								</div>
								<div class="flex-1 min-w-0">
									<div class="flex items-center gap-3 flex-wrap">
										<span class="font-semibold text-white text-base">{check.name}</span>
										{#if !check.passed}
											<span class="rounded-lg px-2.5 py-1 text-sm font-medium {check.severity === 'error' ? 'bg-red-500/30 text-red-300' : 'bg-amber-500/30 text-amber-300'}">
												{check.severity === 'error' ? 'Critical' : 'Warning'}
											</span>
										{/if}
									</div>
									<div class="text-sm text-slate-400 mt-2">
										Expected: <span class="text-slate-300">{check.expected}</span>
									</div>
									<div class="text-sm text-slate-400">
										Result: <span class="{icon.color} font-medium">{check.value}</span>
									</div>
									{#if !check.passed && check.reason}
										<div class="mt-3 p-3 rounded-lg bg-slate-950/50 border border-slate-700/30">
											<p class="text-sm {icon.color} leading-relaxed">{check.reason}</p>
										</div>
									{/if}
								</div>
							</div>
						</div>
					{/each}
				</div>
			</div>
		{/if}

		<!-- Per-Mode Results -->
		<div class="rounded-xl bg-slate-800/50 border border-slate-700/50 p-6">
			<h3 class="mb-4 text-base font-semibold text-slate-300 uppercase tracking-wider">Per-Mode Compliance</h3>
			<div class="space-y-3">
				{#each Object.entries(compliance.mode_results) as [modeName, result]}
					<details class="group rounded-xl bg-slate-900/70 border border-slate-700/30 overflow-hidden">
						<summary class="flex cursor-pointer items-center gap-4 p-5 hover:bg-slate-800/50 transition-colors">
							<div class="flex h-9 w-9 items-center justify-center rounded-lg {result.passed ? 'bg-emerald-500/20' : 'bg-red-500/20'}">
								{#if result.passed}
									<svg class="h-5 w-5 text-emerald-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
										<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
									</svg>
								{:else}
									<svg class="h-5 w-5 text-red-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
										<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
									</svg>
								{/if}
							</div>
							<span class="font-semibold text-white text-base flex-1 capitalize">{modeName}</span>
							<div class="flex items-center gap-4 text-sm">
								<span class="text-emerald-400 font-medium">{result.passed_count} passed</span>
								{#if result.failed_count > 0}<span class="text-red-400 font-medium">{result.failed_count} failed</span>{/if}
								{#if result.warning_count > 0}<span class="text-amber-400 font-medium">{result.warning_count} warn</span>{/if}
								<svg class="h-5 w-5 text-slate-500 transition-transform group-open:rotate-180" fill="none" stroke="currentColor" viewBox="0 0 24 24">
									<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
								</svg>
							</div>
						</summary>

						<!-- Expanded checks -->
						<div class="border-t border-slate-700/50 p-4 space-y-2 bg-slate-950/30">
							{#each result.checks as check}
								{@const icon = getIcon(check.passed, check.severity)}
								<div class="flex items-start gap-3 p-4 rounded-lg {check.passed ? 'bg-slate-900/30' : icon.bg.replace('/20', '/10')} border {check.passed ? 'border-transparent' : 'border-slate-700/30'}">
									<div class="flex h-7 w-7 flex-shrink-0 items-center justify-center rounded-lg {icon.bg} mt-0.5">
										{#if check.passed}
											<svg class="h-4 w-4 {icon.color}" fill="none" stroke="currentColor" viewBox="0 0 24 24">
												<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
											</svg>
										{:else}
											<svg class="h-4 w-4 {icon.color}" fill="none" stroke="currentColor" viewBox="0 0 24 24">
												<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
											</svg>
										{/if}
									</div>
									<div class="flex-1 min-w-0">
										<div class="flex items-center gap-2 flex-wrap">
											<span class="text-sm font-medium text-white">{check.name}</span>
											{#if !check.passed}
												<span class="rounded px-1.5 py-0.5 text-xs font-medium {check.severity === 'error' ? 'bg-red-500/30 text-red-300' : check.severity === 'warning' ? 'bg-amber-500/30 text-amber-300' : 'bg-blue-500/30 text-blue-300'}">
													{check.severity}
												</span>
											{/if}
										</div>
										<div class="text-sm text-slate-500 mt-1">
											{check.expected} → <span class="{icon.color} font-medium">{check.value}</span>
										</div>
										{#if !check.passed && check.reason}
											<p class="text-sm {icon.color} mt-2 leading-relaxed">{check.reason}</p>
										{/if}
									</div>
								</div>
							{/each}
						</div>
					</details>
				{/each}
			</div>
		</div>

		<!-- Legend -->
		<div class="flex items-center justify-center gap-8 text-sm text-slate-500 pt-2">
			<div class="flex items-center gap-2">
				<div class="h-4 w-4 rounded-lg bg-emerald-500/30"></div>
				<span>Passed</span>
			</div>
			<div class="flex items-center gap-2">
				<div class="h-4 w-4 rounded-lg bg-red-500/30"></div>
				<span>Critical</span>
			</div>
			<div class="flex items-center gap-2">
				<div class="h-4 w-4 rounded-lg bg-amber-500/30"></div>
				<span>Warning</span>
			</div>
			<div class="flex items-center gap-2">
				<div class="h-4 w-4 rounded-lg bg-blue-500/30"></div>
				<span>Info</span>
			</div>
		</div>
	{/if}
</div>
