import { tanstackRouter } from "@tanstack/router-plugin/vite";
import react from "@vitejs/plugin-react";
import { defineConfig } from "vite-plus";

// apiPath matches the paths the dev server sends to the backend: /api and everything under /api/.
export const apiPath = /^\/api(?=\/|$)/;

export default defineConfig({
  plugins: [
    // The router plugin writes src/routeTree.gen.ts from the files in src/routes. It must come before the React plugin.
    tanstackRouter({
      target: "react",
      autoCodeSplitting: true,
      routeFileIgnorePattern: "\\.test\\.",
    }),
    react(),
  ],
  server: {
    proxy: {
      // The browser calls the backend at /api on the frontend's own origin. The backend's routes have no /api prefix.
      // The pattern stops at a path boundary, so a frontend route such as /api-keys stays with the frontend.
      [apiPath.source]: {
        target: "http://localhost:3000",
        rewrite: (path) => path.replace(/^\/api/, ""),
      },
    },
  },
  test: {
    environment: "happy-dom",
    include: ["src/**/*.test.{ts,tsx}"],
    setupFiles: ["src/test-setup.ts"],
  },
});
