/**
 * E2E coverage for audit finding C-01:
 *   captcha-gated registration  ->  email verification  ->  $5 welcome credit.
 *
 * Pre-req: the docker-compose stack is up (`docker compose up -d`). The spec
 * does NOT spawn its own server — see playwright.config.ts.
 *
 * The test runs against http://localhost (or E2E_BASE_URL) and:
 *   1. logs in as admin via GraphQL and forces captcha=dev / registrationMode=open
 *   2. registers a fresh user in the browser; asserts the verify banner shows
 *      and the user balance is 0
 *   3. greps `docker logs llm-router-server` for the verification URL the
 *      backend logs when EMAIL_ENABLED=false (auth.resolvers.go:176-179)
 *   4. visits the verification URL and asserts the success copy
 *   5. asserts balance is now 5 and the banner is gone
 *
 * The test creates a throwaway user (random email) each run — no cleanup is
 * needed because the next run uses a different email and the DB happily grows.
 * If you care about hygiene, truncate users / transactions in dev between
 * sweeps; the spec does NOT do this on purpose to keep the failure mode
 * (e.g. "balance never settled") inspectable.
 */

import { execFileSync } from 'node:child_process';
import { randomBytes } from 'node:crypto';
import { expect, test } from '@playwright/test';
import type { APIRequestContext } from '@playwright/test';

// ── Helpers ─────────────────────────────────────────────────────────────

const ADMIN_EMAIL = 'admin@example.com';
const ADMIN_PASSWORD = 'Dlocaladmin12345';
const NEW_USER_PASSWORD = 'Stronger99!x';

interface GqlResponse<T> {
  data?: T;
  errors?: Array<{ message: string }>;
}

async function gql<T = unknown>(
  ctx: APIRequestContext,
  query: string,
  variables: Record<string, unknown> = {},
  headers: Record<string, string> = {},
): Promise<T> {
  const res = await ctx.post('/graphql', {
    headers: { 'Content-Type': 'application/json', ...headers },
    data: { query, variables },
  });
  expect(res.ok(), `GraphQL HTTP ${res.status()} for query ${query.slice(0, 60)}...`).toBeTruthy();
  const body = (await res.json()) as GqlResponse<T>;
  if (body.errors?.length) {
    throw new Error(`GraphQL errors: ${JSON.stringify(body.errors)}`);
  }
  if (body.data === undefined || body.data === null) {
    throw new Error('GraphQL response had no data field');
  }
  return body.data;
}

/**
 * Read the most recent verification URL the server logged for this email.
 * Backend writes `{"msg":"email verification link (SMTP disabled)","url":"...verify-email?token=..."}`
 * (auth.resolvers.go:176-179) when EMAIL_ENABLED=false, which is the
 * docker-compose default.
 *
 * Retries for a few seconds because the email send is non-blocking
 * (goroutine off the register resolver).
 *
 * We use execFileSync (not execSync) so we can't accidentally inject shell
 * metacharacters — every argument is a literal known at compile time.
 */
async function pollForVerificationToken(): Promise<string> {
  const deadline = Date.now() + 10_000;
  let lastErr: unknown = null;
  while (Date.now() < deadline) {
    try {
      const logs = execFileSync(
        'docker',
        ['logs', 'llm-router-server', '--since', '60s'],
        { encoding: 'utf8', maxBuffer: 16 * 1024 * 1024, stdio: ['ignore', 'pipe', 'pipe'] },
      );
      // Most recent match wins — same user can have only one outstanding
      // token (CreateVerificationToken invalidates older ones).
      const matches = [...logs.matchAll(/verify-email\?token=([a-f0-9]+)/g)];
      if (matches.length > 0) {
        return matches[matches.length - 1][1];
      }
    } catch (err) {
      lastErr = err;
    }
    await new Promise((r) => setTimeout(r, 500));
  }
  throw new Error(
    `Could not find verification token in docker logs within 10s (last error: ${lastErr})`,
  );
}

// ── Suite ───────────────────────────────────────────────────────────────

test.describe('audit C-01 — registration captcha + email verification + $5 welcome credit', () => {
  // Run serially within file (workers=1 in config already enforces it).
  test.describe.configure({ mode: 'serial' });

  // The throwaway test user is built once per file run.
  const email = `e2e-${Date.now()}-${randomBytes(3).toString('hex')}@example.com`;

  let adminCtx: APIRequestContext;

  test.beforeAll(async ({ playwright }) => {
    // Pre-flight: clear the per-IP welcome-credit throttle in Redis.
    // helpers_auth.go::checkWelcomeCreditEligibility increments `reg_credit:<ip>`
    // on every register; >3 in 24h denies the credit. Sequential test runs from
    // the same dev IP burn through that budget fast, so we always start clean.
    // Best-effort: ignore failures because the throttle is also disabled when
    // Redis is unavailable.
    try {
      execFileSync(
        'docker',
        [
          'exec',
          'llm-router-redis',
          'sh',
          '-c',
          'redis-cli -a "$REDIS_PASSWORD" --no-auth-warning EVAL "for _,k in ipairs(redis.call(\'KEYS\', ARGV[1])) do redis.call(\'DEL\', k) end" 0 "reg_credit:*"',
        ],
        { encoding: 'utf8', stdio: ['ignore', 'pipe', 'pipe'] },
      );
    } catch {
      // intentional — best-effort.
    }

    // Pre-flight: log in as admin and force captcha=dev / registrationMode=open.
    // The admin JWT is exchanged for an access cookie which we reuse for the
    // settings mutations. baseURL is needed so request.post('/graphql') works.
    adminCtx = await playwright.request.newContext({
      baseURL: process.env.E2E_BASE_URL ?? 'http://localhost',
    });
    const loginData = await gql<{ login: { token: string; user: { role: string } } }>(
      adminCtx,
      `mutation L($input: LoginInput!) {
        login(input: $input) { token user { role } }
      }`,
      { input: { email: ADMIN_EMAIL, password: ADMIN_PASSWORD } },
    );
    expect(loginData.login.user.role).toBe('admin');

    // The login mutation sets HttpOnly cookies on adminCtx automatically, so
    // subsequent @auth(role: ADMIN) mutations succeed without extra wiring.
    await gql(
      adminCtx,
      `mutation U($input: SystemSettingsInput!) {
        updateSystemSettings(input: $input) { captcha }
      }`,
      { input: { category: 'captcha', data: JSON.stringify({ provider: 'dev' }) } },
    );
    await gql(
      adminCtx,
      `mutation U($input: SystemSettingsInput!) {
        updateSystemSettings(input: $input) { security }
      }`,
      {
        input: {
          category: 'security',
          data: JSON.stringify({
            registrationMode: 'open',
            cookieSecureMode: 'auto',
          }),
        },
      },
    );
  });

  test.afterAll(async () => {
    // Restore the dev-friendly defaults explicitly. The pre-flight already
    // sets them, but if someone changes the desired-state in the future this
    // is the guard rail. We tolerate failure because the admin context may
    // already be disposed when an earlier step blew up.
    try {
      await gql(
        adminCtx,
        `mutation U($input: SystemSettingsInput!) {
          updateSystemSettings(input: $input) { captcha }
        }`,
        { input: { category: 'captcha', data: JSON.stringify({ provider: 'dev' }) } },
      );
      await gql(
        adminCtx,
        `mutation U($input: SystemSettingsInput!) {
          updateSystemSettings(input: $input) { security }
        }`,
        {
          input: {
            category: 'security',
            data: JSON.stringify({
              registrationMode: 'open',
              cookieSecureMode: 'auto',
            }),
          },
        },
      );
    } catch {
      // intentional — afterAll cleanup is best-effort.
    }
    await adminCtx.dispose();
  });

  test('signs up, sees banner & $0 balance, verifies email, sees $5 credit & no banner', async ({
    page,
    playwright,
  }) => {
    // ── 1. Browser registration ────────────────────────────────────────
    await page.goto('/login');

    // The login/register pair is a role=tablist with the second tab labelled
    // "Sign Up" (auth.register). Click to switch.
    await page.getByRole('tab', { name: /sign up/i }).click();

    await page.getByLabel(/name/i).fill('E2E Tester');
    await page.getByLabel(/email/i).fill(email);
    // Two password fields — disambiguate by label.
    await page.getByLabel(/^password$/i).fill(NEW_USER_PASSWORD);
    await page.getByLabel(/confirm password/i).fill(NEW_USER_PASSWORD);

    // Dev captcha is a passive stub: literal "Captcha (dev mode — token = dev-ok)"
    // is rendered, captchaToken is set automatically on mount.
    await expect(page.getByText(/dev mode/i)).toBeVisible();

    await page.getByRole('button', { name: /create account/i }).click();

    // After register success the page navigates to /dashboard.
    await page.waitForURL('**/dashboard', { timeout: 15_000 });

    // ── 2. Verify banner is visible, balance is 0 ──────────────────────
    const banner = page.getByRole('status').filter({ hasText: /welcome credit|verify your email/i });
    await expect(banner).toBeVisible({ timeout: 10_000 });

    // Build a per-user request context that inherits the browser's auth
    // cookies so { me { balance } } resolves under the same identity.
    const userCtx = await playwright.request.newContext({
      baseURL: process.env.E2E_BASE_URL ?? 'http://localhost',
      storageState: await page.context().storageState(),
    });
    const before = await gql<{ me: { balance: string | number; emailVerified: boolean } }>(
      userCtx,
      `query { me { balance emailVerified } }`,
    );
    expect(before.me.emailVerified).toBe(false);
    // Balance is the Money scalar — serialized as a fixed-scale decimal string
    // ("0.00000000"). Compare numerically to be format-tolerant.
    expect(parseFloat(String(before.me.balance))).toBe(0);

    // ── 3. Pull verification token from server logs ────────────────────
    const token = await pollForVerificationToken();
    expect(token, 'verification token in server logs').toMatch(/^[a-f0-9]+$/);

    // ── 4. Verify the email ────────────────────────────────────────────
    await page.goto(`/verify-email?token=${token}`);
    // VerifyEmailPage renders "Email Verified!" as the success heading. The
    // body copy also mentions the welcome credit, but we anchor on the heading
    // (role=heading) so this stays a single-element match in strict mode.
    await expect(
      page.getByRole('heading', { name: /email verified/i }),
    ).toBeVisible({ timeout: 10_000 });

    // ── 5. Balance is now $5; banner is gone ───────────────────────────
    const after = await gql<{ me: { balance: string | number; emailVerified: boolean } }>(
      userCtx,
      `query { me { balance emailVerified } }`,
    );
    expect(after.me.emailVerified).toBe(true);
    expect(parseFloat(String(after.me.balance))).toBe(5);

    await page.goto('/dashboard');
    // The banner reads the verification status from the Apollo cache — the
    // bootstrap me query above already wrote the verified=true value, so the
    // banner should be hidden as soon as the dashboard renders. We assert
    // "hidden" rather than "doesn't exist" because the component returns null.
    await expect(
      page.getByRole('status').filter({ hasText: /welcome credit|verify your email/i }),
    ).toHaveCount(0);

    await userCtx.dispose();
  });
});
