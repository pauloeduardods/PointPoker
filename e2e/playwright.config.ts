import { defineConfig, devices } from "@playwright/test";

// Starts the Go backend and the Vite dev server against the database from
// the root .env (run `mise run db:start` first). Set E2E_BASE_URL to test an
// already running stack instead (e.g. http://localhost:8000 from `mise run up`).
const apiPort = process.env.API_PORT || "8080";
const external = process.env.E2E_BASE_URL;

export default defineConfig({
  testDir: "./tests",
  timeout: 60_000,
  expect: { timeout: 10_000 },
  fullyParallel: false,
  reporter: [["list"]],
  use: {
    baseURL: external || "http://localhost:5173",
    trace: "retain-on-failure",
    ...devices["Desktop Chrome"],
  },
  webServer: external
    ? undefined
    : [
        {
          command: "go run ./cmd/server",
          cwd: "../backend",
          url: `http://localhost:${apiPort}/api/health`,
          reuseExistingServer: true,
          timeout: 120_000,
        },
        {
          command: "npm run dev -- --port 5173 --strictPort",
          cwd: "../frontend",
          url: "http://localhost:5173",
          reuseExistingServer: true,
          timeout: 60_000,
        },
      ],
});
