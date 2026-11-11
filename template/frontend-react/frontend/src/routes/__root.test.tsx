import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { RouterProvider, createMemoryHistory } from "@tanstack/react-router";
import { render, screen } from "@testing-library/react";
import { expect, test } from "vite-plus/test";

import { createAppRouter } from "../router";

test("a route that doesn't exist shows the not-found view", async () => {
  const router = createAppRouter(createMemoryHistory({ initialEntries: ["/no-such-page"] }));

  render(
    <QueryClientProvider client={new QueryClient()}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );

  const heading = await screen.findByRole("heading", { level: 1 });
  expect(heading.textContent).toBe("Page not found");
});
