import { derived } from 'svelte/store';
import { locale } from 'svelte-i18n';

/**
 * Number formatter that respects the current locale
 */
export const numberFormatter = derived(locale, ($locale) => {
	return new Intl.NumberFormat($locale ?? 'en');
});

/**
 * Percent formatter (0.9567 -> "95.67%")
 */
export const percentFormatter = derived(locale, ($locale) => {
	return new Intl.NumberFormat($locale ?? 'en', {
		style: 'percent',
		minimumFractionDigits: 2,
		maximumFractionDigits: 2
	});
});

/**
 * Compact number formatter for large numbers (1000 -> "1K")
 */
export const compactFormatter = derived(locale, ($locale) => {
	return new Intl.NumberFormat($locale ?? 'en', {
		notation: 'compact',
		compactDisplay: 'short'
	});
});

/**
 * Currency formatter
 */
export function createCurrencyFormatter(currency: string = 'USD') {
	return derived(locale, ($locale) => {
		return new Intl.NumberFormat($locale ?? 'en', {
			style: 'currency',
			currency
		});
	});
}

/**
 * Format a number with the current locale
 */
export function formatNumber(value: number, formatter: Intl.NumberFormat): string {
	return formatter.format(value);
}

/**
 * Format a multiplier (e.g., 100 -> "100x")
 */
export function formatMultiplier(value: number, decimals: number = 2): string {
	if (Number.isInteger(value) && value >= 10) {
		return value.toFixed(0) + 'x';
	}
	return value.toFixed(decimals) + 'x';
}

/**
 * Format percentage for display (0.9567 -> "95.67%")
 */
export function formatPercent(value: number, decimals: number = 2): string {
	return (value * 100).toFixed(decimals) + '%';
}

/**
 * Format large numbers in a compact way based on locale
 * 1000 -> "1K", 1000000 -> "1M"
 */
export function formatCompactNumber(value: number, locale: string = 'en'): string {
	if (value >= 1_000_000) {
		const m = value / 1_000_000;
		return m.toFixed(m % 1 === 0 ? 0 : 1) + 'M';
	}
	if (value >= 1_000) {
		const k = value / 1_000;
		return k.toFixed(k % 1 === 0 ? 0 : 1) + 'K';
	}
	return value.toLocaleString(locale);
}
