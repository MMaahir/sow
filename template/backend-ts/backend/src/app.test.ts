import { expect, test } from "vite-plus/test";

import { app } from "./app.ts";

test.each(["/healthz", "/readyz"])("GET %s answers 200 with status ok", async (path) => {
  const response = await app.request(path);

  expect(response.status).toBe(200);
  expect(await response.json()).toEqual({ status: "ok" });
});

test("GET /openapi.json describes both health endpoints", async () => {
  const response = await app.request("/openapi.json");
  const document = (await response.json()) as { paths: Record<string, unknown> };

  expect(Object.keys(document.paths)).toEqual(["/healthz", "/readyz"]);
});
