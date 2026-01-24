import { browser } from '$app/environment';
import { init, register, getLocaleFromNavigator, locale, _ } from 'svelte-i18n';

// Supported locales
export const SUPPORTED_LOCALES = ['en', 'ru', 'es', 'zh', 'fr', 'it', 'de', 'ko', 'pt', 'el', 'tr', 'vi', 'th', 'fi'] as const;
export type SupportedLocale = (typeof SUPPORTED_LOCALES)[number];

export const LOCALE_NAMES: Record<SupportedLocale, string> = {
	en: 'English',
	ru: 'Русский',
	es: 'Español',
	zh: '中文',
	fr: 'Français',
	it: 'Italiano',
	de: 'Deutsch',
	ko: '한국어',
	pt: 'Português',
	el: 'Ελληνικά',
	tr: 'Türkçe',
	vi: 'Tiếng Việt',
	th: 'ไทย',
	fi: 'Suomi'
};

export const LOCALE_FLAGS: Record<SupportedLocale, string> = {
	en: '🇺🇸',
	ru: '🇷🇺',
	es: '🇪🇸',
	zh: '🇨🇳',
	fr: '🇫🇷',
	it: '🇮🇹',
	de: '🇩🇪',
	ko: '🇰🇷',
	pt: '🇵🇹',
	el: '🇬🇷',
	tr: '🇹🇷',
	vi: '🇻🇳',
	th: '🇹🇭',
	fi: '🇫🇮'
};

// LocalStorage key for persisting locale
const LOCALE_STORAGE_KEY = 'mtools-locale';

// Register locales with lazy loading
register('en', () => import('./locales/en.json'));
register('ru', () => import('./locales/ru.json'));
register('es', () => import('./locales/es.json'));
register('zh', () => import('./locales/zh.json'));
register('fr', () => import('./locales/fr.json'));
register('it', () => import('./locales/it.json'));
register('de', () => import('./locales/de.json'));
register('ko', () => import('./locales/ko.json'));
register('pt', () => import('./locales/pt.json'));
register('el', () => import('./locales/el.json'));
register('tr', () => import('./locales/tr.json'));
register('vi', () => import('./locales/vi.json'));
register('th', () => import('./locales/th.json'));
register('fi', () => import('./locales/fi.json'));

/**
 * Get the initial locale from:
 * 1. URL query parameter (?lang=xx)
 * 2. localStorage
 * 3. Browser navigator
 * 4. Fallback to 'en'
 */
function getInitialLocale(): string {
	if (!browser) return 'en';

	// Check URL param
	const urlParams = new URLSearchParams(window.location.search);
	const urlLocale = urlParams.get('lang');
	if (urlLocale && SUPPORTED_LOCALES.includes(urlLocale as SupportedLocale)) {
		return urlLocale;
	}

	// Check localStorage
	const storedLocale = localStorage.getItem(LOCALE_STORAGE_KEY);
	if (storedLocale && SUPPORTED_LOCALES.includes(storedLocale as SupportedLocale)) {
		return storedLocale;
	}

	// Check browser locale
	const browserLocale = getLocaleFromNavigator()?.split('-')[0];
	if (browserLocale && SUPPORTED_LOCALES.includes(browserLocale as SupportedLocale)) {
		return browserLocale;
	}

	return 'en';
}

// Initialize i18n
init({
	fallbackLocale: 'en',
	initialLocale: getInitialLocale()
});

/**
 * Set and persist the locale
 */
export function setLocale(newLocale: SupportedLocale): void {
	locale.set(newLocale);
	if (browser) {
		localStorage.setItem(LOCALE_STORAGE_KEY, newLocale);
	}
}

/**
 * Get current locale value
 */
export function getCurrentLocale(): SupportedLocale {
	let current: SupportedLocale = 'en';
	locale.subscribe((value) => {
		current = (value as SupportedLocale) || 'en';
	})();
	return current;
}

// Re-export commonly used functions
export { locale, _ };
