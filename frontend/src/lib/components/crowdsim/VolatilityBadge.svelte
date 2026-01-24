<script lang="ts">
	import type { CrowdSimVolatilityProfile } from '$lib/api/types';
	import { _ } from '$lib/i18n';

	let {
		profile,
		score
	}: {
		profile: CrowdSimVolatilityProfile;
		score?: number;
	} = $props();

	function getProfileStyles(p: CrowdSimVolatilityProfile): {
		bg: string;
		text: string;
		border: string;
		descKey: string;
	} {
		switch (p) {
			case 'low':
				return {
					bg: 'bg-green-900/30',
					text: 'text-green-400',
					border: 'border-green-500',
					descKey: 'volatility.lowDesc'
				};
			case 'medium':
				return {
					bg: 'bg-yellow-900/30',
					text: 'text-yellow-400',
					border: 'border-yellow-500',
					descKey: 'volatility.mediumDesc'
				};
			case 'high':
				return {
					bg: 'bg-red-900/30',
					text: 'text-red-400',
					border: 'border-red-500',
					descKey: 'volatility.highDesc'
				};
			default:
				return {
					bg: 'bg-gray-900/30',
					text: 'text-gray-400',
					border: 'border-gray-500',
					descKey: 'volatility.unknownDesc'
				};
		}
	}

	let styles = $derived(getProfileStyles(profile));
</script>

<div class="rounded-lg {styles.bg} border {styles.border} p-4">
	<div class="flex items-center justify-between">
		<div>
			<div class="text-xs text-gray-500">{$_('volatility.profile')}</div>
			<div class="text-2xl font-bold {styles.text}">
				{$_(`badges.${profile}`)}
			</div>
		</div>
		{#if score !== undefined}
			<div class="text-right">
				<div class="text-xs text-gray-500">{$_('volatility.compositeScore')}</div>
				<div class="text-2xl font-bold text-white">{score.toFixed(3)}</div>
			</div>
		{/if}
	</div>
	<p class="mt-2 text-sm text-gray-400">{$_(styles.descKey)}</p>
</div>
