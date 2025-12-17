// Open Game helper module - shared functionality for opening games in a new tab

import { api } from '$lib/api';

// Default game UUID for testing - can be overridden in settings
export const DEFAULT_GAME_UUID = '00000000-0000-0000-0000-000000000000';
export const DEFAULT_GAME_VERSION = '1';

export const CURRENCIES = [
	{ code: 'USD', display: '$', name: 'United States Dollar' },
	{ code: 'CAD', display: 'CA$', name: 'Canadian Dollar' },
	{ code: 'JPY', display: '¥', name: 'Japanese Yen' },
	{ code: 'EUR', display: '€', name: 'Euro' },
	{ code: 'RUB', display: '₽', name: 'Russian Ruble' },
	{ code: 'CNY', display: 'CN¥', name: 'Chinese Yuan' },
	{ code: 'PHP', display: '₱', name: 'Philippine Peso' },
	{ code: 'INR', display: '₹', name: 'Indian Rupee' },
	{ code: 'IDR', display: 'Rp', name: 'Indonesian Rupiah' },
	{ code: 'KRW', display: '₩', name: 'South Korean Won' },
	{ code: 'BRL', display: 'R$', name: 'Brazilian Real' },
	{ code: 'MXN', display: 'MX$', name: 'Mexican Peso' },
	{ code: 'DKK', display: 'KR', name: 'Danish Krone' },
	{ code: 'PLN', display: 'zł', name: 'Polish Złoty' },
	{ code: 'VND', display: '₫', name: 'Vietnamese Đồng' },
	{ code: 'TRY', display: '₺', name: 'Turkish Lira' },
	{ code: 'CLP', display: 'CLP', name: 'Chilean Peso' },
	{ code: 'ARS', display: 'ARS', name: 'Argentine Peso' },
	{ code: 'PEN', display: 'S/', name: 'Peruvian Sol' },
	{ code: 'NGN', display: '₦', name: 'Nigerian Naira' },
	{ code: 'SAR', display: 'SAR', name: 'Saudi Arabia Riyal' },
	{ code: 'ILS', display: 'ILS', name: 'Israel Shekel' },
	{ code: 'AED', display: 'AED', name: 'United Arab Emirates Dirham' },
	{ code: 'TWD', display: 'NT$', name: 'Taiwan New Dollar' },
	{ code: 'NOK', display: 'kr', name: 'Norway Krone' },
	{ code: 'KWD', display: 'KD', name: 'Kuwaiti Dinar' },
	{ code: 'JOD', display: 'JD', name: 'Jordanian Dinar' },
	{ code: 'CRC', display: '₡', name: 'Costa Rica Colon' },
	{ code: 'TND', display: 'TND', name: 'Tunisian Dinar' },
	{ code: 'SGD', display: 'SG$', name: 'Singapore Dollar' },
	{ code: 'MYR', display: 'RM', name: 'Malaysia Ringgit' },
	{ code: 'OMR', display: 'OMR', name: 'Oman Rial' },
	{ code: 'QAR', display: 'QAR', name: 'Qatar Riyal' },
	{ code: 'BHD', display: 'BD', name: 'Bahraini Dinar' },
	{ code: 'XGC', display: 'GC', name: 'Stake Gold Coin' },
	{ code: 'XSC', display: 'SC', name: 'Stake Cash' },
] as const;

export const LANGUAGES = [
	{ code: 'ar', name: 'Arabic' },
	{ code: 'de', name: 'German' },
	{ code: 'en', name: 'English' },
	{ code: 'es', name: 'Spanish' },
	{ code: 'fi', name: 'Finnish' },
	{ code: 'fr', name: 'French' },
	{ code: 'hi', name: 'Hindi' },
	{ code: 'id', name: 'Indonesian' },
	{ code: 'ja', name: 'Japanese' },
	{ code: 'ko', name: 'Korean' },
	{ code: 'po', name: 'Polish' },
	{ code: 'pt', name: 'Portuguese' },
	{ code: 'ru', name: 'Russian' },
	{ code: 'tr', name: 'Turkish' },
	{ code: 'zh', name: 'Chinese' },
	{ code: 'vi', name: 'Vietnamese' },
] as const;

const STORAGE_KEY = 'lgs_open_game_settings';

export interface OpenGameSettings {
	domain: string;
	session: string;
	currency: string;
	balance: number;
	language: string;
	device: 'desktop' | 'mobile';
	demo: boolean;
	social: boolean;
	gameUUID: string;
	gameVersion: string;
}

export const defaultSettings: OpenGameSettings = {
	domain: 'localhost:4234',
	session: '',
	currency: 'USD',
	balance: 1000,
	language: 'en',
	device: 'desktop',
	demo: true,
	social: false,
	gameUUID: DEFAULT_GAME_UUID,
	gameVersion: DEFAULT_GAME_VERSION,
};

export function loadGameSettings(): OpenGameSettings {
	if (typeof localStorage === 'undefined') {
		return { ...defaultSettings };
	}
	try {
		const stored = localStorage.getItem(STORAGE_KEY);
		if (stored) {
			return { ...defaultSettings, ...JSON.parse(stored) };
		}
	} catch {
		// Ignore parse errors
	}
	return { ...defaultSettings };
}

export function saveGameSettings(settings: OpenGameSettings): void {
	if (typeof localStorage === 'undefined') return;
	try {
		localStorage.setItem(STORAGE_KEY, JSON.stringify(settings));
	} catch {
		// Ignore storage errors
	}
}

export function getCurrencyDisplay(code: string): string {
	return CURRENCIES.find(c => c.code === code)?.display ?? code;
}

export interface OpenGameOptions {
	sessionID: string;
	balance: number; // in display units (e.g., 1000 = $1000)
	currency: string;
	language: string;
	device: 'desktop' | 'mobile';
	demo: boolean;
	social: boolean;
	domain: string;
}

/**
 * Opens a game in a new browser tab
 * First checks backend health, then opens the game URL
 */
export async function openGame(options: OpenGameOptions): Promise<void> {
	// Check if backend is healthy before opening
	await api.health();

	const session = options.sessionID.trim() || `test-${Date.now()}`;

	// Get RGS URL - use HTTPS on port 7755 (LGS HTTPS port)
	const apiUrl = new URL(api.getBaseUrl());
	const rgsUrl = `https://${apiUrl.hostname}:7755`;

	// Build URL with query parameters
	const params = new URLSearchParams({
		sessionID: session,
		rgs_url: rgsUrl,
		lang: options.language,
		currency: options.currency,
		device: options.device,
		social: options.social.toString(),
		demo: options.demo.toString(),
	});

	const protocol = options.domain.startsWith('https://') || options.domain.startsWith('http://') ? '' : 'http://';
	const domain = options.domain.replace(/^https?:\/\//, '');
	const url = `${protocol}${domain}/?${params.toString()}`;

	// Open in new tab (noopener for security, noreferrer to prevent referrer leak)
	window.open(url, '_blank', 'noopener,noreferrer');
}

export interface OpenReplayOptions {
	mode: string;
	eventId: number;
	gameUUID: string;
	gameVersion: string;
	currency: string;
	amount: number; // bet amount in display units (e.g., 1 = $1)
	language: string;
	device: 'desktop' | 'mobile';
	social: boolean;
	domain: string;
}

/**
 * Opens a game in replay mode in a new browser tab
 * Replay mode loads a specific event for viewing without affecting balance
 */
export function openReplay(options: OpenReplayOptions): void {
	const amountInApiUnits = Math.round(options.amount * 1000000);

	// Get RGS URL - use HTTPS on port 7755 (LGS HTTPS port)
	const apiUrl = new URL(api.getBaseUrl());
	const rgsUrl = `https://${apiUrl.hostname}:7755`;

	// Build URL with replay query parameters
	const params = new URLSearchParams({
		replay: 'true',
		game: options.gameUUID,
		version: options.gameVersion,
		mode: options.mode,
		event: options.eventId.toString(),
		rgs_url: rgsUrl,
		currency: options.currency,
		amount: amountInApiUnits.toString(),
		lang: options.language,
		device: options.device,
		social: options.social.toString(),
	});

	const protocol = options.domain.startsWith('https://') || options.domain.startsWith('http://') ? '' : 'http://';
	const domain = options.domain.replace(/^https?:\/\//, '');
	const url = `${protocol}${domain}/?${params.toString()}`;

	window.open(url, '_blank');
}
