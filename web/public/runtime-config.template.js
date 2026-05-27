/*
 * Runtime configuration injected at container start.
 *
 * The web image bakes a single static bundle at build time. Anything that
 * must vary between dev / staging / prod environments (Sentry DSN, captcha
 * site keys, ...) is read from this file via `window.__RUNTIME_CONFIG__`
 * so the same image can be promoted across environments without rebuild.
 *
 * The Dockerfile entrypoint runs `envsubst` on this template and writes
 * the result to /usr/share/nginx/html/runtime-config.js — see audit H-08.
 *
 * The `${VAR}` placeholders are replaced literally at container start.
 * Vite's import.meta.env.VITE_* remains the fallback for `npm run dev`,
 * which never goes through this template.
 */
window.__RUNTIME_CONFIG__ = {
  SENTRY_DSN: "${VITE_SENTRY_DSN}",
  SENTRY_ENVIRONMENT: "${VITE_SENTRY_ENVIRONMENT}",
  CAPTCHA_SITE_KEY: "${VITE_CAPTCHA_SITE_KEY}",
  DEV_CAPTCHA_BYPASS_TOKEN: "${VITE_DEV_CAPTCHA_BYPASS_TOKEN}",
};
