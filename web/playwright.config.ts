import { defineConfig, devices } from '@playwright/test';

// Playwright config for the e2e suite covering audit finding C-01:
// captcha-gated registration -> email verification -> $5 welcome credit.
//
// We run a single Chromium project against the docker-compose stack
// (`docker compose up -d`). There is intentionally NO `webServer` block —
// the test assumes the user is operating the stack themselves and points
// at the nginx-fronted web app on http://localhost (override with E2E_BASE_URL).
//
// Tests mutate DB state (registration, balance, system_settings) so we
// pin workers to 1 to keep ordering predictable.

export default defineConfig({
  testDir: './e2e',
  outputDir: './test-results',

  fullyParallel: false,
  workers: 1,
  forbidOnly: !!process.env.CI,
  retries: 0,
  timeout: 30_000,
  expect: { timeout: 10_000 },

  reporter: [
    ['line'],
    ['html', { open: 'never', outputFolder: 'playwright-report' }],
  ],

  use: {
    baseURL: process.env.E2E_BASE_URL ?? 'http://localhost',
    headless: true,
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
    video: 'off',
    actionTimeout: 10_000,
    navigationTimeout: 15_000,
  },

  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
    },
  ],
});
