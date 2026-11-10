// Checks that the Vite+ pins in pnpm-workspace.yaml match the Vite+ that is
// installed. vite-plus, its vite alias and any direct Vitest dependency only
// work as a set, and Renovate bumps only vite-plus, so CI runs this check.
// mise run upgrade-vite-plus re-pins the rest.
import { execFileSync } from "node:child_process";
import { globSync, readFileSync } from "node:fs";
import { join } from "node:path";
import { fileURLToPath } from "node:url";

import { parse } from "yaml";

type Versions = Record<string, string>;

type Workspace = {
  packages?: string[];
  catalog?: Versions;
  overrides?: Versions;
};

type Manifest = {
  dependencies?: Versions;
  devDependencies?: Versions;
  optionalDependencies?: Versions;
};

type Toolchain = {
  nodes: { id: string; version?: string }[];
};

const root = fileURLToPath(new URL("..", import.meta.url));
const workspace = parse(readFileSync(join(root, "pnpm-workspace.yaml"), "utf8")) as Workspace;
const catalog = workspace.catalog ?? {};
const overrides = workspace.overrides ?? {};

const toolchain = JSON.parse(
  execFileSync("vp", ["toolchain", "--json"], { cwd: root, encoding: "utf8" }),
) as Toolchain;

function bundled(id: string): string {
  const version = toolchain.nodes.find((node) => node.id === id)?.version;
  if (version === undefined) {
    throw new Error(`vp toolchain --json reports no ${id} version`);
  }
  return version;
}

const vitePlus = bundled("vite-plus");
const core = bundled("vite-plus-core");
const vitest = bundled("vitest");

// resolve returns the version a dependency or override asks for. It looks the
// version up in the default catalog when the spec says "catalog:".
function resolve(name: string, spec: string): string {
  if (spec !== "catalog:" && spec !== "catalog:default") {
    return spec;
  }
  return catalog[name] ?? `${spec} with no ${name} catalog entry`;
}

const catalogVitePlus = catalog["vite-plus"] ?? "nothing";
const catalogVite = catalog.vite ?? "nothing";
const problems: string[] = [];

if (catalogVitePlus !== vitePlus) {
  problems.push(
    `vite-plus: the catalog pins ${catalogVitePlus}, but vp toolchain reports ${vitePlus}`,
  );
}
if (
  catalogVite !== `npm:@voidzero-dev/vite-plus-core@${catalogVitePlus}` ||
  catalogVite !== `npm:@voidzero-dev/vite-plus-core@${core}`
) {
  problems.push(
    `vite: the catalog pins ${catalogVite}, but vite-plus is ${catalogVitePlus} in the catalog and @voidzero-dev/vite-plus-core is ${core} in vp toolchain`,
  );
}

const manifests = globSync(
  (workspace.packages ?? [])
    .filter((pattern) => !pattern.startsWith("!"))
    .map((pattern) => `${pattern}/package.json`),
  {
    cwd: root,
    exclude: (workspace.packages ?? [])
      .filter((pattern) => pattern.startsWith("!"))
      .map((pattern) => `${pattern.slice(1)}/package.json`),
  },
);
// vitestUser is the first package.json that depends on Vitest directly.
let vitestUser: string | undefined;
for (const file of ["package.json", ...manifests]) {
  const manifest = JSON.parse(readFileSync(join(root, file), "utf8")) as Manifest;
  const dependencies = {
    ...manifest.dependencies,
    ...manifest.devDependencies,
    ...manifest.optionalDependencies,
  };
  for (const [name, spec] of Object.entries(dependencies)) {
    if (name !== "vitest" && !name.startsWith("@vitest/")) {
      continue;
    }
    vitestUser ??= file;
    const version = resolve(name, spec);
    if (version !== vitest) {
      problems.push(`${name}: ${file} pins ${version}, but vp toolchain reports Vitest ${vitest}`);
    }
  }
}

// The vitest@* override also pins the Vitest that vite-plus depends on, so a
// stale one fails the check even when no package depends on Vitest directly.
const vitestOverride = overrides["vitest@*"];
if (vitestOverride !== undefined && resolve("vitest", vitestOverride) !== vitest) {
  problems.push(
    `vitest: the vitest@* override pins ${resolve("vitest", vitestOverride)}, but vp toolchain reports Vitest ${vitest}`,
  );
}
if (vitestOverride === undefined && vitestUser !== undefined) {
  problems.push(
    `vitest: ${vitestUser} depends on Vitest directly, but no vitest@* override pins Vitest ${vitest}`,
  );
}

if (problems.length > 0) {
  console.error(problems.join("\n"));
  console.error("Run mise run upgrade-vite-plus to re-pin them, then commit the changes.");
  process.exit(1);
}
console.log(`The Vite+ pins match vp toolchain: vite-plus ${vitePlus}, Vitest ${vitest}.`);
