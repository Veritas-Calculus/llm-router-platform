import { create } from 'zustand';

export interface SiteConfig {
  siteName: string;
  subtitle: string;
  logoUrl: string;
  faviconUrl: string;
}

interface SiteConfigState {
  config: SiteConfig;
  setConfig: (config: Partial<SiteConfig>) => void;
}

const defaults: SiteConfig = {
  siteName: 'Router',
  subtitle: 'Your unified LLM gateway',
  logoUrl: '',
  faviconUrl: '',
};

export const useSiteConfigStore = create<SiteConfigState>()((set) => ({
  config: { ...defaults },
  setConfig: (partial) =>
    set((state) => ({ config: { ...state.config, ...partial } })),
}));
