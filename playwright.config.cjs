const { defineConfig } = require('@playwright/test');
module.exports = defineConfig({
  testDir: './tests/browser',
  timeout: 25000,
  workers: 1,
  retries: 0,
  reporter: 'list',
  use: {
    baseURL: 'http://127.0.0.1:37964',
    viewport: { width: 1440, height: 1000 },
    headless: true,
    trace: 'off',
    screenshot: 'off', // Never persist invitation contents or session tokens.
    launchOptions: process.env.PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH ? { executablePath: process.env.PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH } : {},
  },
  webServer: [37964, 37965].map(port => ({
    command: `node scripts/test-server.mjs ${port}`,
    url: `http://127.0.0.1:${port}/`,
    reuseExistingServer: false,
    timeout: 20000,
    gracefulShutdown: { signal: 'SIGTERM', timeout: 5000 },
  })),
});
