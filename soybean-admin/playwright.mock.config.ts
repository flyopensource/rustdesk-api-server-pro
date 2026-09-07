import { defineConfig } from '@playwright/test';
import { mkdtempSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';

export default defineConfig({
  testDir: './tests/mock',
  timeout: 45_000,
  workers: 1,
  reporter: 'list',
  outputDir: mkdtempSync(join(tmpdir(), 'rustdesk-web-mock-')),
  use: { baseURL: 'http://127.0.0.1:19527', headless: true, channel: 'chrome' },
  webServer: {
    command: 'pnpm exec vite --mode test --host 127.0.0.1 --port 19527 --strictPort',
    url: 'http://127.0.0.1:19527',
    timeout: 60_000
  }
});
