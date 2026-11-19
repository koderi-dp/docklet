import { defineConfig, devices } from "@playwright/test";

const port = process.env.DOCKLET_E2E_PORT ?? "9260";

// The tests share one Docklet instance and one engine, so they run one after another.
export default defineConfig({
  testDir: "./tests",
  globalSetup: "./support/globalSetup.ts",
  fullyParallel: false,
  workers: 1,
  retries: 0,
  timeout: 60_000,
  expect: { timeout: 15_000 },
  reporter: [["list"], ["html", { open: "never" }]],
  use: {
    baseURL: `http://localhost:${port}`,
    viewport: { width: 1440, height: 900 },
    trace: "retain-on-failure",
    screenshot: "only-on-failure",
  },
  projects: [
    // Adds the test engine as a connection; every other test needs it.
    { name: "connect", testMatch: /connect\.setup\.ts/, use: { ...devices["Desktop Chrome"], viewport: { width: 1440, height: 900 } } },
    {
      name: "smoke",
      testMatch: /.*\.spec\.ts/,
      dependencies: ["connect"],
      use: { ...devices["Desktop Chrome"], viewport: { width: 1440, height: 900 } },
    },
  ],
});
