/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly VITE_API_BASE_URL?: string;
  readonly VITE_SENTRY_DSN?: string;
  readonly VITE_SENTRY_ENVIRONMENT?: string;
  readonly VITE_CAPTCHA_PROVIDER?: string;
  readonly VITE_CAPTCHA_SITE_KEY?: string;
  readonly VITE_DEV_CAPTCHA_BYPASS_TOKEN?: string;
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}
