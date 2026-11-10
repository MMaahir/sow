import { defineConfig } from "vite-plus";

export default defineConfig({
  lint: {
    ignorePatterns: ["**/dist/**", "**/client.gen.ts", "**/routeTree.gen.ts"],
    options: {
      typeAware: true,
      typeCheck: true,
    },
    overrides: [
      {
        files: ["frontend/**"],
        plugins: ["react"],
        rules: { "react/rules-of-hooks": "error" },
      },
    ],
  },
  fmt: {
    ignorePatterns: [
      "**/dist/**",
      "**/openapi.json",
      "**/client.gen.ts",
      "**/routeTree.gen.ts",
      "pnpm-lock.yaml",
    ],
  },
});
