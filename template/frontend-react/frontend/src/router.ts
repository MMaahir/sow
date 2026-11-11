import { createRouter } from "@tanstack/react-router";
import type { RouterHistory } from "@tanstack/react-router";

import { routeTree } from "./routeTree.gen";

// Tests pass a memory history. The browser uses the default, which reads the address bar.
export function createAppRouter(history?: RouterHistory) {
  return createRouter({ routeTree, history });
}

declare module "@tanstack/react-router" {
  interface Register {
    router: ReturnType<typeof createAppRouter>;
  }
}
