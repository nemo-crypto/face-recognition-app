import { defineConfig, devices } from '@playwright/test';
import { mkdtempSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';

const data = mkdtempSync(join(tmpdir(), 'face-studio-e2e-'));
export default defineConfig({
  testDir: './tests',
  timeout: 30000,
  workers: 1,
  fullyParallel: false,
  use: {
    baseURL: 'http://127.0.0.1:18085', trace: 'retain-on-failure',
    permissions: ['camera'],
    launchOptions: { args: ['--use-fake-device-for-media-stream', '--use-fake-ui-for-media-stream'] },
  },
  projects: [
    { name: 'desktop', use: { ...devices['Desktop Chrome'], viewport: { width: 1440, height: 1080 } } },
    { name: 'mobile', use: { ...devices['iPhone 13'], defaultBrowserType: 'chromium' } },
  ],
  webServer: {
    command: `go run ./backend/cmd/server -addr 127.0.0.1:18085 -data "${data}" -web web/dist`,
    url: 'http://127.0.0.1:18085/api/health',
    env: { FACE_API_TOKEN: '', FACE_PROVIDER_URL: '', FACE_PROVIDER_TOKEN: '' },
    reuseExistingServer: false,
    timeout: 30000,
  },
});
