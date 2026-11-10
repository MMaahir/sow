# Project structure for the generated backend and frontend

Research started on 2026-11-10. This note is in progress. Sections below fill in as each source is read.

Method: every repository below was read through the GitHub API (`gh api repos/OWNER/REPO`, the recursive git tree, and file contents) at the commit recorded next to it. Official docs were read at their source pages. No blog post is used for a claim about how a repository is laid out.

## Backend repositories read so far

### openstatusHQ/openstatus, `apps/server`

9,258 stars, last push 2026-11-10, read at commit `36288495c8544d4498986be7d72c37ee7e666837`.

Notes pending write-up: Node, `@hono/zod-openapi`, tsdown build, `@/` path alias through `@oxc-node/core/register`, `src/index.ts` builds the app and `src/serve.ts` starts it, `src/routes/v1/<resource>/{index,get,get_all,post,put,schema}.ts` with a test next to each verb file, tests call `app.request`.
