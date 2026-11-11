import { defineConfig, devices } from "@playwright/test";

// The tests start no server. They run against the app already running at E2E_BASE_URL, which mise loads from .env.
const baseURL = process.env["E2E_BASE_URL"];
if (!baseURL) {
  throw new Error(
    "E2E_BASE_URL isn't set: copy .env.example to .env, or set it to the address of the running app",
  );
}

export default defineConfig({
  testDir: "e2e",
  globalSetup: "./e2e/global-setup.ts",
  forbidOnly: Boolean(process.env["CI"]),
  use: {
    baseURL,
    trace: "retain-on-failure",
  },
  projects: [{ name: "chromium", use: { ...devices["Desktop Chrome"] } }],
});
