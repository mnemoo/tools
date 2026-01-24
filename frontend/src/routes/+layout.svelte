<script lang="ts">
	import './layout.css';
	import '$lib/i18n';
	import favicon from '$lib/assets/favicon.svg';
	import { isLoading } from 'svelte-i18n';
	import PreloadWarningModal from '$lib/components/PreloadWarningModal.svelte';

	let { children } = $props();
</script>

<svelte:head>
	<link rel="icon" href={favicon} />
</svelte:head>

{#if $isLoading}
	<!-- i18n loading state -->
	<div class="min-h-screen bg-[var(--color-void)] flex items-center justify-center">
		<div class="text-[var(--color-mist)] font-mono text-sm">Loading...</div>
	</div>
{:else}
	{@render children()}
{/if}

<!-- Global modals rendered at root level -->
<PreloadWarningModal />
