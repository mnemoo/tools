import { init, addMessages, locale, _ } from 'svelte-i18n';
import { derived } from 'svelte/store';

// Import locales synchronously
import en from './locales/en.json';
import ru from './locales/ru.json';
import es from './locales/es.json';
import zh from './locales/zh.json';
import fr from './locales/fr.json';
import it from './locales/it.json';
import de from './locales/de.json';
import ko from './locales/ko.json';
import pt from './locales/pt.json';
import el from './locales/el.json';
import tr from './locales/tr.json';
import vi from './locales/vi.json';
import th from './locales/th.json';
import fi from './locales/fi.json';

export type SupportedLocale = 'en' | 'ru' | 'es' | 'zh' | 'fr' | 'it' | 'de' | 'ko' | 'pt' | 'el' | 'tr' | 'vi' | 'th' | 'fi';
export const SUPPORTED_LOCALES: SupportedLocale[] = ['en', 'ru', 'es', 'zh', 'fr', 'it', 'de', 'ko', 'pt', 'el', 'tr', 'vi', 'th', 'fi'];

// Add messages synchronously
addMessages('en', en);
addMessages('ru', ru);
addMessages('es', es);
addMessages('zh', zh);
addMessages('fr', fr);
addMessages('it', it);
addMessages('de', de);
addMessages('ko', ko);
addMessages('pt', pt);
addMessages('el', el);
addMessages('tr', tr);
addMessages('vi', vi);
addMessages('th', th);
addMessages('fi', fi);

// Initialize with default locale immediately
init({
    fallbackLocale: 'en',
    initialLocale: 'en',
});

export function initI18n(initialLocale: SupportedLocale = 'en') {
    locale.set(initialLocale);
}

export function setLocale(newLocale: SupportedLocale) {
    locale.set(newLocale);
}

export { _, locale };
export const isLoading = derived(locale, ($locale) => !$locale);
