import { readdirSync, readFileSync } from "node:fs";
import { join, relative, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const scriptDirectory = resolve(fileURLToPath(new URL(".", import.meta.url)));
const repositoryRoot = resolve(scriptDirectory, "..");
const websiteRoot = join(repositoryRoot, "website");
const layers = ["book", "guide", "reference"];
const ignoredDirectories = new Set([".vitepress", "node_modules"]);

function markdownFiles(layer) {
  const root = join(websiteRoot, layer);
  return readdirSync(root)
    .filter((name) => name.endsWith(".md"))
    .sort()
    .map((name) => ({ layer, path: join(root, name) }));
}

function publicMarkdownFiles(directory) {
  return readdirSync(directory, { withFileTypes: true }).flatMap((entry) => {
    if (ignoredDirectories.has(entry.name)) return [];
    const path = join(directory, entry.name);
    if (entry.isDirectory()) return publicMarkdownFiles(path);
    return entry.isFile() && entry.name.endsWith(".md") ? [path] : [];
  });
}

function lineAt(source, offset) {
  return source.slice(0, offset).split("\n").length;
}

function normalizedParagraphs(source) {
  const withoutFrontmatter = source.replace(/^---\n[\s\S]*?\n---\n/, "");
  const withoutFences = withoutFrontmatter.replace(/```[\s\S]*?```/g, "");
  return [...withoutFences.matchAll(/(?:^|\n\s*\n)([^\n#|<][\s\S]*?)(?=\n\s*\n|$)/g)]
    .map((match) => ({
      line: lineAt(source, match.index ?? 0),
      value: match[1].replace(/\s+/g, " ").trim(),
    }))
    .filter(({ value }) => value.length >= 180 && !value.startsWith("- "));
}

function codeBlocks(source) {
  return [...source.matchAll(/```([^\n]*)\n([\s\S]*?)```/g)]
    .map((match) => ({
      language: match[1].trim().split(/\s+/)[0],
      line: lineAt(source, match.index ?? 0),
      value: match[2].trim().replace(/[ \t]+$/gm, ""),
    }))
    .filter(({ language, value }) => {
      const lines = value.split("\n").filter((line) => line.trim() !== "");
      return !["text", "sh", "bash", "console"].includes(language) && lines.length >= 4 && value.length >= 80;
    });
}

const documents = layers.flatMap(markdownFiles).map((document) => ({
  ...document,
  source: readFileSync(document.path, "utf8"),
}));

const findings = [];
const publicDocuments = publicMarkdownFiles(websiteRoot)
  .sort()
  .map((path) => ({ path, source: readFileSync(path, "utf8") }));
const repositoryProseDocuments = [
  join(repositoryRoot, "README.md"),
  join(repositoryRoot, "CHANGELOG.md"),
  ...publicMarkdownFiles(join(repositoryRoot, "docs")),
]
  .sort()
  .map((path) => ({ path, source: readFileSync(path, "utf8") }));
const metadataValues = {
  title: new Map(),
  description: new Map(),
};

function registerMetadata(field, value, path) {
  const normalized = value.trim().replace(/^(?:"([\s\S]*)"|'([\s\S]*)')$/, "$1$2").toLowerCase();
  const locations = metadataValues[field].get(normalized) ?? [];
  locations.push(path);
  metadataValues[field].set(normalized, locations);
}

function checkProseHygiene(document) {
  const location = relative(repositoryRoot, document.path);
  const permitsFormerIdentity = new Set([
    "README.md",
    "CHANGELOG.md",
    "docs/migrating-from-v0.1.md",
    "website/guide/migrating-from-v0-1.md",
  ]).has(location);
  if (/\b(?:TODO|TBD|FIXME|PLACEHOLDER)\b/.test(document.source)) {
    findings.push(`${location}: contains unfinished editorial marker`);
  }
  if (/(?:^|[\s("'`])\/(?:home|Users|media)\/|[A-Za-z]:\\Users\\/.test(document.source)) {
    findings.push(`${location}: contains a machine-specific absolute path`);
  }
  if (!permitsFormerIdentity && /\b(?:OnsenTamago|Onsen Tamago|Onsentamago|Onsen-Tamago)\b/i.test(document.source)) {
    findings.push(`${location}: contains the former OnsenTamago name`);
  }
  if (/\b(?:Yunagi|YuNagi)\b/i.test(document.source)) {
    findings.push(`${location}: contains the former Yunagi name`);
  }
}

for (const document of publicDocuments) {
  const location = relative(repositoryRoot, document.path);
  const frontmatter = document.source.match(/^---\n([\s\S]*?)\n---(?:\n|$)/);
  if (!frontmatter) {
    findings.push(`${location}: missing frontmatter`);
    continue;
  }
  const title = frontmatter[1].match(/^title:\s*(\S.*)$/m)?.[1];
  const description = frontmatter[1].match(/^description:\s*(\S.*)$/m)?.[1];
  if (!title) findings.push(`${location}: missing frontmatter title`);
  else registerMetadata("title", title, document.path);
  if (!description) findings.push(`${location}: missing frontmatter description`);
  else registerMetadata("description", description, document.path);

  const homeLayout = /^layout:\s*home\s*$/m.test(frontmatter[1]);
  const h1Count = (document.source.match(/^#\s+\S.*$/gm) ?? []).length;
  if (!homeLayout && h1Count !== 1) findings.push(`${location}: expected exactly one H1, found ${h1Count}`);
  if (homeLayout && h1Count > 0) findings.push(`${location}: home layout must use its hero title instead of a Markdown H1`);

  checkProseHygiene(document);
}

for (const document of repositoryProseDocuments) checkProseHygiene(document);

for (const [field, values] of Object.entries(metadataValues)) {
  for (const locations of values.values()) {
    if (locations.length < 2) continue;
    findings.push(
      `duplicate frontmatter ${field}: ${locations.map((path) => relative(repositoryRoot, path)).join(", ")}`,
    );
  }
}

function findCrossLayerDuplicates(kind, entries) {
  const byValue = new Map();
  for (const entry of entries) {
    const group = byValue.get(entry.value) ?? [];
    group.push(entry);
    byValue.set(entry.value, group);
  }

  for (const group of byValue.values()) {
    const hasManual = group.some(({ layer }) => layer === "book");
    const hasOtherLayer = group.some(({ layer }) => layer !== "book");
    if (!hasManual || !hasOtherLayer) continue;
    const locations = group.map(({ path, line }) => `${relative(repositoryRoot, path)}:${line}`).join(", ");
    findings.push(`duplicate ${kind} across Manual and another layer: ${locations}`);
  }
}

findCrossLayerDuplicates(
  "paragraph",
  documents.flatMap((document) => normalizedParagraphs(document.source).map((entry) => ({ ...document, ...entry }))),
);
findCrossLayerDuplicates(
  "code block",
  documents.flatMap((document) => codeBlocks(document.source).map((entry) => ({ ...document, ...entry }))),
);

if (findings.length > 0) {
  process.stderr.write(`${findings.join("\n")}\n`);
  process.exit(1);
}

process.stdout.write(
  `verified metadata and path hygiene in ${publicDocuments.length} public pages; ` +
    `verified repository prose hygiene in ${repositoryProseDocuments.length} files; ` +
    `verified cross-layer content separation in ${documents.length} Manual, Guide, and Reference pages\n`,
);
