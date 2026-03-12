import { create } from 'zustand';
import { persist } from 'zustand/middleware';

export type Locale = 'zh-CN' | 'en';

interface LangState {
    locale: Locale;
    setLocale: (locale: Locale) => void;
}

function detectBrowserLocale(): Locale {
    if (typeof navigator === 'undefined') return 'en';
    const lang = navigator.language || '';
    if (lang.startsWith('zh')) return 'zh-CN';
    return 'en';
}

export const useLangStore = create<LangState>()(
    persist(
        (set) => ({
            locale: detectBrowserLocale(),
            setLocale: (locale: Locale) => set({ locale }),
        }),
        { name: 'lang-storage' }
    )
);
