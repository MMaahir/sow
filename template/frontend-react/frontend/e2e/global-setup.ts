import type { FullConfig } from "@playwright/test";

// globalSetup stops the run with the address when nothing answers there, before any browser starts.
export default async function globalSetup(config: FullConfig) {
  const baseURL = config.projects[0]?.use.baseURL;
  try {
    await fetch(baseURL ?? "", { signal: AbortSignal.timeout(10_000) });
  } catch (error) {
    throw new Error(
      `nothing answers at ${baseURL}: start the app with docker compose up --wait, or set E2E_BASE_URL in .env to where it runs`,
      { cause: error },
    );
  }
}
