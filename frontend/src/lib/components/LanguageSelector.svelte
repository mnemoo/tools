<script lang="ts">
	import { locale, SUPPORTED_LOCALES, LOCALE_NAMES, LOCALE_FLAGS, setLocale, type SupportedLocale } from '$lib/i18n';

	let isOpen = $state(false);

	function handleSelect(newLocale: SupportedLocale) {
		setLocale(newLocale);
		isOpen = false;
	}

	function toggleDropdown() {
		isOpen = !isOpen;
	}

	function handleClickOutside(event: MouseEvent) {
		const target = event.target as HTMLElement;
		if (!target.closest('.language-selector')) {
			isOpen = false;
		}
	}
</script>

<svelte:window onclick={handleClickOutside} />

<div class="language-selector relative">
	<button
		onclick={toggleDropdown}
		class="flex items-center gap-2 px-3 py-1.5 rounded-lg bg-[var(--color-graphite)] hover:bg-[var(--color-slate)] border border-white/10 transition-colors"
	>
		<span class="text-base">{LOCALE_FLAGS[$locale as SupportedLocale] ?? LOCALE_FLAGS.en}</span>
		<span class="text-xs font-mono text-[var(--color-light)]">{($locale ?? 'en').toUpperCase()}</span>
		<svg
			class="w-3 h-3 text-[var(--color-mist)] transition-transform {isOpen ? 'rotate-180' : ''}"
			fill="none"
			viewBox="0 0 24 24"
			stroke="currentColor"
			stroke-width="2"
		>
			<path stroke-linecap="round" stroke-linejoin="round" d="M19 9l-7 7-7-7" />
		</svg>
	</button>

	{#if isOpen}
		<div class="absolute right-0 top-full mt-1 py-1 min-w-[140px] rounded-lg bg-[var(--color-graphite)] border border-white/10 shadow-xl z-50">
			{#each SUPPORTED_LOCALES as loc}
				<button
					onclick={() => handleSelect(loc)}
					class="w-full flex items-center gap-3 px-3 py-2 text-left hover:bg-[var(--color-slate)] transition-colors {$locale === loc ? 'bg-[var(--color-cyan)]/10' : ''}"
				>
					<span class="text-base">{LOCALE_FLAGS[loc]}</span>
					<span class="text-sm text-[var(--color-light)]">{LOCALE_NAMES[loc]}</span>
					{#if $locale === loc}
						<svg class="w-4 h-4 text-[var(--color-cyan)] ml-auto" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
							<path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7" />
						</svg>
					{/if}
				</button>
			{/each}
		</div>
	{/if}
</div>
