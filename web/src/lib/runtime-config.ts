/**
 * Runtime configuration accessor.
 *
 * Reads values written to `window.__RUNTIME_CONFIG__` by the web container
 * entrypoint (envsubst over /runtime-config.template.js → /runtime-config.js,
 * loaded from index.html before the SPA bundle). Falls back to the
 * Vite-baked `import.meta.env.VITE_*` so `npm run dev` keeps working
 * without nginx.
 *
 * The template uses literal `${VAR}` placeholders. When envsubst runs, a
 * missing env var leaves the literal string in place ("${VITE_SENTRY_DSN}"
 * etc.), so we normalize that to an empty value here — otherwise we'd
 * accidentally treat a placeholder as a valid DSN. See audit H-08.
 */

interface RuntimeConfig {
  SENTRY_DSN?: string;
  SENTRY_ENVIRONMENT?: string;
  CAPTCHA_PROVIDER?: string;
  CAPTCHA_SITE_KEY?: string;
  DEV_CAPTCHA_BYPASS_TOKEN?: string;
}

declare global {
  interface Window {
    __RUNTIME_CONFIG__?: RuntimeConfig;
  }
}

// A value of "${VAR}" means envsubst saw an empty/unset variable and left
// the literal placeholder. Treat as missing.
function clean(value: string | undefined | null): string {
  if (typeof value !== 'string') return '';
  const trimmed = value.trim();
  if (!trimmed) return '';
  if (trimmed.startsWith('${') && trimmed.endsWith('}')) return '';
  return trimmed;
}

/**
 * Get a runtime config value with Vite env fallback.
 *
 * Lookup order:
 *   1. window.__RUNTIME_CONFIG__[key] (runtime injection — production path)
 *   2. import.meta.env[viteKey]       (build-time inlining — dev path)
 *
 * The dev path is necessary because `npm run dev` serves index.html through
 * Vite, which doesn't see the nginx-generated /runtime-config.js.
 */
export function getRuntimeConfig(key: keyof RuntimeConfig, viteKey?: string): string {
  const fromWindow = clean(window.__RUNTIME_CONFIG__?.[key]);
  if (fromWindow) return fromWindow;
  const envKey = viteKey ?? `VITE_${key}`;
  const env = (import.meta.env as Record<string, string | undefined>)[envKey];
  return clean(env);
}
