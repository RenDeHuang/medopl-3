import assert from "node:assert/strict";
import { mkdtemp, readFile, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import test, { after } from "node:test";
import { gzipSync } from "node:zlib";
import { build } from "vite";

const entryKey = "index.html";
const loginKey = "apps/console-ui/src/pages/LoginPage.tsx";
const consoleKey = "apps/console-ui/src/pages/ConsolePage.tsx";
let buildResult;
let buildOutDir;

async function buildConsole() {
  buildResult ||= (async () => {
    const outDir = buildOutDir = await mkdtemp(join(tmpdir(), "opl-login-bundle-"));
    await build({
      configFile: new URL("../../vite.config.ts", import.meta.url).pathname,
      logLevel: "silent",
      build: { emptyOutDir: true, manifest: true, outDir }
    });
    const manifest = JSON.parse(await readFile(join(outDir, ".vite/manifest.json"), "utf8"));
    return { manifest, outDir };
  })();
  return buildResult;
}

function staticJavaScript(manifest, roots) {
  const keys = new Set();
  function visit(key) {
    if (keys.has(key)) return;
    const chunk = manifest[key];
    assert.ok(chunk, `manifest is missing ${key}`);
    keys.add(key);
    for (const dependency of chunk.imports || []) visit(dependency);
  }
  for (const root of roots) visit(root);
  return new Set([...keys].map((key) => manifest[key].file).filter((file) => file.endsWith(".js")));
}

function antDesignFiles(manifest) {
  return Object.values(manifest)
    .filter((chunk) => chunk.name === "antd" || chunk.name === "antd-pro")
    .map((chunk) => chunk.file)
    .sort();
}

after(async () => {
  if (buildOutDir) await rm(buildOutDir, { force: true, recursive: true });
});

test("login entry graph excludes Console UI frameworks", async () => {
  const { manifest } = await buildConsole();
  const entryFiles = staticJavaScript(manifest, [entryKey]);
  const loginFiles = staticJavaScript(manifest, [loginKey]);
  const loginGraph = new Set([...entryFiles, ...loginFiles]);
  const forbidden = [...loginGraph].filter((file) => /\/antd(?:-pro)?-[^/]+\.js$/.test(file));
  assert.deepEqual(forbidden, [], `login graph includes ${forbidden.join(", ")}`);
});

test("login first-screen JavaScript stays within its gzip budget", async () => {
  const { manifest, outDir } = await buildConsole();
  const loginGraph = staticJavaScript(manifest, [entryKey, loginKey]);
  let gzipBytes = 0;
  for (const file of loginGraph) gzipBytes += gzipSync(await readFile(join(outDir, file))).byteLength;
  assert.ok(gzipBytes <= 350_000, `login gzip JS is ${gzipBytes} bytes across ${[...loginGraph].join(", ")}`);
});

test("Ant Design chunks are reachable only from the Console lazy entry", async () => {
  const { manifest } = await buildConsole();
  const antFiles = antDesignFiles(manifest);
  assert.deepEqual(
    antFiles.map((file) => file.replace(/^.*\/(antd(?:-pro)?)-.*$/, "$1")),
    ["antd", "antd-pro"]
  );

  const consoleFiles = staticJavaScript(manifest, [consoleKey]);
  for (const file of antFiles) assert.ok(consoleFiles.has(file), `${file} is outside the Console boundary`);

  const publicEntries = manifest[entryKey].dynamicImports.filter((key) => key !== consoleKey);
  const publicFiles = staticJavaScript(manifest, [entryKey, ...publicEntries]);
  for (const file of antFiles) assert.ok(!publicFiles.has(file), `${file} is reachable before Console`);
});

test("the HTML entry preloads eager dependencies without preloading Console or Ant Design", async () => {
  const { outDir } = await buildConsole();
  const html = await readFile(join(outDir, "index.html"), "utf8");
  const preloads = [...html.matchAll(/<link rel="modulepreload"[^>]+href="([^"]+)"/g)].map((match) => match[1]);
  assert.ok(preloads.length > 0, "index.html has no modulepreload links");
  assert.deepEqual(preloads.filter((file) => /ConsolePage|antd/i.test(file)), []);
});

test("the built app icon is an optimized 192px PNG", async () => {
  const { outDir } = await buildConsole();
  const icon = await readFile(join(outDir, "opl-app-icon.png"));
  assert.deepEqual(icon.subarray(0, 8), Buffer.from("89504e470d0a1a0a", "hex"));
  assert.equal(icon.readUInt32BE(16), 192);
  assert.equal(icon.readUInt32BE(20), 192);
  assert.ok(icon.byteLength < 100_000, `icon is ${icon.byteLength} bytes`);
});
