import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { RouterProvider, createMemoryHistory } from "@tanstack/react-router";
import { render, screen } from "@testing-library/react";
import { afterEach, expect, test, vi } from "vite-plus/test";

import { createAppRouter } from "../router";

afterEach(() => {
  vi.unstubAllGlobals();
});

// stubBackend replaces fetch, so every request gets this status and JSON body.
function stubBackend(status: number, body: unknown) {
  const fetch = vi.fn(async () => Response.json(body, { status }));
  vi.stubGlobal("fetch", fetch);
  return fetch;
}

function renderHome() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const router = createAppRouter(createMemoryHistory({ initialEntries: ["/"] }));
  render(
    <QueryClientProvider client={queryClient}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
  return queryClient;
}

test("the starter route shows its heading", async () => {
  stubBackend(200, { status: "ok" });

  renderHome();

  const heading = await screen.findByRole("heading", { level: 1 });
  expect(heading.textContent).toBe("The frontend is running");
});

test("the starter route shows the status from GET /api/healthz", async () => {
  // The status isn't "ok", so a page that always shows "ok" fails this test.
  const fetch = stubBackend(200, { status: "starting" });

  renderHome();

  expect(await screen.findByText("Backend status: starting")).toBeTruthy();
  expect(fetch).toHaveBeenCalledWith("/api/healthz", expect.objectContaining({ method: "GET" }));
});

test("a failing /api/healthz puts the query in the error state", async () => {
  stubBackend(503, { status: "down" });

  const queryClient = renderHome();

  expect(await screen.findByText("Backend status: unreachable")).toBeTruthy();
  expect(queryClient.getQueryState(["/api/healthz"])?.status).toBe("error");
});
