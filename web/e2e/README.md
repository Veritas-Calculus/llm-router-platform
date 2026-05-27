# E2E tests

Minimal Playwright suite covering the audit C-01 path end-to-end:
captcha-gated registration -> email verification -> $5 welcome credit.

## One-time setup

```bash
cd web
npm install
npm run test:e2e:install   # downloads the Chromium driver (~150 MB)
```

`test:e2e:install` is separate from `npm install` on purpose — the browser
download is heavy and isn't needed for the regular build/lint/test loop.

## Pre-req: docker-compose stack is running

The spec does **not** spawn its own server. Start the local stack first:

```bash
docker compose up -d
```

The web container listens on `http://localhost`. To target a different host
or port, set `E2E_BASE_URL`.

## Run

```bash
cd web
npm run test:e2e           # headless, line + html reporter
npm run test:e2e:ui        # Playwright UI mode for debugging
```

The HTML report lands at `web/playwright-report/index.html`. Traces are
retained on failure under `web/test-results/`.

## What it covers

The single `captcha-verify-credit.spec.ts` exercises the C-01 audit
remediation:

- captcha=dev (configured via GraphQL admin login in `beforeAll`)
- random throwaway email (`e2e-<timestamp>-<rand>@example.com`)
- assertion: register -> dashboard -> banner visible -> balance $0
- pulls the verification URL from `docker logs llm-router-server`
  (backend logs the link when `EMAIL_ENABLED=false`)
- assertion: verify -> success copy -> balance $5 -> banner gone

## Notes

- The docker-logs read for the verification link has a small race; we poll
  for up to 10 s with `--since 60s` to be tolerant.
- Test users are left in the DB (each run uses a fresh email). If you want
  a clean slate, truncate `users` between sweeps; the spec deliberately
  doesn't to keep "what happened?" debuggable.
- The spec is **not** wired into `npm run build` / `npm run lint`; it's a
  separate runner so it never blocks unrelated work.
