import { defineConfig } from '@playwright/test';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
const root=path.dirname(fileURLToPath(import.meta.url));
export default defineConfig({
  testDir: './tests', fullyParallel: false, workers: 1, retries: 0,
  timeout: 300_000, expect: { timeout: 20_000 },
  outputDir: path.join(root,'../.e2e-artifacts/results'),
  reporter: [['list'], ['html',{outputFolder:path.join(root,'../.e2e-artifacts/report'),open:'never'}]],
  use: {
    browserName:'chromium', channel:process.env.E2E_BROWSER_CHANNEL || 'chrome',
    ignoreHTTPSErrors:true, viewport:{width:1440,height:900}, locale:'ja-JP',
    trace:'retain-on-failure', screenshot:'only-on-failure', video:'retain-on-failure',
    actionTimeout:20_000, navigationTimeout:30_000,
    launchOptions: {
      slowMo:Number(process.env.E2E_SLOW_MO || 0),
      args:[`--host-resolver-rules=MAP *.${process.env.E2E_DOMAIN || 't.isucon.dev'} ${process.env.E2E_TARGET_IP || '127.0.0.1'}`, '--no-proxy-server'],
    },
  },
});
