<script lang="ts">
	import { onMount, tick } from 'svelte';
	import { api, type AllModesComplianceResult } from '$lib/api';
	import { _ } from '$lib/i18n';

	interface Props {
		expandedMode?: string | null;
	}
	let { expandedMode = null }: Props = $props();

	let loading = $state(true);
	let error = $state<string | null>(null);
	let compliance = $state<AllModesComplianceResult | null>(null);
	let expandedModes = $state<Set<string>>(new Set());

	// Auto-expand the specified mode when prop changes and scroll to it
	$effect(() => {
		if (expandedMode && compliance?.mode_results) {
			expandedModes = new Set([expandedMode]);
			// Scroll to the expanded mode after DOM update
			tick().then(() => {
				const element = document.querySelector(`[data-mode="${expandedMode}"]`);
				if (element) {
					element.scrollIntoView({ behavior: 'smooth', block: 'center' });
					// Focus the toggle button for accessibility
					const button = element.querySelector('button');
					if (button) button.focus();
				}
			});
		}
	});

	function toggleMode(modeName: string) {
		const newSet = new Set(expandedModes);
		if (newSet.has(modeName)) {
			newSet.delete(modeName);
		} else {
			newSet.add(modeName);
		}
		expandedModes = newSet;
	}

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
						{compliance.all_passed ? $_('compliance.allChecksPassed') : $_('compliance.complianceIssuesFound')}
					</h2>
					<p class="text-base text-slate-400">{$_('compliance.modesAnalyzed', { values: { count: Object.keys(compliance.mode_results).length } })}</p>
				</div>
			</div>
		</div>

		<!-- Cross-Mode Checks -->
		{#if compliance.global_checks.length > 0}
			<div class="rounded-xl bg-slate-800/50 border border-slate-700/50 p-6">
				<h3 class="mb-4 text-base font-semibold text-slate-300 uppercase tracking-wider">{$_('compliance.crossModeChecks')}</h3>
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
										<span class="font-semibold text-white text-base">{$_(check.nameKey)}</span>
										{#if !check.passed}
											<span class="rounded-lg px-2.5 py-1 text-sm font-medium {check.severity === 'error' ? 'bg-red-500/30 text-red-300' : 'bg-amber-500/30 text-amber-300'}">
												{check.severity === 'error' ? $_('compliance.critical') : $_('compliance.warning')}
											</span>
										{/if}
									</div>
									<div class="text-sm text-slate-400 mt-2">
										{$_('compliance.expected')}: <span class="text-slate-300">{check.expected}</span>
									</div>
									<div class="text-sm text-slate-400">
										{$_('compliance.result')}: <span class="{icon.color} font-medium">{check.value}</span>
									</div>
									{#if !check.passed && check.reasonKey}
										<div class="mt-3 p-3 rounded-lg bg-slate-950/50 border border-slate-700/30">
											<p class="text-sm {icon.color} leading-relaxed">{$_(check.reasonKey)}</p>
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
			<h3 class="mb-4 text-base font-semibold text-slate-300 uppercase tracking-wider">{$_('compliance.perModeCompliance')}</h3>
			<div class="space-y-3">
				{#each Object.entries(compliance.mode_results) as [modeName, result]}
					{@const isExpanded = expandedModes.has(modeName)}
					<div class="rounded-xl bg-slate-900/70 border border-slate-700/30 overflow-hidden" data-mode={modeName}>
						<button
							type="button"
							class="w-full flex cursor-pointer items-center gap-4 p-5 hover:bg-slate-800/50 transition-colors text-left"
							onclick={() => toggleMode(modeName)}
						>
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
								<span class="text-emerald-400 font-medium">{$_('compliance.passedCount', { values: { count: result.passed_count } })}</span>
								{#if result.failed_count > 0}<span class="text-red-400 font-medium">{$_('compliance.failedCount', { values: { count: result.failed_count } })}</span>{/if}
								{#if result.warning_count > 0}<span class="text-amber-400 font-medium">{$_('compliance.warnCount', { values: { count: result.warning_count } })}</span>{/if}
								<svg class="h-5 w-5 text-slate-500 transition-transform {isExpanded ? 'rotate-180' : ''}" fill="none" stroke="currentColor" viewBox="0 0 24 24">
									<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
								</svg>
							</div>
						</button>

						<!-- Expanded checks -->
						{#if isExpanded}
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
											<span class="text-sm font-medium text-white">{$_(check.nameKey)}</span>
											{#if !check.passed}
												<span class="rounded px-1.5 py-0.5 text-xs font-medium {check.severity === 'error' ? 'bg-red-500/30 text-red-300' : check.severity === 'warning' ? 'bg-amber-500/30 text-amber-300' : 'bg-blue-500/30 text-blue-300'}">
													{check.severity}
												</span>
											{/if}
										</div>
										<div class="text-sm text-slate-500 mt-1">
											{check.expected} → <span class="{icon.color} font-medium">{check.value}</span>
										</div>
										{#if !check.passed && check.reasonKey}
											<p class="text-sm {icon.color} mt-2 leading-relaxed">{$_(check.reasonKey)}</p>
										{/if}
									</div>
								</div>
							{/each}
						</div>
						{/if}
					</div>
				{/each}
			</div>
		</div>

		<!-- Legend -->
		<div class="flex items-center justify-center gap-8 text-sm text-slate-500 pt-2">
			<div class="flex items-center gap-2">
				<div class="h-4 w-4 rounded-lg bg-emerald-500/30"></div>
				<span>{$_('compliance.passed')}</span>
			</div>
			<div class="flex items-center gap-2">
				<div class="h-4 w-4 rounded-lg bg-red-500/30"></div>
				<span>{$_('compliance.critical')}</span>
			</div>
			<div class="flex items-center gap-2">
				<div class="h-4 w-4 rounded-lg bg-amber-500/30"></div>
				<span>{$_('compliance.warning')}</span>
			</div>
			<div class="flex items-center gap-2">
				<div class="h-4 w-4 rounded-lg bg-blue-500/30"></div>
				<span>{$_('compliance.info')}</span>
			</div>
		</div>
	{/if}
</div>
