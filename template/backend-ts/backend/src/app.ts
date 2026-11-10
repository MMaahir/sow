import { OpenAPIHono, createRoute, z } from "@hono/zod-openapi";

const StatusSchema = z.object({ status: z.literal("ok") }).openapi("Status");

const statusResponse = {
  200: {
    description: "The service is up.",
    content: { "application/json": { schema: StatusSchema } },
  },
};

const healthz = createRoute({
  method: "get",
  path: "/healthz",
  summary: "Liveness: the process is running.",
  responses: statusResponse,
});

const readyz = createRoute({
  method: "get",
  path: "/readyz",
  summary: "Readiness: the service can take requests.",
  responses: statusResponse,
});

export const openApiInfo = {
  openapi: "3.1.0",
  info: { title: "backend", version: "0.0.0" },
};

export const app = new OpenAPIHono()
  .openapi(healthz, (c) => c.json({ status: "ok" as const }, 200))
  .openapi(readyz, (c) => c.json({ status: "ok" as const }, 200));

app.doc31("/openapi.json", openApiInfo);
