import { create } from 'zustand';
import { persist } from 'zustand/middleware';

type Theme = 'light' | 'dark' | 'system';

interface ThemeState {
    theme: Theme;
    setTheme: (theme: Theme) => void;
    resolvedTheme: 'light' | 'dark';
}

function getSystemTheme(): 'light' | 'dark' {
    if (typeof window === 'undefined') return 'light';
    return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
}

function applyTheme(resolved: 'light' | 'dark') {
    const root = document.documentElement;
    if (resolved === 'dark') {
        root.classList.add('dark');
    } else {
        root.classList.remove('dark');
    }
}

export const useThemeStore = create<ThemeState>()(
    persist(
        (set) => ({
            theme: 'system' as Theme,
            resolvedTheme: getSystemTheme(),
            setTheme: (theme: Theme) => {
                const resolved = theme === 'system' ? getSystemTheme() : theme;
                applyTheme(resolved);
                set({ theme, resolvedTheme: resolved });
            },
        }),
        {
            name: 'theme-storage',
            onRehydrateStorage: () => (state) => {
                if (state) {
                    const resolved = state.theme === 'system' ? getSystemTheme() : state.theme;
                    applyTheme(resolved);
                    state.resolvedTheme = resolved;
                }
            },
        }
    )
);

// Listen for system theme changes
if (typeof window !== 'undefined') {
    window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', (e) => {
        const state = useThemeStore.getState();
        if (state.theme === 'system') {
            const resolved = e.matches ? 'dark' : 'light';
            applyTheme(resolved);
            useThemeStore.setState({ resolvedTheme: resolved });
        }
    });
}
