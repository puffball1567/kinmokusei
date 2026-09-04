import { existsSync, readdirSync, readFileSync, statSync } from "node:fs";
import { dirname, extname, join, normalize, relative, resolve, sep } from "node:path";
import { fileURLToPath } from "node:url";

const scriptDirectory = dirname(fileURLToPath(import.meta.url));
const repositoryRoot = resolve(scriptDirectory, "..");
const websiteRoot = join(repositoryRoot, "website");
const ignoredDirectories = new Set(["node_modules", ".vitepress"]);

function markdownFiles(directory) {
  const files = [];
  for (const entry of readdirSync(directory).sort()) {
    if (ignoredDirectories.has(entry)) continue;
    const path = join(directory, entry);
    const metadata = statSync(path);
    if (metadata.isDirectory()) files.push(...markdownFiles(path));
    else if (metadata.isFile() && extname(path) === ".md") files.push(path);
  }
  return files;
}

function slugify(value) {
  return value
    .replace(/<[^>]*>/g, "")
    .replace(/!?\[([^\]]*)\]\([^)]*\)/g, "$1")
    .replace(/[`*_~]/g, "")
    .trim()
    .toLowerCase()
    .replace(/[\s]+/g, "-")
    .replace(/[!"#$%&'()*+,./:;<=>?@[\\\]^{}|]/g, "")
    .replace(/^-+|-+$/g, "");
}

function anchorsFor(path) {
  const anchors = new Set();
  const seen = new Map();
  let fence = false;
  for (const line of readFileSync(path, "utf8").split(/\r?\n/)) {
    if (/^\s*```/.test(line)) {
      fence = !fence;
      continue;
    }
    if (fence) continue;
    const heading = line.match(/^#{1,6}\s+(.+?)\s*#*\s*$/);
    if (!heading) continue;
    const base = slugify(heading[1]);
    const count = seen.get(base) || 0;
    seen.set(base, count + 1);
    anchors.add(count === 0 ? base : `${base}-${count}`);
  }
  return anchors;
}

function resolveMarkdownTarget(source, pathname) {
  const decoded = decodeURIComponent(pathname);
  const absolute = decoded.startsWith("/")
    ? join(websiteRoot, decoded.replace(/^\/+/, ""))
    : resolve(dirname(source), decoded || ".");

  const candidates = [];
  if (extname(absolute) === ".md") candidates.push(absolute);
  else if (extname(absolute)) candidates.push(absolute);
  else {
    candidates.push(`${absolute}.md`);
    candidates.push(join(absolute, "index.md"));
  }
  return candidates.find((candidate) => existsSync(candidate));
}

const failures = [];
const files = markdownFiles(websiteRoot);
const anchors = new Map(files.map((file) => [normalize(file), anchorsFor(file)]));
const incoming = new Map(files.map((file) => [normalize(file), new Set()]));

function registerIncoming(source, target) {
  const normalizedSource = normalize(source);
  const normalizedTarget = normalize(target);
  if (normalizedSource === normalizedTarget || !incoming.has(normalizedTarget)) return;
  incoming.get(normalizedTarget).add(normalizedSource);
}

for (const file of files) {
  const lines = readFileSync(file, "utf8").split(/\r?\n/);
  let fence = false;
  for (let index = 0; index < lines.length; index += 1) {
    const line = lines[index];
    if (/^\s*```/.test(line)) {
      fence = !fence;
      continue;
    }
    if (fence) continue;

    const targets = [];
    for (const match of line.matchAll(/!?\[[^\]]*\]\(([^)\s]+)(?:\s+["'][^"']*["'])?\)/g)) targets.push(match[1]);
    for (const match of line.matchAll(/\bhref=["']([^"']+)["']/g)) targets.push(match[1]);

    for (let target of targets) {
      target = target.replace(/^<|>$/g, "");
      if (/^(?:https?:|mailto:|tel:|data:)/.test(target)) continue;
      const [pathname, rawAnchor] = target.split("#", 2);
      const resolved = pathname ? resolveMarkdownTarget(file, pathname) : file;
      if (!resolved) {
        failures.push(`${relative(repositoryRoot, file)}:${index + 1}: missing target ${target}`);
        continue;
      }
      if (extname(resolved) === ".md") registerIncoming(file, resolved);
      if (rawAnchor && extname(resolved) === ".md") {
        const anchor = decodeURIComponent(rawAnchor).toLowerCase();
        if (!anchors.get(normalize(resolved))?.has(anchor)) {
          failures.push(`${relative(repositoryRoot, file)}:${index + 1}: missing anchor #${rawAnchor} in ${relative(repositoryRoot, resolved)}`);
        }
      }
    }
  }
}

const navigationPath = join(websiteRoot, ".vitepress", "config.mts");
const navigationSource = readFileSync(navigationPath, "utf8");
for (const match of navigationSource.matchAll(/\blink:\s*["']([^"']+)["']/g)) {
  const target = match[1];
  if (/^(?:https?:|mailto:|tel:|data:)/.test(target)) continue;
  const [pathname] = target.split("#", 1);
  const resolved = resolveMarkdownTarget(join(websiteRoot, "index.md"), pathname);
  if (resolved && extname(resolved) === ".md") registerIncoming(navigationPath, resolved);
}

for (const file of files) {
  if (normalize(file) === normalize(join(websiteRoot, "index.md"))) continue;
  const source = readFileSync(file, "utf8");
  const frontmatter = source.match(/^---\n([\s\S]*?)\n---/)?.[1] ?? "";
  if (/^unlisted:\s*true\s*$/m.test(frontmatter)) continue;
  if (incoming.get(normalize(file))?.size === 0) {
    failures.push(`${relative(repositoryRoot, file)}: page has no incoming documentation or navigation link`);
  }
}

if (failures.length > 0) {
  process.stderr.write(`${failures.join("\n")}\n`);
  process.exit(1);
}

process.stdout.write(`verified internal links, anchors, and discoverability in ${files.length} documentation pages\n`);
