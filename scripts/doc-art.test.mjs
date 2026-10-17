import assert from "node:assert/strict";
import { readFileSync, existsSync } from "node:fs";
import { test } from "node:test";

const root = new URL("../website/", import.meta.url);
const source = (path) => readFileSync(new URL(path, root), "utf8");

function pngHeader(path) {
  const bytes = readFileSync(new URL(path, root));
  assert.deepEqual(bytes.subarray(0, 8), Buffer.from([137, 80, 78, 71, 13, 10, 26, 10]));
  assert.equal(bytes.toString("ascii", 12, 16), "IHDR");
  return { width: bytes.readUInt32BE(16), height: bytes.readUInt32BE(20), colorType: bytes[25] };
}

test("home illustration uses the transparent clockwork osmanthus asset", () => {
  const home = source("index.md");
  assert.match(home, /src: \/hero-kinmokusei\.png/);
  assert.match(home, /alt: A clockwork osmanthus branch/);
  assert.doesNotMatch(home, /sunset|calm water/);
  assert.equal(existsSync(new URL("public/hero-kinmokusei.svg", root)), false);
  const header = pngHeader("public/hero-kinmokusei.png");
  assert.equal(header.width, header.height);
  assert.ok(header.width >= 640);
  assert.equal(header.colorType, 6, "hero must retain RGBA transparency");
});

test("small SVG marks describe the mechanical flower and retain four petals", () => {
  for (const name of ["logo.svg", "favicon.svg"]) {
    const svg = source(`public/${name}`);
    assert.match(svg, /aria-labelledby="title desc"/);
    assert.match(svg, /<title id="title">Kinmokusei<\/title>/);
    assert.match(svg, /<desc id="desc">[^<]*osmanthus/);
    assert.equal([...svg.matchAll(/<use href="#petal"/g)].length, 4);
    assert.doesNotMatch(svg, /setting sun|horizon|url\(#sky\)/);
  }
});

test("sharing metadata points to a 1200 by 630 PNG instead of an SVG", () => {
  const config = source(".vitepress/config.mts");
  assert.match(config, /property: "og:image", content: `\$\{site\}\/social-card\.png`/);
  assert.match(config, /name: "twitter:image", content: `\$\{site\}\/social-card\.png`/);
  assert.match(config, /property: "og:image:width", content: "1200"/);
  assert.match(config, /property: "og:image:height", content: "630"/);
  const header = pngHeader("public/social-card.png");
  assert.equal(header.width, 1200);
  assert.equal(header.height, 630);
  assert.match(source("public/social-card.svg"), /href="hero-kinmokusei\.png"/);
});
