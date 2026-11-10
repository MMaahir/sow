import { writeFile } from "node:fs/promises";

import { app, openApiInfo } from "../src/app.ts";

const document = app.getOpenAPI31Document(openApiInfo);
await writeFile(
  new URL("../openapi.json", import.meta.url),
  `${JSON.stringify(document, null, 2)}\n`,
);
