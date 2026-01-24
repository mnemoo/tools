<script lang="ts">
	import BucketConfigPanel from './BucketConfigPanel.svelte';
	import { _ } from '$lib/i18n';

	let { mode }: { mode: string } = $props();
</script>

<div class="space-y-6">
	<!-- Experimental Warning Banner -->
	<div class="px-4 py-3 rounded-xl bg-amber-500/10 border border-amber-500/30 flex items-start gap-3">
		<svg class="w-5 h-5 text-amber-400 shrink-0 mt-0.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
			<path stroke-linecap="round" stroke-linejoin="round" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
		</svg>
		<div class="text-sm text-amber-200/90">
			<strong class="text-amber-400">{$_('optimizer.experimental')}</strong> {$_('optimizer.disclaimer')}
			{$_('optimizer.disclaimerNote')}
		</div>
	</div>

	<!-- Header -->
	<div class="flex items-center gap-3">
		<div class="w-1 h-6 bg-[var(--color-gold)] rounded-full"></div>
		<h2 class="font-display text-xl text-[var(--color-light)] tracking-wider">{$_('optimizer.weightOptimizer')}</h2>
		<span class="text-xs font-mono text-[var(--color-mist)] uppercase">{mode}</span>
	</div>

	<!-- Description -->
	<div class="px-4 py-3 rounded-xl bg-[var(--color-graphite)]/30 border border-white/[0.02]">
		<p class="text-sm font-mono text-[var(--color-mist)]">
			<span class="text-[var(--color-gold)]">{$_('optimizer.bucketMode')}:</span> {$_('optimizer.bucketModeDesc')}
			{$_('optimizer.bucketModeExample')}
		</p>
	</div>

	<!-- Bucket Config Panel -->
	<BucketConfigPanel {mode} />
</div>
