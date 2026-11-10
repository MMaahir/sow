import { expect, test } from "vite-plus/test";

import { apiPath } from "../vite.config";

test.each(["/api", "/api/healthz", "/api/users/1"])(
  "the dev server sends %s to the backend",
  (path) => {
    expect(apiPath.test(path)).toBe(true);
  },
);

test.each(["/", "/api-keys", "/apis", "/about/api"])(
  "the dev server keeps %s with the frontend",
  (path) => {
    expect(apiPath.test(path)).toBe(false);
  },
);
