# Frontend and backend stacks follow a shared interface

The frontend and backend stacks will probably change while the author is learning, so changing a stack should mean adding one new stack directory, not rewriting the template. Every backend stack must provide the mise tasks `dev`, `test`, `lint` and `build`, a Dockerfile, `/healthz` and `/readyz`, and an `openapi.json`. Every frontend stack must provide the same tasks, a Dockerfile, and a TypeScript client generated from the backend's OpenAPI document. The CLI's tests check every stack against these rules.

## Consequences

- OpenAPI is the contract between frontend and backend, whatever languages they use. Hono's RPC client was rejected because only TypeScript can use it.
- The first frontend is a Vite + React single-page app rather than Next.js. A separate backend already exists, so Next.js's server features would overlap with it. Next.js can return later as a second frontend stack.
