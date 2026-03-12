import { useLangStore, type Locale } from '@/stores/langStore';
import en from '@/locales/en.json';
import zhCN from '@/locales/zh-CN.json';

type TranslationMap = Record<string, string | Record<string, string>>;

const translations: Record<Locale, TranslationMap> = {
    'en': en as unknown as TranslationMap,
    'zh-CN': zhCN as unknown as TranslationMap,
};

/**
 * Get a translated string by dot-notation key.
 * Supports parameter interpolation: t('usage.limit', { count: 100 })
 *
 * @example
 *   t('nav.dashboard')           // "仪表盘" or "Dashboard"
 *   t('common.save')             // "保存" or "Save"
 *   t('errors.rate_limited')     // "请求过于频繁..." or "Too many requests..."
 */
export function t(key: string, params?: Record<string, string | number>): string {
    const locale = useLangStore.getState().locale;
    const map = translations[locale] || translations['en'];

    // Support dot notation: "nav.dashboard" → map["nav"]["dashboard"]
    const parts = key.split('.');
    let value: unknown = map;

    for (const part of parts) {
        if (value && typeof value === 'object' && part in (value as Record<string, unknown>)) {
            value = (value as Record<string, unknown>)[part];
        } else {
            // Fallback to English
            let fallback: unknown = translations['en'];
            for (const p of parts) {
                if (fallback && typeof fallback === 'object' && p in (fallback as Record<string, unknown>)) {
                    fallback = (fallback as Record<string, unknown>)[p];
                } else {
                    return key; // Key not found in any locale
                }
            }
            value = fallback;
            break;
        }
    }

    if (typeof value !== 'string') {
        return key;
    }

    // Parameter interpolation: replace {{param}} with value
    if (params) {
        return value.replace(/\{\{(\w+)\}\}/g, (_, paramKey) => {
            return params[paramKey]?.toString() ?? `{{${paramKey}}}`;
        });
    }

    return value;
}

/**
 * React hook that returns the t function and triggers re-render on locale change.
 */
export function useTranslation() {
    const locale = useLangStore((s) => s.locale);
    const setLocale = useLangStore((s) => s.setLocale);

    // The t function uses the store directly, but we include locale
    // in the hook to trigger re-render when it changes.
    return { t, locale, setLocale };
}

/**
 * Get available locales for the language switcher.
 */
export const availableLocales: { code: Locale; label: string; nativeLabel: string }[] = [
    { code: 'en', label: 'English', nativeLabel: 'English' },
    { code: 'zh-CN', label: 'Chinese (Simplified)', nativeLabel: '�体中文' },
];
