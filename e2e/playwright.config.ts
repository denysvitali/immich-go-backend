import { defineConfig, devices } from '@playwright/test';

export default defineConfig({
  testDir: './tests',
  outputDir: 'test-results',
  timeout: process.env.E2E_REMOTE ? 60_000 : 30_000,
  expect: {
    timeout: process.env.E2E_REMOTE ? 20_000 : 10_000,
  },
  forbidOnly: !!process.env.CI,
  // Remote (WAN) targets need a retry cushion; local runs stay strict.
  retries: process.env.E2E_REMOTE ? 2 : 0,
  workers: 1,
  reporter: process.env.CI ? [['github'], ['html', { open: 'never' }]] : 'list',
  use: {
    baseURL: process.env.IMMICH_WEB_URL ?? 'http://127.0.0.1:3000',
    trace: 'retain-on-failure',
    video: process.env.CI ? 'on' : 'off',
  },
  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
    },
  ],
});
