<script lang="ts">
	import type { CrowdSimVolatilityProfile } from '$lib/api/types';

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
		description: string;
	} {
		switch (p) {
			case 'low':
				return {
					bg: 'bg-green-900/30',
					text: 'text-green-400',
					border: 'border-green-500',
					description: 'Suitable for casual players. Higher win frequency, lower variance.'
				};
			case 'medium':
				return {
					bg: 'bg-yellow-900/30',
					text: 'text-yellow-400',
					border: 'border-yellow-500',
					description: 'Balanced experience. Mix of frequent small wins and occasional bigger payouts.'
				};
			case 'high':
				return {
					bg: 'bg-red-900/30',
					text: 'text-red-400',
					border: 'border-red-500',
					description: 'For thrill-seekers. Lower win frequency but higher potential peaks.'
				};
			default:
				return {
					bg: 'bg-gray-900/30',
					text: 'text-gray-400',
					border: 'border-gray-500',
					description: 'Unknown volatility profile.'
				};
		}
	}

	let styles = $derived(getProfileStyles(profile));
</script>

<div class="rounded-lg {styles.bg} border {styles.border} p-4">
	<div class="flex items-center justify-between">
		<div>
			<div class="text-xs text-gray-500">Volatility Profile</div>
			<div class="text-2xl font-bold {styles.text}">
				{profile.charAt(0).toUpperCase() + profile.slice(1)}
			</div>
		</div>
		{#if score !== undefined}
			<div class="text-right">
				<div class="text-xs text-gray-500">Composite Score</div>
				<div class="text-2xl font-bold text-white">{score.toFixed(3)}</div>
			</div>
		{/if}
	</div>
	<p class="mt-2 text-sm text-gray-400">{styles.description}</p>
</div>
