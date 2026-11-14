import { fileURLToPath } from "node:url"
import { defineConfig, devices } from "@playwright/test"
export default defineConfig({
  testDir: "./tests/browser",
  workers: 1,
  use: { baseURL: "http://127.0.0.1:4300" },
  projects: [
    { name: "chromium", use: { ...devices["Desktop Chrome"] } },
    { name: "webkit", use: { ...devices["iPhone 13"] } },
  ],
  webServer: [
    {
      command: "node tests/browser/api.mjs",
      url: "http://127.0.0.1:4301/healthz",
      reuseExistingServer: false,
    },
    {
      command: "node tests/browser/start.mjs",
      url: "http://127.0.0.1:4300/healthz",
      env: {
        INTERN_METRICS_CONFIG_PATH: fileURLToPath(
          new URL("./tests/browser/metrics.yaml", import.meta.url)
        ),
        HOSTNAME: "127.0.0.1",
        PORT: "4300",
        INTERN_API_BASE_URL: "http://127.0.0.1:4301",
        AUTH_PUBLIC_BASE_URL: "http://127.0.0.1:4300",
      },
      reuseExistingServer: false,
    },
  ],
})
