import { defineConfig } from "vitepress";

const repository = "https://github.com/puffball1567/kinmokusei";
const deploymentBase = "/kinmokusei/";
const site = "https://puffball1567.github.io/kinmokusei";

const sidebar = [
  {
    text: "Learn",
    collapsed: false,
    items: [
      { text: "Overview", link: "/learn/" },
      { text: "Installation", link: "/learn/installation" },
      { text: "Five-minute quick start", link: "/learn/quick-start" },
      { text: "Language tour", link: "/learn/language-tour" },
      { text: "Coming from TypeScript", link: "/learn/from-typescript" },
      { text: "Coming from Go", link: "/learn/from-go" },
      { text: "FAQ", link: "/learn/faq" },
    ],
  },
  {
    text: "Language manual",
    collapsed: false,
    items: [
      { text: "Manual overview", link: "/book/" },
      { text: "Source & lexical structure", link: "/book/source-and-lexical" },
      { text: "Bindings & scope", link: "/book/bindings-and-scope" },
      { text: "Expressions & evaluation", link: "/book/expressions" },
      { text: "Control flow", link: "/book/control-flow" },
      { text: "Modules & imports", link: "/book/modules-and-imports" },
      { text: "Types & values", link: "/book/types-and-values" },
      { text: "Functions & generics", link: "/book/functions-and-generics" },
      { text: "Structs, classes & interfaces", link: "/book/structs-classes-interfaces" },
      { text: "Failures, Result & exceptions", link: "/book/errors-results-exceptions" },
      { text: "Concurrency & tasks", link: "/book/concurrency-and-tasks" },
    ],
  },
  {
    text: "Language guide",
    collapsed: false,
    items: [
      { text: "Declarations & control flow", link: "/guide/language-basics" },
      { text: "Types & data", link: "/guide/types-and-data" },
      { text: "Functions & generics", link: "/guide/functions-and-generics" },
      { text: "Classes & structs", link: "/guide/classes-and-structs" },
      { text: "Errors & nullability", link: "/guide/errors-and-nullability" },
      { text: "Concurrency", link: "/guide/concurrency" },
      { text: "Modules & projects", link: "/guide/projects-and-cli" },
    ],
  },
  {
    text: "Application guide",
    collapsed: false,
    items: [
      { text: "Go interoperability", link: "/guide/go-interop" },
      { text: "HTTP applications", link: "/guide/http" },
      { text: "Testing applications", link: "/guide/testing" },
      { text: "C ABI & FFI", link: "/guide/c-ffi" },
      { text: "Editor & diagnostics", link: "/guide/editor" },
      { text: "Troubleshooting", link: "/guide/troubleshooting" },
      { text: "Generated Go", link: "/guide/generated-go" },
    ],
  },
  {
    text: "Examples",
    collapsed: false,
    items: [
      { text: "Example gallery", link: "/examples/" },
      {
        text: "Recipes",
        collapsed: true,
        items: [
          { text: "Parse input with Result", link: "/examples/result-parsing" },
          { text: "Collections", link: "/examples/collections" },
          { text: "Numeric & bitwise operators", link: "/examples/numeric-operators" },
          { text: "Comparisons & short-circuiting", link: "/examples/comparisons-and-short-circuit" },
          { text: "UTF-8 strings", link: "/examples/unicode-strings" },
          { text: "Generics", link: "/examples/generics" },
          { text: "Variadics & spread", link: "/examples/variadics" },
          { text: "Defined domain types", link: "/examples/defined-types" },
          { text: "Interface polymorphism", link: "/examples/polymorphism" },
          { text: "Go interface type switch", link: "/examples/type-switch" },
          { text: "Inheritance & downcasts", link: "/examples/inheritance" },
          { text: "Typed exceptions", link: "/examples/exceptions" },
          { text: "Nullable flow", link: "/examples/nullable-flow" },
          { text: "Control-flow boundaries", link: "/examples/control-flow-boundaries" },
          { text: "Struct receivers", link: "/examples/struct-receivers" },
          { text: "Tasks", link: "/examples/tasks" },
          { text: "Channels", link: "/examples/channels" },
          { text: "Select", link: "/examples/select" },
          { text: "Relative modules", link: "/examples/modules" },
          { text: "Command-line application", link: "/examples/command-line-app" },
          { text: "JSON", link: "/examples/json" },
          { text: "Filesystem round trip", link: "/examples/filesystem-round-trip" },
          { text: "Go standard-library values", link: "/examples/go-standard-library" },
          { text: "HTTP router testing", link: "/examples/http-router" },
          { text: "Bounded HTTP fetch", link: "/examples/bounded-http-fetch" },
          { text: "External Go module", link: "/examples/external-go-module" },
          { text: "C ABI export", link: "/examples/c-abi" },
          { text: "Incoming C FFI", link: "/examples/incoming-c-ffi" },
        ],
      },
      { text: "React + Go backends", link: "/examples/web-backend" },
    ],
  },
  {
    text: "Reference",
    collapsed: false,
    items: [
      { text: "Reference overview", link: "/reference/" },
      { text: "Glossary", link: "/reference/glossary" },
      { text: "CLI", link: "/reference/cli" },
      { text: "Language syntax", link: "/reference/language" },
      { text: "Type system", link: "/reference/types" },
      { text: "Operators", link: "/reference/operators" },
      { text: "Built-ins", link: "/reference/built-ins" },
      { text: "Standard modules", link: "/reference/standard-library" },
      { text: "C FFI manifest", link: "/reference/c-ffi-manifest" },
      { text: "Go interop matrix", link: "/reference/go-interop" },
      { text: "Project files", link: "/reference/project-files" },
      { text: "Diagnostics", link: "/reference/diagnostics" },
      { text: "Compatibility", link: "/reference/compatibility" },
      { text: "Implementation status", link: "/reference/status" },
    ],
  },
  {
    text: "Project",
    collapsed: false,
    items: [
      { text: "Quality promise", link: "/project/quality" },
      { text: "Contributing to docs", link: "/project/contributing" },
      { text: "Releases & compatibility", link: "/project/releases" },
      { text: "Migrate from v0.1", link: "/guide/migrating-from-v0-1" },
    ],
  },
];

function canonicalPath(relativePath: string): string {
  if (relativePath === "index.md") return "/";
  const route = relativePath.replace(/(?:index)?\.md$/, "").replace(/\/$/, "");
  return `/${route}`;
}

export default defineConfig({
  lang: "en-US",
  title: "Kinmokusei",
  titleTemplate: ":title · Kinmokusei",
  description: "The official guide to Kinmokusei, a TypeScript-inspired source language for the Go ecosystem.",
  base: deploymentBase,
  cleanUrls: true,
  lastUpdated: true,
  sitemap: { hostname: `${site}/` },
  markdown: { lineNumbers: true },
  head: [
    ["link", { rel: "icon", type: "image/svg+xml", href: `${deploymentBase}favicon.svg` }],
    ["meta", { name: "theme-color", content: "#f1f5f5" }],
    ["meta", { name: "color-scheme", content: "light dark" }],
    ["meta", { property: "og:type", content: "website" }],
    ["meta", { property: "og:site_name", content: "Kinmokusei Documentation" }],
    ["meta", { property: "og:image", content: `${site}/social-card.svg` }],
    ["meta", { property: "og:image:alt", content: "Kinmokusei — clear source, readable Go" }],
    ["meta", { name: "twitter:card", content: "summary_large_image" }],
  ],
  transformHead({ pageData }) {
    const canonical = `${site}${canonicalPath(pageData.relativePath)}`;
    const title = pageData.title ? `${pageData.title} · Kinmokusei` : "Kinmokusei";
    const description = pageData.description || "TypeScript-inspired source, predictable Go behavior, readable Go output.";
    return [
      ["link", { rel: "canonical", href: canonical }],
      ["meta", { property: "og:title", content: title }],
      ["meta", { property: "og:description", content: description }],
      ["meta", { property: "og:url", content: canonical }],
    ];
  },
  themeConfig: {
    logo: { src: "/logo.svg", alt: "Kinmokusei" },
    siteTitle: "Kinmokusei",
    nav: [
      { text: "Learn", link: "/learn/" },
      { text: "Manual", link: "/book/" },
      { text: "Guide", link: "/guide/language-basics" },
      { text: "Reference", link: "/reference/" },
      { text: "Examples", link: "/examples/" },
      {
        text: "v0.2 preview",
        items: [
          { text: "Current release", link: `${repository}/releases/latest` },
          { text: "All releases", link: `${repository}/releases` },
          { text: "Implementation status", link: "/reference/status" },
        ],
      },
    ],
    sidebar,
    outline: { level: [2, 3], label: "On this page" },
    returnToTopLabel: "Return to top",
    sidebarMenuLabel: "Documentation menu",
    darkModeSwitchLabel: "Appearance",
    lightModeSwitchTitle: "Use light theme",
    darkModeSwitchTitle: "Use dark theme",
    socialLinks: [{ icon: "github", link: repository }],
    search: {
      provider: "local",
      options: { detailedView: true },
    },
    editLink: {
      pattern: `${repository}/edit/devel/website/:path`,
      text: "Edit this page on GitHub",
    },
    lastUpdated: { text: "Last updated", formatOptions: { dateStyle: "medium" } },
    docFooter: { prev: "Previous", next: "Next" },
    footer: {
      message: "Kinmokusei is a pre-1.0 project. Documentation describes implemented behavior.",
      copyright: "Released under Apache-2.0 · Kinmokusei contributors",
    },
  },
});
