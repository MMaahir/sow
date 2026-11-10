# frontend: react prototype

Run on 2026-11-10 for ticket #59, in a throwaway project outside the repo. The project came from `sow new demo --backend ts`, and the frontend was added to it by hand. The global `vp` CLI wasn't installed on the machine, so every `vp` command ran as `pnpm exec vp` from the project's own `vite-plus`. `docs/research/backend-ts-prototype.md` covers the workspace, the pins and the pin check, and none of that changed.

## Versions proven together

| Package | Version |
|---|---|
| `vite-plus` | 1.5.1 (bundles Vite 8.3.7, Vitest 5.1.0, Oxlint 1.91.0, Oxfmt 0.76.0) |
| `react`, `react-dom` | 19.3.1 |
| `@vitejs/plugin-react` | 6.3.0 (needs Vite 8) |
| `@tanstack/react-router` | 1.170.46 |
| `@tanstack/router-plugin` | 1.169.0 |
| `@tanstack/react-query` | 5.107.1 |
| `@testing-library/react` | 16.3.3 |
| `@testing-library/dom` | 10.4.2 |
| `happy-dom` | 21.0.4 |
| `orval` | 8.47.0 |
| `typescript` | 7.0.2 |
| pnpm | 12.4.2 |
| Node | 24.21.0 |

## The two decisions

### The client generator is orval

`orval` 8.47.0 writes the client from `backend/openapi.json`. This config was proven:

```ts
import { defineConfig } from "orval";

export default defineConfig({
  backend: {
    input: "../backend/openapi.json",
    output: {
      target: "src/client.gen.ts",
      client: "react-query",
      httpClient: "fetch",
      baseUrl: "/api",
    },
  },
});
```

- It works with TypeScript 7. The other two candidates don't. `@hey-api/openapi-ts` 0.99.0 and `openapi-typescript` 7.13.0 both import the TypeScript compiler's JavaScript API, and `typescript` 7.0.2 exports only its version from its main entry. Both stop at start-up with `TypeError: Cannot read properties of undefined`, at `ts.SyntaxKind` and `ts.factory`, because they load the project's TypeScript 7. orval's core builds its output from strings. Two of its packages, `@orval/zod` and `@orval/hono`, do use the compiler API, but they declare their own `typescript` dependency, so pnpm installs TypeScript 6.0.3 for them alongside the project's TypeScript 7, and the project's type-checking stays on 7. If a later orval drops that pin and loads the project's TypeScript, it will fail the same way, and the health test in #60 will catch it.
- It suits TanStack Query. For `GET /healthz` it writes a fetch function `getHealthz`, a key function `getGetHealthzQueryKey`, `getGetHealthzQueryOptions`, and a hook `useGetHealthz`. A route that called `useGetHealthz()` rendered the backend's status in a test that replaced `fetch` with `vi.stubGlobal`. The response body is at `data.data`, because the fetch function returns `{ data, status, headers }`.
- Its default fetch client resolves every response, including an HTTP error, so TanStack Query reports a 503 as `status: "success"`. Setting `override.fetch.forceSuccessResponse: true` makes an error response reject, and the query then reports `status: "error"`. A review of this note reproduced both behaviours with orval 8.47.0. The config #60 copies must set it, and #60 adds a test that a failing response puts the query in the error state.
- It needs no Vite plugin. `pnpm exec orval` runs under Node, so the `generate-client` task is one command.
- Its output was the same on two runs, byte for byte. The prototype ran on one machine only.
- It's past 1.0. `@hey-api/openapi-ts` is still 0.x, which ADR 0007 would pin and gate.

What ticket #60 needs to know:

- orval doesn't format its output, and Oxfmt would rewrite it. The root `vite.config.ts` must leave the generated file out of `fmt` and `lint`, as it does for `routeTree.gen.ts`. With the file ignored, `vp check` still type-checks it through the route that imports it, and it passed.
- Installing orval makes pnpm 12 add `allowBuilds: esbuild: set this to true or false` to `pnpm-workspace.yaml`, because orval depends on esbuild and esbuild has an install script. orval generated the client with the script not run. The template's workspace file should set the entry to `false`.

### The frontend reaches the backend at `/api` on its own origin

The browser never calls the backend's address. Every request goes to `/api/...` on the origin that served the page, and whatever serves the page sends `/api/...` to the backend with the `/api` prefix removed. The generated client's base URL is `/api`.

- The backend's routes keep no prefix, so `/healthz` and `/readyz` stay where Kubernetes probes expect them and the paths in `openapi.json` match the backend's code.
- There's no CORS setup, and the built frontend holds no backend address. One image works in every environment.
- Under `vp dev`, the frontend's `vite.config.ts` does it with `server.proxy`:

  ```ts
  server: {
    proxy: {
      "/api": {
        target: "http://localhost:3000",
        rewrite: (path) => path.replace(/^\/api/, ""),
      },
    },
  },
  ```

  With the backend's `dev` task running on port 3000, `GET http://localhost:5199/api/healthz` on the dev server returned 200 and `{"status":"ok"}`, and `GET /api/nope` returned 404.
- The dev server's proxy matches `^/api(?=[/?]|$)` against the request URL, so `/api`, `/api/...` and `/api?...` go to the backend and a frontend route such as `/api-keys` doesn't. A plain `"/api"` key would match by prefix and send `/api-keys` to the backend. `src/api-proxy.test.ts` checks the pattern.
- In containers, nginx does the same job. The author's nginx config needs two rules, and neither was run in this prototype:
  - Requests under `/api/` go to the backend service with the prefix removed. In nginx that is `location /api/ { proxy_pass http://backend:3000/; }`, where the trailing slash on `proxy_pass` removes the prefix. `backend` is the Compose service name and 3000 is the backend's default `PORT`.
  - Every other path that isn't a file serves `index.html`, because TanStack Router reads the route from the address bar. In nginx that is `try_files $uri /index.html`.
- The Compose file publishes only the frontend, on port 8080 as the 0.3 spec says. The backend needs no published port for the browser.

## What works as ADR 0011 says

- `vp dev`, `vp build`, `vp test run` and `vp check` all work in `frontend/` with the root config from `backend: ts`. `vp build` writes `dist/index.html` and two script files.
- `vp check` type-checks `.tsx` files. A type error in a source file fails it, and so does one in a test file.
- Tests import from `vite-plus/test`. No direct `vitest` dependency is needed, so the workspace still has no Vitest pin.
- The pin check passes unchanged with the frontend in the workspace.

## Surprises

- **The route tree is a generated file that must be committed.** `@tanstack/router-plugin` writes `src/routeTree.gen.ts` from the files in `src/routes/` when `vp dev`, `vp build` or `vp test` runs. `vp check` needs the file and doesn't write it. With the file deleted, `vp check` failed with two type errors, so the template ships it. Building a fresh project leaves it unchanged. Its style isn't Oxfmt's, so the root config leaves it out of `fmt` and `lint`.
- **A test file next to a route needs `routeFileIgnorePattern`.** Without it the plugin warns that `index.test.tsx` exports no route. The pattern `\.test\.` keeps tests next to the routes they test.
- **The plugin creates `.tanstack/` in the frontend directory.** The generated `.gitignore` lists it.
- **Oxlint's React rules are off by default.** A hook called inside an `if` passed `vp check`. `vp check` reads lint settings from the root config only, so the root `vite.config.ts` has an override for `frontend/**` that turns on the `react` plugin and sets `react/rules-of-hooks` to an error. The plugin alone reports `react-hooks/exhaustive-deps` as a warning.
- **Testing Library doesn't clean up on its own here.** It unmounts after each test only when `afterEach` is a global, and the template imports test functions. A setup file calls `afterEach(cleanup)`.
- **happy-dom, not jsdom.** Under jsdom 31.0.0 the starter test passed and printed `Not implemented: Window's scrollTo() method`, which TanStack Router calls to restore scroll. happy-dom 21.0.4 prints nothing.
- **The frontend tsconfig differs from the backend's.** It uses `"moduleResolution": "bundler"`, `"jsx": "react-jsx"`, the DOM libraries and `"types": ["vite-plus/client"]`. Relative imports have no extension, as in the generated route tree. Node never runs these files, so the backend's `.ts` extension rule doesn't apply.
- **`index.html` is a template.** It's the one frontend file that needs the project name, for the page title.

## Not proven here

- Everything `docs/research/backend-ts-prototype.md` lists as needing the global `vp` in managed mode.
- The nginx rules above and the frontend image. The Dockerfile and the nginx config are the author's (#67), and so is the Compose file (#68).
- That orval's output is the same on another machine or operating system. Ticket #60's health test and the author's CI check cover it.
