import { defineConfig } from "orval";

export default defineConfig({
  backend: {
    input: "../backend/openapi.json",
    output: {
      target: "src/client.gen.ts",
      client: "react-query",
      httpClient: "fetch",
      // The browser calls the backend at /api on the frontend's own origin.
      baseUrl: "/api",
      override: {
        // A response that isn't 2xx throws, so TanStack Query puts the query in the error state.
        fetch: { forceSuccessResponse: true },
      },
    },
  },
});
